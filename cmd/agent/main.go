package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

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

func runRegister(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("register", flag.ExitOnError)
	configPath := fs.String("config", agent.DefaultConfigPath, "agent config path")
	controllerURL := fs.String("controller", "https://controller.englishlisten.cn", "controller URL")
	joinToken := fs.String("join-token", "", "customer join token")
	hostname := fs.String("hostname", agent.DefaultHostname(), "device hostname")
	listenPort := fs.Int("listen-port", agent.DefaultListenPort, "wireguard listen port")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *joinToken == "" {
		return fmt.Errorf("--join-token is required")
	}
	privateKey, publicKey, err := agent.GenerateKeyPair()
	if err != nil {
		return err
	}
	client := agent.NewAPIClient(*controllerURL)
	resp, err := client.Register(ctx, agent.RegisterRequest{
		JoinToken:     *joinToken,
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
		ControllerURL: *controllerURL,
		DeviceID:      resp.DeviceID,
		DeviceToken:   resp.DeviceToken,
		Hostname:      *hostname,
		OS:            agent.DefaultOS(),
		Arch:          agent.DefaultArch(),
		ClientVersion: version.Version,
		PrivateKey:    privateKey,
		PublicKey:     publicKey,
		VirtualIP:     resp.VirtualIP,
		NetmapVersion: resp.NetmapVersion,
		InterfaceName: agent.DefaultInterface,
		ListenPort:    *listenPort,
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
	outputPath := fs.String("out", "/etc/wireguard/sdwan0.conf", "wireguard config output path")
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
	_ = agent.SaveConfig(*configPath, cfg)
	return agent.ApplyWireGuardConfig(cfg.InterfaceName, *outputPath)
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

func usage() {
	fmt.Print(`sdwan-agent ` + version.Version + `

Usage:
  sdwan-agent register --controller http://localhost --join-token TOKEN
  sdwan-agent poll
  sdwan-agent netmap
  sdwan-agent render --out /tmp/sdwan0.conf
  sudo sdwan-agent up
  sdwan-agent version

Linux dependencies for network apply:
  wireguard-tools
  wg
  wg-quick
`)
}
