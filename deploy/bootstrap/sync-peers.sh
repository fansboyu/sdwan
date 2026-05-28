#!/usr/bin/env sh
set -eu

IFACE="${BOOTSTRAP_WG_INTERFACE:-sdwan-bootstrap}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-sdwan-postgres-1}"
POSTGRES_USER="${POSTGRES_USER:-sdwan}"
POSTGRES_DB="${POSTGRES_DB:-sdwan}"

query_devices() {
  if command -v docker >/dev/null 2>&1 && docker ps --format '{{.Names}}' | grep -qx "$POSTGRES_CONTAINER"; then
    docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -F ' ' \
      -c "SELECT public_key, host(virtual_ip) FROM devices WHERE status = 'active' AND public_key <> '' ORDER BY virtual_ip ASC;"
    return
  fi

  if [ -z "${DATABASE_URL:-}" ]; then
    echo "DATABASE_URL is required when ${POSTGRES_CONTAINER} is not running locally" >&2
    exit 1
  fi

  psql "$DATABASE_URL" -At -F ' ' \
    -c "SELECT public_key, host(virtual_ip) FROM devices WHERE status = 'active' AND public_key <> '' ORDER BY virtual_ip ASC;"
}

if ! wg show "$IFACE" >/dev/null 2>&1; then
  echo "WireGuard interface ${IFACE} does not exist" >&2
  exit 1
fi

query_devices | while read -r public_key virtual_ip; do
  if [ -z "${public_key:-}" ] || [ -z "${virtual_ip:-}" ]; then
    continue
  fi
  wg set "$IFACE" peer "$public_key" allowed-ips "${virtual_ip}/32" persistent-keepalive 25
done
