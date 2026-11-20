package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"englishlisten/sdwan/internal/storage/sqlc"
	"englishlisten/sdwan/internal/version"
	"github.com/jackc/pgx/v5"
)

const (
	pathModeDirect       = "direct"
	pathModeAuto         = "auto"
	pathModeRelay        = "relay"
	minPathClientVersion = "v1.2.0"

	pathFailureWindow  = 30 * time.Second
	pathRecoveryWindow = 30 * time.Second
	pathRelayMinDwell  = 60 * time.Second
	relayHeartbeatTTL  = 15 * time.Second
	relayHandshakeTTL  = 120 * time.Second
)

type PeerStatReport struct {
	PublicKey       string     `json:"public_key"`
	LatestHandshake *time.Time `json:"latest_handshake_at,omitempty"`
	RxBytes         int64      `json:"rx_bytes"`
	TxBytes         int64      `json:"tx_bytes"`
}

type PathQualityReport struct {
	PeerDeviceID      string     `json:"peer_device_id"`
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
}

type AppliedPathReport struct {
	ClientDeviceID     string `json:"client_device_id"`
	Generation         int64  `json:"generation"`
	EffectivePath      string `json:"effective_path"`
	VerificationStatus string `json:"verification_status"`
	VerificationError  string `json:"verification_error,omitempty"`
}

type PathAssignment struct {
	ClientDeviceID     string     `json:"client_device_id"`
	PeerVirtualIP      string     `json:"peer_virtual_ip"`
	DesiredPath        string     `json:"desired_path"`
	State              string     `json:"state"`
	Generation         int64      `json:"generation"`
	VerifiedPath       string     `json:"verified_path"`
	VerifiedGeneration int64      `json:"verified_generation"`
	VerificationState  string     `json:"verification_state"`
	VerificationError  string     `json:"verification_error"`
	VerifiedAt         *time.Time `json:"verified_at,omitempty"`
}

func (s *Service) SetPathMode(ctx context.Context, user sqlc.User, mode string) error {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != pathModeDirect && mode != pathModeAuto && mode != pathModeRelay {
		return errors.New("path_mode must be direct, auto, or relay")
	}
	if mode != pathModeDirect {
		if !capabilitiesForPlan(user.PlanCode).EnableSelfRelay {
			return ErrUpgradeRequired
		}
		relay, err := s.store.Queries.GetActiveRelayByUser(ctx, user.ID)
		if err != nil {
			return errors.New("an active relay is required")
		}
		if !relayIsHealthy(relay) {
			return fmt.Errorf("active relay is not data-plane healthy: %s", relayHealthStatus(relay))
		}
		devices, err := s.store.Queries.ListDevicesByUser(ctx, user.ID)
		if err != nil {
			return err
		}
		for _, device := range devices {
			if device.Status == "active" && version.Compare(device.ClientVersion, minPathClientVersion) < 0 {
				return fmt.Errorf("device %s must upgrade to %s", device.Hostname, minPathClientVersion)
			}
		}
	}
	if err := s.store.Queries.UpdateUserPathMode(ctx, user.ID, mode); err != nil {
		return err
	}
	if err := s.ensurePeerPaths(ctx, user.ID, mode); err != nil {
		return err
	}
	return s.store.Queries.BumpNetmapVersion(ctx, user.ID)
}

