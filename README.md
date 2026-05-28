# SD-WAN Controller

`sdwan` 是一个最小版 Tailscale-like 产品骨架。当前版本是 `v1.1.7`，重点是把 bootstrap 侧从定时脚本升级为常驻 `sdwan-bootstrap-agent`，让客户端 public key 同步和真实 endpoint 回写更接近实时。

默认控制器域名：

```text
controller.englishlisten.cn
```

## 产品边界

第一版取消“区域”概念，一个账号就是一个独立 Overlay 网络。

- 用户通过邮箱注册和登录。
- 每个账号默认获得 `100.64.0.0/10` 下的独立 `/24` 地址池。
- 每个账号默认最多 254 台设备，自动避开 `.0` 和 `.255`。
- 登录后获得 `Admin Token`，它同时作为设备首次入网 Token。
- 设备注册成功后获得独立 `Device Token`，后续 `poll` 和 `netmap` 使用 Device Token。
- 同一账号下设备默认全互通。
- 当前版本无 STUN endpoint 探测，真实公网 endpoint 由 bootstrap 侧观察。

暂不做：ACL、MagicDNS、Exit Node、真实支付、WebSocket 推送、多 Controller 高可用、Relay fallback。

## 技术路线

- Controller：Go、`net/http`、PostgreSQL、`pgx/v5`
- 前端：Vue 3、Vite、原生 CSS，默认中文，可切换英文
- Agent：Go，Linux 侧调用系统 WireGuard 工具链
- Bootstrap Agent：Go，宿主机常驻进程，管理 `sdwan-bootstrap`
- 数据面：Linux kernel WireGuard、`wg`、`wg-quick`
- 部署：Docker Compose + 宿主机 systemd
- 反向代理：生产环境由服务器上的独立 Caddy/Nginx 分流

## 服务关系图

```text
Admin Browser
  |
  | HTTPS
  v
Admin Web(Vue/nginx)
  |
  | /admin/*
  v
Controller(Go API)
  |
  | SQL
  v
PostgreSQL


Linux Agent A/B
  |
  | HTTPS register / poll / netmap
  v
Controller

Linux Agent A/B
  |
  | write /etc/wireguard/sdwan0.conf
  | wg-quick down/up
  v
kernel WireGuard(sdwan0)
  |
  | UDP handshake / keepalive
  v
sdwan-bootstrap(宿主机 WireGuard)
  ^
  |
sdwan-bootstrap-agent
  |
  | GET /api/v1/bootstrap/peers
  | POST /api/v1/bootstrap/endpoints
  v
Controller
```

## Bootstrap Agent 流程

`sdwan-bootstrap-agent` 是宿主机常驻服务，替代旧的 `sync-peers.sh + report-endpoints.sh` 定时任务。

```text
1. 启动 bootstrap-agent
2. 读取 /etc/sdwan/bootstrap-agent.json
3. 检查 sdwan-bootstrap 接口是否存在
4. 调用 GET /api/v1/bootstrap/peers
5. 对每个 active device 执行 wg set
6. 周期性 wg show sdwan-bootstrap dump
7. 如果 peer endpoint 变化，调用 POST /api/v1/bootstrap/endpoints
8. Controller 写入 device_endpoints(endpoint_type=bootstrap)
9. Controller bump netmap_version
10. Linux Agent 下一轮 poll/netmap 拿到新 endpoint
```

核心原则：

```text
临时 STUN socket 得到的 endpoint 不可信。
bootstrap 看到的是 kernel WireGuard socket 发出来的真实公网 endpoint。
```

## 本地 Docker Compose

```bash
docker compose up -d --build
```

本地端口：

```text
Web:        http://localhost:8081
Controller: http://localhost:18080
```

生产建议分流：

```text
https://controller.englishlisten.cn/        -> Web
https://controller.englishlisten.cn/api/*   -> Controller
https://controller.englishlisten.cn/admin/* -> Controller
udp://controller.englishlisten.cn:51872     -> bootstrap WireGuard
```

## 数据库设计

| 表 | 作用 |
| --- | --- |
| `users` | 用户账号、Overlay 地址池、套餐 code、设备上限、netmap 版本 |
| `admin_sessions` | 登录会话，保存 Admin Token hash |
| `plans` | 套餐定义 |
| `subscriptions` | 用户套餐订阅，当前预留 |
| `devices` | 设备节点，保存 public key、virtual IP、Device Token hash |
| `device_endpoints` | 设备 endpoint，当前主要存 `lan` 和 `bootstrap` |
| `subnet_routes` | 快启子网服务预留表 |
| `relays` | 自建 Relay 预留表 |
| `audit_logs` | 操作日志预留表 |

## Controller 环境变量

```text
DATABASE_URL
CONTROLLER_URL
LISTEN_ADDR
DEFAULT_MAX_DEVICES
POLL_INTERVAL_SECONDS
MIN_SUPPORTED_CLIENT_VERSION
LATEST_CLIENT_VERSION
BOOTSTRAP_WG_PUBLIC_KEY
BOOTSTRAP_WG_ENDPOINT
BOOTSTRAP_WG_ALLOWED_IP
BOOTSTRAP_REPORT_TOKEN
```

