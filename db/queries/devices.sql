-- name: CreateDevice :one
INSERT INTO devices (
  id, customer_id, hostname, os, arch, public_key, virtual_ip,
  device_token_hash, client_version, os_version
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING id, customer_id, hostname, os, arch, public_key, host(virtual_ip),
  status, client_version, os_version, last_seen_at, created_at;

-- name: GetDeviceByTokenHash :one
SELECT id, customer_id, hostname, os, arch, public_key, host(virtual_ip),
  status, client_version, os_version, last_seen_at, created_at
FROM devices
WHERE device_token_hash = $1;

-- name: ListDevicesByCustomer :many
SELECT id, customer_id, hostname, os, arch, public_key, host(virtual_ip),
  status, client_version, os_version, last_seen_at, created_at
FROM devices
WHERE customer_id = $1
ORDER BY virtual_ip ASC;

-- name: CountActiveDevicesByCustomer :one
SELECT count(*)::int
FROM devices
WHERE customer_id = $1 AND status = 'active';

-- name: UpdateDeviceHeartbeat :exec
UPDATE devices
SET last_seen_at = now(), client_version = $2, os_version = $3
WHERE id = $1;

-- name: DisableDevice :exec
UPDATE devices
SET status = 'disabled'
WHERE id = $1;
