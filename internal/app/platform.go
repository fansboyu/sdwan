package app

import (
	"context"
	"time"

	"englishlisten/sdwan/internal/storage/sqlc"
)

func (s *Service) PlatformCredentials() (string, string) {
	if s == nil {
		return "", ""
	}
	return s.cfg.PlatformEmail, s.cfg.PlatformPasswordHash
}

func (s *Service) PlatformTenants(ctx context.Context) ([]sqlc.PlatformTenant, error) {
	return s.store.Queries.PlatformTenants(ctx)
}

func (s *Service) PlatformTenantDetail(ctx context.Context, id string) (map[string]any, error) {
	q := s.store.Queries
	u, err := q.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}
	devices, err := q.ListDevicesByUser(ctx, id)
	if err != nil {
		return nil, err
	}
	routes, err := q.ListSubnetRoutesByUser(ctx, id)
	if err != nil {
		return nil, err
	}
	paths, err := q.ListPeerPathsByUser(ctx, id)
	if err != nil {
		return nil, err
	}
	relays, err := q.ListRelaysByUser(ctx, id)
	if err != nil {
		return nil, err
	}
	for i := range relays {
		relays[i].HealthStatus = relayHealthStatus(relays[i])
	}
	return map[string]any{"user": u, "devices": devices, "routes": routes, "paths": paths, "relays": relays, "checked_at": time.Now()}, nil
}
