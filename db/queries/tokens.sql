-- name: CreateJoinToken :one
INSERT INTO join_tokens (id, customer_id, token_hash, name, max_uses, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, customer_id, name, max_uses, used_count, expires_at, revoked_at, created_at;

-- name: GetJoinTokenByHash :one
SELECT id, customer_id, token_hash, name, max_uses, used_count, expires_at, revoked_at, created_at
FROM join_tokens
WHERE token_hash = $1;

-- name: IncrementJoinTokenUse :exec
UPDATE join_tokens
SET used_count = used_count + 1
WHERE id = $1;
