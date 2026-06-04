-- name: CreateSubscription :one
INSERT INTO subscriptions (
  id, user_id, plan_code, status, source, free_months, starts_at, expires_at
) VALUES (
  $1, $2, $3, 'active', $4, $5, $6, $7
)
RETURNING id, user_id, plan_code, status, source, free_months, starts_at, expires_at, created_at;

-- name: GetActiveSubscriptionByUser :one
SELECT id, user_id, plan_code, status, source, free_months, starts_at, expires_at, created_at
FROM subscriptions
WHERE user_id = $1
  AND status = 'active'
ORDER BY created_at DESC
LIMIT 1;

-- name: UpdateActiveSubscriptionPlan :one
UPDATE subscriptions
SET plan_code = $2
WHERE id = $1
  AND status = 'active'
RETURNING id, user_id, plan_code, status, source, free_months, starts_at, expires_at, created_at;

-- name: SumFreeUpgradeMonthsByUser :one
SELECT COALESCE(sum(free_months), 0)::int
FROM subscriptions
WHERE user_id = $1
  AND source = 'free_upgrade';

-- name: CancelActiveSubscriptionsByUser :execrows
UPDATE subscriptions
SET status = 'canceled'
WHERE user_id = $1
  AND status = 'active';

-- name: ExpireActiveSubscriptionsByUser :execrows
UPDATE subscriptions
SET status = 'expired'
WHERE user_id = $1
  AND status = 'active'
  AND expires_at IS NOT NULL
  AND expires_at <= now();
