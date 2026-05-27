-- name: ListPlans :many
SELECT code, name, price_cents, max_devices, enable_subnet, enable_self_relay, created_at
FROM plans
ORDER BY price_cents ASC;
