package agent

import (
	"os"
	"runtime"
)

func DefaultHostname() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return "linux-agent"
	}
	return hostname
}

func DefaultOS() string {
	return runtime.GOOS
}

func DefaultArch() string {
	return runtime.GOARCH
}
