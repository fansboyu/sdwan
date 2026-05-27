package agent

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

func GenerateKeyPair() (privateKey string, publicKey string, err error) {
	private, err := exec.Command("wg", "genkey").Output()
	if err != nil {
		return "", "", fmt.Errorf("generate wireguard private key with wg: %w", err)
	}
	cmd := exec.Command("wg", "pubkey")
	cmd.Stdin = bytes.NewReader(private)
	public, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("derive wireguard public key with wg: %w", err)
	}
	return strings.TrimSpace(string(private)), strings.TrimSpace(string(public)), nil
}

type WGRenderInput struct {
	PrivateKey    string
	VirtualIP     string
	ListenPort    int
	InterfaceName string
	Netmap        Netmap
}

func RenderWireGuardConfig(input WGRenderInput) (string, error) {
	if input.PrivateKey == "" {
		return "", errors.New("private key is required")
	}
	if input.VirtualIP == "" {
		return "", errors.New("virtual ip is required")
	}
	if input.ListenPort == 0 {
		input.ListenPort = DefaultListenPort
	}

	var b strings.Builder
	b.WriteString("[Interface]\n")
	b.WriteString("PrivateKey = " + input.PrivateKey + "\n")
	b.WriteString("Address = " + hostOnly(input.VirtualIP) + "/32\n")
	b.WriteString(fmt.Sprintf("ListenPort = %d\n", input.ListenPort))
	b.WriteString("\n")

	sort.Slice(input.Netmap.Peers, func(i, j int) bool {
		return input.Netmap.Peers[i].VirtualIP < input.Netmap.Peers[j].VirtualIP
	})
	for _, peer := range input.Netmap.Peers {
		if peer.PublicKey == "" {
			continue
		}
		b.WriteString("[Peer]\n")
		b.WriteString("# " + peer.Hostname + " " + peer.VirtualIP + "\n")
		b.WriteString("PublicKey = " + peer.PublicKey + "\n")
		if len(peer.AllowedIPs) == 0 {
			b.WriteString("AllowedIPs = " + hostOnly(peer.VirtualIP) + "/32\n")
		} else {
			b.WriteString("AllowedIPs = " + strings.Join(peer.AllowedIPs, ", ") + "\n")
		}
		if len(peer.Endpoints) > 0 {
			b.WriteString("Endpoint = " + peer.Endpoints[0] + "\n")
		}
		if peer.PersistentKeepalive > 0 {
			b.WriteString(fmt.Sprintf("PersistentKeepalive = %d\n", peer.PersistentKeepalive))
		}
		b.WriteString("\n")
	}
	return b.String(), nil
}

func WriteWireGuardConfig(path string, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o600)
}

func ApplyWireGuardConfig(interfaceName, configPath string) error {
	if runtime.GOOS != "linux" {
		return errors.New("wg-quick apply is supported only on linux")
	}
	if interfaceName == "" {
		interfaceName = DefaultInterface
	}
	_ = exec.Command("wg-quick", "down", interfaceName).Run()
	cmd := exec.Command("wg-quick", "up", configPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wg-quick up failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func hostOnly(value string) string {
	value = strings.TrimSpace(value)
	if host, _, err := net.SplitHostPort(value); err == nil {
		return host
	}
	return strings.TrimSuffix(value, "/32")
}
