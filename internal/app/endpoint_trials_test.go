package app

import (
	"testing"
	"time"

	"englishlisten/sdwan/internal/storage/sqlc"
)

func TestDirectEndpointTrialPromotesAlternative(t *testing.T) {
	client := []string{"pub-a", "lan-a"}
	hub := []string{"pub-b", "lan-b"}
	path := sqlc.PeerPath{}
	want := [][2]string{{"lan-a", "pub-b"}, {"pub-a", "lan-b"}, {"lan-a", "lan-b"}}
	for _, pair := range want {
		a, b, ok := nextDirectEndpoints(client, hub, path)
		if !ok || a != pair[0] || b != pair[1] {
			t.Fatalf("got %q %q %v, want %v", a, b, ok, pair)
		}
		path.ClientEndpoint, path.HubEndpoint = a, b
	}
	if _, _, ok := nextDirectEndpoints(client, hub, path); ok {
		t.Fatal("unexpected trial after exhausting pairs")
	}
	if out := preferEndpoint(hub, path.HubEndpoint); out[0] != "lan-b" || hub[0] != "pub-b" {
		t.Fatal("expected promotion without mutating the input")
	}
}

func TestOldBootstrapEndpointIsDemoted(t *testing.T) {
	now := time.Now()
	got := orderedEndpointAddresses([]sqlc.DeviceEndpoint{
		{EndpointType: "bootstrap", Address: "old", UpdatedAt: now.Add(-3 * time.Minute)},
		{EndpointType: "lan", Address: "lan", UpdatedAt: now},
	})
	if len(got) != 2 || got[0] != "lan" {
		t.Fatalf("unexpected ordering: %v", got)
	}
}
