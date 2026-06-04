package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"englishlisten/sdwan/internal/relayagent"
	"englishlisten/sdwan/internal/version"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println(version.Version)
		return
	}

	fs := flag.NewFlagSet("sdwan-relay-agent", flag.ExitOnError)
	configPath := fs.String("config", relayagent.DefaultConfigPath, "relay agent config path")
	writeConfig := fs.Bool("write-example-config", false, "write an example config file and exit")
	controllerURL := fs.String("controller", "https://controller.englishlisten.cn", "controller URL for example config")
	relayToken := fs.String("relay-token", "", "relay API token for example config")
	interfaceName := fs.String("interface", relayagent.DefaultInterface, "wireguard relay interface for example config")
	if err := fs.Parse(os.Args[1:]); err != nil {
		fatal(err)
	}

	if *writeConfig {
		if *relayToken == "" {
			fatal(fmt.Errorf("--relay-token is required with --write-example-config"))
		}
		err := relayagent.WriteExampleConfig(*configPath, relayagent.Config{
			ControllerURL:       *controllerURL,
			RelayToken:          *relayToken,
			InterfaceName:       *interfaceName,
			SyncIntervalSeconds: int(relayagent.DefaultSyncInterval.Seconds()),
			RemoveStalePeers:    false,
		})
		if err != nil {
			fatal(err)
		}
		fmt.Println(*configPath)
		return
	}

	cfg, err := relayagent.LoadConfig(*configPath)
	if err != nil {
		fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err = relayagent.NewRunner(cfg).Run(ctx)
	if err != nil && ctx.Err() == nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
