#!/usr/bin/env sh
set -eu

VERSION="${SDWAN_VERSION:-v1.2.1}"
CONTROLLER_URL="${SDWAN_CONTROLLER_URL:-https://controller.englishlisten.cn}"
DOWNLOAD_BASE="${SDWAN_DOWNLOAD_BASE:-${CONTROLLER_URL}/downloads/${VERSION}}"
INSTALL_PATH="${SDWAN_RELAY_INSTALL_PATH:-/usr/local/bin/sdwan-relay-agent}"
CONFIG_DIR="/etc/sdwan"
CONFIG_PATH="${CONFIG_DIR}/relay-agent.json"
UNIT_PATH="/etc/systemd/system/sdwan-relay-agent.service"
INTERFACE="${SDWAN_RELAY_INTERFACE:-sdwan-relay}"
ARCH="$(uname -m)"
RELAY_TOKEN="${SDWAN_RELAY_TOKEN:-}"
TMP_BINARY=""
TMP_UNIT=""

log() { printf '%s\n' "$*"; }
fail() { log "Error: $*" >&2; exit 1; }
cleanup() {
  if [ -r /dev/tty ]; then stty echo </dev/tty 2>/dev/null || true; fi
  [ -z "$TMP_BINARY" ] || rm -f "$TMP_BINARY"
  [ -z "$TMP_UNIT" ] || rm -f "$TMP_UNIT" "${TMP_UNIT}.rendered"
}
trap cleanup EXIT HUP INT TERM

if [ "$(id -u)" -ne 0 ]; then
  fail "Run as root: curl -fsSL ${DOWNLOAD_BASE}/install-relay.sh | sudo sh"
fi
if [ "$ARCH" != "x86_64" ] && [ "$ARCH" != "amd64" ]; then
  fail "This release provides Linux amd64 only; detected ${ARCH}."
fi
command -v systemctl >/dev/null 2>&1 || fail "systemd is required."
command -v wg >/dev/null 2>&1 || fail "Install wireguard-tools and configure ${INTERFACE} first."
command -v curl >/dev/null 2>&1 || fail "Install curl first."
[ -f "/etc/wireguard/${INTERFACE}.conf" ] || fail "Missing /etc/wireguard/${INTERFACE}.conf. Prepare the WireGuard interface before installing the agent."
case "$INTERFACE" in
  *[!A-Za-z0-9_.-]*) fail "Unsupported interface name: ${INTERFACE}" ;;
esac

if [ -z "$RELAY_TOKEN" ]; then
  [ -r /dev/tty ] || fail "Set SDWAN_RELAY_TOKEN or run this installer from an interactive terminal."
  printf 'Paste the one-time Relay Token (input hidden): ' >/dev/tty
  stty -echo </dev/tty || fail "Could not disable terminal echo while reading the Relay Token."
  IFS= read -r RELAY_TOKEN </dev/tty || fail "Could not read the Relay Token."
  stty echo </dev/tty || true
  printf '\n' >/dev/tty
fi
[ -n "$RELAY_TOKEN" ] || fail "Relay Token cannot be empty."

TMP_BINARY="$(mktemp)"
TMP_UNIT="$(mktemp)"
curl -fsSL "${DOWNLOAD_BASE}/sdwan-relay-agent-linux-amd64" -o "$TMP_BINARY" || fail "Could not download Relay Agent ${VERSION}."

UNIT_SOURCE="$(dirname "$0")/../systemd/sdwan-relay-agent.service"
if [ -f "$UNIT_SOURCE" ]; then
  cp "$UNIT_SOURCE" "$TMP_UNIT"
else
  curl -fsSL "${DOWNLOAD_BASE}/sdwan-relay-agent.service" -o "$TMP_UNIT" || fail "Could not download the Relay Agent systemd unit."
fi
sed "s/wg-quick@sdwan-relay/wg-quick@${INTERFACE}/g" "$TMP_UNIT" > "${TMP_UNIT}.rendered"

if ! systemctl enable --now "wg-quick@${INTERFACE}.service"; then
  fail "Could not start wg-quick@${INTERFACE}.service; check its config and logs."
fi
wg show "$INTERFACE" >/dev/null 2>&1 || fail "WireGuard interface ${INTERFACE} is not up."

install -d -m 0700 "$CONFIG_DIR"
install -m 0755 "$TMP_BINARY" "$INSTALL_PATH"
install -m 0644 "${TMP_UNIT}.rendered" "$UNIT_PATH"
umask 077
printf '{\n  "controller_url": "%s",\n  "relay_token": "%s",\n  "interface_name": "%s",\n  "sync_interval_seconds": 5\n}\n' \
  "$CONTROLLER_URL" "$RELAY_TOKEN" "$INTERFACE" > "${CONFIG_PATH}.tmp"
chmod 0600 "${CONFIG_PATH}.tmp"
mv "${CONFIG_PATH}.tmp" "$CONFIG_PATH"
unset RELAY_TOKEN

systemctl daemon-reload
systemctl enable --now sdwan-relay-agent.service || {
  systemctl --no-pager --full status sdwan-relay-agent.service || true
  fail "Relay Agent failed to start. Inspect: journalctl -u sdwan-relay-agent -n 100"
}

log "Relay Agent ${VERSION} is active."
log "Check: systemctl status sdwan-relay-agent --no-pager"
log "Logs:  journalctl -u sdwan-relay-agent -n 100 --no-pager"
if command -v sysctl >/dev/null 2>&1 && [ "$(sysctl -n net.ipv4.ip_forward 2>/dev/null || printf 0)" != "1" ]; then
  log "Warning: IPv4 forwarding is disabled; Relay peer sync may work, but traffic forwarding will not."
fi
