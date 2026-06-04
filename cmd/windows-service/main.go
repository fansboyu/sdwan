//go:build windows

package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"englishlisten/sdwan/internal/agent"
	"englishlisten/sdwan/internal/version"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
)

const (
	defaultRouteCIDR          = "100.64.0.0/10"
	defaultBootstrapRouteCIDR = "100.254.254.254/32"
	defaultWindowsListenPort  = 41642
	defaultMTU                = 1420
	serviceName               = "SDWANService"
	serviceDisplay            = "SD-WAN Service"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "register":
		err = runRegister(os.Args[2:])
	case "run":
		err = runService(os.Args[2:])
	case "service":
		err = runWindowsService(os.Args[2:])
	case "sync":
		err = runSync(os.Args[2:])
	case "down":
		err = runDown(os.Args[2:])
	case "install-service":
		err = installService(os.Args[2:])
	case "uninstall-service":
		err = uninstallService()
	case "start-service":
		err = controlService(true)
	case "stop-service":
		err = controlService(false)
	case "status":
		err = runStatus(os.Args[2:])
	case "diagnose":
		err = runDiagnose(os.Args[2:])
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

func runRegister(args []string) error {
	fs := flag.NewFlagSet("register", flag.ExitOnError)
	configPath := fs.String("config", agent.DefaultConfigPath, "agent config path")
	controllerURL := fs.String("controller", "https://controller.englishlisten.cn", "controller URL")
	adminToken := fs.String("admin-token", "", "admin token used for first-time registration")
	hostname := fs.String("hostname", agent.DefaultHostname(), "device hostname")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*adminToken) == "" {
		return errors.New("--admin-token is required")
	}

	privateKey, publicKey, err := agent.GenerateKeyPair()
	if err != nil {
		return err
	}
	resp, err := agent.NewAPIClient(*controllerURL).Register(context.Background(), agent.RegisterRequest{
		AdminToken:    *adminToken,
		Hostname:      *hostname,
		OS:            "windows",
		Arch:          runtime.GOARCH,
		PublicKey:     publicKey,
		ClientVersion: version.Version,
	})
	if err != nil {
		return err
	}
	cfg := agent.Config{
		ControllerURL: *controllerURL,
		DeviceID:      resp.DeviceID,
		DeviceToken:   resp.DeviceToken,
		Hostname:      *hostname,
		OS:            "windows",
		Arch:          runtime.GOARCH,
		ClientVersion: version.Version,
		PrivateKey:    privateKey,
		PublicKey:     publicKey,
		VirtualIP:     resp.VirtualIP,
		NetmapVersion: resp.NetmapVersion,
		InterfaceName: agent.DefaultInterface,
		ListenPort:    defaultWindowsListenPort,
	}
	if err := agent.SaveConfig(*configPath, cfg); err != nil {
		return err
	}
	return printJSON(map[string]any{
		"device_id":   cfg.DeviceID,
		"virtual_ip":  cfg.VirtualIP,
		"config_path": *configPath,
	})
}

func runService(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	configPath := fs.String("config", agent.DefaultConfigPath, "agent config path")
	routeCIDR := fs.String("route", defaultRouteCIDR, "overlay route CIDR")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runner := &serviceRunner{configPath: *configPath, routeCIDR: *routeCIDR}
	return runner.Run(ctx)
}

func runWindowsService(args []string) error {
	configPath := agent.DefaultConfigPath
	routeCIDR := defaultRouteCIDR
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 < len(args) {
				configPath = args[i+1]
				i++
			}
		case "--route":
			if i+1 < len(args) {
				routeCIDR = args[i+1]
				i++
			}
		}
	}
	isService, err := svc.IsWindowsService()
	if err != nil {
		return err
	}
	if !isService {
		return runService([]string{"--config", configPath, "--route", routeCIDR})
	}
	return svc.Run(serviceName, &windowsServiceHandler{configPath: configPath, routeCIDR: routeCIDR})
}

func runSync(args []string) error {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	configPath := fs.String("config", agent.DefaultConfigPath, "agent config path")
	routeCIDR := fs.String("route", defaultRouteCIDR, "overlay route CIDR")
	if err := fs.Parse(args); err != nil {
		return err
	}
	runner := &serviceRunner{configPath: *configPath, routeCIDR: *routeCIDR}
	defer runner.Close()
	return runner.RunOnce(context.Background())
}

