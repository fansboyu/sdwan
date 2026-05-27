-- name: CreateAdminUser :one
INSERT INTO admin_users (id, email, password_hash)
VALUES ($1, $2, $3)
RETURNING id, email, status, created_at;

-- name: GetAdminUserByEmail :one
SELECT id, email, password_hash, status, created_at
FROM admin_users
WHERE email = $1;

-- name: CreateAdminSession :one
INSERT INTO admin_sessions (id, admin_user_id, token_hash, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING id, admin_user_id, expires_at, revoked_at, created_at;

-- name: GetAdminUserBySessionTokenHash :one
SELECT u.id, u.email, u.status, u.created_at
FROM admin_sessions s
JOIN admin_users u ON u.id = s.admin_user_id
WHERE s.token_hash = $1
  AND s.revoked_at IS NULL
  AND s.expires_at > now()
  AND u.status = 'active';
