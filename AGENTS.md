# AGENTS.md

This file is for AI coding agents and human maintainers who need to understand this repository quickly.

## Project Summary

`sdwan` is a minimal Tailscale-like SD-WAN product.

Current version: `v1.2.0`

The project is currently an MVP with:

- A Go Controller API.
- PostgreSQL storage.
- A Vue admin console.
- A Linux Agent that renders WireGuard config and calls `wg-quick`.
- A host-level Bootstrap WireGuard interface named `sdwan-bootstrap`.
- A long-running `sdwan-bootstrap-agent` that syncs device public keys into `sdwan-bootstrap` and reports real WireGuard endpoints back to Controller.

The project intentionally does not implement full Tailscale/magicsock behavior yet.

Important current boundary:

```text
Controller manages identity, IP allocation, devices, netmap, and endpoints.
Linux Agent manages local config and calls kernel WireGuard.
kernel WireGuard handles encrypted packet transport.
sdwan-bootstrap-agent observes real WireGuard endpoint from the public bootstrap server.
Per-client automatic Relay fallback is implemented for the main-site topology.
STUN endpoint probing has been removed.
```

## Product Model

One account equals one overlay network.

- Users register and login by email.
- Login returns an `Admin Token`.
- The `Admin Token` is also used as the first-time device enrollment token.
- Each registered device gets a `Device Token`.
- Device polling and netmap use the `Device Token`.
- Each user gets an isolated `/24` under `100.64.0.0/10`.
- Device IP allocation skips `.0` and `.255`.
- Default max devices is 254.

Reserved future capabilities:

- `subnet_routes`
- `relays`
- `subscriptions`
- `audit_logs`

## Architecture

```text
Admin Browser
  |
  v
Web(Vue/nginx)
  |
  | /admin/*
  v
Controller(Go)
  |
  v
PostgreSQL


Linux Agent
  |
  | register / poll / netmap over HTTPS
  v
Controller

Linux Agent
  |
  | writes /etc/wireguard/sdwan0.conf
  | uses wg syncconf, with wg-quick fallback
  v
kernel WireGuard(sdwan0)
  |
  | UDP WireGuard packets
  v
Peers / sdwan-bootstrap


sdwan-bootstrap-agent
  |
  | GET /api/v1/bootstrap/peers
  | POST /api/v1/bootstrap/endpoints
  v
Controller
  |
  v
PostgreSQL

sdwan-bootstrap-agent
  |
  | wg set / wg show dump
  v
host WireGuard interface: sdwan-bootstrap
```

## NAT Endpoint Design

Do not reintroduce temporary STUN endpoint probing in the Linux Agent.

Reason:

```text
Agent temporary UDP socket != kernel WireGuard UDP socket
```

So STUN would report the NAT mapping of the wrong socket.

Current MVP route:

```text
1. Device registers public_key and virtual_ip with Controller.
2. bootstrap-agent fetches active devices from Controller.
3. bootstrap-agent runs:
   wg set sdwan-bootstrap peer PUBLIC_KEY allowed-ips VIRTUAL_IP/32 persistent-keepalive 25
4. Client kernel WireGuard sends handshake/keepalive to bootstrap endpoint.
5. Bootstrap server observes the real source IP:port of the kernel WireGuard socket.
6. bootstrap-agent reads `wg show sdwan-bootstrap dump`.
7. bootstrap-agent reports changed endpoints to Controller.
8. Controller stores endpoint_type=bootstrap in device_endpoints.
9. Other agents pull updated netmap and use bootstrap endpoint first.
```

Important limitation:

```text
Bootstrap can discover a real endpoint, but it does not guarantee P2P success for symmetric NAT.
Relay fallback is still required for production-grade connectivity.
```

## Important Runtime Ports

```text
Controller HTTP inside host: 127.0.0.1:18080
Web inside host:        127.0.0.1:8081
Bootstrap WireGuard:    UDP 51872
Linux Agent WireGuard:  UDP 41641
```

Production domain:

```text
controller.englishlisten.cn
```

Caddy normally routes:

```text
/        -> web:8081
/api/*   -> controller:18080
/admin/* -> controller:18080
/install.sh and /downloads/* -> /opt/sdwan/downloads
```

## Key Directories And Files

Controller:

