package agent

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"golang.org/x/crypto/curve25519"
)

func GenerateKeyPair() (privateKey string, publicKey string, err error) {
	var private [32]byte
	if _, err := rand.Read(private[:]); err != nil {
		return "", "", fmt.Errorf("generate wireguard private key: %w", err)
	}
	private[0] &= 248
	private[31] &= 127
	private[31] |= 64

	public, err := curve25519.X25519(private[:], curve25519.Basepoint)
	if err != nil {
		return "", "", fmt.Errorf("derive wireguard public key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(private[:]), base64.StdEncoding.EncodeToString(public), nil
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
		writePeer(&b, peer)
	}
	if input.Netmap.BootstrapPeer != nil {
		writePeer(&b, *input.Netmap.BootstrapPeer)
	}
	return b.String(), nil
}

func writePeer(b *strings.Builder, peer NetmapPeer) {
	if peer.PublicKey == "" {
		return
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

func WriteWireGuardConfig(path string, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o600)
}

func ApplyWireGuardConfig(interfaceName, configPath string) error {
	if interfaceName == "" {
		interfaceName = DefaultInterface
	}
	switch runtime.GOOS {
	case "linux":
		_ = exec.Command("wg-quick", "down", interfaceName).Run()
		cmd := exec.Command("wg-quick", "up", configPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("wg-quick up failed: %w: %s", err, strings.TrimSpace(string(output)))
		}
		return nil
	case "windows":
		return installWindowsTunnel(interfaceName, configPath)
	default:
		return fmt.Errorf("wireguard apply is not supported on %s", runtime.GOOS)
	}
}

func DownWireGuardTunnel(interfaceName string) error {
	if interfaceName == "" {
		interfaceName = DefaultInterface
	}
	switch runtime.GOOS {
	case "linux":
		output, err := exec.Command("wg-quick", "down", interfaceName).CombinedOutput()
		if err != nil {
			return fmt.Errorf("wg-quick down failed: %w: %s", err, strings.TrimSpace(string(output)))
		}
		return nil
	case "windows":
		wireguardPath, err := wireGuardToolPath("wireguard")
		if err != nil {
			return err
		}
		output, err := exec.Command(wireguardPath, "/uninstalltunnelservice", interfaceName).CombinedOutput()
		if err != nil && !strings.Contains(strings.ToLower(string(output)), "does not exist") {
			return fmt.Errorf("wireguard uninstall tunnel failed: %w: %s", err, strings.TrimSpace(string(output)))
		}
		return nil
	default:
		return fmt.Errorf("wireguard down is not supported on %s", runtime.GOOS)
	}
}

func installWindowsTunnel(interfaceName, configPath string) error {
	wireguardPath, err := wireGuardToolPath("wireguard")
	if err != nil {
		return err
	}
	_ = exec.Command(wireguardPath, "/uninstalltunnelservice", interfaceName).Run()
	cmd := exec.Command(wireguardPath, "/installtunnelservice", configPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wireguard install tunnel failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func wireGuardToolPath(name string) (string, error) {
	exe := name
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(exe), ".exe") {
		exe += ".exe"
	}
	if path, err := exec.LookPath(exe); err == nil {
		return path, nil
	}
	if runtime.GOOS == "windows" {
		for _, base := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")} {
			if base == "" {
				continue
			}
			path := filepath.Join(base, "WireGuard", exe)
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	}
	return "", fmt.Errorf("%s not found; install WireGuard and make sure %s is available", exe, exe)
}

func hostOnly(value string) string {
	value = strings.TrimSpace(value)
	if host, _, err := net.SplitHostPort(value); err == nil {
		return host
	}
	return strings.TrimSuffix(value, "/32")
}
