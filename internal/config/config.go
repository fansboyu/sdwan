package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
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
	STUNServers               []string
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
		MinSupportedClientVersion: getenv("MIN_SUPPORTED_CLIENT_VERSION", "v1.1.3"),
		LatestClientVersion:       getenv("LATEST_CLIENT_VERSION", "v1.1.3"),
		STUNServers:               getenvList("STUN_SERVERS", []string{"stun:controller.englishlisten.cn:3478"}),
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

func getenvList(key string, fallback []string) []string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	if len(result) == 0 {
		return fallback
	}
	return result
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
