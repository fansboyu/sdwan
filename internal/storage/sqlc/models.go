package sqlc

import "time"

type User struct {
	ID            string    `json:"id"`
	Email         string    `json:"email"`
	PasswordHash  string    `json:"-"`
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
	GatewayServiceStatus  string     `json:"gateway_service_status"`
	GatewayHistory        string     `json:"gateway_history"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type SubnetRouteConflict struct {
	ID         string     `json:"id"`
	DeviceID   string     `json:"device_id"`
	CIDR       string     `json:"cidr"`
	Details    string     `json:"details"`
	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

type Relay struct {
	ID                 string     `json:"id"`
	UserID             string     `json:"user_id"`
	Name               string     `json:"name"`
	PublicKey          string     `json:"public_key"`
	VirtualIP          string     `json:"virtual_ip"`
	Endpoint           string     `json:"endpoint"`
	Status             string     `json:"status"`
	LastSeenAt         *time.Time `json:"last_seen_at,omitempty"`
	InterfaceUp        bool       `json:"interface_up"`
	SyncedPeerCount    int32      `json:"synced_peer_count"`
	Version            string     `json:"version"`
	LastError          string     `json:"last_error"`
	LastSyncAt         *time.Time `json:"last_sync_at,omitempty"`
	SyncOK             bool       `json:"sync_ok"`
	HandshakePeerCount int32      `json:"handshake_peer_count"`
	LastHandshakeAt    *time.Time `json:"last_handshake_at,omitempty"`
	HealthStatus       string     `json:"health_status,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
}

type PeerPath struct {
	HubEndpoint             string     `json:"hub_endpoint,omitempty"`
	ClientEndpoint          string     `json:"client_endpoint,omitempty"`
	EndpointTrial           int        `json:"endpoint_trial"`
	ClientAppliedGeneration *int64     `json:"client_applied_generation"`
	HubAppliedGeneration    *int64     `json:"hub_applied_generation"`
	UserID                  string     `json:"user_id"`
	ClientDeviceID          string     `json:"client_device_id"`
	MainSiteDeviceID        string     `json:"main_site_device_id"`
	CurrentPath             string     `json:"current_path"`
	DesiredPath             string     `json:"desired_path"`
	State                   string     `json:"state"`
	Generation              int64      `json:"generation"`
	VerifiedPath            string     `json:"verified_path"`
	VerifiedGeneration      int64      `json:"verified_generation"`
	VerificationState       string     `json:"verification_state"`
	VerificationError       string     `json:"verification_error"`
	VerifiedAt              *time.Time `json:"verified_at,omitempty"`
	SwitchedAt              time.Time  `json:"switched_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
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

type DevicePathQuality struct {
	DeviceID          string     `json:"device_id"`
	PeerDeviceID      string     `json:"peer_device_id"`
	PathType          string     `json:"path_type"`
	CurrentPath       bool       `json:"current_path"`
	Endpoint          string     `json:"endpoint"`
	LatestHandshakeAt *time.Time `json:"latest_handshake_at,omitempty"`
	RxBytes           int64      `json:"rx_bytes"`
	TxBytes           int64      `json:"tx_bytes"`
	LastRxAt          *time.Time `json:"last_rx_at,omitempty"`
	FailCount         int32      `json:"fail_count"`
	LastOKAt          *time.Time `json:"last_ok_at,omitempty"`
	LastFailedAt      *time.Time `json:"last_failed_at,omitempty"`
	UpdatedAt         time.Time  `json:"updated_at"`
}
