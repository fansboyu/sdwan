//go:build linux

package agent

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func ApplyWireGuardConfigSmart(cfg Config, configPath string, netmap Netmap) (ApplyResult, error) {
	interfaceName := cfg.InterfaceName
	if interfaceName == "" {
		interfaceName = DefaultInterface
	}
	desiredRoutes := DesiredRoutes(netmap)
	if !linuxInterfaceExists(interfaceName) {
		if err := ApplyWireGuardConfig(interfaceName, configPath); err != nil {
			return ApplyResult{}, err
		}
		return ApplyResult{LastRoutes: desiredRoutes}, nil
	}
	if err := syncWireGuardConfig(interfaceName, configPath); err != nil {
		if fallbackErr := ApplyWireGuardConfig(interfaceName, configPath); fallbackErr != nil {
			return ApplyResult{}, fmt.Errorf("wg syncconf failed: %w; fallback failed: %w", err, fallbackErr)
		}
		return ApplyResult{LastRoutes: desiredRoutes}, nil
	}
	if err := syncLinuxRoutes(interfaceName, cfg.LastRoutes, desiredRoutes); err != nil {
		return ApplyResult{}, err
	}
	return ApplyResult{LastRoutes: desiredRoutes}, nil
}

func syncWireGuardConfig(interfaceName, configPath string) error {
	strip := exec.Command("wg-quick", "strip", configPath)
	stripped, err := strip.Output()
	if err != nil {
		return fmt.Errorf("wg-quick strip failed: %w", err)
	}
	cmd := exec.Command("wg", "syncconf", interfaceName, "/dev/stdin")
	cmd.Stdin = bytes.NewReader(stripped)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wg syncconf failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func syncLinuxRoutes(interfaceName string, lastRoutes, desiredRoutes []string) error {
	last := stringSet(normalizeRoutes(lastRoutes))
	desired := normalizeRoutes(desiredRoutes)
	desiredSet := stringSet(desired)
	for _, route := range desired {
		if err := ipRouteReplace(route, interfaceName); err != nil {
			return err
		}
	}
	for route := range last {
		if desiredSet[route] {
			continue
		}
		if err := ipRouteDelete(route, interfaceName); err != nil && !isRouteMissing(err) {
			return err
		}
	}
	return nil
}

func stringSet(values []string) map[string]bool {
	result := map[string]bool{}
	for _, value := range values {
		result[value] = true
	}
	return result
}

func linuxInterfaceExists(interfaceName string) bool {
	if interfaceName == "" {
		return false
	}
	cmd := exec.Command("ip", "link", "show", "dev", interfaceName)
	return cmd.Run() == nil
}

func ipRouteReplace(route, interfaceName string) error {
	output, err := exec.Command("ip", "route", "replace", route, "dev", interfaceName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ip route replace %s dev %s: %w: %s", route, interfaceName, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func ipRouteDelete(route, interfaceName string) error {
	output, err := exec.Command("ip", "route", "delete", route, "dev", interfaceName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ip route delete %s dev %s: %w: %s", route, interfaceName, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func isRouteMissing(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "no such process") ||
		strings.Contains(text, "not found") ||
		strings.Contains(text, "cannot find")
}
