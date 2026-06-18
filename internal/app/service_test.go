package app

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"englishlisten/sdwan/internal/storage/sqlc"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestAllocateDeviceIPSkipsDotZero(t *testing.T) {
	ip, err := allocateDeviceIP("100.64.0.0/24", nil)
	if err != nil {
		t.Fatalf("allocateDeviceIP returned error: %v", err)
	}
	if ip != "100.64.0.1" {
		t.Fatalf("expected first usable IP to skip .0, got %s", ip)
	}
}

func TestAllocateDeviceIPSkipsDot255(t *testing.T) {
	devices := make([]sqlc.Device, 0, 254)
	for i := 1; i <= 254; i++ {
		devices = append(devices, sqlc.Device{VirtualIP: "100.64.0." + strconv.Itoa(i)})
	}

	_, err := allocateDeviceIP("100.64.0.0/24", devices)
	if err == nil {
		t.Fatal("expected no available IP after .1-.254 are used and .0/.255 are reserved")
	}
}

func TestOrderedEndpointAddressesPrefersBootstrap(t *testing.T) {
	now := time.Now()
	endpoints := []sqlc.DeviceEndpoint{
		{EndpointType: "lan", Address: "192.168.1.10:41641", UpdatedAt: now.Add(5 * time.Second)},
		{EndpointType: "bootstrap", Address: "111.228.42.62:37425", UpdatedAt: now},
		{EndpointType: "manual", Address: "203.0.113.10:41641", UpdatedAt: now.Add(10 * time.Second)},
	}

	ordered := orderedEndpointAddresses(endpoints)
	if len(ordered) != 3 {
		t.Fatalf("expected 3 endpoints, got %d", len(ordered))
	}
	if ordered[0] != "111.228.42.62:37425" {
		t.Fatalf("expected bootstrap endpoint first, got %s", ordered[0])
	}
}

func TestIsDeviceVirtualIPConflict(t *testing.T) {
	err := &pgconn.PgError{
		Code:           "23505",
		ConstraintName: "devices_user_id_virtual_ip_key",
	}
	if !isDeviceVirtualIPConflict(err) {
		t.Fatal("expected virtual IP unique constraint to be retryable")
	}

	err = &pgconn.PgError{
		Code:           "23505",
		ConstraintName: "devices_user_id_public_key_key",
	}
	if isDeviceVirtualIPConflict(err) {
		t.Fatal("expected public key unique constraint to remain non-retryable")
	}

	if isDeviceVirtualIPConflict(errors.New("other error")) {
		t.Fatal("expected non-postgres error to remain non-retryable")
	}
}

func TestNormalizeSubnetRoutes(t *testing.T) {
	routes, err := normalizeSubnetRoutes([]string{"192.168.1.12/24", "192.168.1.0/24", "10.0.0.0/8"})
	if err != nil {
		t.Fatalf("normalizeSubnetRoutes returned error: %v", err)
	}
	if len(routes) != 2 {
		t.Fatalf("expected 2 deduplicated routes, got %d", len(routes))
	}
	if routes[0] != "10.0.0.0/8" || routes[1] != "192.168.1.0/24" {
		t.Fatalf("unexpected normalized routes: %v", routes)
	}
}

func TestNormalizeSubnetRoutesRejectsOverlayCIDR(t *testing.T) {
	if _, err := normalizeSubnetRoutes([]string{"100.64.0.0/24"}); err == nil {
		t.Fatal("expected overlay CIDR overlap to be rejected")
	}
	if _, err := normalizeSubnetRoutes([]string{"0.0.0.0/0"}); err == nil {
		t.Fatal("expected default route to be rejected")
	}
}

func TestValidateAdvertisedSubnetRoutesRejectsSameAccountOverlap(t *testing.T) {
	existing := []sqlc.SubnetRoute{{
		ID:         "srt_existing",
		DeviceID:   "dev_main",
		Cidr:       "192.168.1.0/24",
		Advertised: true,
	}}
	err := validateAdvertisedSubnetRoutes("dev_other", []string{"192.168.1.128/25"}, existing)
	if err == nil {
		t.Fatal("expected overlapping route from another device to be rejected")
	}
}

func TestValidateAdvertisedSubnetRoutesAllowsSameDeviceReplacement(t *testing.T) {
	existing := []sqlc.SubnetRoute{{
		ID:         "srt_existing",
		DeviceID:   "dev_main",
		Cidr:       "192.168.1.0/24",
		Advertised: true,
	}}
	err := validateAdvertisedSubnetRoutes("dev_main", []string{"192.168.1.0/24"}, existing)
	if err != nil {
		t.Fatalf("expected same device exact route to remain allowed: %v", err)
	}
}
