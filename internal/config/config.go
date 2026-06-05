package config

import (
	"log/slog"
	"os"
	"strconv"
	"time"

	"englishlisten/sdwan/internal/version"
)

type Config struct {
	DatabaseURL               string
	ListenAddr                string
	LogLevel                  slog.Level
	Version                   string
	ControllerURL             string
	DefaultMaxDevices         int32
	DefaultPollInterval       time.Duration
	MinSupportedClientVersion string
	LatestClientVersion       string
	BootstrapPublicKey        string
	BootstrapEndpoint         string
	BootstrapAllowedIP        string
	BootstrapReportToken      string
	ResendAPIKey              string
	ResendFrom                string
	EmailCodeTTL              time.Duration
	EmailCodeCooldown         time.Duration
	EmailCodeMaxAttempts      int32
}

func Load() Config {
	return Config{
		DatabaseURL:               getenv("DATABASE_URL", "postgres://sdwan:sdwan@localhost:5432/sdwan?sslmode=disable"),
		ListenAddr:                getenv("LISTEN_ADDR", ":8080"),
		LogLevel:                  parseLogLevel(getenv("LOG_LEVEL", "info")),
		Version:                   version.Version,
		ControllerURL:             getenv("CONTROLLER_URL", "https://controller.englishlisten.cn"),
		DefaultMaxDevices:         int32(getenvInt("DEFAULT_MAX_DEVICES", 254)),
		DefaultPollInterval:       time.Duration(getenvInt("POLL_INTERVAL_SECONDS", 15)) * time.Second,
		MinSupportedClientVersion: getenv("MIN_SUPPORTED_CLIENT_VERSION", version.Version),
		LatestClientVersion:       getenv("LATEST_CLIENT_VERSION", version.Version),
		BootstrapPublicKey:        getenv("BOOTSTRAP_WG_PUBLIC_KEY", ""),
		BootstrapEndpoint:         getenv("BOOTSTRAP_WG_ENDPOINT", ""),
		BootstrapAllowedIP:        getenv("BOOTSTRAP_WG_ALLOWED_IP", "100.254.254.254/32"),
		BootstrapReportToken:      getenv("BOOTSTRAP_REPORT_TOKEN", ""),
		ResendAPIKey:              getenv("RESEND_API_KEY", ""),
		ResendFrom:                getenv("RESEND_FROM", getenv("MAIL_FROM", "")),
		EmailCodeTTL:              time.Duration(getenvInt("EMAIL_CODE_TTL_SECONDS", 600)) * time.Second,
		EmailCodeCooldown:         time.Duration(getenvInt("EMAIL_CODE_COOLDOWN_SECONDS", 60)) * time.Second,
		EmailCodeMaxAttempts:      int32(getenvInt("EMAIL_CODE_MAX_ATTEMPTS", 5)),
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseLogLevel(value string) slog.Level {
	switch value {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
