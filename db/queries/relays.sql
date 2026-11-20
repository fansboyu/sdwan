-- name: ListRelaysByUser :many
SELECT id, user_id, name, public_key, host(virtual_ip), endpoint, status, last_seen_at,
       interface_up, synced_peer_count, version, last_error, last_sync_at, sync_ok, handshake_peer_count, last_handshake_at, created_at
FROM relays
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: CreateRelay :one
INSERT INTO relays (id, user_id, name, public_key, relay_token_hash, virtual_ip, endpoint, status)
VALUES ($1, $2, $3, $4, $5, $6::inet, $7, 'pending')
RETURNING id, user_id, name, public_key, host(virtual_ip), endpoint, status, last_seen_at,
          interface_up, synced_peer_count, version, last_error, last_sync_at, sync_ok, handshake_peer_count, last_handshake_at, created_at;

-- name: GetRelay :one
SELECT id, user_id, name, public_key, host(virtual_ip), endpoint, status, last_seen_at,
       interface_up, synced_peer_count, version, last_error, last_sync_at, sync_ok, handshake_peer_count, last_handshake_at, created_at
FROM relays
WHERE id = $1;

-- name: GetRelayByTokenHash :one
SELECT id, user_id, name, public_key, host(virtual_ip), endpoint, status, last_seen_at,
       interface_up, synced_peer_count, version, last_error, last_sync_at, sync_ok, handshake_peer_count, last_handshake_at, created_at
FROM relays
WHERE relay_token_hash = $1;

-- name: GetActiveRelayByUser :one
SELECT id, user_id, name, public_key, host(virtual_ip), endpoint, status, last_seen_at,
       interface_up, synced_peer_count, version, last_error, last_sync_at, sync_ok, handshake_peer_count, last_handshake_at, created_at
FROM relays
WHERE user_id = $1
  AND status = 'active'
ORDER BY created_at DESC
LIMIT 1;

-- name: DisableRelaysByUser :execrows
UPDATE relays
SET status = 'disabled'
WHERE user_id = $1
  AND status = 'active';

-- name: SetRelayStatus :one
UPDATE relays
SET status = $3
WHERE id = $1
  AND user_id = $2
RETURNING id, user_id, name, public_key, host(virtual_ip), endpoint, status, last_seen_at,
          interface_up, synced_peer_count, version, last_error, last_sync_at, sync_ok, handshake_peer_count, last_handshake_at, created_at;

-- name: UpdateRelayHeartbeat :exec
UPDATE relays
SET last_seen_at = now(),
    interface_up = $2,
    synced_peer_count = $3,
    handshake_peer_count = $4,
    last_handshake_at = $5,
    version = $6,
    last_error = $7,
    sync_ok = $8,
    last_sync_at = CASE WHEN $8 THEN now() ELSE last_sync_at END
WHERE id = $1;
