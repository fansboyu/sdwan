package app

import (
	"errors"
	"testing"
	"time"

	"englishlisten/sdwan/internal/storage/sqlc"
)

func TestRelayHealthRequiresFreshSyncInterfaceAndHandshake(t *testing.T) {
	now := time.Now()
	base := sqlc.Relay{LastSeenAt: &now, LastSyncAt: &now, InterfaceUp: true, SyncOK: true, HandshakePeerCount: 2, LastHandshakeAt: &now}
	if !relayIsHealthy(base) || relayHealthStatus(base) != "healthy" {
		t.Fatal("expected relay with recent control/data-plane checks to be healthy")
	}
	base.LastHandshakeAt = nil
	base.HandshakePeerCount = 0
	if relayIsHealthy(base) || relayHealthStatus(base) != "awaiting_data_plane" {
		t.Fatal("expected missing real handshake to remain unverified")
	}
	base.LastHandshakeAt = &now
	base.HandshakePeerCount = 1
	base.SyncOK = false
	if relayIsHealthy(base) || relayHealthStatus(base) != "degraded" {
		t.Fatal("expected failed peer sync to make relay degraded")
	}
	stale := now.Add(-relayHeartbeatTTL - time.Second)
	base.LastSeenAt = &stale
	if relayHealthStatus(base) != "offline" {
		t.Fatal("expected stale heartbeat to make relay offline")
	}
}

func TestDirectPathDegradationPolicy(t *testing.T) {
	if !directPathActive(pathModeDirect, pathModeRelay, false) {
		t.Fatal("direct mode should keep direct route ownership")
	}
	if !directPathActive(pathModeAuto, pathModeRelay, false) {
		t.Fatal("auto mode should fail back to direct when relay is unhealthy")
	}
	if directPathActive(pathModeRelay, pathModeDirect, false) {
		t.Fatal("forced relay mode must fail closed rather than leak traffic direct")
	}
}

func TestResolvePathVerificationRequiresBothEndpoints(t *testing.T) {
	client := sqlc.AppliedPathStatus{Generation: 4, EffectivePath: "relay", VerificationStatus: "verified"}
	main := sqlc.AppliedPathStatus{Generation: 4, EffectivePath: "relay", VerificationStatus: "verified"}
	state, _, verified := resolvePathVerification("relay", 4, client, nil, main, nil)
	if state != "verified" || !verified {
		t.Fatalf("expected verified, got state=%q verified=%v", state, verified)
	}
	main.VerificationStatus = "pending"
	state, _, verified = resolvePathVerification("relay", 4, client, nil, main, nil)
	if state != "verifying" || verified {
		t.Fatalf("expected verifying, got state=%q verified=%v", state, verified)
	}
	main.VerificationStatus = "verified"
	main.EffectivePath = "direct"
	state, _, verified = resolvePathVerification("relay", 4, client, nil, main, nil)
	if state != "path_mismatch" || verified {
		t.Fatalf("expected path mismatch, got state=%q verified=%v", state, verified)
	}
	state, _, verified = resolvePathVerification("relay", 4, client, errors.New("not reported"), main, nil)
	if state != "verifying" || verified {
		t.Fatalf("missing endpoint must remain verifying, got state=%q verified=%v", state, verified)
	}
}

func TestResolvePathVerificationSurfacesProbeFailure(t *testing.T) {
	client := sqlc.AppliedPathStatus{Generation: 2, EffectivePath: "direct", VerificationStatus: "failed", VerificationError: "no reply"}
	main := sqlc.AppliedPathStatus{Generation: 2, EffectivePath: "direct", VerificationStatus: "verified"}
	state, message, verified := resolvePathVerification("direct", 2, client, nil, main, nil)
	if state != "failed" || verified || message != "no reply" {
		t.Fatalf("unexpected result state=%q message=%q verified=%v", state, message, verified)
	}
}
