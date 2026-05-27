package sqlc

import "time"

type Customer struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	AddressCidr   string    `json:"address_cidr"`
	MaxDevices    int32     `json:"max_devices"`
	NetmapVersion int64     `json:"netmap_version"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

type AdminUser struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

type AdminSession struct {
	ID          string     `json:"id"`
	AdminUserID string     `json:"admin_user_id"`
	ExpiresAt   time.Time  `json:"expires_at"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type JoinToken struct {
	ID         string     `json:"id"`
	CustomerID string     `json:"customer_id"`
	TokenHash  string     `json:"-"`
	Name       string     `json:"name"`
	MaxUses    int32      `json:"max_uses"`
	UsedCount  int32      `json:"used_count"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type Device struct {
	ID            string     `json:"id"`
	CustomerID    string     `json:"customer_id"`
	Hostname      string     `json:"hostname"`
	OS            string     `json:"os"`
	Arch          string     `json:"arch"`
	PublicKey     string     `json:"public_key"`
	VirtualIP     string     `json:"virtual_ip"`
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
