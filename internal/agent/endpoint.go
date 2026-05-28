package agent

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

type EndpointDetector struct {
	Timeout time.Duration
}

func (d EndpointDetector) Detect(ctx context.Context, cfg Config) []EndpointReport {
	_ = ctx
	return dedupeEndpoints(d.detectLAN(cfg.ListenPort))
}

func (d EndpointDetector) detectLAN(listenPort int) []EndpointReport {
	if listenPort == 0 {
		listenPort = DefaultListenPort
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var endpoints []EndpointReport
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if ignoredInterface(iface.Name) {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip := ipFromAddr(addr)
			if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			if ip4 := ip.To4(); ip4 != nil {
				if !isPrivateIPv4(ip4) {
					continue
				}
				endpoints = append(endpoints, EndpointReport{
					Type:    "lan",
					Address: fmt.Sprintf("%s:%d", ip4.String(), listenPort),
					Source:  iface.Name,
				})
				continue
			}
			if ip.To16() != nil && !ip.IsPrivate() {
				endpoints = append(endpoints, EndpointReport{
					Type:    "ipv6",
					Address: fmt.Sprintf("[%s]:%d", ip.String(), listenPort),
					Source:  iface.Name,
				})
			}
		}
	}
	return endpoints
}

func ignoredInterface(name string) bool {
	name = strings.ToLower(name)
	return name == "lo" ||
		strings.HasPrefix(name, "docker") ||
		strings.HasPrefix(name, "br-") ||
		strings.HasPrefix(name, "veth") ||
		strings.HasPrefix(name, "virbr")
}

func ipFromAddr(addr net.Addr) net.IP {
	switch value := addr.(type) {
	case *net.IPNet:
		return value.IP
	case *net.IPAddr:
		return value.IP
	default:
		return nil
	}
}

func isPrivateIPv4(ip net.IP) bool {
	return ip[0] == 10 ||
		(ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31) ||
		(ip[0] == 192 && ip[1] == 168)
}

func dedupeEndpoints(endpoints []EndpointReport) []EndpointReport {
	seen := map[string]bool{}
	result := make([]EndpointReport, 0, len(endpoints))
	for _, endpoint := range endpoints {
		key := endpoint.Type + "|" + endpoint.Address
		if endpoint.Address == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, endpoint)
	}
	return result
}
