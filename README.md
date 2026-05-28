# SD-WAN Controller

`sdwan` 是一个最小版 Tailscale-like 产品骨架。当前版本是 `v1.1.4`，目标是先跑通软件层面的闭环：账号注册、设备入网、虚拟 IP 分配、HTTP polling、Netmap 下发、Linux Agent 渲染 WireGuard 配置。

默认控制器域名：

```text
controller.englishlisten.cn
```

## 当前产品边界

第一版为了降低复杂度，取消“区域”概念。一个账号就是一个独立的 Overlay 网络：

- 用户只通过邮箱注册和登录。
- 每个账号默认分配一个 `100.64.0.0/10` 下的独立 `/24` 地址池。
- 默认每个账号最多 254 台设备，自动避开 `.0` 和 `.255`。
- 登录后获得 `Admin Token`，它同时作为设备首次入网 Token。
- 设备注册成功后获得独立 `Device Token`，后续 `poll` 和 `netmap` 使用 Device Token。
- Controller 通过 HTTP polling 下发网络变更，暂不使用 WebSocket。
- 同一账号下的设备默认全互通。

暂不做：ACL、MagicDNS、Exit Node、真实支付、WebSocket 推送、多 Controller 高可用。

## 服务等级

当前只建模套餐能力，支付接口暂不接入：

| code | 名称 | 价格 | 能力 |
| --- | --- | --- | --- |
| `free` | 基础组网 | 免费 | 基础设备组网 |
| `subnet` | 快启子网服务 | 9.9 元/月 | 开启子网路由能力 |
| `relay` | 自行搭建 Relay | 29.9 元/月 | 子网路由 + 自建 Relay 能力 |

## 技术路线

- Controller：Go、`net/http`、PostgreSQL、`pgx/v5`
- 查询层：sqlc 风格手写查询封装，后续可接入 `sqlc generate`
- 数据库：PostgreSQL
- 前端：Vue 3、Vite、原生 CSS，默认中文，可切换英文
- Agent：Go，Linux 侧调用系统 WireGuard 工具链
- STUN：`coturn/coturn:4.6.3`
- 部署：Docker Compose
- 反向代理：生产环境由服务器上独立 Caddy/Nginx 分流，本仓库不强制托管 Caddy

## 架构

```text
Admin Web(Vue/nginx) ---> Controller(Go) ---> PostgreSQL

Linux/Windows Agent
      |
      | HTTPS register / polling / netmap
      v
Controller

Agent <---- WireGuard P2P ----> Agent
```

本地 Docker Compose 暴露：

```text
Web:        http://localhost:8081
Controller: http://localhost:18080
STUN:       udp://localhost:3478
```

生产建议分流：

```text
https://controller.englishlisten.cn/        -> Web
https://controller.englishlisten.cn/api/*   -> Controller
https://controller.englishlisten.cn/admin/* -> Controller
udp://controller.englishlisten.cn:3478      -> STUN
```

## 数据库设计

| 表 | 作用 |
| --- | --- |
| `users` | 用户账号、Overlay 地址池、套餐 code、设备上限、netmap 版本 |
| `admin_sessions` | 登录会话，保存 Admin Token hash |
| `plans` | 套餐定义 |
| `subscriptions` | 用户套餐订阅，当前先预留 |
| `devices` | 设备节点，直接归属用户 |
| `device_endpoints` | 设备上报的 LAN/STUN endpoint |
| `subnet_routes` | 快启子网服务预留表 |
| `relays` | 自建 Relay 预留表 |
| `audit_logs` | 操作日志预留表 |

地址分配保护：

```text
1. users.overlay_cidr UNIQUE 保证账号地址池不重复
2. 创建账号时使用 PostgreSQL pg_advisory_xact_lock 串行分配 /24
3. devices(user_id, virtual_ip) UNIQUE 保证同账号下设备 IP 不重复
4. 设备 IP 从 /24 内逐个分配，并跳过 .0 和 .255
```

## Token 模型

```text
Admin Token:
  用户登录后获得。
  用于访问 /admin/*。
  同时作为设备首次注册时的入网 Token。

Device Token:
  设备注册成功后获得。
  由 Agent 保存在 /etc/sdwan/agent.json。
  后续 /api/v1/devices/poll 和 /api/v1/netmap 使用它鉴权。
```

Token 明文只返回一次，数据库只保存 SHA-256 hash。

## Bootstrap WireGuard Peer

`v1.1.4` 增加 Bootstrap WG Peer，用于让内核 WireGuard 自己向一个公网 WireGuard 节点发包，从而让 bootstrap 节点观察到客户端 kernel WireGuard socket 的真实公网 endpoint。

Controller 环境变量：

```text
BOOTSTRAP_WG_PUBLIC_KEY   bootstrap WireGuard 公钥
BOOTSTRAP_WG_ENDPOINT     bootstrap WireGuard 公网地址，例如 controller.englishlisten.cn:41641
BOOTSTRAP_WG_ALLOWED_IP   默认 100.127.255.1/32
BOOTSTRAP_REPORT_TOKEN    bootstrap 节点回写 endpoint 使用的 Bearer token
```

当这些变量配置完整时，`/api/v1/netmap` 会额外下发：

