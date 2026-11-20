package agent

import (
	"testing"
	"time"
)

func TestApplyLocalPathFallbackSwitchesToRelay(t *testing.T) {
	old := time.Now().UTC().Add(-time.Minute)
	relayHandshake := time.Now().UTC()
	netmap := localFallbackTestNetmap()
	qualities := []PathQualityState{
		{
			PeerDeviceID:      "main",
			PeerPublicKey:     "direct-key",
			PathType:          "direct",
			CurrentPath:       true,
			LatestHandshakeAt: &old,
			FailCount:         2,
		},
		{
			PeerDeviceID:      "relay",
			PeerPublicKey:     "relay-key",
			PathType:          "relay",
			CurrentPath:       false,
			LatestHandshakeAt: &relayHandshake,
		},
	}

	effective, updated, changed := ApplyLocalPathFallback(netmap, qualities)
	if !changed {
		t.Fatalf("expected local fallback change")
	}
	if effective.Peers[0].PathActive || len(effective.Peers[0].AllowedIPs) != 0 {
		t.Fatalf("direct peer should be inactive: %#v", effective.Peers[0])
	}
	if !effective.Peers[1].PathActive || len(effective.Peers[1].AllowedIPs) != 2 {
		t.Fatalf("relay peer should own business routes: %#v", effective.Peers[1])
	}
	if q := findQuality(updated, "relay", "relay"); q == nil || !q.CurrentPath {
		t.Fatalf("relay quality not marked current: %#v", updated)
	}
}

func TestApplyLocalPathFallbackDoesNotSelectUnverifiedRelay(t *testing.T) {
	old := time.Now().UTC().Add(-time.Minute)
	qualities := []PathQualityState{
		{PeerDeviceID: "main", PathType: "direct", CurrentPath: true, LatestHandshakeAt: &old, FailCount: 3},
		{PeerDeviceID: "relay", PathType: "relay"},
	}

	effective, _, changed := ApplyLocalPathFallback(localFallbackTestNetmap(), qualities)
	if changed {
		t.Fatal("must not switch to a relay without a recent handshake")
	}
	if !effective.Peers[0].PathActive || effective.Peers[1].PathActive {
		t.Fatalf("unverified relay was selected: %#v", effective.Peers)
	}
}

func TestApplyLocalPathFallbackPreservesControllerSelectedRelay(t *testing.T) {
	netmap := localFallbackTestNetmap()
	netmap.Peers[0].PathActive = false
	netmap.Peers[0].AllowedIPs = nil
	netmap.Peers[1].PathActive = true
	netmap.Peers[1].AllowedIPs = []string{"100.64.1.1/32", "192.168.50.0/24"}
	qualities := []PathQualityState{
		{PeerDeviceID: "main", PathType: "direct"},
		{PeerDeviceID: "relay", PathType: "relay", CurrentPath: true},
	}

	effective, _, changed := ApplyLocalPathFallback(netmap, qualities)
	if changed {
		t.Fatal("must not unilaterally change a controller-selected relay path without verified direct recovery")
	}
	if effective.Peers[0].PathActive || !effective.Peers[1].PathActive {
		t.Fatalf("controller-selected relay assignment was not preserved: %#v", effective.Peers)
	}
}

