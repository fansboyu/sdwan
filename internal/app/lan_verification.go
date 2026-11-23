package app

import (
	"context"
	"errors"
	"net/netip"

	"englishlisten/sdwan/internal/storage/sqlc"
)

type LANReport struct {
	Target    string `json:"target"`
	Reachable bool   `json:"reachable"`
}

func (s *Service) ReportEmployeeLAN(ctx context.Context, token string, report LANReport) error {
	device, err := s.deviceFromToken(ctx, token)
	if err != nil {
		return err
	}
	if device.SiteRole == siteRoleMain {
		return ErrUnauthorized
	}
	target, err := netip.ParseAddrPort(report.Target)
	if err != nil || target.Port() == 0 || !target.Addr().Is4() {
		return errors.New("an IPv4 address and TCP port are required")
	}
	routes, err := s.store.Queries.ListActiveSubnetRoutesByUser(ctx, device.UserID)
	if err != nil {
		return err
	}
	peers, err := s.store.Queries.ListDevicesByUser(ctx, device.UserID)
	if err != nil {
		return err
	}
	for _, route := range routes {
		prefix, err := netip.ParsePrefix(route.Cidr)
		if err != nil || !prefix.Contains(target.Addr()) {
			continue
		}
		for _, peer := range peers {
			if peer.ID == route.DeviceID && peer.SiteRole == siteRoleMain && peer.Status == "active" {
				return s.store.Queries.UpsertLANVerification(ctx, device.ID, route.ID, target.String(), report.Reachable)
			}
		}
	}
	return errors.New("target is outside the active Hub routes")
}

func (s *Service) EmployeeLANVerifications(ctx context.Context, user string) ([]sqlc.LANVerification, error) {
	return s.store.Queries.ListLANVerifications(ctx, user)
}