func runDown(args []string) error {
	fs := flag.NewFlagSet("down", flag.ExitOnError)
	configPath := fs.String("config", agent.DefaultConfigPath, "agent config path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := agent.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	for _, route := range append([]string{}, cfg.LastRoutes...) {
		_ = removeRoute(route, cfg.InterfaceName)
	}
	_ = removeRoute(defaultRouteCIDR, cfg.InterfaceName)
	_ = removeRoute(defaultBootstrapRouteCIDR, cfg.InterfaceName)
	cfg.LastRoutes = nil
	_ = agent.SaveConfig(*configPath, cfg)
	return nil
}

func installService(args []string) error {
	fs := flag.NewFlagSet("install-service", flag.ExitOnError)
	configPath := fs.String("config", agent.DefaultConfigPath, "agent config path")
	routeCIDR := fs.String("route", defaultRouteCIDR, "overlay route CIDR")
	autoStart := fs.Bool("auto-start", true, "start service automatically")
	if err := fs.Parse(args); err != nil {
		return err
	}
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	if existing, err := m.OpenService(serviceName); err == nil {
		existing.Close()
		return fmt.Errorf("%s is already installed", serviceName)
	}
	startType := mgr.StartManual
	if *autoStart {
		startType = mgr.StartAutomatic
	}
	s, err := m.CreateService(serviceName, exePath, mgr.Config{
		DisplayName:      serviceDisplay,
		Description:      "SD-WAN userspace WireGuard service",
		StartType:        uint32(startType),
		DelayedAutoStart: *autoStart,
	}, "service", "--config", *configPath, "--route", *routeCIDR)
	if err != nil {
		return err
	}
	defer s.Close()
	_ = eventlog.InstallAsEventCreate(serviceName, eventlog.Error|eventlog.Warning|eventlog.Info)
	return nil
}

func uninstallService() error {
	_ = controlService(false)
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(serviceName)
	if err != nil {
		return err
	}
	defer s.Close()
	if err := s.Delete(); err != nil {
		return err
	}
	_ = eventlog.Remove(serviceName)
	return nil
}

func controlService(start bool) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(serviceName)
	if err != nil {
		return err
	}
	defer s.Close()
	if start {
		return s.Start()
	}
	status, err := s.Control(svc.Stop)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
			return nil
		}
		return err
	}
	deadline := time.Now().Add(15 * time.Second)
	for status.State != svc.Stopped && time.Now().Before(deadline) {
		time.Sleep(300 * time.Millisecond)
		status, err = s.Query()
		if err != nil {
			return err
		}
	}
	return nil
}

func runStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	configPath := fs.String("config", agent.DefaultConfigPath, "agent config path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := agent.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	cfg.PrivateKey = ""
	cfg.DeviceToken = ""
	return printJSON(cfg)
}

type diagnoseResult struct {
	ConfigPath           string   `json:"config_path"`
	ConfigExists         bool     `json:"config_exists"`
	DeviceID             string   `json:"device_id,omitempty"`
	VirtualIP            string   `json:"virtual_ip,omitempty"`
	ListenPort           int      `json:"listen_port,omitempty"`
	ControllerURL        string   `json:"controller_url,omitempty"`
	ServiceInstalled     bool     `json:"service_installed"`
	ServiceRunning       bool     `json:"service_running"`
	WintunDLLExists      bool     `json:"wintun_dll_exists"`
	InterfaceName        string   `json:"interface_name,omitempty"`
	InterfaceExists      bool     `json:"interface_exists"`
	IPv4Configured       bool     `json:"ipv4_configured"`
	OverlayRouteExists   bool     `json:"overlay_route_exists"`
	BootstrapRouteExists bool     `json:"bootstrap_route_exists"`
	ListenPortInUse      bool     `json:"listen_port_in_use"`
	ControllerReachable  bool     `json:"controller_reachable"`
	ControllerError      string   `json:"controller_error,omitempty"`
	LastRoutes           []string `json:"last_routes,omitempty"`
}

