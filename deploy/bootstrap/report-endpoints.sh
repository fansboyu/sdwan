#!/usr/bin/env sh
set -eu

IFACE="${BOOTSTRAP_WG_INTERFACE:-sdwan-bootstrap}"
CONTROLLER_URL="${CONTROLLER_URL:-https://controller.englishlisten.cn}"

if [ -z "${BOOTSTRAP_REPORT_TOKEN:-}" ]; then
  echo "BOOTSTRAP_REPORT_TOKEN is required" >&2
  exit 1
fi

wg show "$IFACE" dump | awk 'NR > 1 && $3 != "(none)" { print $1 " " $3 }' | while read -r public_key endpoint; do
  curl -fsS \
    -X POST "${CONTROLLER_URL%/}/api/v1/bootstrap/endpoints" \
    -H "Authorization: Bearer ${BOOTSTRAP_REPORT_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{\"public_key\":\"${public_key}\",\"endpoint\":\"${endpoint}\"}" >/dev/null
done
