package bootstrapagent

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const (
	DefaultConfigPath     = "/etc/sdwan/bootstrap-agent.json"
	DefaultInterface      = "sdwan-bootstrap"
	DefaultSyncInterval   = 5 * time.Second
	DefaultReportInterval = 2 * time.Second
)

type Config struct {
	ControllerURL         string `json:"controller_url"`
	BootstrapToken        string `json:"bootstrap_token"`
	InterfaceName         string `json:"interface_name"`
	SyncIntervalSeconds   int    `json:"sync_interval_seconds"`
	ReportIntervalSeconds int    `json:"report_interval_seconds"`
	RemoveStalePeers      bool   `json:"remove_stale_peers"`
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
	if cfg.ControllerURL == "" || cfg.BootstrapToken == "" {
		return Config{}, errors.New("controller_url and bootstrap_token are required")
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
	if c.ReportIntervalSeconds <= 0 {
		c.ReportIntervalSeconds = int(DefaultReportInterval.Seconds())
	}
}

func (c Config) SyncInterval() time.Duration {
	if c.SyncIntervalSeconds <= 0 {
		return DefaultSyncInterval
	}
	return time.Duration(c.SyncIntervalSeconds) * time.Second
}

func (c Config) ReportInterval() time.Duration {
	if c.ReportIntervalSeconds <= 0 {
		return DefaultReportInterval
	}
	return time.Duration(c.ReportIntervalSeconds) * time.Second
}