func runDiagnose(args []string) error {
	fs := flag.NewFlagSet("diagnose", flag.ExitOnError)
	configPath := fs.String("config", agent.DefaultConfigPath, "agent config path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result := diagnoseResult{
		ConfigPath:       *configPath,
		ConfigExists:     fileExists(*configPath),
		ServiceInstalled: serviceInstalled(),
		ServiceRunning:   serviceRunning(),
		WintunDLLExists:  fileExists(filepath.Join(executableDir(), "wintun.dll")),
	}
	cfg, err := agent.LoadConfig(*configPath)
	if err == nil {
		result.DeviceID = cfg.DeviceID
		result.VirtualIP = cfg.VirtualIP
		result.ListenPort = defaultPort(cfg.ListenPort)
		result.ControllerURL = cfg.ControllerURL
		result.InterfaceName = cfg.InterfaceName
		result.LastRoutes = cfg.LastRoutes
		result.InterfaceExists = powershellBool(fmt.Sprintf(`$null -ne (Get-NetAdapter -Name %s -ErrorAction SilentlyContinue)`, psQuote(cfg.InterfaceName)))
		result.IPv4Configured = powershellBool(fmt.Sprintf(`$null -ne (Get-NetIPAddress -InterfaceAlias %s -AddressFamily IPv4 -IPAddress %s -ErrorAction SilentlyContinue)`, psQuote(cfg.InterfaceName), psQuote(cfg.VirtualIP)))
		result.OverlayRouteExists = powershellBool(fmt.Sprintf(`$null -ne (Get-NetRoute -InterfaceAlias %s -DestinationPrefix %s -ErrorAction SilentlyContinue)`, psQuote(cfg.InterfaceName), psQuote(defaultRouteCIDR)))
		result.BootstrapRouteExists = powershellBool(fmt.Sprintf(`$null -ne (Get-NetRoute -InterfaceAlias %s -DestinationPrefix %s -ErrorAction SilentlyContinue)`, psQuote(cfg.InterfaceName), psQuote(defaultBootstrapRouteCIDR)))
		result.ListenPortInUse = udpPortInUse(result.ListenPort)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := agent.NewAPIClient(cfg.ControllerURL).Poll(ctx, cfg.DeviceToken, agent.PollRequest{CurrentNetmapVersion: cfg.NetmapVersion, ClientVersion: version.Version}); err != nil {
			result.ControllerError = err.Error()
		} else {
			result.ControllerReachable = true
		}
	}
	return printJSON(result)
}

type serviceRunner struct {
	configPath string
	routeCIDR  string

	mu      sync.Mutex
	cfg     agent.Config
	tun     tun.Device
	dev     *device.Device
	started bool
}

type windowsServiceHandler struct {
	configPath string
	routeCIDR  string
}

func (h *windowsServiceHandler) Execute(_ []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (bool, uint32) {
	s <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())
	runner := &serviceRunner{configPath: h.configPath, routeCIDR: h.routeCIDR}
	errCh := make(chan error, 1)
	go func() {
		errCh <- runner.Run(ctx)
	}()
	s <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				s <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				s <- svc.Status{State: svc.StopPending}
				cancel()
				runner.Close()
				<-errCh
				s <- svc.Status{State: svc.Stopped}
				return false, 0
			default:
			}
		case err := <-errCh:
			if err != nil && !errors.Is(err, context.Canceled) {
				return true, 1
			}
			s <- svc.Status{State: svc.Stopped}
			return false, 0
		}
	}
}

