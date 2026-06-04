-- name: UpsertDeviceEndpoint :one
WITH upserted AS (
INSERT INTO device_endpoints (id, device_id, endpoint_type, address, source, rtt_ms, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, now())
ON CONFLICT (device_id, endpoint_type, address)
DO UPDATE SET source = EXCLUDED.source, rtt_ms = EXCLUDED.rtt_ms, updated_at = now()
RETURNING xmax = 0 AS inserted
)
SELECT inserted FROM upserted;

-- name: PruneDeviceEndpoints :execrows
WITH ranked AS (
  SELECT
    id,
    row_number() OVER (
      PARTITION BY device_id, endpoint_type
      ORDER BY updated_at DESC, id DESC
    ) AS rn
  FROM device_endpoints
  WHERE device_id = $1
    AND endpoint_type = $2
)
DELETE FROM device_endpoints
WHERE id IN (
  SELECT id FROM ranked WHERE rn > $3
);

-- name: ListEndpointsByUser :many
SELECT e.id, e.device_id, e.endpoint_type, e.address, e.source, e.rtt_ms, e.updated_at
FROM device_endpoints e
JOIN devices d ON d.id = e.device_id
WHERE d.user_id = $1 AND d.status = 'active'
ORDER BY e.updated_at DESC;
