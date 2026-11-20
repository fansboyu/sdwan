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
	return RunSyncCycle(ctx, SyncCycleOptions{
		ConfigPath:            opts.ConfigPath,
		WGPath:                opts.WGPath,
		Detector:              detector,
		ForceApply:            forceApply,
		IncludeSubnetGateways: true,
		ApplyNetmap: func(ctx context.Context, cfg *Config, netmap Netmap) error {
			return applyNetmap(ctx, opts, cfg, netmap)
		},
	})
}

func applyNetmap(ctx context.Context, opts DaemonOptions, cfg *Config, netmap Netmap) error {
	_ = ctx
	content, err := RenderWireGuardConfig(WGRenderInput{
		PrivateKey:    cfg.PrivateKey,
		VirtualIP:     cfg.VirtualIP,
		ListenPort:    cfg.ListenPort,
		InterfaceName: cfg.InterfaceName,
		Netmap:        netmap,
	})
	if err != nil {
		return err
	}
	if err := WriteWireGuardConfig(opts.WGPath, content); err != nil {
		return err
	}

	if opts.Apply {
		result, err := ApplyWireGuardConfigSmart(*cfg, opts.WGPath, netmap)
		if err != nil {
			return err
		}
		cfg.LastRoutes = result.LastRoutes
	}

	cfg.NetmapVersion = netmap.Version
	cfg.AppliedPaths = AppliedPathReports(ctx, netmap)
	cfg.LastConfigPath = filepath.Clean(opts.WGPath)
	return nil
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

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