func (r *serviceRunner) Run(ctx context.Context) error {
	interval := 2 * time.Second
	for {
		if err := r.RunOnce(ctx); err != nil {
			log.Printf("sync failed: %v", err)
			interval = 15 * time.Second
		} else {
			interval = r.nextInterval()
		}

		select {
		case <-ctx.Done():
			r.Close()
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

func (r *serviceRunner) RunOnce(ctx context.Context) error {
	cfg, err := agent.LoadConfig(r.configPath)
	if err != nil {
		return err
	}
	client := agent.NewAPIClient(cfg.ControllerURL)
	detector := agent.EndpointDetector{Timeout: 3 * time.Second}
	endpoints := detector.Detect(ctx, cfg)

	pollResp, err := client.Poll(ctx, cfg.DeviceToken, agent.PollRequest{
		CurrentNetmapVersion: cfg.NetmapVersion,
		ClientVersion:        version.Version,
		OSVersion:            cfg.OSVersion,
		Endpoints:            endpoints,
		AdvertiseRoutes:      cfg.AdvertiseRoutes,
	})
	if err != nil {
		return err
	}
	if !pollResp.NetmapChanged && r.isStarted() {
		log.Printf("poll ok: version=%d endpoints=%d", pollResp.NetmapVersion, len(endpoints))
		return nil
	}

	netmap, err := client.Netmap(ctx, cfg.DeviceToken)
	if err != nil {
		return err
	}
	appliedCfg, err := r.applyNetmap(cfg, netmap)
	if err != nil {
		r.Close()
		return err
	}
	cfg = appliedCfg
	cfg.NetmapVersion = netmap.Version
	cfg.ClientVersion = version.Version
	if err := agent.SaveConfig(r.configPath, cfg); err != nil {
		return err
	}
	log.Printf("netmap applied: version=%d role=%s peers=%d endpoints=%d bootstrap=%v",
		netmap.Version, defaultString(netmap.Self.SiteRole, "client"), len(netmap.Peers), len(endpoints), netmap.BootstrapPeer != nil)
	return nil
}

func (r *serviceRunner) applyNetmap(cfg agent.Config, netmap agent.Netmap) (agent.Config, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.started {
		tunDev, err := tun.CreateTUN(cfg.InterfaceName, defaultMTU)
		if err != nil {
			return cfg, fmt.Errorf("create wintun: %w", err)
		}
		realName, err := tunDev.Name()
		if err == nil && realName != "" {
			cfg.InterfaceName = realName
		}
		logger := device.NewLogger(device.LogLevelError, fmt.Sprintf("(%s) ", cfg.InterfaceName))
		dev := device.NewDevice(tunDev, conn.NewDefaultBind(), logger)
		r.tun = tunDev
		r.dev = dev
		r.started = true
	}

	uapi, err := renderUAPI(cfg, netmap)
	if err != nil {
		return cfg, err
	}
	if err := r.dev.IpcSet(uapi); err != nil {
		return cfg, fmt.Errorf("configure wireguard engine: %w", err)
	}
	routes := agent.DesiredRoutes(netmap)
	if err := configureInterface(cfg.InterfaceName, cfg.VirtualIP, cfg.LastRoutes, routes); err != nil {
		return cfg, err
	}
	if err := r.dev.Up(); err != nil {
		return cfg, fmt.Errorf("start wireguard engine: %w", err)
	}
	cfg.LastRoutes = routes
	r.cfg = cfg
	return cfg, nil
}

func (r *serviceRunner) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.dev != nil {
		r.dev.Close()
	}
	if r.tun != nil {
		_ = r.tun.Close()
	}
	if r.cfg.InterfaceName != "" {
		for _, route := range r.cfg.LastRoutes {
			_ = removeRoute(route, r.cfg.InterfaceName)
		}
		_ = removeRoute(r.routeCIDR, r.cfg.InterfaceName)
		_ = removeRoute(defaultBootstrapRouteCIDR, r.cfg.InterfaceName)
	}
	r.started = false
}

func (r *serviceRunner) isStarted() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.started
}

func (r *serviceRunner) nextInterval() time.Duration {
	cfg, err := agent.LoadConfig(r.configPath)
	if err != nil || cfg.NetmapVersion == 0 {
		return 15 * time.Second
	}
	return 15 * time.Second
}

func renderUAPI(cfg agent.Config, netmap agent.Netmap) (string, error) {
	privateKey, err := wgBase64ToHex(cfg.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("private key: %w", err)
	}
	var b strings.Builder
	writeUAPI(&b, "private_key", privateKey)
	writeUAPI(&b, "listen_port", strconv.Itoa(defaultPort(cfg.ListenPort)))
	writeUAPI(&b, "replace_peers", "true")

	peers := append([]agent.NetmapPeer{}, netmap.Peers...)
	if netmap.BootstrapPeer != nil {
		peers = append(peers, *netmap.BootstrapPeer)
	}
	sort.Slice(peers, func(i, j int) bool {
		return peers[i].VirtualIP < peers[j].VirtualIP
	})
	for _, peer := range peers {
		if strings.TrimSpace(peer.PublicKey) == "" {
			continue
		}
		publicKey, err := wgBase64ToHex(peer.PublicKey)
		if err != nil {
			return "", fmt.Errorf("peer %s public key: %w", peer.DeviceID, err)
		}
		writeUAPI(&b, "public_key", publicKey)
		writeUAPI(&b, "protocol_version", "1")
		writeUAPI(&b, "replace_allowed_ips", "true")
		for _, allowedIP := range peerAllowedIPs(peer) {
			writeUAPI(&b, "allowed_ip", allowedIP)
		}
		if len(peer.Endpoints) > 0 && strings.TrimSpace(peer.Endpoints[0]) != "" {
			endpoint, err := resolveEndpoint(strings.TrimSpace(peer.Endpoints[0]))
			if err != nil {
				return "", fmt.Errorf("peer %s endpoint: %w", peer.DeviceID, err)
			}
			writeUAPI(&b, "endpoint", endpoint)
		}
		if peer.PersistentKeepalive > 0 {
			writeUAPI(&b, "persistent_keepalive_interval", strconv.Itoa(peer.PersistentKeepalive))
		}
	}
	return b.String(), nil
}

func resolveEndpoint(endpoint string) (string, error) {
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		return "", err
	}
	if host == "" || port == "" {
		return "", fmt.Errorf("invalid endpoint %q", endpoint)
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		return net.JoinHostPort(ip.String(), port), nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return "", err
	}
	if len(ips) == 0 {
		return "", fmt.Errorf("no addresses for %s", host)
	}
	for _, ip := range ips {
		if ipv4 := ip.To4(); ipv4 != nil {
			return net.JoinHostPort(ipv4.String(), port), nil
		}
	}
	return net.JoinHostPort(ips[0].String(), port), nil
}

