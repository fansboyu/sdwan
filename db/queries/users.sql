-- name: CreateUser :one
INSERT INTO users (id, email, password_hash, overlay_cidr, max_devices)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, email, password_hash, plan_code, overlay_cidr::text, max_devices, netmap_version, status, created_at;

-- name: GetUser :one
SELECT id, email, password_hash, plan_code, overlay_cidr::text, max_devices, netmap_version, status, created_at
FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, plan_code, overlay_cidr::text, max_devices, netmap_version, status, created_at
FROM users
WHERE email = $1;

-- name: GetUserBySessionTokenHash :one
SELECT u.id, u.email, u.password_hash, u.plan_code, u.overlay_cidr::text, u.max_devices, u.netmap_version, u.status, u.created_at
FROM admin_sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = $1
  AND s.revoked_at IS NULL
  AND s.expires_at > now()
  AND u.status = 'active';

-- name: LockOverlayIPAllocator :exec
SELECT pg_advisory_xact_lock(2026052801);

-- name: GetLastUserCIDR :one
SELECT overlay_cidr::text FROM users ORDER BY created_at DESC LIMIT 1;

-- name: BumpNetmapVersion :exec
UPDATE users SET netmap_version = netmap_version + 1 WHERE id = $1;
