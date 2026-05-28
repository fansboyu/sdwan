package bootstrapagent

import (
	"context"
	"log"
	"strings"
	"time"
)

type Runner struct {
	Config Config
	API    *APIClient
	WG     WGManager

	lastEndpoints map[string]string
}

func NewRunner(cfg Config) *Runner {
	cfg.withDefaults()
	return &Runner{
		Config:        cfg,
		API:           NewAPIClient(cfg.ControllerURL, cfg.BootstrapToken),
		WG:            WGManager{InterfaceName: cfg.InterfaceName},
		lastEndpoints: map[string]string{},
	}
}

func (r *Runner) Run(ctx context.Context) error {
	if err := r.WG.EnsureInterface(ctx); err != nil {
		return err
	}
	if err := r.SyncPeers(ctx); err != nil {
		log.Printf("bootstrap peer sync failed: %v", err)
	}
	if err := r.ReportEndpoints(ctx); err != nil {
		log.Printf("bootstrap endpoint report failed: %v", err)
	}

	syncTicker := time.NewTicker(r.Config.SyncInterval())
	defer syncTicker.Stop()
	reportTicker := time.NewTicker(r.Config.ReportInterval())
	defer reportTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-syncTicker.C:
			if err := r.SyncPeers(ctx); err != nil {
				log.Printf("bootstrap peer sync failed: %v", err)
			}
		case <-reportTicker.C:
			if err := r.ReportEndpoints(ctx); err != nil {
				log.Printf("bootstrap endpoint report failed: %v", err)
			}
		}
	}
}

func (r *Runner) SyncPeers(ctx context.Context) error {
	peers, err := r.API.Peers(ctx)
	if err != nil {
		return err
	}
	desired := map[string]Peer{}
	for _, peer := range peers {
		if strings.TrimSpace(peer.PublicKey) == "" || strings.TrimSpace(peer.VirtualIP) == "" {
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
				delete(r.lastEndpoints, publicKey)
			}
		}
	}
	log.Printf("bootstrap peers synced: desired=%d", len(desired))
	return nil
}

func (r *Runner) ReportEndpoints(ctx context.Context) error {
	states, err := r.WG.DumpPeers(ctx)
	if err != nil {
		return err
	}
	reported := 0
	for publicKey, state := range states {
		if state.Endpoint == "" {
			continue
		}
		if r.lastEndpoints[publicKey] == state.Endpoint {
			continue
		}
		if err := r.API.ReportEndpoint(ctx, publicKey, state.Endpoint); err != nil {
			return err
		}
		r.lastEndpoints[publicKey] = state.Endpoint
		reported++
	}
	if reported > 0 {
		log.Printf("bootstrap endpoints reported: changed=%d total=%d", reported, len(states))
	}
	return nil
}
