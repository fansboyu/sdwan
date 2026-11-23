package agent

import (
	"context"
	"fmt"
	"net/netip"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

const overlayProbeTimeout = 2 * time.Second

func AppliedPathReports(ctx context.Context, netmap Netmap, iface string) []AppliedPath {
	assignments := netmap.PathAssignments
	result := make([]AppliedPath, len(assignments))
	jobs := make(chan int)
	workers := 8
	if len(assignments) < workers {
		workers = len(assignments)
	}
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				assignment := assignments[index]
				report := AppliedPath{ClientDeviceID: assignment.ClientDeviceID, Generation: assignment.Generation, VerificationStatus: "failed"}
				path, err := EffectivePathForAssignment(netmap, assignment)
				if err == nil {
					report.EffectivePath = path
					err = verifyLinuxPath(ctx, netmap, assignment, iface)
					if err == nil {
						err = ProbeOverlayPeer(ctx, assignment.PeerVirtualIP)
					}
					if err == nil {
						report.VerificationStatus = "verified"
					} else {
						report.VerificationError = err.Error()
					}
				} else {
					report.VerificationError = err.Error()
				}
				result[index] = report
			}
		}()
	}
	for i := range assignments {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return result
}

func verifyLinuxPath(ctx context.Context, netmap Netmap, assignment PathAssignment, iface string) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	if iface == "" {
		iface = "sdwan0"
	}
	target, err := netip.ParseAddr(assignment.PeerVirtualIP)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, overlayProbeTimeout)
	defer cancel()
	route, err := exec.CommandContext(ctx, "ip", "-4", "route", "get", target.String()).Output()
	if err != nil || !strings.Contains(" "+strings.Join(strings.Fields(string(route)), " ")+" ", " dev "+iface+" ") {
		return fmt.Errorf("overlay route to %s is not installed on %s", target, iface)
	}
	allowed, err := exec.CommandContext(ctx, "wg", "show", iface, "allowed-ips").Output()
	if err != nil {
		return fmt.Errorf("cannot inspect WireGuard peers on %s: %w", iface, err)
	}
	expected := ""
	best := -1
	for _, peer := range netmap.Peers {
		for _, value := range peer.AllowedIPs {
			if prefix, err := netip.ParsePrefix(value); err == nil && prefix.Contains(target) && prefix.Bits() > best {
				best, expected = prefix.Bits(), peer.PublicKey
			}
		}
	}
	actual := ""
	best = -1
	for _, line := range strings.Split(string(allowed), "\n") {
		fields := strings.Fields(strings.ReplaceAll(line, ",", " "))
		if len(fields) < 2 {
			continue
		}
		for _, value := range fields[1:] {
			if prefix, err := netip.ParsePrefix(value); err == nil && prefix.Contains(target) && prefix.Bits() > best {
				best, actual = prefix.Bits(), fields[0]
			}
		}
	}
	if expected == "" || actual != expected {
		return fmt.Errorf("WireGuard peer owning %s does not match the current assignment", target)
	}
	return nil
}

func EffectivePathForAssignment(netmap Netmap, assignment PathAssignment) (string, error) {
	target, err := netip.ParseAddr(strings.TrimSpace(assignment.PeerVirtualIP))
	if err != nil || !target.Is4() {
		return "", fmt.Errorf("invalid overlay peer IP %q", assignment.PeerVirtualIP)
	}
	for _, peer := range netmap.Peers {
		for _, value := range peer.AllowedIPs {
			prefix, parseErr := netip.ParsePrefix(strings.TrimSpace(value))
			if parseErr == nil && prefix.Contains(target) {
				if peer.PathRole == "direct" || peer.PathRole == "relay" {
					return peer.PathRole, nil
				}
			}
		}
	}
	return "", fmt.Errorf("no active peer route for overlay IP %s", target)
}

func ProbeOverlayPeer(parent context.Context, address string) error {
	ip, err := netip.ParseAddr(strings.TrimSpace(address))
	if err != nil || !ip.Is4() {
		return fmt.Errorf("invalid overlay peer IP %q", address)
	}
	ctx, cancel := context.WithTimeout(parent, overlayProbeTimeout)
	defer cancel()
	var command *exec.Cmd
	if runtime.GOOS == "windows" {
		command = exec.CommandContext(ctx, "ping", "-n", "1", "-w", "1000", ip.String())
	} else {
		command = exec.CommandContext(ctx, "ping", "-n", "-c", "1", "-W", "1", ip.String())
	}
	if output, err := command.CombinedOutput(); err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("overlay ping to %s failed: %s", ip, message)
	}
	return nil
}
