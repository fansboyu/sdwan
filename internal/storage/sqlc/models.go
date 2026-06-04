package sqlc

import "time"

type User struct {
	ID            string    `json:"id"`
	Email         string    `json:"email"`
	PasswordHash  string    `json:"-"`
	PlanCode      string    `json:"plan_code"`
	OverlayCidr   string    `json:"overlay_cidr"`
	MaxDevices    int32     `json:"max_devices"`
	NetmapVersion int64     `json:"netmap_version"`
	RelayMode     bool      `json:"relay_mode"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

type AdminSession struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type Plan struct {
	Code            string    `json:"code"`
	Name            string    `json:"name"`
	PriceCents      int32     `json:"price_cents"`
	MaxDevices      int32     `json:"max_devices"`
	EnableSubnet    bool      `json:"enable_subnet"`
	EnableSelfRelay bool      `json:"enable_self_relay"`
	CreatedAt       time.Time `json:"created_at"`
}

type Subscription struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	PlanCode   string     `json:"plan_code"`
	Status     string     `json:"status"`
	Source     string     `json:"source"`
	FreeMonths int32      `json:"free_months"`
	StartsAt   time.Time  `json:"starts_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type Device struct {
	ID            string     `json:"id"`
	UserID        string     `json:"user_id"`
	Hostname      string     `json:"hostname"`
	OS            string     `json:"os"`
	Arch          string     `json:"arch"`
	PublicKey     string     `json:"public_key"`
	VirtualIP     string     `json:"virtual_ip"`
	SiteRole      string     `json:"site_role"`
	Status        string     `json:"status"`
	ClientVersion string     `json:"client_version"`
	OSVersion     string     `json:"os_version"`
	LastSeenAt    *time.Time `json:"last_seen_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type DeviceEndpoint struct {
	ID           string    `json:"id"`
	DeviceID     string    `json:"device_id"`
	EndpointType string    `json:"endpoint_type"`
	Address      string    `json:"address"`
	Source       string    `json:"source"`
	RttMs        *int32    `json:"rtt_ms,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type SubnetRoute struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	DeviceID   string    `json:"device_id"`
	Cidr       string    `json:"cidr"`
	Status     string    `json:"status"`
	Advertised bool      `json:"advertised"`
	Approved   bool      `json:"approved"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Relay struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	Name       string     `json:"name"`
	PublicKey  string     `json:"public_key"`
	VirtualIP  string     `json:"virtual_ip"`
	Endpoint   string     `json:"endpoint"`
	Status     string     `json:"status"`
	LastSeenAt *time.Time `json:"last_seen_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}
