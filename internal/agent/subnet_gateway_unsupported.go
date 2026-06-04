//go:build !linux

package agent

import "errors"

var errSubnetGatewayUnsupported = errors.New("subnet gateway is only supported on Linux with iptables")

func EnableSubnetGateway(_ SubnetGatewayOptions) (SubnetGatewayStatus, error) {
	return SubnetGatewayStatus{Supported: false, Error: errSubnetGatewayUnsupported.Error()}, errSubnetGatewayUnsupported
}

func DisableSubnetGateway(_ SubnetGatewayOptions) (SubnetGatewayStatus, error) {
	return SubnetGatewayStatus{Supported: false, Error: errSubnetGatewayUnsupported.Error()}, errSubnetGatewayUnsupported
}

func CheckSubnetGatewayStatus(opts SubnetGatewayOptions) (SubnetGatewayStatus, error) {
	opts = opts.withDefaults()
	return SubnetGatewayStatus{
		Supported:    false,
		LANCIDR:      opts.LANCIDR,
		OutInterface: opts.OutInterface,
		WGInterface:  opts.WGInterface,
		OverlayCIDR:  opts.OverlayCIDR,
		Error:        errSubnetGatewayUnsupported.Error(),
	}, nil
}
