package bootstrapagent

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type WGPeerState struct {
	PublicKey string
	Endpoint  string
	AllowedIP string
}

type WGManager struct {
	InterfaceName string
}

func (m WGManager) EnsureInterface(ctx context.Context) error {
	if m.InterfaceName == "" {
		return errors.New("wireguard interface is required")
	}
	cmd := exec.CommandContext(ctx, "wg", "show", m.InterfaceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wireguard interface %s is not available: %w: %s", m.InterfaceName, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (m WGManager) SetPeer(ctx context.Context, peer Peer) error {
	publicKey := strings.TrimSpace(peer.PublicKey)
	virtualIP := strings.TrimSpace(peer.VirtualIP)
	if publicKey == "" || virtualIP == "" {
		return nil
	}
	allowedIPs := peer.AllowedIPs
	if len(allowedIPs) == 0 {
		allowedIPs = []string{virtualIP + "/32"}
	}
	cmd := exec.CommandContext(ctx, "wg", "set", m.InterfaceName,
		"peer", publicKey,
		"allowed-ips", strings.Join(allowedIPs, ","),
		"persistent-keepalive", "25",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("set bootstrap peer %s: %w: %s", publicKey, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (m WGManager) RemovePeer(ctx context.Context, publicKey string) error {
	publicKey = strings.TrimSpace(publicKey)
	if publicKey == "" {
		return nil
	}
	cmd := exec.CommandContext(ctx, "wg", "set", m.InterfaceName, "peer", publicKey, "remove")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("remove bootstrap peer %s: %w: %s", publicKey, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (m WGManager) DumpPeers(ctx context.Context) (map[string]WGPeerState, error) {
	cmd := exec.CommandContext(ctx, "wg", "show", m.InterfaceName, "dump")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("dump bootstrap peers: %w: %s", err, strings.TrimSpace(string(output)))
	}
	result := map[string]WGPeerState{}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		if lineNumber == 1 {
			continue
		}
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) < 4 {
			continue
		}
		publicKey := strings.TrimSpace(fields[0])
		if publicKey == "" {
			continue
		}
		endpoint := strings.TrimSpace(fields[2])
		if endpoint == "(none)" {
			endpoint = ""
		}
		result[publicKey] = WGPeerState{
			PublicKey: publicKey,
			Endpoint:  endpoint,
			AllowedIP: strings.TrimSpace(fields[3]),
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
