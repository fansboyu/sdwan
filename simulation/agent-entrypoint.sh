#!/bin/sh
set -eu

CONTROLLER_URL="${SDWAN_CONTROLLER_URL:-http://controller:8080}"
ADMIN_TOKEN="${SDWAN_ADMIN_TOKEN:-sdwan_admin_docker_lab_only}"
HOSTNAME="${SDWAN_DEVICE_HOSTNAME:-$(hostname)}"
ROLE="${SDWAN_SITE_ROLE:-client}"
CONFIG_PATH="/etc/sdwan/agent.json"
WG_PATH="/etc/wireguard/sdwan0.conf"

mkdir -p /etc/sdwan /etc/wireguard

if [ ! -s "$CONFIG_PATH" ]; then
  set -- register --controller "$CONTROLLER_URL" --admin-token "$ADMIN_TOKEN" --hostname "$HOSTNAME" --config "$CONFIG_PATH"
  if [ -n "${SDWAN_ADVERTISE_ROUTES:-}" ]; then
    set -- "$@" --advertise-routes "$SDWAN_ADVERTISE_ROUTES"
  fi
  sdwan-agent "$@"
fi

DEVICE_ID="$(jq -r '.device_id' "$CONFIG_PATH")"
DEVICE_TOKEN="$(jq -r '.device_token' "$CONFIG_PATH")"

if [ "$ROLE" = "main_site" ]; then
  curl -fsS -X POST \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    "${CONTROLLER_URL}/admin/devices/${DEVICE_ID}/main-site" >/dev/null
fi

if [ -n "${SDWAN_LAB_MANUAL_ENDPOINT:-}" ]; then
  curl -fsS -X POST \
    -H "Authorization: Bearer ${DEVICE_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "{\"current_netmap_version\":0,\"client_version\":\"v1.2.2\",\"os_version\":\"docker-lab\",\"endpoints\":[{\"type\":\"manual\",\"addr\":\"${SDWAN_LAB_MANUAL_ENDPOINT}\",\"source\":\"docker-lab\"}]}" \
    "${CONTROLLER_URL}/api/v1/devices/poll" >/dev/null
fi

exec sdwan-agent daemon --config "$CONFIG_PATH" --wg-config "$WG_PATH" --apply
