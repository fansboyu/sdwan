# SD-WAN Controller

`sdwan` 是一个轻量级 Tailscale-like SD-WAN 产品骨架。当前版本是 `v1.1.9`，已经包含控制端、Web 管理台、Linux Agent、Windows Agent、Bootstrap Agent、Relay Agent、子网路由和免费升级业务逻辑。

生产默认控制器域名：

```text
controller.englishlisten.cn
```

## 当前定位

一个账号就是一个独立 Overlay 网络。

- 用户通过邮箱注册和登录。
- 每个用户默认分配 `100.64.0.0/10` 下的独立 `/24` 地址池。
- 设备注册后自动分配虚拟 IP，并避开 `.0` 和 `.255`。
- 登录后得到 `Admin Token`，它可作为首台设备入网 Token。
- 设备注册成功后得到独立 `Device Token`，后续 `poll` 和 `netmap` 使用设备 Token。
- 同账号设备默认互通。
- Endpoint 由 Agent 上报和 Bootstrap/Relay 侧观察，控制端每设备每类型保留最近 3 个。

暂不包含 ACL、MagicDNS、Exit Node、真实支付、审计日志、多 Controller 高可用和 DERP 风格自动 fallback。

## 功能模块

### Controller

- Go + `net/http` + PostgreSQL + `pgx/v5`
- 用户注册、登录、账号状态、订阅状态
- 设备注册、心跳、Endpoint 上报、Netmap 下发
- 子网路由审批和下发
- Relay 节点管理和 Relay 模式开关
- 数据库 migration 自动执行

### Web 管理台

- Vue 3 + Vite
- 深色科技风登录页和主页
- 设备列表、设备详情、主节点设置
- 子网路由审批
- 套餐升级和免费升级入口
- Relay 节点创建、启用、禁用

### Linux Agent

- 使用系统 WireGuard 工具链：`wg`、`wg-quick`
- 支持设备注册、守护进程轮询、Endpoint 上报、Netmap 应用
- 支持 `advertise_routes` 配置和命令行维护
- 首次启动使用 `wg-quick up`
- 后续配置变更使用 `wg syncconf` 和路由差异同步，减少中断
- 支持 Linux 子网网关命令，开启 `ip_forward` 和 iptables 转发/NAT 规则

### Windows Agent

- 使用 userspace WireGuard + Wintun
- 包含 Windows Service 和托盘程序
- 支持注册、连接、断开、诊断
- 按 Netmap 计算实际需要的路由，不再默认固定写入 `100.64.0.0/10`
- 使用 `last_routes` 做路由差异同步和清理

### Bootstrap Agent

- 运行在 Bootstrap 主机上
- 维护 `sdwan-bootstrap` WireGuard 接口 peer 列表
- 观察真实 peer endpoint，并回写到控制端
- 替代旧的定时脚本同步方案

### Relay Agent

- 运行在自建 Relay 主机上
- 拉取控制端下发的 Relay peer 列表
- 维护 `sdwan-relay` WireGuard 接口
- 上报 Relay 心跳
- 当前 Relay 模式采用账号级开关：开启后客户端主要连接 Relay，由 Relay 允许同账号全网互通

## 套餐逻辑

当前有两个付费能力版本：

- `subnet`：快启子网服务，支持主节点和子网路由。
- `relay`：自行搭建 Relay，包含 `subnet` 的全部能力，并开放 Relay 模式。

当前用户量较少时支持免费升级：

- 每个账号最多可免费升级 12 个月。
- 从免费版升级到 `subnet` 会按剩余免费月数创建免费订阅。
- 已免费升级到 `subnet` 后，再升级到 `relay` 不额外增加月份，只升级能力范围。
- `relay` 是更高版本，拥有全部功能。

## 网络模式

### Hub 模式

默认模式。

- 同账号设备通过控制端下发 peer 信息互联。
- 主节点设备可以发布 LAN 子网路由。
- 控制台审批后，客户端收到对应子网路由。
- 主节点机器本身仍需要开启系统转发和必要 NAT。

### Relay 模式

账号开启 Relay 模式后：

- 客户端 Netmap 优先下发 Relay peer。
- 客户端通过 Relay 访问同账号 Overlay 设备。
- 当前策略是 Relay 模式下允许全网互通。
- `relay` 套餐包含 `subnet`，因此仍可使用已审批的子网路由。

## 架构图

```text
Admin Browser
  |
  | HTTP/HTTPS
  v
Web Console(Vue/nginx)
  |
  | /admin/*
  v
Controller(Go API)
  |
  | SQL
  v
PostgreSQL


Linux / Windows Agent
  |
  | register / poll / netmap / endpoints
  v
Controller
  |
  | netmap
  v
WireGuard interface


Bootstrap Agent
  |
  | peers / observed endpoints
  v
Controller


Relay Agent
  |
  | relay peers / heartbeat
  v
Controller
```

## 端口

本地 Docker 默认：

- Web：`http://127.0.0.1:8081`
- Controller：`http://127.0.0.1:18080`
- PostgreSQL：容器内部 `5432`

Agent 默认：

- Linux Agent WireGuard：UDP `41641`
- Windows Agent WireGuard：UDP `41642`
- Bootstrap WireGuard：UDP `51872`

## 本地运行

```powershell
docker compose up -d --build
```

访问：

```text
http://127.0.0.1:8081
```

检查 Controller：

```powershell
curl http://127.0.0.1:18080/healthz
curl http://127.0.0.1:18080/api/v1/server/version
```

## 邮箱验证码

注册账号需要邮箱一次性验证码。Controller 使用 Resend API 发送邮件，建议在 Resend 中验证二级发信域名：

```text
mail.englishlisten.cn
```

推荐发件人：

