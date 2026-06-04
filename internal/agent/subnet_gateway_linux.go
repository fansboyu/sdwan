//go:build linux

package agent

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"strings"
)

func EnableSubnetGateway(opts SubnetGatewayOptions) (SubnetGatewayStatus, error) {
	opts = opts.withDefaults()
	if err := validateSubnetGatewayOptions(opts); err != nil {
		return statusWithError(opts, err), err
	}
	if os.Geteuid() != 0 {
		err := errors.New("subnet gateway enable requires root privileges")
		return statusWithError(opts, err), err
	}
	if _, err := requireCommand("iptables"); err != nil {
		return statusWithError(opts, err), err
	}
	if err := os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1\n"), 0o644); err != nil {
		err := fmt.Errorf("enable ip_forward: %w", err)
		return statusWithError(opts, err), err
	}
	if err := os.WriteFile(subnetGatewaySysctlPath, []byte(subnetGatewaySysctlContent), 0o644); err != nil {
		err := fmt.Errorf("write persistent sysctl: %w", err)
		return statusWithError(opts, err), err
	}
	for _, rule := range subnetGatewayRules(opts) {
		if err := ensureIPTablesRule(rule); err != nil {
			return statusWithError(opts, err), err
		}
	}
	return CheckSubnetGatewayStatus(opts)
}

func DisableSubnetGateway(opts SubnetGatewayOptions) (SubnetGatewayStatus, error) {
	opts = opts.withDefaults()
	if err := validateSubnetGatewayOptions(opts); err != nil {
		return statusWithError(opts, err), err
	}
	if os.Geteuid() != 0 {
		err := errors.New("subnet gateway disable requires root privileges")
		return statusWithError(opts, err), err
	}
	for _, rule := range subnetGatewayRules(opts) {
		if err := deleteIPTablesRule(rule); err != nil {
			return statusWithError(opts, err), err
		}
	}
	if err := os.Remove(subnetGatewaySysctlPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		err := fmt.Errorf("remove persistent sysctl: %w", err)
		return statusWithError(opts, err), err
	}
	return CheckSubnetGatewayStatus(opts)
}

func CheckSubnetGatewayStatus(opts SubnetGatewayOptions) (SubnetGatewayStatus, error) {
	opts = opts.withDefaults()
	status := SubnetGatewayStatus{
		Supported:    true,
		LANCIDR:      opts.LANCIDR,
		OutInterface: opts.OutInterface,
		WGInterface:  opts.WGInterface,
		OverlayCIDR:  opts.OverlayCIDR,
	}
	if err := validateSubnetGatewayOptions(opts); err != nil {
		status.Error = err.Error()
		return status, err
	}
	status.IPForward = readTrimmed("/proc/sys/net/ipv4/ip_forward") == "1"
	status.PersistentSysctl = readTrimmed(subnetGatewaySysctlPath) == strings.TrimSpace(subnetGatewaySysctlContent)
	status.WireGuardInterface = interfaceExists(opts.WGInterface)
	status.OutInterfacePresent = interfaceExists(opts.OutInterface)

	rules := subnetGatewayRules(opts)
	status.NATRule = iptablesRuleExists(rules[0])
	status.ForwardToLANRule = iptablesRuleExists(rules[1])
	status.ForwardFromLANRule = iptablesRuleExists(rules[2])
	status.Enabled = status.IPForward &&
		status.PersistentSysctl &&
		status.NATRule &&
		status.ForwardToLANRule &&
		status.ForwardFromLANRule
	return status, nil
}

type iptablesRule struct {
	table string
	chain string
	args  []string
}

