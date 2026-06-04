//go:build !linux

package agent

func ApplyWireGuardConfigSmart(cfg Config, configPath string, netmap Netmap) (ApplyResult, error) {
	if err := ApplyWireGuardConfig(cfg.InterfaceName, configPath); err != nil {
		return ApplyResult{}, err
	}
	return ApplyResult{LastRoutes: DesiredRoutes(netmap)}, nil
}
