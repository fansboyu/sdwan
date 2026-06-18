package agent

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"englishlisten/sdwan/internal/version"
)

var (
	DefaultConfigPath          = defaultConfigPath()
	DefaultWireGuardConfigPath = defaultWireGuardConfigPath()
)

const (
	DefaultInterface  = "sdwan0"
	DefaultListenPort = 41641
)

type Config struct {
	ControllerURL   string                 `json:"controller_url"`
	DeviceID        string                 `json:"device_id"`
	DeviceToken     string                 `json:"device_token"`
	Hostname        string                 `json:"hostname"`
	OS              string                 `json:"os"`
	Arch            string                 `json:"arch"`
	OSVersion       string                 `json:"os_version"`
	ClientVersion   string                 `json:"client_version"`
	PrivateKey      string                 `json:"private_key"`
	PublicKey       string                 `json:"public_key"`
	VirtualIP       string                 `json:"virtual_ip"`
	NetmapVersion   int64                  `json:"netmap_version"`
	InterfaceName   string                 `json:"interface_name"`
	ListenPort      int                    `json:"listen_port"`
	LastConfigPath  string                 `json:"last_config_path"`
	LastRoutes      []string               `json:"last_routes,omitempty"`
	AdvertiseRoutes []string               `json:"advertise_routes,omitempty"`
	SubnetGateways  []SubnetGatewayOptions `json:"subnet_gateways,omitempty"`
	AppliedPaths    []AppliedPath          `json:"applied_paths,omitempty"`
}

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	cfg.withDefaults()
	return cfg, nil
}

func SaveConfig(path string, cfg Config) error {
	cfg.withDefaults()
	if cfg.PrivateKey == "" || cfg.DeviceToken == "" {
		return errors.New("agent config is missing private key or device token")
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func (c *Config) withDefaults() {
	if c.InterfaceName == "" {
		c.InterfaceName = DefaultInterface
	}
	if c.ListenPort == 0 {
		c.ListenPort = DefaultListenPort
	}
	if c.ClientVersion == "" {
		c.ClientVersion = version.Version
	}
}

func defaultConfigPath() string {
	if runtime.GOOS == "windows" {
		if programData := os.Getenv("ProgramData"); programData != "" {
			return filepath.Join(programData, "sdwan", "agent.json")
		}
		return filepath.Join(`C:\ProgramData`, "sdwan", "agent.json")
	}
	return "/etc/sdwan/agent.json"
}

func defaultWireGuardConfigPath() string {
	if runtime.GOOS == "windows" {
		if programData := os.Getenv("ProgramData"); programData != "" {
			return filepath.Join(programData, "sdwan", DefaultInterface+".conf")
		}
		return filepath.Join(`C:\ProgramData`, "sdwan", DefaultInterface+".conf")
	}
	return "/etc/wireguard/" + DefaultInterface + ".conf"
}
