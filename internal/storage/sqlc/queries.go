package sqlc

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type DBTX interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type Queries struct {
	db DBTX
}

func New(db DBTX) *Queries {
	return &Queries{db: db}
}

type CreateUserParams struct {
	ID           string
	Email        string
	PasswordHash string
	OverlayCidr  string
	MaxDevices   int32
}

func (q *Queries) CreateUser(ctx context.Context, arg CreateUserParams) (User, error) {
	row := q.db.QueryRow(ctx, `INSERT INTO users (id, email, password_hash, overlay_cidr, max_devices)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, email, password_hash, plan_code, overlay_cidr::text, max_devices, netmap_version, status, created_at`,
		arg.ID, arg.Email, arg.PasswordHash, arg.OverlayCidr, arg.MaxDevices)
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.PlanCode, &u.OverlayCidr, &u.MaxDevices, &u.NetmapVersion, &u.Status, &u.CreatedAt)
	return u, err
}

func (q *Queries) GetUser(ctx context.Context, id string) (User, error) {
	row := q.db.QueryRow(ctx, `SELECT id, email, password_hash, plan_code, overlay_cidr::text, max_devices, netmap_version, status, created_at
FROM users
WHERE id = $1`, id)
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.PlanCode, &u.OverlayCidr, &u.MaxDevices, &u.NetmapVersion, &u.Status, &u.CreatedAt)
	return u, err
}

func (q *Queries) GetUserByEmail(ctx context.Context, email string) (User, error) {
	row := q.db.QueryRow(ctx, `SELECT id, email, password_hash, plan_code, overlay_cidr::text, max_devices, netmap_version, status, created_at
FROM users
WHERE email = $1`, email)
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.PlanCode, &u.OverlayCidr, &u.MaxDevices, &u.NetmapVersion, &u.Status, &u.CreatedAt)
	return u, err
}

func (q *Queries) GetUserBySessionTokenHash(ctx context.Context, tokenHash string) (User, error) {
	row := q.db.QueryRow(ctx, `SELECT u.id, u.email, u.password_hash, u.plan_code, u.overlay_cidr::text, u.max_devices, u.netmap_version, u.status, u.created_at
FROM admin_sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = $1
  AND s.revoked_at IS NULL
  AND s.expires_at > now()
  AND u.status = 'active'`, tokenHash)
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.PlanCode, &u.OverlayCidr, &u.MaxDevices, &u.NetmapVersion, &u.Status, &u.CreatedAt)
	return u, err
}

func (q *Queries) LockOverlayIPAllocator(ctx context.Context) error {
	_, err := q.db.Exec(ctx, `SELECT pg_advisory_xact_lock(2026052801)`)
	return err
}

func (q *Queries) GetLastUserCIDR(ctx context.Context) (string, error) {
	var cidr string
	err := q.db.QueryRow(ctx, `SELECT overlay_cidr::text FROM users ORDER BY created_at DESC LIMIT 1`).Scan(&cidr)
	return cidr, err
}

func (q *Queries) BumpNetmapVersion(ctx context.Context, userID string) error {
	_, err := q.db.Exec(ctx, `UPDATE users SET netmap_version = netmap_version + 1 WHERE id = $1`, userID)
	return err
}

type CreateAdminSessionParams struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
}

func (q *Queries) CreateAdminSession(ctx context.Context, arg CreateAdminSessionParams) (AdminSession, error) {
	row := q.db.QueryRow(ctx, `INSERT INTO admin_sessions (id, user_id, token_hash, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, expires_at, revoked_at, created_at`,
		arg.ID, arg.UserID, arg.TokenHash, arg.ExpiresAt)
	var s AdminSession
	err := row.Scan(&s.ID, &s.UserID, &s.ExpiresAt, &s.RevokedAt, &s.CreatedAt)
	return s, err
}

