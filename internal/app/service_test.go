package app

import (
	"strconv"
	"testing"
	"time"

	"englishlisten/sdwan/internal/storage/sqlc"
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
