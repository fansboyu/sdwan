package agent

import (
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultOverlayCIDR         = "100.64.0.0/10"
	SubnetGatewayRuleComment   = "sdwan-subnet-gateway"
	subnetGatewaySysctlPath    = "/etc/sysctl.d/99-sdwan-subnet-gateway.conf"
	subnetGatewaySysctlContent = "net.ipv4.ip_forward=1\n"
)

type SubnetGatewayOptions struct {
	LANCIDR        string   `json:"lan_cidr"`
	OutInterface   string   `json:"out_interface"`
	WGInterface    string   `json:"wg_interface"`
	OverlayCIDR    string   `json:"overlay_cidr"`
	LANTarget      string   `json:"lan_target,omitempty"`
	ServiceTargets []string `json:"service_targets,omitempty"`
}

type ServiceProbe struct {
	Target    string `json:"target"`
	Reachable bool   `json:"reachable"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
	Error     string `json:"error,omitempty"`
}

type SubnetGatewayStatus struct {
	Supported                bool           `json:"supported"`
	Enabled                  bool           `json:"enabled"`
	IPForward                bool           `json:"ip_forward"`
	PersistentSysctl         bool           `json:"persistent_sysctl"`
	NATRule                  bool           `json:"nat_rule"`
	ForwardToLANRule         bool           `json:"forward_to_lan_rule"`
	ForwardFromLANRule       bool           `json:"forward_from_lan_rule"`
	WireGuardInterface       bool           `json:"wireguard_interface"`
	OutInterfacePresent      bool           `json:"out_interface_present"`
	RouteMatchesOutInterface bool           `json:"route_matches_out_interface"`
	LANTargetReachable       bool           `json:"lan_target_reachable"`
	LANCIDR                  string         `json:"lan_cidr"`
	OutInterface             string         `json:"out_interface"`
	SuggestedOutInterface    string         `json:"suggested_out_interface,omitempty"`
	RouteInterface           string         `json:"route_interface,omitempty"`
	LANTarget                string         `json:"lan_target,omitempty"`
	WGInterface              string         `json:"wg_interface"`
	OverlayCIDR              string         `json:"overlay_cidr"`
	Warnings                 []string       `json:"warnings,omitempty"`
	ServiceProbes            []ServiceProbe `json:"service_probes,omitempty"`
	ServiceTargets           []string       `json:"service_targets,omitempty"`
	Error                    string         `json:"error,omitempty"`
}

func probeTCPServices(cidr string, targets []string) []ServiceProbe {
	if len(targets) > 8 {
		targets = targets[:8]
	}
	prefix, err := netip.ParsePrefix(strings.TrimSpace(cidr))
	results := make([]ServiceProbe, 0, len(targets))
	for _, raw := range targets {
		target := strings.TrimSpace(raw)
		result := ServiceProbe{Target: target}
		if err != nil {
			result.Error = "invalid route CIDR"
			results = append(results, result)
			continue
		}
		host, portText, splitErr := net.SplitHostPort(target)
		ip, ipErr := netip.ParseAddr(host)
		port, portErr := strconv.Atoi(portText)
		if splitErr != nil || ipErr != nil || portErr != nil || port < 1 || port > 65535 || !prefix.Contains(ip) {
			result.Error = "target must be an IP:port inside the advertised subnet"
			results = append(results, result)
			continue
		}
		start := time.Now()
		conn, dialErr := net.DialTimeout("tcp", target, 700*time.Millisecond)
		result.LatencyMs = time.Since(start).Milliseconds()
		if dialErr != nil {
			result.Error = fmt.Sprintf("TCP connect failed: %v", dialErr)
		} else {
			result.Reachable = true
			_ = conn.Close()
		}
		results = append(results, result)
	}
	return results
}

func (o SubnetGatewayOptions) withDefaults() SubnetGatewayOptions {
	if o.WGInterface == "" {
		o.WGInterface = DefaultInterface
	}
	if o.OverlayCIDR == "" {
		o.OverlayCIDR = DefaultOverlayCIDR
	}
	return o
}
