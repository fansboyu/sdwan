package agent

const (
	DefaultOverlayCIDR         = "100.64.0.0/10"
	SubnetGatewayRuleComment   = "sdwan-subnet-gateway"
	subnetGatewaySysctlPath    = "/etc/sysctl.d/99-sdwan-subnet-gateway.conf"
	subnetGatewaySysctlContent = "net.ipv4.ip_forward=1\n"
)

type SubnetGatewayOptions struct {
	LANCIDR      string `json:"lan_cidr"`
	OutInterface string `json:"out_interface"`
	WGInterface  string `json:"wg_interface"`
	OverlayCIDR  string `json:"overlay_cidr"`
}

type SubnetGatewayStatus struct {
	Supported           bool   `json:"supported"`
	Enabled             bool   `json:"enabled"`
	IPForward           bool   `json:"ip_forward"`
	PersistentSysctl    bool   `json:"persistent_sysctl"`
	NATRule             bool   `json:"nat_rule"`
	ForwardToLANRule    bool   `json:"forward_to_lan_rule"`
	ForwardFromLANRule  bool   `json:"forward_from_lan_rule"`
	WireGuardInterface  bool   `json:"wireguard_interface"`
	OutInterfacePresent bool   `json:"out_interface_present"`
	LANCIDR             string `json:"lan_cidr"`
	OutInterface        string `json:"out_interface"`
	WGInterface         string `json:"wg_interface"`
	OverlayCIDR         string `json:"overlay_cidr"`
	Error               string `json:"error,omitempty"`
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