```json
{
  "bootstrap_peer": {
    "device_id": "bootstrap",
    "hostname": "controller-bootstrap",
    "allowed_ips": ["100.127.255.1/32"],
    "endpoints": ["controller.englishlisten.cn:41641"],
    "persistent_keepalive": 25
  }
}
```

Agent 会把它渲染进 `/etc/wireguard/sdwan0.conf`，触发 kernel WireGuard 对 bootstrap 节点发 handshake/keepalive。

bootstrap 节点可运行脚本回写观察到的 endpoint：

```bash
sudo BOOTSTRAP_WG_INTERFACE=sdwan-bootstrap \
  CONTROLLER_URL=https://controller.englishlisten.cn \
  BOOTSTRAP_REPORT_TOKEN=your_report_token \
  /opt/sdwan/deploy/bootstrap/report-endpoints.sh
```

该脚本会读取：

```bash
wg show sdwan-bootstrap dump
```

并把每个 peer 的 `public_key + endpoint` 回写到：

```text
POST /api/v1/bootstrap/endpoints
```

注意：bootstrap 方案能拿到 kernel WireGuard 到 bootstrap 节点的真实 NAT 映射，但对 symmetric NAT 仍不保证 P2P 成功。无法直连时仍需要 Relay。

## 本地运行

```bash
docker compose up -d --build
```

访问：

```text
http://localhost:8081
```

查看日志：

```bash
docker compose logs -f controller
docker compose logs -f web
docker compose logs -f stun
```

如果从旧版 schema 升级，本地开发库需要重置：

```bash
docker compose stop controller
docker exec sdwan-postgres-1 psql -U sdwan -d sdwan -v ON_ERROR_STOP=1 \
  -c "DROP TABLE IF EXISTS audit_logs, relays, subnet_routes, device_endpoints, devices, subscriptions, plans, regions, admin_sessions, admin_users, users CASCADE;"
docker compose up -d --build controller
```

## API

### 查询版本

```bash
curl http://localhost:18080/api/v1/server/version
```

### 邮箱注册

```bash
curl -X POST http://localhost:18080/admin/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"password123"}'
```

### 邮箱登录

```bash
curl -X POST http://localhost:18080/admin/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"password123"}'
```

### 当前账号

```bash
curl http://localhost:18080/admin/auth/me \
  -H "Authorization: Bearer sdwan_admin_xxx"
```

### 控制台总览

```bash
curl http://localhost:18080/admin/account \
  -H "Authorization: Bearer sdwan_admin_xxx"
```

返回账号地址池、设备数、套餐能力、套餐列表、子网路由预留数据、Relay 预留数据。

### 套餐列表

```bash
curl http://localhost:18080/admin/plans
```

### 设备列表

```bash
curl http://localhost:18080/admin/devices \
  -H "Authorization: Bearer sdwan_admin_xxx"
```

### 设备详情

```bash
curl http://localhost:18080/admin/devices/dev_xxx \
  -H "Authorization: Bearer sdwan_admin_xxx"
```

### 设备注册

```bash
curl -X POST http://localhost:18080/api/v1/devices/register \
  -H "Content-Type: application/json" \
  -d '{
    "admin_token": "sdwan_admin_xxx",
    "hostname": "linux-01",
    "os": "linux",
    "arch": "amd64",
    "public_key": "wireguard-public-key",
    "client_version": "v1.1.4"
  }'
```

### 设备 Polling

```bash
curl -X POST http://localhost:18080/api/v1/devices/poll \
  -H "Authorization: Bearer sdwan_device_xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "current_netmap_version": 1,
    "client_version": "v1.1.4",
    "endpoints": [
      {"type":"lan","addr":"192.168.1.10:41641","source":"local"}
    ]
  }'
```

### 获取 Netmap

```bash
curl http://localhost:18080/api/v1/netmap \
  -H "Authorization: Bearer sdwan_device_xxx"
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

Agent 最终运行方式：

```text
1. 加载 /etc/sdwan/agent.json
2. 检测 LAN/STUN endpoints
3. 调用 /api/v1/devices/poll
4. 如果 netmap_changed=true，拉取 /api/v1/netmap
5. 渲染 /etc/wireguard/sdwan0.conf
6. 执行 wg-quick down/up
7. 更新本地 netmap_version
8. 等待 poll_interval_seconds 后进入下一轮
```

构建 Linux Agent 二进制：

```bash
docker run --rm \
  -v /opt/sdwan:/src \
  -w /src \
  -e GOPROXY=https://goproxy.cn,direct \
  golang:1.25-alpine \
  sh -c 'GOOS=linux GOARCH=amd64 go build -o downloads/v1.1.4/sdwan-agent-linux-amd64 ./cmd/agent'
```

## 验证

```bash
go test ./...

cd web
npm run build
```

## 后续路线

1. 接入真实支付和套餐升级。
2. 实现 `subnet_routes` 的审批、下发和 Agent 路由配置。
3. 实现自建 Relay 注册、健康检查和 Netmap 下发。
4. 增加设备禁用、删除、重命名。
5. 增加 Windows Agent。
6. 稳定后再考虑 WebSocket 推送替换 HTTP polling。