func (s *Service) recordPathReports(ctx context.Context, device sqlc.Device, stats []PeerStatReport, applied []AppliedPathReport) error {
	for _, stat := range stats {
		if strings.TrimSpace(stat.PublicKey) == "" {
			continue
		}
		if err := s.store.Queries.UpsertDevicePeerStat(ctx, device.ID, strings.TrimSpace(stat.PublicKey), stat.LatestHandshake, stat.RxBytes, stat.TxBytes); err != nil {
			return err
		}
	}
	for _, item := range applied {
		if item.ClientDeviceID == "" || item.Generation <= 0 {
			continue
		}
		if item.EffectivePath != "" && item.EffectivePath != pathModeDirect && item.EffectivePath != pathModeRelay {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(item.VerificationStatus))
		if status != "verified" && status != "failed" {
			status = "pending"
		}
		if err := s.store.Queries.UpsertAppliedPath(ctx, device.ID, item.ClientDeviceID, item.Generation, item.EffectivePath, status, item.VerificationError); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) recordPathQualityReports(ctx context.Context, device sqlc.Device, reports []PathQualityReport) error {
	for _, report := range reports {
		peerID := strings.TrimSpace(report.PeerDeviceID)
		pathType := strings.ToLower(strings.TrimSpace(report.PathType))
		if peerID == "" || (pathType != pathModeDirect && pathType != pathModeRelay) {
			continue
		}
		if err := s.store.Queries.UpsertDevicePathQuality(ctx, sqlc.UpsertDevicePathQualityParams{
			DeviceID:          device.ID,
			PeerDeviceID:      peerID,
			PathType:          pathType,
			CurrentPath:       report.CurrentPath,
			Endpoint:          strings.TrimSpace(report.Endpoint),
			LatestHandshakeAt: report.LatestHandshakeAt,
			RxBytes:           report.RxBytes,
			TxBytes:           report.TxBytes,
			LastRxAt:          report.LastRxAt,
			FailCount:         report.FailCount,
			LastOKAt:          report.LastOKAt,
			LastFailedAt:      report.LastFailedAt,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensurePeerPaths(ctx context.Context, userID, mode string) error {
	devices, err := s.store.Queries.ListDevicesByUser(ctx, userID)
	if err != nil {
		return err
	}
	var mainSite *sqlc.Device
	clients := make([]sqlc.Device, 0)
	for i := range devices {
		if devices[i].Status != "active" {
			continue
		}
		if devices[i].SiteRole == siteRoleMain {
			copy := devices[i]
			mainSite = &copy
		} else {
			clients = append(clients, devices[i])
		}
	}
	if mainSite == nil {
		return nil
	}
	desired := pathModeDirect
	if mode == pathModeRelay {
		desired = pathModeRelay
	}
	ids := make([]string, 0, len(clients))
	for _, client := range clients {
		ids = append(ids, client.ID)
		if _, err := s.store.Queries.EnsurePeerPath(ctx, userID, client.ID, mainSite.ID, desired); err != nil {
			return err
		}
	}
	return s.store.Queries.DeletePeerPathsNotIn(ctx, userID, mainSite.ID, ids)
}

func (s *Service) reconcilePeerPaths(ctx context.Context, user sqlc.User) (bool, error) {
	mode := defaultString(user.PathMode, pathModeDirect)
	if err := s.ensurePeerPaths(ctx, user.ID, mode); err != nil {
		return false, err
	}
	paths, err := s.store.Queries.ListPeerPathsByUser(ctx, user.ID)
	if err != nil {
		return false, err
	}
	devices, err := s.store.Queries.ListDevicesByUser(ctx, user.ID)
	if err != nil {
		return false, err
	}
	byID := map[string]sqlc.Device{}
	for _, d := range devices {
		byID[d.ID] = d
	}
	relay, relayErr := s.store.Queries.GetActiveRelayByUser(ctx, user.ID)
	relayHealthy := relayErr == nil && relay.LastSeenAt != nil && time.Since(*relay.LastSeenAt) <= relayHeartbeatTTL
	changed := false
	now := time.Now()
	for _, path := range paths {
		client, cok := byID[path.ClientDeviceID]
		mainSite, mok := byID[path.MainSiteDeviceID]
		if !cok || !mok {
			continue
		}
		directHealthy := s.pairHandshakeHealthy(ctx, client.ID, mainSite.PublicKey, mainSite.ID, client.PublicKey, pathFailureWindow)
		if path.DesiredPath != path.CurrentPath {
			clientStatus, cErr := s.store.Queries.GetAppliedPathStatus(ctx, client.ID, client.ID)
			mainStatus, mErr := s.store.Queries.GetAppliedPathStatus(ctx, mainSite.ID, client.ID)
			state, message, verified := resolvePathVerification(path.DesiredPath, path.Generation, clientStatus, cErr, mainStatus, mErr)
			if err := s.store.Queries.UpdatePeerPathVerification(ctx, user.ID, client.ID, state, path.DesiredPath, message, path.Generation, verified); err != nil {
				return false, err
			}
			if verified {
				if err := s.store.Queries.CompletePeerPath(ctx, user.ID, client.ID); err != nil {
					return false, err
				}
				changed = true
			}
			continue
		}
		clientStatus, cErr := s.store.Queries.GetAppliedPathStatus(ctx, client.ID, client.ID)
		mainStatus, mErr := s.store.Queries.GetAppliedPathStatus(ctx, mainSite.ID, client.ID)
		state, message, verified := resolvePathVerification(path.CurrentPath, path.Generation, clientStatus, cErr, mainStatus, mErr)
		if err := s.store.Queries.UpdatePeerPathVerification(ctx, user.ID, client.ID, state, path.CurrentPath, message, path.Generation, verified); err != nil {
			return false, err
		}
		switch mode {
		case pathModeDirect:
			if path.DesiredPath != pathModeDirect {
				if _, err := s.store.Queries.SetPeerPathDesired(ctx, user.ID, client.ID, pathModeDirect, "preparing_direct"); err != nil {
					return false, err
				}
				changed = true
			}
		case pathModeRelay:
			if relayHealthy && path.DesiredPath != pathModeRelay {
				if _, err := s.store.Queries.SetPeerPathDesired(ctx, user.ID, client.ID, pathModeRelay, "preparing_relay"); err != nil {
					return false, err
				}
				changed = true
			}
		case pathModeAuto:
			if path.CurrentPath == pathModeDirect {
				if directHealthy {
					if path.State != pathModeDirect {
						if err := s.store.Queries.SetPeerPathState(ctx, user.ID, client.ID, pathModeDirect); err != nil {
							return false, err
						}
					}
				} else if path.State != "direct_suspect" {
					if err := s.store.Queries.SetPeerPathState(ctx, user.ID, client.ID, "direct_suspect"); err != nil {
						return false, err
					}
				} else if now.Sub(path.UpdatedAt) >= pathFailureWindow && relayHealthy {
					if _, err := s.store.Queries.SetPeerPathDesired(ctx, user.ID, client.ID, pathModeRelay, "preparing_relay"); err != nil {
						return false, err
					}
					changed = true
				}
			} else if mode == pathModeAuto && !relayHealthy {
				if _, err := s.store.Queries.SetPeerPathDesired(ctx, user.ID, client.ID, pathModeDirect, "preparing_direct"); err != nil {
					return false, err
				}
				changed = true
			} else if now.Sub(path.SwitchedAt) >= pathRelayMinDwell {
				if directHealthy {
					if path.State != "direct_recovering" {
						if err := s.store.Queries.SetPeerPathState(ctx, user.ID, client.ID, "direct_recovering"); err != nil {
							return false, err
						}
					} else if now.Sub(path.UpdatedAt) >= pathRecoveryWindow {
						if _, err := s.store.Queries.SetPeerPathDesired(ctx, user.ID, client.ID, pathModeDirect, "preparing_direct"); err != nil {
							return false, err
						}
						changed = true
					}
				} else if path.State == "direct_recovering" {
					if err := s.store.Queries.SetPeerPathState(ctx, user.ID, client.ID, pathModeRelay); err != nil {
						return false, err
					}
				}
			}
		}
	}
	return changed, nil
}

func resolvePathVerification(desired string, generation int64, client sqlc.AppliedPathStatus, clientErr error, main sqlc.AppliedPathStatus, mainErr error) (string, string, bool) {
	if clientErr != nil || mainErr != nil || client.Generation < generation || main.Generation < generation {
		return "verifying", "等待客户端和主站点应用并验证当前路径", false
	}
	if client.VerificationStatus == "failed" || main.VerificationStatus == "failed" {
		message := strings.TrimSpace(client.VerificationError)
		if message == "" {
			message = strings.TrimSpace(main.VerificationError)
		}
		if message == "" {
			message = "Overlay 连通性探测失败"
		}
		return "failed", message, false
	}
	if client.EffectivePath != desired || main.EffectivePath != desired {
		return "path_mismatch", fmt.Sprintf("期望 %s；客户端实际 %s，主站点实际 %s", desired, client.EffectivePath, main.EffectivePath), false
	}
	if client.VerificationStatus != "verified" || main.VerificationStatus != "verified" {
		return "verifying", "等待两端 Overlay 连通性探测成功", false
	}
	return "verified", "客户端与主站点均已验证", true
}

func (s *Service) pairHandshakeHealthy(ctx context.Context, leftID, rightKey, rightID, leftKey string, window time.Duration) bool {
	left, lerr := s.store.Queries.GetDevicePeerStat(ctx, leftID, rightKey)
	right, rerr := s.store.Queries.GetDevicePeerStat(ctx, rightID, leftKey)
	if lerr != nil || rerr != nil {
		return false
	}
	cutoff := time.Now().Add(-window)
	return left.LatestHandshake != nil && right.LatestHandshake != nil && left.LatestHandshake.After(cutoff) && right.LatestHandshake.After(cutoff)
}

func relayIsHealthy(relay sqlc.Relay) bool {
	return relayControlHealthy(relay) && relay.HandshakePeerCount > 0 && relay.LastHandshakeAt != nil && time.Since(*relay.LastHandshakeAt) <= relayHandshakeTTL
}

func relayControlHealthy(relay sqlc.Relay) bool {
	now := time.Now()
	return relay.LastSeenAt != nil && now.Sub(*relay.LastSeenAt) <= relayHeartbeatTTL &&
		relay.InterfaceUp && relay.SyncOK && relay.LastError == "" && relay.LastSyncAt != nil && now.Sub(*relay.LastSyncAt) <= relayHeartbeatTTL
}

func relayHealthStatus(relay sqlc.Relay) string {
	if relay.LastSeenAt == nil || time.Since(*relay.LastSeenAt) > relayHeartbeatTTL {
		return "offline"
	}
	if !relay.InterfaceUp || !relay.SyncOK || relay.LastError != "" || relay.LastSyncAt == nil || time.Since(*relay.LastSyncAt) > relayHeartbeatTTL {
		return "degraded"
	}
	if relay.HandshakePeerCount == 0 || relay.LastHandshakeAt == nil || time.Since(*relay.LastHandshakeAt) > relayHandshakeTTL {
		return "awaiting_data_plane"
	}
	return "healthy"
}

func (s *Service) pathAwareNetmap(ctx context.Context, user sqlc.User, self sqlc.Device, devices []sqlc.Device, endpoints map[string][]sqlc.DeviceEndpoint, routesByDevice map[string][]string, _ []sqlc.SubnetRoute) (Netmap, bool, error) {
	mode := defaultString(user.PathMode, pathModeDirect)
	if mode == pathModeDirect && version.Compare(self.ClientVersion, minPathClientVersion) < 0 {
		return Netmap{}, false, nil
	}
	paths, err := s.store.Queries.ListPeerPathsByUser(ctx, user.ID)
	if err != nil {
		return Netmap{}, false, err
	}
	if len(paths) == 0 {
		if err := s.ensurePeerPaths(ctx, user.ID, mode); err != nil {
			return Netmap{}, false, err
		}
		paths, err = s.store.Queries.ListPeerPathsByUser(ctx, user.ID)
		if err != nil {
			return Netmap{}, false, err
		}
	}
	pathByClient := map[string]sqlc.PeerPath{}
	for _, path := range paths {
		pathByClient[path.ClientDeviceID] = path
	}
	byID := map[string]sqlc.Device{}
	for _, d := range devices {
		byID[d.ID] = d
	}
	var relay *sqlc.Relay
	if item, err := s.store.Queries.GetActiveRelayByUser(ctx, user.ID); err == nil {
		relay = &item
	}
	peers := make([]NetmapPeer, 0)
	assignments := make([]PathAssignment, 0)
	var generation int64
	if self.SiteRole == siteRoleMain {
		relayAllowed := []string{}
		for _, path := range paths {
			client, ok := byID[path.ClientDeviceID]
			if !ok || client.Status != "active" {
				continue
			}
			relayUsable := relay != nil && relayIsHealthy(*relay)
			active := directPathActive(mode, path.DesiredPath, relayUsable)
			allowed := []string{}
			if active {
				allowed = []string{client.VirtualIP + "/32"}
			}
			peers = append(peers, NetmapPeer{DeviceID: client.ID, Hostname: client.Hostname, VirtualIP: client.VirtualIP, PublicKey: client.PublicKey, AllowedIPs: allowed, Endpoints: orderedEndpointAddresses(endpoints[client.ID]), PersistentKeepalive: probeKeepalive(active), PathRole: pathModeDirect, PathActive: active})
			if !active {
				relayAllowed = append(relayAllowed, client.VirtualIP+"/32")
			}
			assignments = append(assignments, pathAssignment(path, client.VirtualIP))
			if path.Generation > generation {
				generation = path.Generation
			}
		}
		if relay != nil {
			peers = append(peers, relayNetmapPeer(*relay, dedupeStrings(relayAllowed), len(relayAllowed) > 0))
		}
	} else {
		path, ok := pathByClient[self.ID]
		if !ok {
			return Netmap{}, false, nil
		}
		mainSite, ok := byID[path.MainSiteDeviceID]
		if !ok {
			return Netmap{}, false, nil
		}
		relayUsable := relay != nil && relayIsHealthy(*relay)
		directActive := directPathActive(mode, path.DesiredPath, relayUsable)
		directAllowed := []string{}
		businessAllowed := []string{mainSite.VirtualIP + "/32"}
		businessAllowed = append(businessAllowed, routesByDevice[mainSite.ID]...)
		if directActive {
			directAllowed = businessAllowed
		}
		peers = append(peers, NetmapPeer{DeviceID: mainSite.ID, Hostname: mainSite.Hostname, VirtualIP: mainSite.VirtualIP, PublicKey: mainSite.PublicKey, AllowedIPs: directAllowed, Endpoints: orderedEndpointAddresses(endpoints[mainSite.ID]), PersistentKeepalive: probeKeepalive(directActive), PathRole: pathModeDirect, PathActive: directActive})
		if relay != nil {
			relayAllowed := []string{}
			if !directActive {
				relayAllowed = businessAllowed
			}
			peers = append(peers, relayNetmapPeer(*relay, relayAllowed, !directActive))
		}
		assignments = []PathAssignment{pathAssignment(path, mainSite.VirtualIP)}
		generation = path.Generation
	}
	relays := []interface{}{}
	candidates := []RelayCandidate{}
	if relay != nil {
		relays = []interface{}{*relay}
		candidates = []RelayCandidate{relayCandidate(*relay)}
	}
	return Netmap{Version: user.NetmapVersion, OverlayCIDR: user.OverlayCidr, Self: NetmapSelf{DeviceID: self.ID, Hostname: self.Hostname, VirtualIP: self.VirtualIP, PublicKey: self.PublicKey, SiteRole: defaultString(self.SiteRole, siteRoleClient)}, Peers: peers, Relays: relays, PathMode: mode, RelayPolicy: mode, RelayCandidates: candidates, PathGeneration: generation, PathAssignments: assignments}, true, nil
}

func pathAssignment(path sqlc.PeerPath, peerVirtualIP string) PathAssignment {
	return PathAssignment{
		ClientDeviceID: path.ClientDeviceID, PeerVirtualIP: peerVirtualIP,
		DesiredPath: path.DesiredPath, State: path.State, Generation: path.Generation,
		VerifiedPath: path.VerifiedPath, VerifiedGeneration: path.VerifiedGeneration,
		VerificationState: path.VerificationState, VerificationError: path.VerificationError,
		VerifiedAt: path.VerifiedAt,
	}
}

func relayNetmapPeer(relay sqlc.Relay, allowed []string, active bool) NetmapPeer {
	return NetmapPeer{DeviceID: relay.ID, Hostname: relay.Name, VirtualIP: relay.VirtualIP, PublicKey: relay.PublicKey, AllowedIPs: dedupeStrings(allowed), Endpoints: []string{relay.Endpoint}, PersistentKeepalive: probeKeepalive(active), PathRole: pathModeRelay, PathActive: active}
}

func directPathActive(mode, desiredPath string, relayUsable bool) bool {
	switch mode {
	case pathModeDirect:
		return true
	case pathModeAuto:
		return desiredPath == pathModeDirect || !relayUsable
	default:
		return false
	}
}

func relayCandidate(relay sqlc.Relay) RelayCandidate {
	return RelayCandidate{
		RelayID:   relay.ID,
		Name:      relay.Name,
		PublicKey: relay.PublicKey,
		VirtualIP: relay.VirtualIP,
		Endpoint:  relay.Endpoint,
		Healthy:   relayIsHealthy(relay),
	}
}

func probeKeepalive(active bool) int {
	if active {
		return 25
	}
	return 10
}

func ignoreNoRows(err error) bool { return err == nil || errors.Is(err, pgx.ErrNoRows) }
