package agent

import (
	"net/netip"
	"sort"
	"strings"
)

type ApplyResult struct {
	LastRoutes []string
}

func DesiredRoutes(netmap Netmap) []string {
	var routes []string
	for _, peer := range netmap.Peers {
		routes = append(routes, peerAllowedIPs(peer)...)
	}
	if netmap.BootstrapPeer != nil {
		routes = append(routes, peerAllowedIPs(*netmap.BootstrapPeer)...)
	}
	return normalizeRoutes(routes)
}

func peerAllowedIPs(peer NetmapPeer) []string {
	if peer.PathRole != "" && !peer.PathActive {
		return nil
	}
	if len(peer.AllowedIPs) > 0 {
		return peer.AllowedIPs
	}
	if peer.VirtualIP == "" {
		return nil
	}
	return []string{hostOnly(peer.VirtualIP) + "/32"}
}

func normalizeRoutes(values []string) []string {
	seen := map[string]bool{}
	var result []string
	for _, value := range values {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(value))
		if err != nil {
			continue
		}
		prefix = prefix.Masked()
		if !prefix.Addr().Is4() || prefix.Bits() == 0 {
			continue
		}
		route := prefix.String()
		if seen[route] {
			continue
		}
		seen[route] = true
		result = append(result, route)
	}
	sort.Strings(result)
	return result
}
