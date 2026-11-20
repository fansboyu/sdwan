package agent

import (
	"sort"
	"strings"
	"time"
)

const (
	pathQualityHealthyWindow = 45 * time.Second
	localFallbackWindow      = 30 * time.Second
	localRecoveryWindow      = 30 * time.Second
	localRelayMinDwell       = 60 * time.Second
)

type PathQualityState struct {
	PeerDeviceID      string     `json:"peer_device_id"`
	PeerPublicKey     string     `json:"peer_public_key"`
	PathType          string     `json:"path_type"`
	CurrentPath       bool       `json:"current_path"`
	Endpoint          string     `json:"endpoint"`
	LatestHandshakeAt *time.Time `json:"latest_handshake_at,omitempty"`
	RxBytes           int64      `json:"rx_bytes"`
	TxBytes           int64      `json:"tx_bytes"`
	LastRxAt          *time.Time `json:"last_rx_at,omitempty"`
	FailCount         int32      `json:"fail_count"`
	LastOKAt          *time.Time `json:"last_ok_at,omitempty"`
	LastFailedAt      *time.Time `json:"last_failed_at,omitempty"`
	HealthySince      *time.Time `json:"healthy_since,omitempty"`
	SwitchedAt        *time.Time `json:"switched_at,omitempty"`
	LocalReason       string     `json:"local_reason,omitempty"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func RefreshPathQualitiesFromStats(existing []PathQualityState, stats []PeerStat) []PathQualityState {
	now := time.Now().UTC()
	byKey := peerStatsByKey(stats)
	result := make([]PathQualityState, 0, len(existing))
	for _, item := range existing {
		result = append(result, refreshPathQuality(item, byKey[item.PeerPublicKey], now))
	}
	return result
}

func UpdatePathQualitiesFromNetmap(netmap Netmap, stats []PeerStat, existing []PathQualityState) []PathQualityState {
	now := time.Now().UTC()
	byKey := peerStatsByKey(stats)
	previous := map[string]PathQualityState{}
	for _, item := range existing {
		previous[pathQualityKey(item.PeerDeviceID, item.PathType)] = item
	}
	result := make([]PathQualityState, 0, len(netmap.Peers))
	for _, peer := range netmap.Peers {
		pathType := strings.TrimSpace(peer.PathRole)
		if pathType == "" {
			pathType = "direct"
		}
		if pathType != "direct" && pathType != "relay" {
			continue
		}
		item := previous[pathQualityKey(peer.DeviceID, pathType)]
		item.PeerDeviceID = peer.DeviceID
		item.PeerPublicKey = strings.TrimSpace(peer.PublicKey)
		item.PathType = pathType
		item.CurrentPath = peer.PathActive
		item.Endpoint = firstString(peer.Endpoints)
		result = append(result, refreshPathQuality(item, byKey[item.PeerPublicKey], now))
	}
	return result
}

func PathQualityReports(items []PathQualityState) []PathQualityReport {
	reports := make([]PathQualityReport, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.PeerDeviceID) == "" || strings.TrimSpace(item.PathType) == "" {
			continue
		}
		reports = append(reports, PathQualityReport{
			PeerDeviceID:      item.PeerDeviceID,
			PathType:          item.PathType,
			CurrentPath:       item.CurrentPath,
			Endpoint:          item.Endpoint,
			LatestHandshakeAt: item.LatestHandshakeAt,
			RxBytes:           item.RxBytes,
			TxBytes:           item.TxBytes,
			LastRxAt:          item.LastRxAt,
			FailCount:         item.FailCount,
			LastOKAt:          item.LastOKAt,
			LastFailedAt:      item.LastFailedAt,
		})
	}
	return reports
}

func ApplyLocalPathFallback(netmap Netmap, existing []PathQualityState) (Netmap, []PathQualityState, bool) {
	mode := strings.TrimSpace(netmap.RelayPolicy)
	if mode == "" {
		mode = strings.TrimSpace(netmap.PathMode)
	}
	qualities := clonePathQualities(existing)
	if mode != "auto" {
		return netmap, qualities, false
	}
	netmap.Peers = append([]NetmapPeer(nil), netmap.Peers...)

	relayIndex := -1
	for i, peer := range netmap.Peers {
		if peer.PathRole == "relay" && strings.TrimSpace(peer.PublicKey) != "" && len(peer.Endpoints) > 0 {
			relayIndex = i
			break
		}
	}
	if relayIndex < 0 {
		return netmap, qualities, false
	}

	now := time.Now().UTC()
	relay := netmap.Peers[relayIndex]
	relayAllowed := []string{}
	changed := false
	for i := range netmap.Peers {
		if i == relayIndex || netmap.Peers[i].PathRole == "relay" {
			continue
		}
		if netmap.Peers[i].PathRole != "" && netmap.Peers[i].PathRole != "direct" {
			continue
		}
		directAllowed := businessAllowedIPs(netmap.Peers[i], relay, netmap.Self.SiteRole == "main_site")
		if len(directAllowed) == 0 {
			continue
		}
		directQuality := findQuality(qualities, netmap.Peers[i].DeviceID, "direct")
		relayQuality := findQuality(qualities, relay.DeviceID, "relay")
		useRelay := shouldUseRelay(directQuality, relayQuality, now)
		if useRelay {
			if netmap.Peers[i].PathActive || len(netmap.Peers[i].AllowedIPs) > 0 {
				changed = true
			}
			netmap.Peers[i].PathRole = "direct"
			netmap.Peers[i].PathActive = false
			netmap.Peers[i].AllowedIPs = nil
			relayAllowed = append(relayAllowed, directAllowed...)
			qualities = markCurrentPath(qualities, netmap.Peers[i].DeviceID, "direct", false, "local_direct_unhealthy", now)
			qualities = markCurrentPath(qualities, relay.DeviceID, "relay", true, "local_relay_fallback", now)
		} else {
			if !netmap.Peers[i].PathActive || !sameStringSet(netmap.Peers[i].AllowedIPs, directAllowed) {
				changed = true
			}
			netmap.Peers[i].PathRole = "direct"
			netmap.Peers[i].PathActive = true
			netmap.Peers[i].AllowedIPs = dedupeSorted(directAllowed)
			qualities = markCurrentPath(qualities, netmap.Peers[i].DeviceID, "direct", true, "local_direct_selected", now)
		}
	}

	relayAllowed = dedupeSorted(relayAllowed)
	if len(relayAllowed) > 0 {
		if !netmap.Peers[relayIndex].PathActive || !sameStringSet(netmap.Peers[relayIndex].AllowedIPs, relayAllowed) {
			changed = true
		}
		netmap.Peers[relayIndex].PathActive = true
		netmap.Peers[relayIndex].AllowedIPs = relayAllowed
	} else {
		if netmap.Peers[relayIndex].PathActive || len(netmap.Peers[relayIndex].AllowedIPs) > 0 {
			changed = true
		}
		netmap.Peers[relayIndex].PathActive = false
		netmap.Peers[relayIndex].AllowedIPs = nil
		qualities = markCurrentPath(qualities, relay.DeviceID, "relay", false, "local_direct_selected", now)
	}
	return netmap, qualities, changed
}

func refreshPathQuality(item PathQualityState, stat PeerStat, now time.Time) PathQualityState {
	wasHealthy := pathQualityHealthy(item, now, pathQualityHealthyWindow)
	if stat.PublicKey != "" {
		if stat.RxBytes > item.RxBytes {
			t := now
			item.LastRxAt = &t
		}
		item.LatestHandshakeAt = stat.LatestHandshake
		item.RxBytes = stat.RxBytes
		item.TxBytes = stat.TxBytes
	}
	healthy := pathQualityHealthy(item, now, pathQualityHealthyWindow)
	if healthy {
		item.FailCount = 0
		t := now
		item.LastOKAt = &t
		if !wasHealthy || item.HealthySince == nil {
			item.HealthySince = &t
		}
	} else {
		item.FailCount++
		t := now
		item.LastFailedAt = &t
		item.HealthySince = nil
	}
	item.UpdatedAt = now
	return item
}

func shouldUseRelay(direct, relay *PathQualityState, now time.Time) bool {
	if direct == nil {
		return false
	}
	directHealthy := pathQualityHealthy(*direct, now, localFallbackWindow)
	if direct.CurrentPath {
		if relay == nil || !pathQualityHealthy(*relay, now, pathQualityHealthyWindow) {
			return false
		}
		return !directHealthy && direct.FailCount >= 2
	}
	if relay != nil && relay.CurrentPath {
		if relay.SwitchedAt != nil && now.Sub(*relay.SwitchedAt) < localRelayMinDwell {
			return true
		}
		if direct.HealthySince != nil && now.Sub(*direct.HealthySince) >= localRecoveryWindow {
			return false
		}
		return true
	}
	if direct.HealthySince != nil && now.Sub(*direct.HealthySince) >= localRecoveryWindow {
		return false
	}
	return relay != nil && pathQualityHealthy(*relay, now, pathQualityHealthyWindow)
}

func pathQualityHealthy(item PathQualityState, now time.Time, window time.Duration) bool {
	if item.LatestHandshakeAt != nil && now.Sub(*item.LatestHandshakeAt) <= window {
		return true
	}
	return item.LastRxAt != nil && now.Sub(*item.LastRxAt) <= window
}

func businessAllowedIPs(direct NetmapPeer, relay NetmapPeer, localIsMainSite bool) []string {
	if len(direct.AllowedIPs) > 0 {
		return append([]string(nil), direct.AllowedIPs...)
	}
	if localIsMainSite && direct.VirtualIP != "" {
		return []string{hostOnly(direct.VirtualIP) + "/32"}
	}
	if !direct.PathActive && len(relay.AllowedIPs) > 0 {
		return append([]string(nil), relay.AllowedIPs...)
	}
	if direct.VirtualIP != "" {
		return []string{hostOnly(direct.VirtualIP) + "/32"}
	}
	return append([]string(nil), relay.AllowedIPs...)
}

func findQuality(items []PathQualityState, peerID, pathType string) *PathQualityState {
	for i := range items {
		if items[i].PeerDeviceID == peerID && items[i].PathType == pathType {
			return &items[i]
		}
	}
	return nil
}

func markCurrentPath(items []PathQualityState, peerID, pathType string, current bool, reason string, now time.Time) []PathQualityState {
	for i := range items {
		if items[i].PeerDeviceID != peerID || items[i].PathType != pathType {
			continue
		}
		if items[i].CurrentPath != current {
			t := now
			items[i].SwitchedAt = &t
		}
		items[i].CurrentPath = current
		items[i].LocalReason = reason
		items[i].UpdatedAt = now
		return items
	}
	t := now
	return append(items, PathQualityState{
		PeerDeviceID: peerID,
		PathType:     pathType,
		CurrentPath:  current,
		SwitchedAt:   &t,
		LocalReason:  reason,
		UpdatedAt:    now,
	})
}

func clonePathQualities(items []PathQualityState) []PathQualityState {
	out := make([]PathQualityState, len(items))
	copy(out, items)
	return out
}

func sameStringSet(left, right []string) bool {
	left = dedupeSorted(left)
	right = dedupeSorted(right)
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func dedupeSorted(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func peerStatsByKey(stats []PeerStat) map[string]PeerStat {
	result := map[string]PeerStat{}
	for _, stat := range stats {
		if strings.TrimSpace(stat.PublicKey) != "" {
			result[stat.PublicKey] = stat
		}
	}
	return result
}

func pathQualityKey(peerID, pathType string) string {
	return peerID + "|" + pathType
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
