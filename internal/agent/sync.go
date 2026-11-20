package agent

import (
	"context"
	"log"
	"time"
)

// SyncCycleOptions contains the platform-specific parts of one controller
// synchronization cycle. Platform runners keep ownership of WireGuard and
// route configuration while sharing the controller and netmap lifecycle.
type SyncCycleOptions struct {
	ConfigPath            string
	WGPath                string
	ClientVersion         string
	Detector              EndpointDetector
	PeerStats             []PeerStat
	SubnetGatewayStatuses []SubnetGatewayStatus
	IncludeSubnetGateways bool
	ForceApply            bool
	IsStarted             func() bool
	ApplyNetmap           func(context.Context, *Config, Netmap) error
}

// RunSyncCycle polls the controller, applies a changed netmap when needed,
// and persists the resulting local state. It returns the controller's next
// polling interval (or 15 seconds when the response omits one).
func RunSyncCycle(ctx context.Context, opts SyncCycleOptions) (time.Duration, error) {
	requestedWGPath := opts.WGPath
	if opts.ConfigPath == "" {
		opts.ConfigPath = DefaultConfigPath
	}
	if opts.WGPath == "" {
		opts.WGPath = DefaultWireGuardConfigPath
	}
	if opts.ApplyNetmap == nil {
		opts.ApplyNetmap = func(ctx context.Context, cfg *Config, netmap Netmap) error {
			return applyNetmap(ctx, DaemonOptions{WGPath: opts.WGPath, Apply: true}, cfg, netmap)
		}
	}
	if opts.IsStarted == nil {
		opts.IsStarted = func() bool { return true }
	}

	cfg, err := LoadConfig(opts.ConfigPath)
	if err != nil {
		return 0, err
	}
	if cfg.LastConfigPath == "" && requestedWGPath != "" {
		cfg.LastConfigPath = requestedWGPath
	}
	if opts.ClientVersion != "" {
		cfg.ClientVersion = opts.ClientVersion
	}

	client := NewAPIClient(cfg.ControllerURL)
	detector := opts.Detector
	if detector.Timeout <= 0 {
		detector.Timeout = 3 * time.Second
	}
	endpoints := detector.Detect(ctx, cfg)
	peerStats := opts.PeerStats
	if peerStats == nil {
		peerStats, _ = CollectPeerStats(cfg.InterfaceName)
	}
	cfg.PathQualities = RefreshPathQualitiesFromStats(cfg.PathQualities, peerStats)

	subnetGateways := opts.SubnetGatewayStatuses
	if opts.IncludeSubnetGateways {
		subnetGateways = collectSubnetGatewayStatuses(cfg)
	}
	pollResp, err := client.Poll(ctx, cfg.DeviceToken, PollRequest{
		CurrentNetmapVersion: cfg.NetmapVersion,
		ClientVersion:        cfg.ClientVersion,
		OSVersion:            cfg.OSVersion,
		Endpoints:            endpoints,
		AdvertiseRoutes:      cfg.AdvertiseRoutes,
		SubnetGateways:       subnetGateways,
		PeerStats:            peerStats,
		PathQuality:          PathQualityReports(cfg.PathQualities),
		AppliedPaths:         cfg.AppliedPaths,
	})
	if err != nil {
		return 0, err
	}
	interval := time.Duration(pollResp.PollIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 15 * time.Second
	}

	if !pollResp.NetmapChanged && !opts.ForceApply && opts.IsStarted() {
		if cfg.LastNetmap != nil {
			effectiveNetmap, qualities, localChanged := ApplyLocalPathFallback(*cfg.LastNetmap, cfg.PathQualities)
			cfg.PathQualities = qualities
			if localChanged {
				if err := opts.ApplyNetmap(ctx, &cfg, effectiveNetmap); err != nil {
					return interval, err
				}
				cfg.NetmapVersion = effectiveNetmap.Version
				cfg.AppliedPaths = AppliedPathReports(ctx, effectiveNetmap)
				if err := SaveConfig(opts.ConfigPath, cfg); err != nil {
					return interval, err
				}
				log.Printf("local path fallback applied: version=%d peers=%d", effectiveNetmap.Version, len(effectiveNetmap.Peers))
				return interval, nil
			}
			cfg.AppliedPaths = AppliedPathReports(ctx, effectiveNetmap)
		}
		if err := SaveConfig(opts.ConfigPath, cfg); err != nil {
			return interval, err
		}
		log.Printf("poll ok: status=%s version=%d endpoints=%d", pollResp.DeviceStatus, pollResp.NetmapVersion, len(endpoints))
		return interval, nil
	}

	netmap, err := client.Netmap(ctx, cfg.DeviceToken)
	if err != nil {
		return interval, err
	}
	cfg.LastNetmap = &netmap
	cfg.PathQualities = UpdatePathQualitiesFromNetmap(netmap, peerStats, cfg.PathQualities)
	effectiveNetmap, qualities, _ := ApplyLocalPathFallback(netmap, cfg.PathQualities)
	cfg.PathQualities = qualities
	if err := opts.ApplyNetmap(ctx, &cfg, effectiveNetmap); err != nil {
		return interval, err
	}
	cfg.NetmapVersion = netmap.Version
	cfg.AppliedPaths = AppliedPathReports(ctx, effectiveNetmap)
	if err := SaveConfig(opts.ConfigPath, cfg); err != nil {
		return interval, err
	}
	log.Printf("netmap applied: version=%d role=%s peers=%d endpoints=%d bootstrap=%v",
		effectiveNetmap.Version, defaultString(effectiveNetmap.Self.SiteRole, "client"), len(effectiveNetmap.Peers), len(endpoints), effectiveNetmap.BootstrapPeer != nil)
	return interval, nil
}
