-- name: UpsertDeviceEndpoint :exec
INSERT INTO device_endpoints (id, device_id, endpoint_type, address, source, rtt_ms, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, now())
ON CONFLICT (device_id, endpoint_type, address)
DO UPDATE SET source = EXCLUDED.source, rtt_ms = EXCLUDED.rtt_ms, updated_at = now()
WHERE device_endpoints.source IS DISTINCT FROM EXCLUDED.source
   OR device_endpoints.rtt_ms IS DISTINCT FROM EXCLUDED.rtt_ms
RETURNING id;

-- name: ListEndpointsByUser :many
SELECT e.id, e.device_id, e.endpoint_type, e.address, e.source, e.rtt_ms, e.updated_at
FROM device_endpoints e
JOIN devices d ON d.id = e.device_id
WHERE d.user_id = $1 AND d.status = 'active'
ORDER BY e.updated_at DESC;