func TestApplyLocalPathFallbackKeepsHubClientRoutesSeparate(t *testing.T) {
	now := time.Now().UTC()
	relaySwitchedAt := now.Add(-2 * time.Minute)
	healthySince := now.Add(-time.Minute)
	netmap := Netmap{
		RelayPolicy: "auto",
		Self:        NetmapSelf{SiteRole: "main_site"},
		Peers: []NetmapPeer{
			{DeviceID: "client-a", VirtualIP: "100.64.1.2", PathRole: "direct", PathActive: false},
			{DeviceID: "client-b", VirtualIP: "100.64.1.3", PathRole: "direct", PathActive: false},
			{DeviceID: "relay", VirtualIP: "100.254.253.1", PublicKey: "relay-key", PathRole: "relay", PathActive: true, AllowedIPs: []string{"100.64.1.2/32", "100.64.1.3/32"}, Endpoints: []string{"relay:51873"}},
		},
	}
	qualities := []PathQualityState{
		{PeerDeviceID: "client-a", PathType: "direct", LatestHandshakeAt: &now, HealthySince: &healthySince},
		{PeerDeviceID: "client-b", PathType: "direct", LatestHandshakeAt: &now, HealthySince: &healthySince},
		{PeerDeviceID: "relay", PathType: "relay", CurrentPath: true, LatestHandshakeAt: &now, SwitchedAt: &relaySwitchedAt},
	}

	effective, _, changed := ApplyLocalPathFallback(netmap, qualities)
	if !changed {
		t.Fatalf("expected both client paths to recover directly: %#v", effective.Peers)
	}
	if !sameStringSet(effective.Peers[0].AllowedIPs, []string{"100.64.1.2/32"}) || !sameStringSet(effective.Peers[1].AllowedIPs, []string{"100.64.1.3/32"}) {
		t.Fatalf("each client should recover only its own overlay route: %#v", effective.Peers[:2])
	}
	if effective.Peers[2].PathActive || len(effective.Peers[2].AllowedIPs) != 0 {
		t.Fatalf("relay should no longer own business routes: %#v", effective.Peers[2])
	}
	if len(netmap.Peers[0].AllowedIPs) != 0 || netmap.Peers[0].PathActive {
		t.Fatalf("fallback mutated the authoritative cached netmap: %#v", netmap.Peers[0])
	}
}

func TestApplyLocalPathFallbackRecoversDirect(t *testing.T) {
	now := time.Now().UTC()
	relayHandshake := now
	healthySince := now.Add(-time.Minute)
	switchedAt := now.Add(-2 * time.Minute)
	netmap := localFallbackTestNetmap()
	netmap.Peers[0].PathActive = false
	netmap.Peers[0].AllowedIPs = nil
	netmap.Peers[1].PathActive = true
	netmap.Peers[1].AllowedIPs = []string{"100.64.1.1/32", "192.168.50.0/24"}
	qualities := []PathQualityState{
		{
			PeerDeviceID:      "main",
			PeerPublicKey:     "direct-key",
			PathType:          "direct",
			CurrentPath:       false,
			LatestHandshakeAt: &now,
			HealthySince:      &healthySince,
		},
		{
			PeerDeviceID:      "relay",
			PeerPublicKey:     "relay-key",
			PathType:          "relay",
			CurrentPath:       true,
			LatestHandshakeAt: &relayHandshake,
			SwitchedAt:        &switchedAt,
		},
	}

	effective, updated, changed := ApplyLocalPathFallback(netmap, qualities)
	if !changed {
		t.Fatalf("expected local recovery change")
	}
	if !effective.Peers[0].PathActive || len(effective.Peers[0].AllowedIPs) != 2 {
		t.Fatalf("direct peer should recover business routes: %#v", effective.Peers[0])
	}
	if effective.Peers[1].PathActive || len(effective.Peers[1].AllowedIPs) != 0 {
		t.Fatalf("relay peer should become probe-only: %#v", effective.Peers[1])
	}
	if q := findQuality(updated, "main", "direct"); q == nil || !q.CurrentPath {
		t.Fatalf("direct quality not marked current: %#v", updated)
	}
}

func localFallbackTestNetmap() Netmap {
	return Netmap{
		RelayPolicy: "auto",
		PathMode:    "auto",
		Peers: []NetmapPeer{
			{
				DeviceID:   "main",
				Hostname:   "main",
				VirtualIP:  "100.64.1.1",
				PublicKey:  "direct-key",
				AllowedIPs: []string{"100.64.1.1/32", "192.168.50.0/24"},
				Endpoints:  []string{"198.51.100.1:41641"},
				PathRole:   "direct",
				PathActive: true,
			},
			{
				DeviceID:   "relay",
				Hostname:   "relay",
				VirtualIP:  "100.254.253.1",
				PublicKey:  "relay-key",
				Endpoints:  []string{"203.0.113.1:51873"},
				PathRole:   "relay",
				PathActive: false,
			},
		},
	}
}
