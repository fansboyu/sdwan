package bootstrapagent

import (
	"testing"
	"time"
)

func TestObserveEndpointRequiresFreshEvidence(t *testing.T) {
	now := time.Now()
	state := WGPeerState{PublicKey: "peer", Endpoint: "192.0.2.1:51820", RxBytes: 40, LatestHandshake: now.Add(-2 * time.Minute)}
	old := observeEndpoint(endpointObservation{}, state, now)
	if !old.evidenceAt.IsZero() {
		t.Fatal("stale dump must not refresh endpoint")
	}
	state.RxBytes++
	fresh := observeEndpoint(old, state, now)
	if !fresh.evidenceAt.Equal(now) {
		t.Fatal("increased receive counter should refresh endpoint")
	}
	state.Endpoint = "192.0.2.2:51820"
	if got := observeEndpoint(fresh, state, now.Add(time.Second)); !got.evidenceAt.IsZero() {
		t.Fatal("address change without new evidence must not reuse previous evidence")
	}
}
