#!/usr/bin/env sh
set -eu

VERSION="${SDWAN_VERSION:-v1.1.1}"
CONTROLLER_URL="${SDWAN_CONTROLLER_URL:-https://controller.englishlisten.cn}"
DOWNLOAD_BASE="${SDWAN_DOWNLOAD_BASE:-${CONTROLLER_URL}/downloads/${VERSION}}"
BIN_NAME="sdwan-agent-linux-amd64"
INSTALL_PATH="${SDWAN_INSTALL_PATH:-/usr/local/bin/sdwan-agent}"
CONFIG_DIR="/etc/sdwan"
WG_DIR="/etc/wireguard"
SERVICE_PATH="/etc/systemd/system/sdwan-agent.service"

log() {
  printf '%s\n' "$*"
}

require_root() {
  if [ "$(id -u)" -ne 0 ]; then
    log "This installer must run as root. Try: curl -fsSL ${CONTROLLER_URL}/install.sh | sudo sh"
    exit 1
  fi
}

detect_platform() {
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  if [ "$os" != "linux" ]; then
    log "Unsupported OS: ${os}. This installer currently supports Linux only."
    exit 1
  fi
  case "$arch" in
    x86_64|amd64)
      BIN_NAME="sdwan-agent-linux-amd64"
      ;;
    *)
      log "Unsupported CPU architecture: ${arch}. Current release only provides linux amd64 binary."
      exit 1
      ;;
  esac
}

install_deps() {
  if command -v apt-get >/dev/null 2>&1; then
    apt-get update
    DEBIAN_FRONTEND=noninteractive apt-get install -y ca-certificates curl wireguard-tools
    return
  fi
  if command -v dnf >/dev/null 2>&1; then
    dnf install -y ca-certificates curl wireguard-tools
    return
  fi
  if command -v yum >/dev/null 2>&1; then
    yum install -y ca-certificates curl wireguard-tools
    return
  fi
  if command -v apk >/dev/null 2>&1; then
    apk add --no-cache ca-certificates curl wireguard-tools
    return
  fi
  if command -v pacman >/dev/null 2>&1; then
    pacman -Sy --noconfirm ca-certificates curl wireguard-tools
    return
  fi
  log "No supported package manager found. Please install curl and wireguard-tools manually."
}

install_binary() {
  tmp_path="$(mktemp)"
  url="${DOWNLOAD_BASE}/${BIN_NAME}"
  log "Downloading ${url}"
  curl -fsSL "$url" -o "$tmp_path"
  chmod 0755 "$tmp_path"
  mv "$tmp_path" "$INSTALL_PATH"
}

write_service() {
  mkdir -p "$CONFIG_DIR" "$WG_DIR"
  cat > "$SERVICE_PATH" <<EOF
[Unit]
Description=SD-WAN Linux Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${INSTALL_PATH} daemon --config ${CONFIG_DIR}/agent.json --wg-config ${WG_DIR}/sdwan0.conf
Restart=always
RestartSec=5
User=root

[Install]
WantedBy=multi-user.target
EOF

  if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload
  fi
}

main() {
  require_root
  detect_platform
  install_deps
  install_binary
  write_service

  log "sdwan-agent ${VERSION} installed to ${INSTALL_PATH}"
  log "Next step:"
  log "  sudo sdwan-agent register --controller ${CONTROLLER_URL} --admin-token sdwan_admin_xxx"
  log "  sudo systemctl enable --now sdwan-agent"
}

main "$@"
