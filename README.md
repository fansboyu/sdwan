# SD-WAN Controller

`sdwan` 是一个最小版 Tailscale-like 产品骨架。第一阶段聚焦软件控制面和客户端接入协议：管理员通过邮箱注册登录，创建客户后获得 Join Token；Linux/Windows 客户端使用 Join Token 注册，Controller 为客户端分配 Overlay 虚拟 IP，并通过 HTTP polling 下发 Netmap。

当前版本：`v1.1.0`

默认控制器域名：

```text
controller.englishlisten.cn
```

## 产品目标

第一版目标是跑通最小闭环：

- 管理员只通过邮箱注册和登录。
- 管理员创建客户。
- 每个客户默认分配一个 `/28` Overlay 地址池，默认支持 16 个客户端。
- 系统为客户生成 Join Token。
- Linux/Windows 客户端使用 Join Token 注册。
- Controller 为客户端分配虚拟 IP。
- 客户端通过 HTTP polling 上报心跳、Endpoint 和版本信息。
- 客户端拉取 Netmap，用于生成 WireGuard peer 配置。
- 同一客户下默认全互通。

第一版暂不做：

- WebSocket 推送。
- 复杂 ACL。
- MagicDNS。
- Exit Node。
- Subnet Router。
- Relay/DERP 转发。
- 多 Controller 高可用。
- 自动客户端升级。
- 用户名、手机号、第三方 OAuth 登录。
- 邮箱验证码和 SMTP 发信。

## 技术路线

后端控制器：

- Go
- 标准库 `net/http`
- PostgreSQL
- sqlc 风格查询层
- `pgx/v5`
- Token 明文只返回一次，数据库只保存 SHA-256 hash
- Docker Compose 部署

前端主控页面：

- Vue 3
- Vite
- JavaScript
- 原生 CSS
- 默认中文展示，可切换英文
- 构建后由 nginx 容器托管

STUN：

- coturn
- STUN-only 模式
- UDP 3478
- 默认地址：`stun:controller.englishlisten.cn:3478`

部署方式：

- 本仓库只提供 Docker Compose。
- 本仓库不内置 Caddy。
- 生产环境由服务器上独立部署的 Caddy/Nginx 做服务分配。

## 整体架构

```text
Admin Web(Vue/nginx) ---> Controller(Go) ---> PostgreSQL

Linux/Windows Client
      |
      | HTTPS polling / netmap
      v
Controller

Client <---- WireGuard P2P ----> Client
```

本地 Docker Compose 暴露：

```text
主控页面：http://localhost:8081
Controller API：http://localhost:18080
STUN：udp://localhost:3478
```

生产时可以由你服务器上的独立 Caddy/Nginx 自行分流：

```text
https://controller.englishlisten.cn/        -> web:80
https://controller.englishlisten.cn/api/*   -> controller:8080
https://controller.englishlisten.cn/admin/* -> controller:8080
udp://controller.englishlisten.cn:3478      -> stun:3478/udp
```

## 地址分配

全局 Overlay 地址池：

```text
100.64.0.0/10
```

每个客户默认分配一个 `/28`：

```text
客户 A：100.64.0.0/28
客户 B：100.64.0.16/28
客户 C：100.64.0.32/28
```

每个客户默认容量：

```text
16 个客户端
```

设备按 `/32` 单地址分配。

### 如何保证客户地址池不重复

当前使用三层保护：

```text
1. 数据库唯一约束
   customers.address_cidr UNIQUE

2. 事务级分配锁
   创建客户时使用 PostgreSQL pg_advisory_xact_lock。
   同一时间只有一个事务能计算和写入新的客户地址池。

3. 客户内设备唯一约束
   devices(customer_id, virtual_ip) UNIQUE
   防止同一客户内重复分配设备 IP。
```

客户地址池分配流程：

```text
开启数据库事务
获取 PostgreSQL advisory transaction lock
查询最后一个客户 CIDR
向后偏移 16 个地址
生成下一个 /28
写入 customers
写入 join_tokens
提交事务
```

## Token 模型

