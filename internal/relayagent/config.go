package relayagent

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const (
	DefaultConfigPath   = "/etc/sdwan/relay-agent.json"
	DefaultInterface    = "sdwan-relay"
	DefaultSyncInterval = 5 * time.Second
)

type Config struct {
	ControllerURL       string `json:"controller_url"`
	RelayToken          string `json:"relay_token"`
	InterfaceName       string `json:"interface_name"`
	SyncIntervalSeconds int    `json:"sync_interval_seconds"`
	RemoveStalePeers    bool   `json:"remove_stale_peers"`
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
	if cfg.ControllerURL == "" || cfg.RelayToken == "" {
		return Config{}, errors.New("controller_url and relay_token are required")
	}
	return cfg, nil
}

func WriteExampleConfig(path string, cfg Config) error {
	cfg.withDefaults()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func (c *Config) withDefaults() {
	if c.InterfaceName == "" {
		c.InterfaceName = DefaultInterface
	}
	if c.SyncIntervalSeconds <= 0 {
		c.SyncIntervalSeconds = int(DefaultSyncInterval.Seconds())
	}
}

func (c Config) SyncInterval() time.Duration {
	if c.SyncIntervalSeconds <= 0 {
		return DefaultSyncInterval
	}
	return time.Duration(c.SyncIntervalSeconds) * time.Second
}
