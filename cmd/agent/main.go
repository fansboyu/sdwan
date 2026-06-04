package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"englishlisten/sdwan/internal/agent"
	"englishlisten/sdwan/internal/version"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	ctx := context.Background()
	var err error
	switch os.Args[1] {
	case "register":
		err = runRegister(ctx, os.Args[2:])
	case "poll":
		err = runPoll(ctx, os.Args[2:])
	case "netmap":
		err = runNetmap(ctx, os.Args[2:])
	case "render":
		err = runRender(ctx, os.Args[2:])
	case "up":
		err = runUp(ctx, os.Args[2:])
	case "down":
		err = runDown(os.Args[2:])
	case "daemon":
		err = runDaemon(os.Args[2:])
	case "subnet-gateway":
		err = runSubnetGateway(os.Args[2:])
	case "routes":
		err = runRoutes(os.Args[2:])
	case "version":
		fmt.Println(version.Version)
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func runDaemon(args []string) error {
	fs := flag.NewFlagSet("daemon", flag.ExitOnError)
	configPath := fs.String("config", agent.DefaultConfigPath, "agent config path")
	wgPath := fs.String("wg-config", agent.DefaultWireGuardConfigPath, "wireguard config output path")
	apply := fs.Bool("apply", true, "apply wireguard config with wg-quick")
	once := fs.Bool("once", false, "run one daemon iteration and exit")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, stop := agentSignalContext()
	defer stop()

	return agent.RunDaemon(ctx, agent.DaemonOptions{
		ConfigPath: *configPath,
		WGPath:     *wgPath,
		Apply:      *apply,
		Once:       *once,
	})
}

func runRegister(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("register", flag.ExitOnError)
	configPath := fs.String("config", agent.DefaultConfigPath, "agent config path")
	controllerURL := fs.String("controller", "https://controller.englishlisten.cn", "controller URL")
	adminToken := fs.String("admin-token", "", "admin token used for first-time device registration")
	hostname := fs.String("hostname", agent.DefaultHostname(), "device hostname")
	listenPort := fs.Int("listen-port", agent.DefaultListenPort, "wireguard listen port")
	advertiseRoutes := fs.String("advertise-routes", "", "comma-separated subnet routes to advertise from this device")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *adminToken == "" {
		return fmt.Errorf("--admin-token is required")
	}
	routes, err := normalizeRouteCSV(*advertiseRoutes)
	if err != nil {
		return err
	}
	privateKey, publicKey, err := agent.GenerateKeyPair()
	if err != nil {
		return err
	}
	client := agent.NewAPIClient(*controllerURL)
	resp, err := client.Register(ctx, agent.RegisterRequest{
		AdminToken:    *adminToken,
		Hostname:      *hostname,
		OS:            agent.DefaultOS(),
		Arch:          agent.DefaultArch(),
		PublicKey:     publicKey,
		ClientVersion: version.Version,
	})
	if err != nil {
		return err
	}
	cfg := agent.Config{
		ControllerURL:   *controllerURL,
		DeviceID:        resp.DeviceID,
		DeviceToken:     resp.DeviceToken,
		Hostname:        *hostname,
		OS:              agent.DefaultOS(),
		Arch:            agent.DefaultArch(),
		ClientVersion:   version.Version,
		PrivateKey:      privateKey,
		PublicKey:       publicKey,
		VirtualIP:       resp.VirtualIP,
		NetmapVersion:   resp.NetmapVersion,
		InterfaceName:   agent.DefaultInterface,
		ListenPort:      *listenPort,
		AdvertiseRoutes: routes,
	}
	if err := agent.SaveConfig(*configPath, cfg); err != nil {
		return err
	}
	printJSON(map[string]any{
		"device_id":   cfg.DeviceID,
		"virtual_ip":  cfg.VirtualIP,
		"config_path": *configPath,
	})
	return nil
}

func runPoll(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("poll", flag.ExitOnError)
	configPath := fs.String("config", agent.DefaultConfigPath, "agent config path")
	endpoint := fs.String("endpoint", "", "optional endpoint to report, for example 192.168.1.10:41641")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := agent.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	var endpoints []agent.EndpointReport
	if *endpoint != "" {
		endpoints = append(endpoints, agent.EndpointReport{Type: "manual", Address: *endpoint, Source: "cli"})
	}
	resp, err := agent.NewAPIClient(cfg.ControllerURL).Poll(ctx, cfg.DeviceToken, agent.PollRequest{
		CurrentNetmapVersion: cfg.NetmapVersion,
		ClientVersion:        cfg.ClientVersion,
		OSVersion:            cfg.OSVersion,
		Endpoints:            endpoints,
		AdvertiseRoutes:      cfg.AdvertiseRoutes,
	})
	if err != nil {
		return err
	}
	if resp.NetmapVersion != cfg.NetmapVersion {
		cfg.NetmapVersion = resp.NetmapVersion
		_ = agent.SaveConfig(*configPath, cfg)
	}
	printJSON(resp)
	return nil
}

func runNetmap(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("netmap", flag.ExitOnError)
	configPath := fs.String("config", agent.DefaultConfigPath, "agent config path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := agent.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	netmap, err := agent.NewAPIClient(cfg.ControllerURL).Netmap(ctx, cfg.DeviceToken)
	if err != nil {
		return err
	}
	printJSON(netmap)
	return nil
}

func runRender(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("render", flag.ExitOnError)
	configPath := fs.String("config", agent.DefaultConfigPath, "agent config path")
	outputPath := fs.String("out", "", "wireguard config output path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, netmap, content, err := renderConfig(ctx, *configPath)
	if err != nil {
		return err
	}
	if *outputPath == "" {
		*outputPath = filepath.Join(os.TempDir(), cfg.InterfaceName+".conf")
	}
	if err := agent.WriteWireGuardConfig(*outputPath, content); err != nil {
		return err
	}
	cfg.NetmapVersion = netmap.Version
	cfg.LastConfigPath = *outputPath
	_ = agent.SaveConfig(*configPath, cfg)
	fmt.Println(*outputPath)
	return nil
}

func runUp(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("up", flag.ExitOnError)
	configPath := fs.String("config", agent.DefaultConfigPath, "agent config path")
	outputPath := fs.String("out", agent.DefaultWireGuardConfigPath, "wireguard config output path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, netmap, content, err := renderConfig(ctx, *configPath)
	if err != nil {
		return err
	}
	if err := agent.WriteWireGuardConfig(*outputPath, content); err != nil {
		return err
	}
	cfg.NetmapVersion = netmap.Version
	cfg.LastConfigPath = *outputPath
	result, err := agent.ApplyWireGuardConfigSmart(cfg, *outputPath, netmap)
	if err != nil {
		return err
	}
	cfg.LastRoutes = result.LastRoutes
	_ = agent.SaveConfig(*configPath, cfg)
	return nil
}

func runDown(args []string) error {
	fs := flag.NewFlagSet("down", flag.ExitOnError)
	configPath := fs.String("config", agent.DefaultConfigPath, "agent config path")
	interfaceName := fs.String("interface", "", "wireguard interface or tunnel name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *interfaceName == "" {
		cfg, err := agent.LoadConfig(*configPath)
		if err == nil {
			*interfaceName = cfg.InterfaceName
		}
	}
	return agent.DownWireGuardTunnel(*interfaceName)
}

func runSubnetGateway(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("subnet-gateway requires a subcommand: enable, status, or disable")
	}
	switch args[0] {
	case "enable":
		return runSubnetGatewayEnable(args[1:])
	case "status":
		return runSubnetGatewayStatus(args[1:])
	case "disable":
		return runSubnetGatewayDisable(args[1:])
	default:
		return fmt.Errorf("unknown subnet-gateway subcommand: %s", args[0])
	}
}

func subnetGatewayFlagSet(name string, args []string) (agent.SubnetGatewayOptions, error) {
	fs := flag.NewFlagSet("subnet-gateway "+name, flag.ExitOnError)
	lanCIDR := fs.String("lan-cidr", "", "LAN subnet CIDR to expose, for example 192.168.50.0/24")
	outInterface := fs.String("out-interface", "", "LAN-facing output interface, for example eth0")
	wgInterface := fs.String("wg-interface", agent.DefaultInterface, "SD-WAN WireGuard interface")
	overlayCIDR := fs.String("overlay-cidr", agent.DefaultOverlayCIDR, "SD-WAN overlay CIDR")
	if err := fs.Parse(args); err != nil {
		return agent.SubnetGatewayOptions{}, err
	}
	return agent.SubnetGatewayOptions{
		LANCIDR:      *lanCIDR,
		OutInterface: *outInterface,
		WGInterface:  *wgInterface,
		OverlayCIDR:  *overlayCIDR,
	}, nil
}

func runSubnetGatewayEnable(args []string) error {
	opts, err := subnetGatewayFlagSet("enable", args)
	if err != nil {
		return err
	}
	status, err := agent.EnableSubnetGateway(opts)
	printJSON(status)
	return err
}

func runSubnetGatewayStatus(args []string) error {
	opts, err := subnetGatewayFlagSet("status", args)
	if err != nil {
		return err
	}
	status, err := agent.CheckSubnetGatewayStatus(opts)
	printJSON(status)
	return err
}

func runSubnetGatewayDisable(args []string) error {
	opts, err := subnetGatewayFlagSet("disable", args)
	if err != nil {
		return err
	}
	status, err := agent.DisableSubnetGateway(opts)
	printJSON(status)
	return err
}

func runRoutes(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("routes requires a subcommand: list, add, or remove")
	}
	switch args[0] {
	case "list":
		return runRoutesList(args[1:])
	case "add":
		return runRoutesAdd(args[1:])
	case "remove":
		return runRoutesRemove(args[1:])
	default:
		return fmt.Errorf("unknown routes subcommand: %s", args[0])
	}
}

func routesFlagSet(name string, args []string) (*flag.FlagSet, *string, error) {
	fs := flag.NewFlagSet("routes "+name, flag.ExitOnError)
	configPath := fs.String("config", agent.DefaultConfigPath, "agent config path")
	if err := fs.Parse(args); err != nil {
		return nil, nil, err
	}
	return fs, configPath, nil
}

func runRoutesList(args []string) error {
	_, configPath, err := routesFlagSet("list", args)
	if err != nil {
		return err
	}
	cfg, err := agent.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	printJSON(map[string]any{"advertise_routes": normalizeRouteList(cfg.AdvertiseRoutes)})
	return nil
}

func runRoutesAdd(args []string) error {
	fs, configPath, err := routesFlagSet("add", args)
	if err != nil {
		return err
	}
	values := fs.Args()
	if len(values) == 0 {
		return fmt.Errorf("routes add requires at least one CIDR")
	}
	cfg, err := agent.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	routes := append([]string{}, cfg.AdvertiseRoutes...)
	for _, value := range values {
		route, err := normalizeRouteCIDR(value)
		if err != nil {
			return err
		}
		routes = append(routes, route)
	}
	cfg.AdvertiseRoutes = normalizeRouteList(routes)
	if err := agent.SaveConfig(*configPath, cfg); err != nil {
		return err
	}
	printJSON(map[string]any{"advertise_routes": cfg.AdvertiseRoutes})
	return nil
}

func runRoutesRemove(args []string) error {
	fs, configPath, err := routesFlagSet("remove", args)
	if err != nil {
		return err
	}
	values := fs.Args()
	if len(values) == 0 {
		return fmt.Errorf("routes remove requires at least one CIDR")
	}
	cfg, err := agent.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	remove := map[string]bool{}
	for _, value := range values {
		route, err := normalizeRouteCIDR(value)
		if err != nil {
			return err
		}
		remove[route] = true
	}
	var kept []string
	for _, route := range normalizeRouteList(cfg.AdvertiseRoutes) {
		if remove[route] {
			continue
		}
		kept = append(kept, route)
	}
	cfg.AdvertiseRoutes = kept
	if err := agent.SaveConfig(*configPath, cfg); err != nil {
		return err
	}
	printJSON(map[string]any{"advertise_routes": cfg.AdvertiseRoutes})
	return nil
}

func renderConfig(ctx context.Context, configPath string) (agent.Config, agent.Netmap, string, error) {
	cfg, err := agent.LoadConfig(configPath)
	if err != nil {
		return agent.Config{}, agent.Netmap{}, "", err
	}
	netmap, err := agent.NewAPIClient(cfg.ControllerURL).Netmap(ctx, cfg.DeviceToken)
	if err != nil {
		return agent.Config{}, agent.Netmap{}, "", err
	}
	content, err := agent.RenderWireGuardConfig(agent.WGRenderInput{
		PrivateKey:    cfg.PrivateKey,
		VirtualIP:     cfg.VirtualIP,
		ListenPort:    cfg.ListenPort,
		InterfaceName: cfg.InterfaceName,
		Netmap:        netmap,
	})
	if err != nil {
		return agent.Config{}, agent.Netmap{}, "", err
	}
	return cfg, netmap, content, nil
}

func printJSON(value any) {
	data, _ := json.MarshalIndent(value, "", "  ")
	fmt.Println(string(data))
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	var result []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		result = append(result, part)
	}
	return result
}

func normalizeRouteCSV(value string) ([]string, error) {
	var routes []string
	for _, item := range splitCSV(value) {
		route, err := normalizeRouteCIDR(item)
		if err != nil {
			return nil, err
		}
		routes = append(routes, route)
	}
	return normalizeRouteList(routes), nil
}

func normalizeRouteCIDR(value string) (string, error) {
	prefix, err := netip.ParsePrefix(strings.TrimSpace(value))
	if err != nil {
		return "", fmt.Errorf("invalid route CIDR %q: %w", value, err)
	}
	prefix = prefix.Masked()
	if !prefix.Addr().Is4() {
		return "", fmt.Errorf("route CIDR must be IPv4: %s", value)
	}
	if prefix.Bits() == 0 {
		return "", fmt.Errorf("default route 0.0.0.0/0 is not supported")
	}
	return prefix.String(), nil
}

func normalizeRouteList(values []string) []string {
	seen := map[string]bool{}
	var result []string
	for _, value := range values {
		route, err := normalizeRouteCIDR(value)
		if err != nil || seen[route] {
			continue
		}
		seen[route] = true
		result = append(result, route)
	}
	sort.Strings(result)
	return result
}

func usage() {
	fmt.Print(`sdwan-agent ` + version.Version + `

Usage:
  sdwan-agent register --controller http://localhost --admin-token ADMIN_TOKEN
  sdwan-agent poll
  sdwan-agent netmap
  sdwan-agent render --out /tmp/sdwan0.conf
  sudo sdwan-agent up
  sudo sdwan-agent down
  sudo sdwan-agent daemon
  sudo sdwan-agent subnet-gateway enable --lan-cidr 192.168.50.0/24 --out-interface eth0
  sdwan-agent subnet-gateway status --lan-cidr 192.168.50.0/24 --out-interface eth0
  sudo sdwan-agent subnet-gateway disable --lan-cidr 192.168.50.0/24 --out-interface eth0
  sdwan-agent routes list
  sudo sdwan-agent routes add 192.168.50.0/24
  sudo sdwan-agent routes remove 192.168.50.0/24
  sdwan-agent version

Network apply dependencies:
  Linux: wireguard-tools, wg, wg-quick
  Windows: WireGuard for Windows, Administrator privileges
`)
}