func peerAllowedIPs(peer agent.NetmapPeer) []string {
	if len(peer.AllowedIPs) > 0 {
		return peer.AllowedIPs
	}
	if strings.TrimSpace(peer.VirtualIP) == "" {
		return nil
	}
	return []string{strings.TrimSpace(peer.VirtualIP) + "/32"}
}

func wgBase64ToHex(value string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return "", err
	}
	if len(raw) != 32 {
		return "", fmt.Errorf("expected 32 bytes, got %d", len(raw))
	}
	return hex.EncodeToString(raw), nil
}

func writeUAPI(b *strings.Builder, key, value string) {
	b.WriteString(key)
	b.WriteByte('=')
	b.WriteString(value)
	b.WriteByte('\n')
}

func configureInterface(interfaceName, virtualIP string, lastRoutes, desiredRoutes []string) error {
	ip, err := netip.ParseAddr(strings.TrimSuffix(virtualIP, "/32"))
	if err != nil {
		return err
	}
	if err := configureIPv4Address(interfaceName, ip.String()); err != nil {
		return err
	}
	desired := dedupeStrings(desiredRoutes)
	desiredSet := stringSet(desired)
	for _, route := range desired {
		_ = removeRoute(route, interfaceName)
		if err := addRoute(route, interfaceName); err != nil {
			return err
		}
	}
	for _, route := range dedupeStrings(lastRoutes) {
		if desiredSet[route] {
			continue
		}
		_ = removeRoute(route, interfaceName)
	}
	return nil
}

func configureIPv4Address(interfaceName, ip string) error {
	script := fmt.Sprintf(`
$ErrorActionPreference = "Stop"
$ifName = %s
$ip = %s
Set-NetIPInterface -InterfaceAlias $ifName -AddressFamily IPv4 -Dhcp Disabled | Out-Null
Get-NetIPAddress -InterfaceAlias $ifName -AddressFamily IPv4 -ErrorAction SilentlyContinue |
  Where-Object { $_.IPAddress -ne $ip } |
  Remove-NetIPAddress -Confirm:$false -ErrorAction SilentlyContinue
$existing = Get-NetIPAddress -InterfaceAlias $ifName -AddressFamily IPv4 -IPAddress $ip -ErrorAction SilentlyContinue
if (-not $existing) {
  try {
    New-NetIPAddress -InterfaceAlias $ifName -IPAddress $ip -PrefixLength 32 -Type Unicast | Out-Null
  } catch {
    $again = Get-NetIPAddress -InterfaceAlias $ifName -AddressFamily IPv4 -IPAddress $ip -ErrorAction SilentlyContinue
    if (-not $again) {
      throw
    }
  }
}
`, psQuote(interfaceName), psQuote(ip))
	return runPowerShell(script)
}