`BOOTSTRAP_REPORT_TOKEN` 同时用于：

```text
GET /api/v1/bootstrap/peers
POST /api/v1/bootstrap/endpoints
```

## Bootstrap Agent 部署

构建二进制：

```bash
docker run --rm \
  -v /opt/sdwan:/src \
  -w /src \
  -e GOPROXY=https://goproxy.cn,direct \
  golang:1.25-alpine \
  sh -c 'mkdir -p downloads/v1.1.7 && GOOS=linux GOARCH=amd64 go build -o downloads/v1.1.7/sdwan-bootstrap-agent-linux-amd64 ./cmd/bootstrap-agent'
```

安装：

```bash
sudo install -m 0755 /opt/sdwan/downloads/v1.1.7/sdwan-bootstrap-agent-linux-amd64 /usr/local/bin/sdwan-bootstrap-agent
sudo install -m 0644 /opt/sdwan/deploy/systemd/sdwan-bootstrap-agent.service /etc/systemd/system/sdwan-bootstrap-agent.service
```

写配置：

```bash
sudo sdwan-bootstrap-agent \
  --write-example-config \
  --config /etc/sdwan/bootstrap-agent.json \
  --controller https://controller.englishlisten.cn \
  --bootstrap-token your_bootstrap_token \
  --interface sdwan-bootstrap
```

配置文件示例：

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

启动：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now sdwan-bootstrap-agent
sudo systemctl status sdwan-bootstrap-agent --no-pager
journalctl -u sdwan-bootstrap-agent -n 80 --no-pager
```

注意：`sdwan-bootstrap-agent` 不创建 `/etc/wireguard/sdwan-bootstrap.conf`，它要求宿主机已经存在并启动：

```bash
sudo wg show sdwan-bootstrap
```

## Bootstrap API

### 拉取 bootstrap peers

```bash
curl http://localhost:18080/api/v1/bootstrap/peers \
  -H "Authorization: Bearer your_bootstrap_token"
```

返回：

```json
{
  "peers": [
    {
      "device_id": "dev_xxx",
      "hostname": "linux-01",
      "public_key": "client-public-key",
      "virtual_ip": "100.64.0.1",
      "status": "active"
    }
  ]
}
```

### 回写真实 endpoint

```bash
curl -X POST http://localhost:18080/api/v1/bootstrap/endpoints \
  -H "Authorization: Bearer your_bootstrap_token" \
  -H "Content-Type: application/json" \
  -d '{"public_key":"client-public-key","endpoint":"111.228.42.62:37425"}'
```

## Linux Agent

安装脚本：

```bash
curl -fsSL https://controller.englishlisten.cn/install.sh | sudo sh
```

注册设备：

```bash
sudo sdwan-agent register \
  --controller https://controller.englishlisten.cn \
  --admin-token sdwan_admin_xxx
```

启动 daemon：

```bash
sudo systemctl enable --now sdwan-agent
```

Agent 运行流程：

```text
1. 加载 /etc/sdwan/agent.json
2. 检测 LAN endpoints
3. 调用 /api/v1/devices/poll
4. 如果 netmap_changed=true，拉取 /api/v1/netmap
5. 渲染 /etc/wireguard/sdwan0.conf
6. 执行 wg-quick down/up
7. 更新本地 netmap_version
8. 等待 poll_interval_seconds 后进入下一轮
```

构建 Linux Agent：

```bash
docker run --rm \
  -v /opt/sdwan:/src \
  -w /src \
  -e GOPROXY=https://goproxy.cn,direct \
  golang:1.25-alpine \
  sh -c 'mkdir -p downloads/v1.1.7 && GOOS=linux GOARCH=amd64 go build -o downloads/v1.1.7/sdwan-agent-linux-amd64 ./cmd/agent'
```

## 常用 API

```bash
curl http://localhost:18080/api/v1/server/version
```

```bash
curl -X POST http://localhost:18080/admin/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"password123"}'
```

```bash
curl -X POST http://localhost:18080/admin/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"password123"}'
```

```bash
curl http://localhost:18080/admin/devices \
  -H "Authorization: Bearer sdwan_admin_xxx"
```

```bash
curl http://localhost:18080/api/v1/netmap \
  -H "Authorization: Bearer sdwan_device_xxx"
```

## 验证

```bash
go test ./...

cd web
npm run build
```

## 后续路线

1. Relay fallback，解决 symmetric NAT 和 UDP 直连失败场景。
2. 多 bootstrap 节点和健康上报。
3. `subnet_routes` 审批、下发和 Agent 路由配置。
4. 设备禁用、删除、重命名。
5. Windows Agent。
6. 稳定后再考虑 WebSocket 推送替换 HTTP polling。
