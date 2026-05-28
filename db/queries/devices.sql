-- name: CreateDevice :one
INSERT INTO devices (
  id, user_id, hostname, os, arch, public_key, virtual_ip,
  device_token_hash, client_version, os_version
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING id, user_id, hostname, os, arch, public_key, host(virtual_ip),
  status, client_version, os_version, last_seen_at, created_at;

-- name: GetDeviceByTokenHash :one
SELECT id, user_id, hostname, os, arch, public_key, host(virtual_ip),
  status, client_version, os_version, last_seen_at, created_at
FROM devices
WHERE device_token_hash = $1;

-- name: GetDevice :one
SELECT id, user_id, hostname, os, arch, public_key, host(virtual_ip),
  status, client_version, os_version, last_seen_at, created_at
FROM devices
WHERE id = $1;

-- name: GetDeviceByPublicKey :one
SELECT id, user_id, hostname, os, arch, public_key, host(virtual_ip),
  status, client_version, os_version, last_seen_at, created_at
FROM devices
WHERE public_key = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: ListDevicesByUser :many
SELECT id, user_id, hostname, os, arch, public_key, host(virtual_ip),
  status, client_version, os_version, last_seen_at, created_at
FROM devices
WHERE user_id = $1
ORDER BY virtual_ip ASC;

-- name: ListActiveDevices :many
SELECT id, user_id, hostname, os, arch, public_key, host(virtual_ip),
  status, client_version, os_version, last_seen_at, created_at
FROM devices
WHERE status = 'active'
  AND public_key <> ''
ORDER BY virtual_ip ASC;

-- name: CountActiveDevicesByUser :one
SELECT count(*)::int
FROM devices
WHERE user_id = $1 AND status = 'active';

-- name: UpdateDeviceHeartbeat :exec
UPDATE devices
SET last_seen_at = now(), client_version = $2, os_version = $3
WHERE id = $1;

-- name: DisableDevice :exec
UPDATE devices
SET status = 'disabled'
WHERE id = $1;

-- name: ListEndpointsByDevice :many
SELECT id, device_id, endpoint_type, address, source, rtt_ms, updated_at
FROM device_endpoints
WHERE device_id = $1
ORDER BY updated_at DESC;