```text
SD-WAN 控制台 <noreply@mail.englishlisten.cn>
```

需要配置环境变量：

```text
RESEND_API_KEY=re_xxx
RESEND_FROM=SD-WAN Controller <noreply@mail.englishlisten.cn>
EMAIL_CODE_TTL_SECONDS=600
EMAIL_CODE_COOLDOWN_SECONDS=60
EMAIL_CODE_MAX_ATTEMPTS=5
```

注册流程：

```text
1. 用户输入邮箱并点击发送验证码
2. Controller 检查邮箱是否已注册
3. Controller 生成 6 位验证码并只保存 hash
4. Resend 发送验证码邮件
5. 用户输入验证码和密码完成注册
6. 验证码校验通过后立即失效
```

## 数据库 migration

正式 migration 位于：

```text
db/migrations/
```

Controller 启动时会自动执行 migration。当前 migration 版本：

```text
5
```

主要迁移：

- `000001_init`：基础用户、设备、endpoint、订阅等表结构
- `000002_add_device_site_role`：设备站点角色
- `000003_add_subscription_free_upgrade`：免费升级订阅字段
- `000004_add_relay_mode`：Relay 模式和 Relay 节点表
- `000005_add_email_verifications`：注册邮箱验证码

## 主要 API

公开接口：

- `GET /healthz`
- `GET /api/v1/server/version`

管理接口：

- `POST /admin/auth/email-code`
- `POST /admin/auth/register`
- `POST /admin/auth/login`
- `GET /admin/account`
- `GET /admin/devices`
- `PATCH /admin/devices/{id}`
- `POST /admin/devices/{id}/main-site`
- `GET /admin/subnet-routes`
- `PATCH /admin/subnet-routes/{id}/approval`
- `POST /admin/subscription/free-upgrade`
- `POST /admin/subscription/cancel`
- `POST /admin/relays`
- `POST /admin/relays/{id}/enable`
- `POST /admin/relays/{id}/disable`
- `POST /admin/relay-mode`

设备接口：

- `POST /api/v1/devices/register`
- `POST /api/v1/devices/poll`
- `GET /api/v1/devices/netmap`
- `POST /api/v1/devices/endpoints`

Bootstrap 接口：

- `GET /api/v1/bootstrap/peers`
- `POST /api/v1/bootstrap/endpoints`

Relay 接口：

- `GET /api/v1/relays/peers`
- `POST /api/v1/relays/heartbeat`

## Linux Agent 常用命令

注册：

```bash
sudo sdwan-agent register \
  --controller https://controller.englishlisten.cn \
  --token <ADMIN_TOKEN> \
  --name linux-node-1
```

启动守护进程：

```bash
sudo sdwan-agent daemon
```

查看、添加、删除发布的子网路由：

```bash
sudo sdwan-agent routes list
sudo sdwan-agent routes add 192.168.50.0/24
sudo sdwan-agent routes remove 192.168.50.0/24
```

开启 Linux 子网网关：

```bash
sudo sdwan-agent subnet-gateway enable \
  --lan-cidr 192.168.50.0/24 \
  --out-interface eth0
```

查看或关闭：

```bash
sudo sdwan-agent subnet-gateway status
sudo sdwan-agent subnet-gateway disable --lan-cidr 192.168.50.0/24
```

## Windows Agent 常用命令

构建：

```powershell
go build -o build\sdwan-service.exe .\cmd\windows-service
go build -o build\sdwan-tray.exe .\cmd\windows-tray
```

安装服务：

```powershell
sdwan-service.exe install
sdwan-service.exe start
```

诊断：

```powershell
sdwan-service.exe diagnose
```

停止并移除：

```powershell
sdwan-service.exe stop
sdwan-service.exe uninstall
```

## Bootstrap Agent 示例配置

```json
{
  "controller_url": "https://controller.englishlisten.cn",
  "bootstrap_token": "change-me",
  "interface": "sdwan-bootstrap",
  "poll_interval_seconds": 10,
  "report_interval_seconds": 10
}
```

运行：

```bash
sudo sdwan-bootstrap-agent daemon --config /etc/sdwan/bootstrap-agent.json
```

## Relay Agent 示例配置

```json
{
  "controller_url": "https://controller.englishlisten.cn",
  "relay_token": "relay-token-from-admin-console",
  "interface": "sdwan-relay",
  "config_path": "/etc/wireguard/sdwan-relay.conf",
  "poll_interval_seconds": 10,
  "report_interval_seconds": 10
}
```

运行：

```bash
sudo sdwan-relay-agent daemon --config /etc/sdwan/relay-agent.json
```

## 构建和测试

Go 测试：

```powershell
go test ./...
```

Web 构建：

```powershell
cd web
npm run build
```

Docker 构建：

```powershell
docker compose build
```

## 生产部署提示

- 建议部署目录：`/opt/sdwan`
- Controller 使用独立 PostgreSQL 或 Compose PostgreSQL
- Web 通过 Caddy/Nginx 反向代理
- 下载文件建议放在 `/downloads/<version>/`
- Linux Agent 安装脚本默认使用 `SDWAN_VERSION=v1.1.9`
- Controller 需要配置强随机 `ADMIN_JWT_SECRET`
- Bootstrap 和 Relay Token 必须独立生成并妥善保存

## 已知限制

- 暂无 ACL，账号内设备默认互通。
- 暂无 MagicDNS。
- 暂无 Exit Node。
- Relay 当前是账号级手动开关，不是自动按连接质量 fallback。
- Relay 健康检查和容量调度仍是基础版本。
- Windows 端暂未实现子网网关能力。
- 支付逻辑暂未接入真实支付渠道。
- 审计日志字段预留但未完整产品化。