func addRoute(routeCIDR, interfaceName string) error {
	if routeCIDR == "" || interfaceName == "" {
		return nil
	}
	script := fmt.Sprintf(`
$ErrorActionPreference = "Stop"
$ifName = %s
$destination = %s
$adapter = Get-NetAdapter -Name $ifName -ErrorAction Stop
Remove-NetRoute -InterfaceAlias $ifName -DestinationPrefix $destination -Confirm:$false -ErrorAction SilentlyContinue
$existing = Get-NetRoute -InterfaceIndex $adapter.ifIndex -DestinationPrefix $destination -ErrorAction SilentlyContinue
if (-not $existing) {
  try {
    New-NetRoute -InterfaceIndex $adapter.ifIndex -DestinationPrefix $destination -NextHop "0.0.0.0" -PolicyStore ActiveStore -RouteMetric 1 | Out-Null
  } catch {
    $again = Get-NetRoute -InterfaceIndex $adapter.ifIndex -DestinationPrefix $destination -ErrorAction SilentlyContinue
    if (-not $again) {
      throw
    }
  }
}
`, psQuote(interfaceName), psQuote(routeCIDR))
	return runPowerShell(script)
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	var result []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func stringSet(values []string) map[string]bool {
	result := map[string]bool{}
	for _, value := range values {
		result[value] = true
	}
	return result
}

func removeRoute(routeCIDR, interfaceName string) error {
	if routeCIDR == "" || interfaceName == "" {
		return nil
	}
	script := fmt.Sprintf(`
$ifName = %s
$destination = %s
Remove-NetRoute -InterfaceAlias $ifName -DestinationPrefix $destination -Confirm:$false -ErrorAction SilentlyContinue
`, psQuote(interfaceName), psQuote(routeCIDR))
	return runPowerShell(script)
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(string(output))
		if strings.Contains(strings.ToLower(text), "element not found") ||
			strings.Contains(text, "找不到元素") {
			return nil
		}
		return fmt.Errorf("%s %s failed: %w: %s", name, strings.Join(args, " "), err, text)
	}
	return nil
}

func runPowerShell(script string) error {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(string(output))
		return fmt.Errorf("powershell failed: %w: %s", err, text)
	}
	return nil
}

func runPowerShellOutput(script string) (string, error) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("powershell failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func powershellBool(expression string) bool {
	output, err := runPowerShellOutput(fmt.Sprintf("if (%s) { 'true' } else { 'false' }", expression))
	return err == nil && strings.EqualFold(strings.TrimSpace(output), "true")
}

func psQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func executableDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func serviceInstalled() bool {
	m, err := mgr.Connect()
	if err != nil {
		return false
	}
	defer m.Disconnect()
	s, err := m.OpenService(serviceName)
	if err != nil {
		return false
	}
	_ = s.Close()
	return true
}

func serviceRunning() bool {
	m, err := mgr.Connect()
	if err != nil {
		return false
	}
	defer m.Disconnect()
	s, err := m.OpenService(serviceName)
	if err != nil {
		return false
	}
	defer s.Close()
	status, err := s.Query()
	return err == nil && status.State == svc.Running
}

func udpPortInUse(port int) bool {
	if port <= 0 {
		return false
	}
	conn, err := net.ListenPacket("udp4", fmt.Sprintf(":%d", port))
	if err != nil {
		return true
	}
	_ = conn.Close()
	return false
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func defaultPort(port int) int {
	if port <= 0 {
		return agent.DefaultListenPort
	}
	return port
}

func printJSON(value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func usage() {
	fmt.Print(`sdwan-service ` + version.Version + `

Usage:
  sdwan-service register --controller https://controller.englishlisten.cn --admin-token TOKEN
  sdwan-service run
  sdwan-service service
  sdwan-service sync
  sdwan-service status
  sdwan-service diagnose
  sdwan-service down
  sdwan-service install-service
  sdwan-service uninstall-service
  sdwan-service start-service
  sdwan-service stop-service
  sdwan-service version

This experimental Windows service path uses wireguard-go + Wintun directly.
It does not implement custom NAT traversal or magicsock.
`)
}
