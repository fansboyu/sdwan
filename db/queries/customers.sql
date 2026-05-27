-- name: CreateCustomer :one
INSERT INTO customers (id, name, address_cidr, max_devices)
VALUES ($1, $2, $3, $4)
RETURNING id, name, address_cidr::text, max_devices, netmap_version, status, created_at;

-- name: ListCustomers :many
SELECT id, name, address_cidr::text, max_devices, netmap_version, status, created_at
FROM customers
ORDER BY created_at DESC;

-- name: GetCustomer :one
SELECT id, name, address_cidr::text, max_devices, netmap_version, status, created_at
FROM customers
WHERE id = $1;

-- name: GetLastCustomerCIDR :one
SELECT address_cidr::text
FROM customers
ORDER BY created_at DESC
LIMIT 1;

-- name: BumpNetmapVersion :exec
UPDATE customers
SET netmap_version = netmap_version + 1
WHERE id = $1;
