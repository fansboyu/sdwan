-- name: ListRelaysByUser :many
SELECT id, user_id, name, public_key, endpoint, status, last_seen_at, created_at
FROM relays
WHERE user_id = $1
ORDER BY created_at DESC;
