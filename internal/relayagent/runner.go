package relayagent

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"englishlisten/sdwan/internal/bootstrapagent"
	"englishlisten/sdwan/internal/version"
)

type Runner struct {
	Config Config
	API    *APIClient
	WG     bootstrapagent.WGManager
}

func NewRunner(cfg Config) *Runner {
	cfg.withDefaults()
	return &Runner{
		Config: cfg,
		API:    NewAPIClient(cfg.ControllerURL, cfg.RelayToken),
		WG:     bootstrapagent.WGManager{InterfaceName: cfg.InterfaceName},
	}
}

func (r *Runner) Run(ctx context.Context) error {
	if err := r.WG.EnsureInterface(ctx); err != nil {
		return err
	}
	if err := r.SyncPeers(ctx); err != nil {
		log.Printf("relay peer sync failed: %v", err)
		if hbErr := r.API.Heartbeat(ctx, HeartbeatRequest{
			InterfaceUp: true,
			Version:     version.Version,
			LastError:   err.Error(),
			SyncOK:      false,
		}); hbErr != nil {
			log.Printf("relay heartbeat failed: %v", hbErr)
		}
	}

	ticker := time.NewTicker(r.Config.SyncInterval())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := r.SyncPeers(ctx); err != nil {
				log.Printf("relay peer sync failed: %v", err)
				if hbErr := r.API.Heartbeat(ctx, HeartbeatRequest{
					InterfaceUp: r.WG.EnsureInterface(ctx) == nil,
					Version:     version.Version,
					LastError:   err.Error(),
					SyncOK:      false,
				}); hbErr != nil {
					log.Printf("relay heartbeat failed: %v", hbErr)
				}
			}
		}
	}
}

func (r *Runner) SyncPeers(ctx context.Context) error {
	resp, err := r.API.Peers(ctx)
	if err != nil {
		return err
	}
	desired := map[string]bootstrapagent.Peer{}
	for _, peer := range resp.Peers {
		if strings.TrimSpace(peer.PublicKey) == "" || len(peer.AllowedIPs) == 0 {
			continue
		}
		desired[peer.PublicKey] = peer
		if err := r.WG.SetPeer(ctx, peer); err != nil {
			return err
		}
	}
	currentPeers, err := r.WG.DumpPeers(ctx)
	if err != nil {
		return err
	}
	for publicKey := range currentPeers {
		if _, ok := desired[publicKey]; !ok {
			if err := r.WG.RemovePeer(ctx, publicKey); err != nil {
				return err
			}
		}
	}
	current, err := r.WG.DumpPeers(ctx)
	if err != nil {
		return fmt.Errorf("dump relay peers: %w", err)
	}
	now := time.Now()
	var handshakeCount int32
	var latestHandshake *time.Time
	for _, peer := range current {
		if peer.LatestHandshake.IsZero() || now.Sub(peer.LatestHandshake) > 120*time.Second {
			continue
		}
		handshakeCount++
		if latestHandshake == nil || peer.LatestHandshake.After(*latestHandshake) {
			observed := peer.LatestHandshake
			latestHandshake = &observed
		}
	}
	if err := r.API.Heartbeat(ctx, HeartbeatRequest{
		InterfaceUp:        true,
		SyncedPeerCount:    int32(len(current)),
		HandshakePeerCount: handshakeCount,
		LastHandshakeAt:    latestHandshake,
		Version:            version.Version,
		SyncOK:             true,
	}); err != nil {
		return err
	}
	log.Printf("relay peers synced: relay=%s desired=%d current=%d", resp.Relay.ID, len(desired), len(current))
	return nil
}
