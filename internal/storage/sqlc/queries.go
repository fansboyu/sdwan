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

type CreateCustomerParams struct {
	ID          string
	Name        string
	AddressCidr string
	MaxDevices  int32
}

func (q *Queries) LockCustomerIPAllocator(ctx context.Context) error {
	_, err := q.db.Exec(ctx, `SELECT pg_advisory_xact_lock(2026052701)`)
	return err
}

type CreateAdminUserParams struct {
	ID           string
	Email        string
	PasswordHash string
}

func (q *Queries) CreateAdminUser(ctx context.Context, arg CreateAdminUserParams) (AdminUser, error) {
	row := q.db.QueryRow(ctx, `INSERT INTO admin_users (id, email, password_hash)
VALUES ($1, $2, $3)
RETURNING id, email, status, created_at`, arg.ID, arg.Email, arg.PasswordHash)
	var u AdminUser
	err := row.Scan(&u.ID, &u.Email, &u.Status, &u.CreatedAt)
	return u, err
}

func (q *Queries) GetAdminUserByEmail(ctx context.Context, email string) (AdminUser, error) {
	row := q.db.QueryRow(ctx, `SELECT id, email, password_hash, status, created_at
FROM admin_users
WHERE email = $1`, email)
	var u AdminUser
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Status, &u.CreatedAt)
	return u, err
}

type CreateAdminSessionParams struct {
	ID          string
	AdminUserID string
	TokenHash   string
	ExpiresAt   time.Time
}

func (q *Queries) CreateAdminSession(ctx context.Context, arg CreateAdminSessionParams) (AdminSession, error) {
	row := q.db.QueryRow(ctx, `INSERT INTO admin_sessions (id, admin_user_id, token_hash, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING id, admin_user_id, expires_at, revoked_at, created_at`,
		arg.ID, arg.AdminUserID, arg.TokenHash, arg.ExpiresAt)
	var s AdminSession
	err := row.Scan(&s.ID, &s.AdminUserID, &s.ExpiresAt, &s.RevokedAt, &s.CreatedAt)
	return s, err
}

func (q *Queries) GetAdminUserBySessionTokenHash(ctx context.Context, tokenHash string) (AdminUser, error) {
	row := q.db.QueryRow(ctx, `SELECT u.id, u.email, u.status, u.created_at
FROM admin_sessions s
JOIN admin_users u ON u.id = s.admin_user_id
WHERE s.token_hash = $1
  AND s.revoked_at IS NULL
  AND s.expires_at > now()
  AND u.status = 'active'`, tokenHash)
	var u AdminUser
	err := row.Scan(&u.ID, &u.Email, &u.Status, &u.CreatedAt)
	return u, err
}

func (q *Queries) CreateCustomer(ctx context.Context, arg CreateCustomerParams) (Customer, error) {
	row := q.db.QueryRow(ctx, `INSERT INTO customers (id, name, address_cidr, max_devices)
VALUES ($1, $2, $3, $4)
RETURNING id, name, address_cidr::text, max_devices, netmap_version, status, created_at`,
		arg.ID, arg.Name, arg.AddressCidr, arg.MaxDevices)
	var c Customer
	err := row.Scan(&c.ID, &c.Name, &c.AddressCidr, &c.MaxDevices, &c.NetmapVersion, &c.Status, &c.CreatedAt)
	return c, err
}