```text
cmd/controller/main.go
internal/httpapi/router.go
internal/app/service.go
internal/config/config.go
internal/version/version.go
```

Linux Agent:

```text
cmd/agent/main.go
internal/agent/api.go
internal/agent/config.go
internal/agent/daemon.go
internal/agent/endpoint.go
internal/agent/wireguard.go
```

Bootstrap Agent:

```text
cmd/bootstrap-agent/main.go
internal/bootstrapagent/api.go
internal/bootstrapagent/config.go
internal/bootstrapagent/runner.go
internal/bootstrapagent/wg.go
deploy/systemd/sdwan-bootstrap-agent.service
```

Storage:

```text
db/migrations/000001_init.up.sql
db/migrations/000001_init.down.sql
db/queries/*.sql
internal/storage/sqlc/models.go
internal/storage/sqlc/queries.go
internal/storage/storage.go
```

Frontend:

```text
web/src/App.vue
web/src/style.css
web/package.json
web/Dockerfile
```

Deployment:

```text
docker-compose.yml
Dockerfile
deploy/install/install.sh
deploy/systemd/sdwan-agent.service
deploy/systemd/sdwan-bootstrap-agent.service
deploy/caddy/Caddyfile
```

Legacy fallback scripts still exist:

```text
deploy/bootstrap/sync-peers.sh
deploy/bootstrap/report-endpoints.sh
```

Prefer `sdwan-bootstrap-agent` for v1.1.7 and later.

## Core API

Public health/version:

```text
GET /healthz
GET /readyz
GET /api/v1/server/version
```

Admin:

```text
POST /admin/auth/register
POST /admin/auth/login
GET  /admin/auth/me
GET  /admin/account
GET  /admin/plans
GET  /admin/devices
GET  /admin/devices/{deviceID}
```

Device:

```text
POST /api/v1/devices/register
POST /api/v1/devices/poll
GET  /api/v1/netmap
```

Bootstrap:

```text
GET  /api/v1/bootstrap/peers
POST /api/v1/bootstrap/endpoints
```

Both bootstrap APIs use:

```text
Authorization: Bearer ${BOOTSTRAP_REPORT_TOKEN}
```

## Database Tables

```text
users
admin_sessions
plans
subscriptions
devices
device_endpoints
subnet_routes
relays
audit_logs
```

Most important tables:

```text
users:
  email, password_hash, overlay_cidr, max_devices, netmap_version, plan_code

devices:
  user_id, hostname, os, arch, public_key, virtual_ip, device_token_hash, status

device_endpoints:
  device_id, endpoint_type, address, source, rtt_ms, updated_at
```

Endpoint priority in netmap:

```text
bootstrap > manual > lan > ipv6 > unknown
```

## Local Development

Run backend/frontend via Docker Compose:

```bash
docker compose up -d --build
```

Local URLs:

```text
Web:        http://localhost:8081
Controller: http://localhost:18080
```

Run tests:

```bash
go test ./...
```

Build frontend:

```bash
cd web
npm run build
```

Build all Go commands:

```bash
go build ./cmd/agent ./cmd/controller ./cmd/bootstrap-agent
```

## Production Deployment Notes

Expected server path:

```text
/opt/sdwan
```

Typical update:

```bash
cd /opt/sdwan
git fetch --tags
git reset --hard
git clean -fd
git checkout v1.2.0
docker compose up -d --build controller web
```

Controller `.env` should include:

```text
BOOTSTRAP_REPORT_TOKEN=...
BOOTSTRAP_WG_PUBLIC_KEY=...
BOOTSTRAP_WG_ENDPOINT=controller.englishlisten.cn:51872
BOOTSTRAP_WG_ALLOWED_IP=100.254.254.254/32
```

Bootstrap WireGuard interface:

```text
/etc/wireguard/sdwan-bootstrap.conf
interface: sdwan-bootstrap
address: 100.254.254.254/32
listen port: 51872
```

Important route on bootstrap host:

```bash
ip route replace 100.64.0.0/10 dev sdwan-bootstrap
```

This route is needed so replies from `100.254.254.254` to client overlay IPs return through `sdwan-bootstrap` instead of the default `eth0` route.

Recommended WireGuard config snippet:

