package agent

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/pion/stun"
)

type EndpointDetector struct {
	Timeout time.Duration
}

func (d EndpointDetector) Detect(ctx context.Context, cfg Config) []EndpointReport {
	var endpoints []EndpointReport
	endpoints = append(endpoints, d.detectLAN(cfg.ListenPort)...)
	endpoints = append(endpoints, d.detectSTUN(ctx, cfg.STUNServers)...)
	return dedupeEndpoints(endpoints)
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

func (d EndpointDetector) detectSTUN(ctx context.Context, servers []string) []EndpointReport {
	timeout := d.Timeout
	if timeout == 0 {
		timeout = 3 * time.Second
	}
	var endpoints []EndpointReport
	for _, server := range servers {
		addr := normalizeSTUNServer(server)
		if addr == "" {
			continue
		}
		start := time.Now()
		mapped, err := querySTUN(ctx, addr, timeout)
		if err != nil {
			continue
		}
		rtt := int32(time.Since(start).Milliseconds())
		endpoints = append(endpoints, EndpointReport{
			Type:    "stun",
			Address: mapped,
			Source:  addr,
			RttMs:   &rtt,
		})
	}
	return endpoints
}

func querySTUN(ctx context.Context, server string, timeout time.Duration) (string, error) {
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "udp", server)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	client, err := stun.NewClient(conn)
	if err != nil {
		return "", err
	}
	defer client.Close()

	message := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	var xorAddr stun.XORMappedAddress
	var stunErr error
	if err := client.Do(message, func(event stun.Event) {
		if event.Error != nil {
			stunErr = event.Error
			return
		}
		stunErr = xorAddr.GetFrom(event.Message)
	}); err != nil {
		return "", err
	}
	if stunErr != nil {
		return "", stunErr
	}
	return net.JoinHostPort(xorAddr.IP.String(), fmt.Sprintf("%d", xorAddr.Port)), nil
}

func normalizeSTUNServer(server string) string {
	server = strings.TrimSpace(server)
	server = strings.TrimPrefix(server, "stun:")
	server = strings.TrimPrefix(server, "stuns:")
	if server == "" {
		return ""
	}
	if _, _, err := net.SplitHostPort(server); err == nil {
		return server
	}
	return net.JoinHostPort(server, "3478")
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
