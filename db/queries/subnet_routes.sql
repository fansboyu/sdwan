-- name: ListSubnetRoutesByUser :many
SELECT id, user_id, device_id, cidr::text, status, advertised, approved, created_at, updated_at
FROM subnet_routes
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: ListActiveSubnetRoutesByUser :many
SELECT id, user_id, device_id, cidr::text, status, advertised, approved, created_at, updated_at
FROM subnet_routes
WHERE user_id = $1
  AND status = 'active'
  AND advertised = true
  AND approved = true
ORDER BY cidr ASC;

-- name: UpsertAdvertisedSubnetRoute :one
WITH upserted AS (
  INSERT INTO subnet_routes (id, user_id, device_id, cidr, status, advertised, approved, updated_at)
  VALUES ($1, $2, $3, $4::cidr, 'pending', true, false, now())
  ON CONFLICT (user_id, cidr)
  DO UPDATE SET
    device_id = EXCLUDED.device_id,
    advertised = true,
    status = CASE WHEN subnet_routes.approved THEN 'active' ELSE 'pending' END,
    updated_at = now()
  WHERE subnet_routes.device_id IS DISTINCT FROM EXCLUDED.device_id
     OR subnet_routes.advertised = false
     OR subnet_routes.status <> CASE WHEN subnet_routes.approved THEN 'active' ELSE 'pending' END
  RETURNING 1
)
SELECT EXISTS(SELECT 1 FROM upserted) AS changed;

-- name: DisableDeviceSubnetRoutesNotIn :execrows
UPDATE subnet_routes
SET advertised = false,
    status = 'inactive',
    updated_at = now()
WHERE device_id = $1
  AND advertised = true
  AND NOT (cidr::text = ANY($2::text[]));

-- name: DisableSubnetRoutesExceptDevice :execrows
UPDATE subnet_routes
SET advertised = false,
    status = 'inactive',
    updated_at = now()
WHERE user_id = $1
  AND device_id <> $2
  AND advertised = true;

-- name: SetSubnetRouteApproved :one
UPDATE subnet_routes
SET approved = $3,
    status = CASE
      WHEN $3 AND advertised THEN 'active'
      WHEN advertised THEN 'pending'
      ELSE 'inactive'
    END,
    updated_at = now()
WHERE id = $1
  AND user_id = $2
RETURNING id, user_id, device_id, cidr::text, status, advertised, approved, created_at, updated_at;

-- name: DisableSubnetRoute :one
UPDATE subnet_routes
SET advertised = false,
    status = 'inactive',
    updated_at = now()
WHERE id = $1
  AND user_id = $2
RETURNING id, user_id, device_id, cidr::text, status, advertised, approved, created_at, updated_at;
