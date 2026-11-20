package bootstrapagent

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"
)

type Runner struct {
	Config Config
	API    *APIClient
	WG     WGManager

	observations map[string]endpointObservation
}

type endpointObservation struct {
	state      WGPeerState
	evidenceAt time.Time
	reportedAt time.Time
	reported   string
}

func observeEndpoint(previous endpointObservation, state WGPeerState, now time.Time) endpointObservation {
	if state.Endpoint != previous.state.Endpoint {
		previous.evidenceAt = time.Time{}
	}
	if state.RxBytes > previous.state.RxBytes && previous.state.PublicKey != "" ||
		state.LatestHandshake.After(previous.state.LatestHandshake) &&
			!state.LatestHandshake.After(now) && now.Sub(state.LatestHandshake) <= time.Minute {
		previous.evidenceAt = now
	}
	previous.state = state
	return previous
}

func NewRunner(cfg Config) *Runner {
	cfg.withDefaults()
	return &Runner{
		Config:       cfg,
		API:          NewAPIClient(cfg.ControllerURL, cfg.BootstrapToken),
		WG:           WGManager{InterfaceName: cfg.InterfaceName},
		observations: make(map[string]endpointObservation),
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
	current, err := r.WG.DumpPeers(ctx)
	if err != nil {
		return err
	}
	for publicKey := range current {
		if _, ok := desired[publicKey]; !ok {
			if err := r.WG.RemovePeer(ctx, publicKey); err != nil {
				return err
			}
			delete(r.observations, publicKey)
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
	now := time.Now()
	var reportErr error
	for publicKey, state := range states {
		observation := observeEndpoint(r.observations[publicKey], state, now)
		r.observations[publicKey] = observation
		if state.Endpoint == "" {
			continue
		}
		if !observation.evidenceAt.After(observation.reportedAt) || now.Sub(observation.evidenceAt) > time.Minute ||
			(observation.reported == state.Endpoint && now.Sub(observation.reportedAt) < 30*time.Second) {
			continue
		}
		if err := r.API.ReportEndpoint(ctx, publicKey, state.Endpoint); err != nil {
			reportErr = errors.Join(reportErr, err)
			continue
		}
		observation.reported = state.Endpoint
		observation.reportedAt = now
		r.observations[publicKey] = observation
		reported++
	}
	if reported > 0 {
		log.Printf("bootstrap endpoints reported: reported=%d total=%d", reported, len(states))
	}
	return reportErr
}
