package relayagent

import (
	"context"
	"log"
	"strings"
	"time"

	"englishlisten/sdwan/internal/bootstrapagent"
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
	if r.Config.RemoveStalePeers {
		current, err := r.WG.DumpPeers(ctx)
		if err != nil {
			return err
		}
		for publicKey := range current {
			if _, ok := desired[publicKey]; !ok {
				if err := r.WG.RemovePeer(ctx, publicKey); err != nil {
					return err
				}
			}
		}
	}
	if err := r.API.Heartbeat(ctx); err != nil {
		return err
	}
	log.Printf("relay peers synced: relay=%s desired=%d", resp.Relay.ID, len(desired))
	return nil
}
