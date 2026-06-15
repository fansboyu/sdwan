//go:build !linux

package agent

func CollectPeerStats(string) ([]PeerStat, error) { return nil, nil }
