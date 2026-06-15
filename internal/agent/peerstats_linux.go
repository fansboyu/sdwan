//go:build linux

package agent

import "os/exec"

func CollectPeerStats(interfaceName string) ([]PeerStat, error) {
	if interfaceName == "" {
		interfaceName = DefaultInterface
	}
	output, err := exec.Command("wg", "show", interfaceName, "dump").Output()
	if err != nil {
		return nil, err
	}
	return ParseWGDump(string(output)), nil
}