系统使用三类 Token：

```text
Admin Token:
  管理员登录后获得。
  用于访问 /admin/* 管理接口。
  当前有效期为 30 天。

Join Token:
  客户级接入 Token。
  用于客户端首次注册。

Device Token:
  设备级 Token。
  注册成功后返回给客户端。
  后续 poll 和 netmap 请求使用它鉴权。
```

安全约束：

- Token 明文只返回一次。
- 数据库只保存 Token hash。
- Controller 不保存客户端 WireGuard 私钥。
- Controller 只保存客户端 WireGuard 公钥。

## 本地运行

启动：

```bash
docker compose up --build
```

访问：

```text
主控页面：http://localhost:8081
Controller API：http://localhost:18080
```

查看日志：

```bash
docker compose logs -f controller
docker compose logs -f stun
```

停止：

```bash
docker compose down
```

## API 接口

### 查询版本

```bash
curl http://localhost:18080/api/v1/server/version
```

### 管理员邮箱注册

```bash
curl -X POST http://localhost:18080/admin/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"password123"}'
```

### 管理员邮箱登录

```bash
curl -X POST http://localhost:18080/admin/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"password123"}'
```

### 创建客户

```bash
curl -X POST http://localhost:18080/admin/customers \
  -H "Authorization: Bearer sdwan_admin_xxx" \
  -H "Content-Type: application/json" \
  -d '{"name":"demo"}'
```

### 客户端注册设备

```bash
curl -X POST http://localhost:18080/api/v1/devices/register \
  -H "Content-Type: application/json" \
  -d '{
    "join_token": "sdwan_join_xxx",
    "hostname": "linux-01",
    "os": "linux",
    "arch": "amd64",
    "public_key": "wireguard-public-key",
    "client_version": "v1.1.0"
  }'
```

### 客户端 Polling

```bash
curl -X POST http://localhost:18080/api/v1/devices/poll \
  -H "Authorization: Bearer sdwan_device_xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "current_netmap_version": 1,
    "client_version": "v1.1.0",
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

## Linux 客户端验证

构建 Linux Agent：

```bash
GOOS=linux GOARCH=amd64 go build -o sdwan-agent ./cmd/agent
```

Linux 依赖：

```bash
sudo apt update
sudo apt install -y wireguard-tools
```

注册设备：

```bash
sudo ./sdwan-agent register \
  --controller http://localhost:18080 \
  --join-token sdwan_join_xxx
```

渲染 WireGuard 配置：

```bash
sudo ./sdwan-agent render --out /tmp/sdwan0.conf
cat /tmp/sdwan0.conf
```

运行 daemon 单次循环，便于调试：

```bash
sudo ./sdwan-agent daemon --once --apply=false
```

作为 systemd 服务运行：

```bash
sudo cp deploy/systemd/sdwan-agent.service /etc/systemd/system/sdwan-agent.service
sudo systemctl daemon-reload
sudo systemctl enable --now sdwan-agent
```

查看日志：

```bash
journalctl -u sdwan-agent -f
```

daemon 主循环：

```text
加载 /etc/sdwan/agent.json
自动检测 LAN/STUN endpoints
调用 /api/v1/devices/poll
netmap_changed=true 时拉取 /api/v1/netmap
渲染 /etc/wireguard/sdwan0.conf
执行 wg-quick down/up
更新本地 netmap_version
等待 poll_interval_seconds 后进入下一轮
```

## 验证命令

```bash
go test ./...

cd web
npm run build

docker compose config
docker compose build
```

## 后续开发路线

1. 主控页面拆分为 `api/`、`views/`、`components/`。
2. 增加客户详情页。
3. 增加设备列表页。
4. 增加设备详情页，展示虚拟 IP、客户端版本、最后在线时间、Endpoint。
5. 增加设备禁用/删除接口。
6. Linux Agent 增加 daemon/systemd 模式。
7. Linux Agent 增加自动 STUN Endpoint 探测。
8. 增加 Windows Agent。
9. 增加 Relay。
10. 再考虑 WebSocket 推送替换 polling。
