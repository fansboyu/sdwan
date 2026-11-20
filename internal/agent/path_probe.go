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

func AppliedPathReports(ctx context.Context, netmap Netmap) []AppliedPath {
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
					if err = ProbeOverlayPeer(ctx, assignment.PeerVirtualIP); err == nil {
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
