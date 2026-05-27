-- name: CreateAdminSession :one
INSERT INTO admin_sessions (id, user_id, token_hash, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, expires_at, revoked_at, created_at;
