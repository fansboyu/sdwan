package agent

import "testing"

func TestEffectivePathForAssignmentUsesAllowedIPOwner(t *testing.T) {
	netmap := Netmap{Peers: []NetmapPeer{
		{DeviceID: "main", PathRole: "direct", AllowedIPs: []string{"100.64.1.1/32"}},
		{DeviceID: "relay", PathRole: "relay", AllowedIPs: []string{"100.64.1.2/32"}},
	}}
	got, err := EffectivePathForAssignment(netmap, PathAssignment{PeerVirtualIP: "100.64.1.1"})
	if err != nil || got != "direct" {
		t.Fatalf("direct route: got %q, err %v", got, err)
	}
	got, err = EffectivePathForAssignment(netmap, PathAssignment{PeerVirtualIP: "100.64.1.2"})
	if err != nil || got != "relay" {
		t.Fatalf("relay route: got %q, err %v", got, err)
	}
}

func TestEffectivePathForAssignmentRejectsMissingOrAmbiguousRoute(t *testing.T) {
	_, err := EffectivePathForAssignment(Netmap{}, PathAssignment{PeerVirtualIP: "100.64.1.1"})
	if err == nil {
		t.Fatal("expected missing route error")
	}
	_, err = EffectivePathForAssignment(Netmap{}, PathAssignment{PeerVirtualIP: "not-an-ip"})
	if err == nil {
		t.Fatal("expected invalid IP error")
	}
}
