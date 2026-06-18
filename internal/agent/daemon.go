package agent

import (
	"context"
	"log"
	"path/filepath"
	"time"
)

type DaemonOptions struct {
	ConfigPath string
	WGPath     string
	Apply      bool
	Once       bool
}

func RunDaemon(ctx context.Context, opts DaemonOptions) error {
	if opts.ConfigPath == "" {
		opts.ConfigPath = DefaultConfigPath
	}
	if opts.WGPath == "" {
		opts.WGPath = DefaultWireGuardConfigPath
	}

	detector := EndpointDetector{Timeout: 3 * time.Second}
	firstRun := true

	for {
		interval, err := runDaemonOnce(ctx, opts, detector, firstRun)
		if err != nil {
			log.Printf("daemon iteration failed: %v", err)
			interval = 15 * time.Second
		}
		firstRun = false
		if opts.Once {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

func runDaemonOnce(ctx context.Context, opts DaemonOptions, detector EndpointDetector, forceApply bool) (time.Duration, error) {
	cfg, err := LoadConfig(opts.ConfigPath)
	if err != nil {
		return 0, err
	}
	if cfg.LastConfigPath == "" {
		cfg.LastConfigPath = opts.WGPath
	}

	client := NewAPIClient(cfg.ControllerURL)
	endpoints := detector.Detect(ctx, cfg)
	peerStats, _ := CollectPeerStats(cfg.InterfaceName)
	subnetGateways := collectSubnetGatewayStatuses(cfg)

	pollResp, err := client.Poll(ctx, cfg.DeviceToken, PollRequest{
		CurrentNetmapVersion: cfg.NetmapVersion,
		ClientVersion:        cfg.ClientVersion,
		OSVersion:            cfg.OSVersion,
		Endpoints:            endpoints,
		AdvertiseRoutes:      cfg.AdvertiseRoutes,
		SubnetGateways:       subnetGateways,
		PeerStats:            peerStats,
		AppliedPaths:         cfg.AppliedPaths,
	})
	if err != nil {
		return 0, err
	}

	interval := time.Duration(pollResp.PollIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 15 * time.Second
	}

	if !pollResp.NetmapChanged && !forceApply {
		log.Printf("poll ok: status=%s version=%d endpoints=%d", pollResp.DeviceStatus, pollResp.NetmapVersion, len(endpoints))
		return interval, nil
	}

	netmap, err := client.Netmap(ctx, cfg.DeviceToken)
	if err != nil {
		return interval, err
	}

	content, err := RenderWireGuardConfig(WGRenderInput{
		PrivateKey:    cfg.PrivateKey,
		VirtualIP:     cfg.VirtualIP,
		ListenPort:    cfg.ListenPort,
		InterfaceName: cfg.InterfaceName,
		Netmap:        netmap,
	})
	if err != nil {
		return interval, err
	}
	if err := WriteWireGuardConfig(opts.WGPath, content); err != nil {
		return interval, err
	}

	if opts.Apply {
		result, err := ApplyWireGuardConfigSmart(cfg, opts.WGPath, netmap)
		if err != nil {
			return interval, err
		}
		cfg.LastRoutes = result.LastRoutes
	}

	cfg.NetmapVersion = netmap.Version
	cfg.AppliedPaths = appliedPaths(netmap.PathAssignments)
	cfg.LastConfigPath = filepath.Clean(opts.WGPath)
	if err := SaveConfig(opts.ConfigPath, cfg); err != nil {
		return interval, err
	}
	log.Printf("netmap applied: version=%d role=%s peers=%d endpoints=%d bootstrap=%v apply=%v",
		netmap.Version, defaultString(netmap.Self.SiteRole, "client"), len(netmap.Peers), len(endpoints), netmap.BootstrapPeer != nil, opts.Apply)
	return interval, nil
}

func collectSubnetGatewayStatuses(cfg Config) []SubnetGatewayStatus {
	items := make([]SubnetGatewayStatus, 0, len(cfg.SubnetGateways))
	for _, opts := range cfg.SubnetGateways {
		if opts.WGInterface == "" {
			opts.WGInterface = cfg.InterfaceName
		}
		status, err := CheckSubnetGatewayStatus(opts)
		if err != nil && status.Error == "" {
			status.Error = err.Error()
		}
		items = append(items, status)
	}
	return items
}

func appliedPaths(assignments []PathAssignment) []AppliedPath {
	result := make([]AppliedPath, 0, len(assignments))
	for _, item := range assignments {
		result = append(result, AppliedPath{ClientDeviceID: item.ClientDeviceID, Generation: item.Generation})
	}
	return result
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
