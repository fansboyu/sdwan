package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"englishlisten/sdwan/internal/bootstrapagent"
	"englishlisten/sdwan/internal/version"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println(version.Version)
		return
	}

	fs := flag.NewFlagSet("sdwan-bootstrap-agent", flag.ExitOnError)
	configPath := fs.String("config", bootstrapagent.DefaultConfigPath, "bootstrap agent config path")
	writeConfig := fs.Bool("write-example-config", false, "write an example config file and exit")
	controllerURL := fs.String("controller", "https://controller.englishlisten.cn", "controller URL for example config")
	bootstrapToken := fs.String("bootstrap-token", "", "bootstrap API token for example config")
	interfaceName := fs.String("interface", bootstrapagent.DefaultInterface, "wireguard bootstrap interface for example config")
	if err := fs.Parse(os.Args[1:]); err != nil {
		fatal(err)
	}

	if *writeConfig {
		if *bootstrapToken == "" {
			fatal(fmt.Errorf("--bootstrap-token is required with --write-example-config"))
		}
		err := bootstrapagent.WriteExampleConfig(*configPath, bootstrapagent.Config{
			ControllerURL:         *controllerURL,
			BootstrapToken:        *bootstrapToken,
			InterfaceName:         *interfaceName,
			SyncIntervalSeconds:   int(bootstrapagent.DefaultSyncInterval.Seconds()),
			ReportIntervalSeconds: int(bootstrapagent.DefaultReportInterval.Seconds()),
			RemoveStalePeers:      false,
		})
		if err != nil {
			fatal(err)
		}
		fmt.Println(*configPath)
		return
	}

	cfg, err := bootstrapagent.LoadConfig(*configPath)
	if err != nil {
		fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err = bootstrapagent.NewRunner(cfg).Run(ctx)
	if err != nil && ctx.Err() == nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
