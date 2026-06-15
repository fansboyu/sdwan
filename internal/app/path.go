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
	pathModeDirect = "direct"
	pathModeAuto   = "auto"
	pathModeRelay  = "relay"

	pathFailureWindow  = 30 * time.Second
	pathRecoveryWindow = 30 * time.Second
	pathRelayMinDwell  = 60 * time.Second
	relayHeartbeatTTL  = 15 * time.Second
)

type PeerStatReport struct {
	PublicKey       string     `json:"public_key"`
	LatestHandshake *time.Time `json:"latest_handshake_at,omitempty"`
	RxBytes         int64      `json:"rx_bytes"`
	TxBytes         int64      `json:"tx_bytes"`
}

type AppliedPathReport struct {
	ClientDeviceID string `json:"client_device_id"`
	Generation     int64  `json:"generation"`
}

type PathAssignment struct {
	ClientDeviceID string `json:"client_device_id"`
	DesiredPath    string `json:"desired_path"`
	State          string `json:"state"`
	Generation     int64  `json:"generation"`
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
		if relay.LastSeenAt == nil || time.Since(*relay.LastSeenAt) > relayHeartbeatTTL {
			return errors.New("active relay is offline")
		}
		devices, err := s.store.Queries.ListDevicesByUser(ctx, user.ID)
		if err != nil {
			return err
		}
		for _, device := range devices {
			if device.Status == "active" && version.Compare(device.ClientVersion, version.Version) < 0 {
				return fmt.Errorf("device %s must upgrade to %s", device.Hostname, version.Version)
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
		if err := s.store.Queries.UpsertAppliedPath(ctx, device.ID, item.ClientDeviceID, item.Generation); err != nil {
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
			clientGen, cErr := s.store.Queries.GetAppliedPathGeneration(ctx, client.ID, client.ID)
			mainGen, mErr := s.store.Queries.GetAppliedPathGeneration(ctx, mainSite.ID, client.ID)
			if cErr == nil && mErr == nil && clientGen >= path.Generation && mainGen >= path.Generation {
				if err := s.store.Queries.CompletePeerPath(ctx, user.ID, client.ID); err != nil {
					return false, err
				}
				changed = true
			}
			continue
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
	return relay.LastSeenAt != nil && time.Since(*relay.LastSeenAt) <= relayHeartbeatTTL
}

func (s *Service) pathAwareNetmap(ctx context.Context, user sqlc.User, self sqlc.Device, devices []sqlc.Device, endpoints map[string][]sqlc.DeviceEndpoint, routesByDevice map[string][]string, _ []sqlc.SubnetRoute) (Netmap, bool, error) {
	mode := defaultString(user.PathMode, pathModeDirect)
	if mode == pathModeDirect && version.Compare(self.ClientVersion, version.Version) < 0 {
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
	if mode != pathModeDirect {
		if item, err := s.store.Queries.GetActiveRelayByUser(ctx, user.ID); err == nil {
			relay = &item
		}
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
			active := path.DesiredPath == pathModeDirect || relay == nil
			allowed := []string{}
			if active {
				allowed = []string{client.VirtualIP + "/32"}
			}
			peers = append(peers, NetmapPeer{DeviceID: client.ID, Hostname: client.Hostname, VirtualIP: client.VirtualIP, PublicKey: client.PublicKey, AllowedIPs: allowed, Endpoints: orderedEndpointAddresses(endpoints[client.ID]), PersistentKeepalive: probeKeepalive(active), PathRole: pathModeDirect, PathActive: active})
			if !active {
				relayAllowed = append(relayAllowed, client.VirtualIP+"/32")
			}
			assignments = append(assignments, PathAssignment{ClientDeviceID: client.ID, DesiredPath: path.DesiredPath, State: path.State, Generation: path.Generation})
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
		directActive := path.DesiredPath == pathModeDirect || relay == nil
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
		assignments = []PathAssignment{{ClientDeviceID: self.ID, DesiredPath: path.DesiredPath, State: path.State, Generation: path.Generation}}
		generation = path.Generation
	}
	relays := []interface{}{}
	if relay != nil {
		relays = []interface{}{*relay}
	}
	return Netmap{Version: user.NetmapVersion, OverlayCIDR: user.OverlayCidr, Self: NetmapSelf{DeviceID: self.ID, Hostname: self.Hostname, VirtualIP: self.VirtualIP, PublicKey: self.PublicKey, SiteRole: defaultString(self.SiteRole, siteRoleClient)}, Peers: peers, Relays: relays, PathMode: mode, PathGeneration: generation, PathAssignments: assignments}, true, nil
}

func relayNetmapPeer(relay sqlc.Relay, allowed []string, active bool) NetmapPeer {
	return NetmapPeer{DeviceID: relay.ID, Hostname: relay.Name, VirtualIP: relay.VirtualIP, PublicKey: relay.PublicKey, AllowedIPs: dedupeStrings(allowed), Endpoints: []string{relay.Endpoint}, PersistentKeepalive: probeKeepalive(active), PathRole: pathModeRelay, PathActive: active}
}

func probeKeepalive(active bool) int {
	if active {
		return 25
	}
	return 10
}

func ignoreNoRows(err error) bool { return err == nil || errors.Is(err, pgx.ErrNoRows) }