func (q *Queries) ListPlans(ctx context.Context) ([]Plan, error) {
	rows, err := q.db.Query(ctx, `SELECT code, name, price_cents, max_devices, enable_subnet, enable_self_relay, created_at
FROM plans
ORDER BY price_cents ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Plan
	for rows.Next() {
		var p Plan
		if err := rows.Scan(&p.Code, &p.Name, &p.PriceCents, &p.MaxDevices, &p.EnableSubnet, &p.EnableSelfRelay, &p.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

type CreateDeviceParams struct {
	ID              string
	UserID          string
	Hostname        string
	OS              string
	Arch            string
	PublicKey       string
	VirtualIP       string
	DeviceTokenHash string
	ClientVersion   string
	OSVersion       string
}

func (q *Queries) CreateDevice(ctx context.Context, arg CreateDeviceParams) (Device, error) {
	row := q.db.QueryRow(ctx, `INSERT INTO devices (
  id, user_id, hostname, os, arch, public_key, virtual_ip,
  device_token_hash, client_version, os_version
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING id, user_id, hostname, os, arch, public_key, host(virtual_ip),
  status, client_version, os_version, last_seen_at, created_at`,
		arg.ID, arg.UserID, arg.Hostname, arg.OS, arg.Arch, arg.PublicKey, arg.VirtualIP,
		arg.DeviceTokenHash, arg.ClientVersion, arg.OSVersion)
	var d Device
	err := row.Scan(&d.ID, &d.UserID, &d.Hostname, &d.OS, &d.Arch, &d.PublicKey, &d.VirtualIP,
		&d.Status, &d.ClientVersion, &d.OSVersion, &d.LastSeenAt, &d.CreatedAt)
	return d, err
}

func (q *Queries) GetDeviceByTokenHash(ctx context.Context, tokenHash string) (Device, error) {
	row := q.db.QueryRow(ctx, `SELECT id, user_id, hostname, os, arch, public_key, host(virtual_ip),
  status, client_version, os_version, last_seen_at, created_at
FROM devices
WHERE device_token_hash = $1`, tokenHash)
	var d Device
	err := row.Scan(&d.ID, &d.UserID, &d.Hostname, &d.OS, &d.Arch, &d.PublicKey, &d.VirtualIP,
		&d.Status, &d.ClientVersion, &d.OSVersion, &d.LastSeenAt, &d.CreatedAt)
	return d, err
}

func (q *Queries) GetDevice(ctx context.Context, id string) (Device, error) {
	row := q.db.QueryRow(ctx, `SELECT id, user_id, hostname, os, arch, public_key, host(virtual_ip),
  status, client_version, os_version, last_seen_at, created_at
FROM devices
WHERE id = $1`, id)
	var d Device
	err := row.Scan(&d.ID, &d.UserID, &d.Hostname, &d.OS, &d.Arch, &d.PublicKey, &d.VirtualIP,
		&d.Status, &d.ClientVersion, &d.OSVersion, &d.LastSeenAt, &d.CreatedAt)
	return d, err
}

func (q *Queries) GetDeviceByPublicKey(ctx context.Context, publicKey string) (Device, error) {
	row := q.db.QueryRow(ctx, `SELECT id, user_id, hostname, os, arch, public_key, host(virtual_ip),
  status, client_version, os_version, last_seen_at, created_at
FROM devices
WHERE public_key = $1
ORDER BY created_at DESC
LIMIT 1`, publicKey)
	var d Device
	err := row.Scan(&d.ID, &d.UserID, &d.Hostname, &d.OS, &d.Arch, &d.PublicKey, &d.VirtualIP,
		&d.Status, &d.ClientVersion, &d.OSVersion, &d.LastSeenAt, &d.CreatedAt)
	return d, err
}

func (q *Queries) ListDevicesByUser(ctx context.Context, userID string) ([]Device, error) {
	rows, err := q.db.Query(ctx, `SELECT id, user_id, hostname, os, arch, public_key, host(virtual_ip),
  status, client_version, os_version, last_seen_at, created_at
FROM devices
WHERE user_id = $1
ORDER BY virtual_ip ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.UserID, &d.Hostname, &d.OS, &d.Arch, &d.PublicKey, &d.VirtualIP,
			&d.Status, &d.ClientVersion, &d.OSVersion, &d.LastSeenAt, &d.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	return items, rows.Err()
}

func (q *Queries) ListActiveDevices(ctx context.Context) ([]Device, error) {
	rows, err := q.db.Query(ctx, `SELECT id, user_id, hostname, os, arch, public_key, host(virtual_ip),
  status, client_version, os_version, last_seen_at, created_at
FROM devices
WHERE status = 'active'
  AND public_key <> ''
ORDER BY virtual_ip ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.UserID, &d.Hostname, &d.OS, &d.Arch, &d.PublicKey, &d.VirtualIP,
			&d.Status, &d.ClientVersion, &d.OSVersion, &d.LastSeenAt, &d.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	return items, rows.Err()
}

func (q *Queries) CountActiveDevicesByUser(ctx context.Context, userID string) (int32, error) {
	var count int32
	err := q.db.QueryRow(ctx, `SELECT count(*)::int FROM devices WHERE user_id = $1 AND status = 'active'`, userID).Scan(&count)
	return count, err
}

func (q *Queries) UpdateDeviceHeartbeat(ctx context.Context, id, clientVersion, osVersion string) error {
	_, err := q.db.Exec(ctx, `UPDATE devices SET last_seen_at = now(), client_version = $2, os_version = $3 WHERE id = $1`,
		id, clientVersion, osVersion)
	return err
}

type UpsertDeviceEndpointParams struct {
	ID           string
	DeviceID     string
	EndpointType string
	Address      string
	Source       string
	RttMs        *int32
}

func (q *Queries) UpsertDeviceEndpoint(ctx context.Context, arg UpsertDeviceEndpointParams) (bool, error) {
	row := q.db.QueryRow(ctx, `INSERT INTO device_endpoints (id, device_id, endpoint_type, address, source, rtt_ms, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, now())
ON CONFLICT (device_id, endpoint_type, address)
DO UPDATE SET source = EXCLUDED.source, rtt_ms = EXCLUDED.rtt_ms, updated_at = now()
WHERE device_endpoints.source IS DISTINCT FROM EXCLUDED.source
RETURNING id`,
		arg.ID, arg.DeviceID, arg.EndpointType, arg.Address, arg.Source, arg.RttMs)
	var id string
	err := row.Scan(&id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (q *Queries) ListEndpointsByUser(ctx context.Context, userID string) ([]DeviceEndpoint, error) {
	rows, err := q.db.Query(ctx, `SELECT e.id, e.device_id, e.endpoint_type, e.address, e.source, e.rtt_ms, e.updated_at
FROM device_endpoints e
JOIN devices d ON d.id = e.device_id
WHERE d.user_id = $1 AND d.status = 'active'
ORDER BY e.updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []DeviceEndpoint
	for rows.Next() {
		var e DeviceEndpoint
		if err := rows.Scan(&e.ID, &e.DeviceID, &e.EndpointType, &e.Address, &e.Source, &e.RttMs, &e.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	return items, rows.Err()
}

func (q *Queries) ListEndpointsByDevice(ctx context.Context, deviceID string) ([]DeviceEndpoint, error) {
	rows, err := q.db.Query(ctx, `SELECT id, device_id, endpoint_type, address, source, rtt_ms, updated_at
FROM device_endpoints
WHERE device_id = $1
ORDER BY updated_at DESC`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []DeviceEndpoint
	for rows.Next() {
		var e DeviceEndpoint
		if err := rows.Scan(&e.ID, &e.DeviceID, &e.EndpointType, &e.Address, &e.Source, &e.RttMs, &e.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	return items, rows.Err()
}

func (q *Queries) ListSubnetRoutesByUser(ctx context.Context, userID string) ([]SubnetRoute, error) {
	rows, err := q.db.Query(ctx, `SELECT id, user_id, device_id, cidr::text, status, advertised, approved, created_at, updated_at
FROM subnet_routes
WHERE user_id = $1
ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []SubnetRoute
	for rows.Next() {
		var r SubnetRoute
		if err := rows.Scan(&r.ID, &r.UserID, &r.DeviceID, &r.Cidr, &r.Status, &r.Advertised, &r.Approved, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, r)
	}
	return items, rows.Err()
}

func (q *Queries) ListRelaysByUser(ctx context.Context, userID string) ([]Relay, error) {
	rows, err := q.db.Query(ctx, `SELECT id, user_id, name, public_key, endpoint, status, last_seen_at, created_at
FROM relays
WHERE user_id = $1
ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Relay
	for rows.Next() {
		var r Relay
		if err := rows.Scan(&r.ID, &r.UserID, &r.Name, &r.PublicKey, &r.Endpoint, &r.Status, &r.LastSeenAt, &r.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, r)
	}
	return items, rows.Err()
}
