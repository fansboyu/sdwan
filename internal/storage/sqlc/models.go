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
	PathMode      string    `json:"path_mode"`
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

type EmailVerification struct {
	ID           string     `json:"id"`
	Email        string     `json:"email"`
	Purpose      string     `json:"purpose"`
	CodeHash     string     `json:"-"`
	ExpiresAt    time.Time  `json:"expires_at"`
	ConsumedAt   *time.Time `json:"consumed_at,omitempty"`
	AttemptCount int32      `json:"attempt_count"`
	CreatedAt    time.Time  `json:"created_at"`
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
	ID                    string     `json:"id"`
	UserID                string     `json:"user_id"`
	DeviceID              string     `json:"device_id"`
	Cidr                  string     `json:"cidr"`
	Status                string     `json:"status"`
	Advertised            bool       `json:"advertised"`
	Approved              bool       `json:"approved"`
	GatewayEnabled        bool       `json:"gateway_enabled"`
	GatewayError          string     `json:"gateway_error"`
	GatewayCheckedAt      *time.Time `json:"gateway_checked_at,omitempty"`
	GatewayOutInterface   string     `json:"gateway_out_interface"`
	GatewayRouteInterface string     `json:"gateway_route_interface"`
	GatewayLANTarget      string     `json:"gateway_lan_target"`
	GatewayLANReachable   bool       `json:"gateway_lan_reachable"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
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

type PeerPath struct {
	UserID           string    `json:"user_id"`
	ClientDeviceID   string    `json:"client_device_id"`
	MainSiteDeviceID string    `json:"main_site_device_id"`
	CurrentPath      string    `json:"current_path"`
	DesiredPath      string    `json:"desired_path"`
	State            string    `json:"state"`
	Generation       int64     `json:"generation"`
	SwitchedAt       time.Time `json:"switched_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type DevicePeerStat struct {
	DeviceID        string     `json:"device_id"`
	PeerPublicKey   string     `json:"peer_public_key"`
	LatestHandshake *time.Time `json:"latest_handshake_at,omitempty"`
	RxBytes         int64      `json:"rx_bytes"`
	TxBytes         int64      `json:"tx_bytes"`
	LastRxAt        *time.Time `json:"last_rx_at,omitempty"`
	ReportedAt      time.Time  `json:"reported_at"`
}
