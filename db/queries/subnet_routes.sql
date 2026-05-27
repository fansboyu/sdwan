-- name: ListSubnetRoutesByUser :many
SELECT id, user_id, device_id, cidr::text, status, advertised, approved, created_at, updated_at
FROM subnet_routes
WHERE user_id = $1
ORDER BY created_at DESC;