```ini
[Interface]
Address = 100.254.254.254/32
ListenPort = 51872
PrivateKey = ...
PostUp = ip route replace 100.64.0.0/10 dev %i
PostDown = ip route del 100.64.0.0/10 dev %i 2>/dev/null || true
```

Install bootstrap agent:

```bash
go build -o sdwan-bootstrap-agent ./cmd/bootstrap-agent
install -m 0755 sdwan-bootstrap-agent /usr/local/bin/sdwan-bootstrap-agent
install -m 0644 deploy/systemd/sdwan-bootstrap-agent.service /etc/systemd/system/sdwan-bootstrap-agent.service
systemctl daemon-reload
systemctl enable --now sdwan-bootstrap-agent
```

Config file:

```text
/etc/sdwan/bootstrap-agent.json
```

Example:

```json
{
  "controller_url": "https://controller.englishlisten.cn",
  "bootstrap_token": "your_bootstrap_token",
  "interface_name": "sdwan-bootstrap",
  "sync_interval_seconds": 5,
  "report_interval_seconds": 2,
  "remove_stale_peers": false
}
```

## Linux Client Deployment

Install:

```bash
curl -fsSL https://controller.englishlisten.cn/install.sh | sudo sh
```

Register:

```bash
sudo sdwan-agent register \
  --controller https://controller.englishlisten.cn \
  --admin-token sdwan_admin_xxx
```

Start:

```bash
sudo systemctl enable --now sdwan-agent
```

Important local files:

```text
/etc/sdwan/agent.json
/etc/wireguard/sdwan0.conf
/etc/systemd/system/sdwan-agent.service
```

## Troubleshooting Checklist

Check controller version:

```bash
curl http://127.0.0.1:18080/api/v1/server/version
```

Check bootstrap agent:

```bash
systemctl status sdwan-bootstrap-agent --no-pager
journalctl -u sdwan-bootstrap-agent -n 100 --no-pager
```

Check bootstrap WireGuard:

```bash
wg show sdwan-bootstrap
ip addr show sdwan-bootstrap
ip route get 100.64.0.3
ss -lunp | grep 51872
```

Check client WireGuard:

```bash
wg show sdwan0
ip addr show sdwan0
ip route get 100.254.254.254
cat /etc/wireguard/sdwan0.conf
```

If handshake works but ping to bootstrap IP fails:

```text
Likely missing route on bootstrap host:
100.64.0.0/10 dev sdwan-bootstrap
```

Use tcpdump:

```bash
# On bootstrap host
tcpdump -ni any udp port 51872
tcpdump -ni sdwan-bootstrap icmp

# On client
tcpdump -ni sdwan0 icmp
```

Interpretation:

```text
UDP 51872 traffic exists:
  NAT/security group is OK.

sdwan-bootstrap sees ICMP echo request but no reply:
  host route/firewall/rp_filter issue.

sdwan-bootstrap sees request and reply:
  client-side receive/firewall issue.

wg show has sent but 0 received:
  peer not synced, wrong key, UDP blocked, or bootstrap not responding.
```

Useful temporary commands:

```bash
iptables -I INPUT -i sdwan-bootstrap -j ACCEPT
iptables -I OUTPUT -o sdwan-bootstrap -j ACCEPT
sysctl -w net.ipv4.conf.all.rp_filter=0
sysctl -w net.ipv4.conf.default.rp_filter=0
sysctl -w net.ipv4.conf.sdwan-bootstrap.rp_filter=0
```

## Known Constraints

- Automatic fallback currently supports one main site and one active self-hosted Relay.
- No userspace magicsock or public multi-tenant Relay.
- No ACL or MagicDNS.
- Bootstrap endpoint discovery improves accuracy but does not guarantee P2P for symmetric NAT.
- Controller should not directly operate host WireGuard from Docker. Keep WireGuard control in `sdwan-bootstrap-agent` on the host.

## Coding Notes

- Prefer small, scoped changes.
- Keep version in `internal/version/version.go`.
- Keep Docker Compose client version environment aligned with release version.
- Update `web/package.json` and `web/package-lock.json` when bumping product version.
- Run `gofmt` on Go files.
- Run `go test ./...` and `npm run build` before release.
- Do not re-add STUN probing in `internal/agent/endpoint.go`.