func subnetGatewayRules(opts SubnetGatewayOptions) []iptablesRule {
	comment := []string{"-m", "comment", "--comment", SubnetGatewayRuleComment}
	return []iptablesRule{
		{
			table: "nat",
			chain: "POSTROUTING",
			args: append([]string{
				"-s", opts.OverlayCIDR,
				"-d", opts.LANCIDR,
				"-o", opts.OutInterface,
			}, append(comment, "-j", "MASQUERADE")...),
		},
		{
			chain: "FORWARD",
			args: append([]string{
				"-i", opts.WGInterface,
				"-o", opts.OutInterface,
				"-d", opts.LANCIDR,
			}, append(comment, "-j", "ACCEPT")...),
		},
		{
			chain: "FORWARD",
			args: append([]string{
				"-i", opts.OutInterface,
				"-o", opts.WGInterface,
				"-s", opts.LANCIDR,
				"-m", "conntrack",
				"--ctstate", "RELATED,ESTABLISHED",
			}, append(comment, "-j", "ACCEPT")...),
		},
	}
}

func ensureIPTablesRule(rule iptablesRule) error {
	if iptablesRuleExists(rule) {
		return nil
	}
	args := rule.baseArgs("-A")
	args = append(args, rule.args...)
	if err := runCommandQuiet("iptables", args...); err != nil {
		return fmt.Errorf("add iptables rule %s: %w", rule.chain, err)
	}
	return nil
}

func deleteIPTablesRule(rule iptablesRule) error {
	for iptablesRuleExists(rule) {
		args := rule.baseArgs("-D")
		args = append(args, rule.args...)
		if err := runCommandQuiet("iptables", args...); err != nil {
			return fmt.Errorf("delete iptables rule %s: %w", rule.chain, err)
		}
	}
	return nil
}

func iptablesRuleExists(rule iptablesRule) bool {
	args := rule.baseArgs("-C")
	args = append(args, rule.args...)
	return runCommandQuiet("iptables", args...) == nil
}

func (r iptablesRule) baseArgs(action string) []string {
	var args []string
	if r.table != "" {
		args = append(args, "-t", r.table)
	}
	args = append(args, action, r.chain)
	return args
}

func validateSubnetGatewayOptions(opts SubnetGatewayOptions) error {
	if strings.TrimSpace(opts.LANCIDR) == "" {
		return errors.New("--lan-cidr is required")
	}
	if strings.TrimSpace(opts.OutInterface) == "" {
		return errors.New("--out-interface is required")
	}
	if strings.TrimSpace(opts.WGInterface) == "" {
		return errors.New("--wg-interface is required")
	}
	lan, err := parseIPv4Prefix(opts.LANCIDR, "--lan-cidr")
	if err != nil {
		return err
	}
	overlay, err := parseIPv4Prefix(opts.OverlayCIDR, "--overlay-cidr")
	if err != nil {
		return err
	}
	if prefixesOverlap(lan, overlay) {
		return errors.New("--lan-cidr cannot overlap --overlay-cidr")
	}
	return nil
}

func parseIPv4Prefix(value, name string) (netip.Prefix, error) {
	prefix, err := netip.ParsePrefix(strings.TrimSpace(value))
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("invalid %s: %w", name, err)
	}
	prefix = prefix.Masked()
	if !prefix.Addr().Is4() {
		return netip.Prefix{}, fmt.Errorf("%s must be IPv4", name)
	}
	if prefix.Bits() == 0 {
		return netip.Prefix{}, fmt.Errorf("%s cannot be 0.0.0.0/0", name)
	}
	return prefix, nil
}

func prefixesOverlap(left, right netip.Prefix) bool {
	return left.Contains(right.Addr()) || right.Contains(left.Addr())
}

func interfaceExists(name string) bool {
	_, err := net.InterfaceByName(name)
	return err == nil
}

func readTrimmed(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func requireCommand(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s is required", name)
	}
	return path, nil
}

func runCommandQuiet(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		text := strings.TrimSpace(stderr.String())
		if text == "" {
			text = err.Error()
		}
		return errors.New(text)
	}
	return nil
}

func statusWithError(opts SubnetGatewayOptions, err error) SubnetGatewayStatus {
	return SubnetGatewayStatus{
		Supported:    true,
		LANCIDR:      opts.LANCIDR,
		OutInterface: opts.OutInterface,
		WGInterface:  opts.WGInterface,
		OverlayCIDR:  opts.OverlayCIDR,
		Error:        err.Error(),
	}
}
