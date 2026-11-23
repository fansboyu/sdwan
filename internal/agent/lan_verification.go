package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"os/exec"
	"runtime"
	"time"
)

func VerifyEmployeeLAN(ctx context.Context, config, address string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("verify-lan currently requires Linux")
	}
	target, err := netip.ParseAddrPort(address)
	if err != nil || target.Port() == 0 || !target.Addr().Is4() {
		return fmt.Errorf("expected an IPv4 address and TCP port")
	}
	cfg, err := LoadConfig(config)
	if err != nil {
		return err
	}
	probeCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	route, routeErr := exec.CommandContext(probeCtx, "ip", "-j", "-4", "route", "get", target.Addr().String()).Output()
	var hops []struct {
		Dev string `json:"dev"`
	}
	iface := cfg.InterfaceName
	if iface == "" {
		iface = "sdwan0"
	}
	if routeErr != nil || json.Unmarshal(route, &hops) != nil || len(hops) != 1 || hops[0].Dev != iface {
		err = fmt.Errorf("target does not use WireGuard interface %s", iface)
	} else {
		var conn net.Conn
		conn, err = (&net.Dialer{Timeout: 2 * time.Second}).DialContext(probeCtx, "tcp4", target.String())
		if conn != nil {
			conn.Close()
		}
	}
	report := struct {
		Target    string `json:"target"`
		Reachable bool   `json:"reachable"`
	}{target.String(), err == nil}
	client := NewAPIClient(cfg.ControllerURL)
	if reportErr := client.doJSON(ctx, "POST", "/api/v1/devices/lan-verification", cfg.DeviceToken, report, nil); reportErr != nil {
		return fmt.Errorf("submit verification: %w", reportErr)
	}
	if err != nil {
		return fmt.Errorf("verification failed and was reported: %w", err)
	}
	fmt.Printf("%s reachable through %s; result reported\n", target, iface)
	return nil
}