func (q *Queries) ListCustomers(ctx context.Context) ([]Customer, error) {
	rows, err := q.db.Query(ctx, `SELECT id, name, address_cidr::text, max_devices, netmap_version, status, created_at
FROM customers
ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Customer
	for rows.Next() {
		var c Customer
		if err := rows.Scan(&c.ID, &c.Name, &c.AddressCidr, &c.MaxDevices, &c.NetmapVersion, &c.Status, &c.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (q *Queries) GetCustomer(ctx context.Context, id string) (Customer, error) {
	row := q.db.QueryRow(ctx, `SELECT id, name, address_cidr::text, max_devices, netmap_version, status, created_at
FROM customers
WHERE id = $1`, id)
	var c Customer
	err := row.Scan(&c.ID, &c.Name, &c.AddressCidr, &c.MaxDevices, &c.NetmapVersion, &c.Status, &c.CreatedAt)
	return c, err
}

func (q *Queries) GetLastCustomerCIDR(ctx context.Context) (string, error) {
	var cidr string
	err := q.db.QueryRow(ctx, `SELECT address_cidr::text FROM customers ORDER BY created_at DESC LIMIT 1`).Scan(&cidr)
	return cidr, err
}

func (q *Queries) BumpNetmapVersion(ctx context.Context, customerID string) error {
	_, err := q.db.Exec(ctx, `UPDATE customers SET netmap_version = netmap_version + 1 WHERE id = $1`, customerID)
	return err
}

type CreateJoinTokenParams struct {
	ID         string
	CustomerID string
	TokenHash  string
	Name       string
	MaxUses    int32
	ExpiresAt  *time.Time
}

func (q *Queries) CreateJoinToken(ctx context.Context, arg CreateJoinTokenParams) (JoinToken, error) {
	row := q.db.QueryRow(ctx, `INSERT INTO join_tokens (id, customer_id, token_hash, name, max_uses, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, customer_id, token_hash, name, max_uses, used_count, expires_at, revoked_at, created_at`,
		arg.ID, arg.CustomerID, arg.TokenHash, arg.Name, arg.MaxUses, arg.ExpiresAt)
	var t JoinToken
	err := row.Scan(&t.ID, &t.CustomerID, &t.TokenHash, &t.Name, &t.MaxUses, &t.UsedCount, &t.ExpiresAt, &t.RevokedAt, &t.CreatedAt)
	return t, err
}

func (q *Queries) GetJoinTokenByHash(ctx context.Context, tokenHash string) (JoinToken, error) {
	row := q.db.QueryRow(ctx, `SELECT id, customer_id, token_hash, name, max_uses, used_count, expires_at, revoked_at, created_at
FROM join_tokens
WHERE token_hash = $1`, tokenHash)
	var t JoinToken
	err := row.Scan(&t.ID, &t.CustomerID, &t.TokenHash, &t.Name, &t.MaxUses, &t.UsedCount, &t.ExpiresAt, &t.RevokedAt, &t.CreatedAt)
	return t, err
}

func (q *Queries) IncrementJoinTokenUse(ctx context.Context, id string) error {
	_, err := q.db.Exec(ctx, `UPDATE join_tokens SET used_count = used_count + 1 WHERE id = $1`, id)
	return err
}

type CreateDeviceParams struct {
	ID              string
	CustomerID      string
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
  id, customer_id, hostname, os, arch, public_key, virtual_ip,
  device_token_hash, client_version, os_version
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING id, customer_id, hostname, os, arch, public_key, host(virtual_ip),
  status, client_version, os_version, last_seen_at, created_at`,
		arg.ID, arg.CustomerID, arg.Hostname, arg.OS, arg.Arch, arg.PublicKey, arg.VirtualIP,
		arg.DeviceTokenHash, arg.ClientVersion, arg.OSVersion)
	var d Device
	err := row.Scan(&d.ID, &d.CustomerID, &d.Hostname, &d.OS, &d.Arch, &d.PublicKey, &d.VirtualIP,
		&d.Status, &d.ClientVersion, &d.OSVersion, &d.LastSeenAt, &d.CreatedAt)
	return d, err
}

func (q *Queries) GetDeviceByTokenHash(ctx context.Context, tokenHash string) (Device, error) {
	row := q.db.QueryRow(ctx, `SELECT id, customer_id, hostname, os, arch, public_key, host(virtual_ip),
  status, client_version, os_version, last_seen_at, created_at
FROM devices
WHERE device_token_hash = $1`, tokenHash)
	var d Device
	err := row.Scan(&d.ID, &d.CustomerID, &d.Hostname, &d.OS, &d.Arch, &d.PublicKey, &d.VirtualIP,
		&d.Status, &d.ClientVersion, &d.OSVersion, &d.LastSeenAt, &d.CreatedAt)
	return d, err
}

func (q *Queries) ListDevicesByCustomer(ctx context.Context, customerID string) ([]Device, error) {
	rows, err := q.db.Query(ctx, `SELECT id, customer_id, hostname, os, arch, public_key, host(virtual_ip),
  status, client_version, os_version, last_seen_at, created_at
FROM devices
WHERE customer_id = $1
ORDER BY virtual_ip ASC`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.CustomerID, &d.Hostname, &d.OS, &d.Arch, &d.PublicKey, &d.VirtualIP,
			&d.Status, &d.ClientVersion, &d.OSVersion, &d.LastSeenAt, &d.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	return items, rows.Err()
}

func (q *Queries) CountActiveDevicesByCustomer(ctx context.Context, customerID string) (int32, error) {
	var count int32
	err := q.db.QueryRow(ctx, `SELECT count(*)::int FROM devices WHERE customer_id = $1 AND status = 'active'`, customerID).Scan(&count)
	return count, err
}

func (q *Queries) UpdateDeviceHeartbeat(ctx context.Context, id, clientVersion, osVersion string) error {
	_, err := q.db.Exec(ctx, `UPDATE devices SET last_seen_at = now(), client_version = $2, os_version = $3 WHERE id = $1`,
		id, clientVersion, osVersion)
	return err
}

func (q *Queries) DisableDevice(ctx context.Context, id string) error {
	_, err := q.db.Exec(ctx, `UPDATE devices SET status = 'disabled' WHERE id = $1`, id)
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

func (q *Queries) UpsertDeviceEndpoint(ctx context.Context, arg UpsertDeviceEndpointParams) error {
	_, err := q.db.Exec(ctx, `INSERT INTO device_endpoints (id, device_id, endpoint_type, address, source, rtt_ms, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, now())
ON CONFLICT (device_id, endpoint_type, address)
DO UPDATE SET source = EXCLUDED.source, rtt_ms = EXCLUDED.rtt_ms, updated_at = now()`,
		arg.ID, arg.DeviceID, arg.EndpointType, arg.Address, arg.Source, arg.RttMs)
	return err
}

func (q *Queries) ListEndpointsByCustomer(ctx context.Context, customerID string) ([]DeviceEndpoint, error) {
	rows, err := q.db.Query(ctx, `SELECT e.id, e.device_id, e.endpoint_type, e.address, e.source, e.rtt_ms, e.updated_at
FROM device_endpoints e
JOIN devices d ON d.id = e.device_id
WHERE d.customer_id = $1 AND d.status = 'active'
ORDER BY e.updated_at DESC`, customerID)
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
