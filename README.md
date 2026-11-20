# SD-WAN Controller

`sdwan` 是一个轻量级 Tailscale-like SD-WAN 产品骨架。当前版本是 `v1.2.5`，已经包含控制端、Web 管理台、Linux Agent、Windows Agent、Bootstrap Agent、Relay Agent 和子网路由。

`v1.2.5`：Bootstrap 仅以近期 WireGuard 握手或收包计数增长作为刷新证据；Controller 在重新生成候选时降低两分钟未见证据的 Bootstrap 地址优先级，并协调单 Hub 链路的直连候选试探。新 Agent 只应用 Controller 的选路，诊断页可查看节点上报的直连 Endpoint（旧 Agent 可能仍上报配置地址）。严格 NAT 或 UDP 屏蔽下不保证直连成功。

生产默认控制器域名：

```text
controller.englishlisten.cn
```

## 架构文档

按服务和客户端拆分的详细说明见 [docs/architecture/README.md](docs/architecture/README.md)：

- [控制器服务](docs/architecture/server-controller.md)
- [发现服务](docs/architecture/server-discovery.md)
- [中继服务](docs/architecture/server-relay.md)
- [Linux 客户端](docs/architecture/client-linux.md)
- [Windows 客户端](docs/architecture/client-windows.md)

完整部署步骤见 [docs/deployment.md](docs/deployment.md)。

本地 Docker 多节点模拟环境见 [docs/simulation.md](docs/simulation.md)，可体验 Controller、Linux Hub、分支节点和 Hub 后方 LAN 的完整链路。

## 当前定位

一个账号就是一个独立 Overlay 网络。

- 用户通过邮箱注册和登录。
- 每个用户默认分配 `100.64.0.0/10` 下的独立 `/24` 地址池。
- 设备注册后自动分配虚拟 IP，并避开 `.0` 和 `.255`。
- 登录后得到 `Admin Token`，它可作为首台设备入网 Token。
- 设备注册成功后得到独立 `Device Token`，后续 `poll` 和 `netmap` 使用设备 Token。
- 同账号设备默认互通。
- Endpoint 由 Agent 上报和 Bootstrap/Relay 侧观察，控制端每设备每类型保留最近 3 个。

暂不包含 ACL、MagicDNS、Exit Node、真实支付、审计日志、多 Controller 高可用和公共多租户 Relay。

## 功能模块

### Controller

- Go + `net/http` + PostgreSQL + `pgx/v5`
- 用户注册、登录和账号状态
- 设备注册、心跳、Endpoint 上报、Netmap 下发
- 子网路由审批和下发
- Relay 节点管理、健康校验和按客户端路径状态机
- 数据库 migration 自动执行

### Web 管理台

- Vue 3 + Vite
- 深色科技风登录页和主页
- 设备列表、设备详情、主节点设置
- 子网路由审批
- Relay 节点创建、启用、禁用
- 直连、自动中继、强制中继三种连接模式
- 每个客户端当前路径、目标路径和 generation 展示

### Linux Agent

- 使用系统 WireGuard 工具链：`wg`、`wg-quick`
- 支持设备注册、守护进程轮询、Endpoint 上报、Netmap 应用
- 支持 `advertise_routes` 配置和命令行维护
- 首次启动使用 `wg-quick up`
- 后续配置变更使用 `wg syncconf` 和路由差异同步，减少中断
- 支持 Linux 子网网关命令，开启 `ip_forward` 和 iptables 转发/NAT 规则
- 每 5 秒采集 WireGuard 握手与收发字节，参与自动 fallback

### Windows Agent

- 使用 userspace WireGuard + Wintun
- 包含 Windows Service 和托盘程序
- 推荐从单文件安装包安装；托盘 Join 后自动注册、安装/启动服务并连接
- 主菜单提供加入、连接和断开，高级设置与诊断入口单独收纳
- Windows Service 与 Linux Agent 共用 Controller poll、netmap 同步和路径状态逻辑
- 按 Netmap 计算实际需要的路由，不再默认固定写入 `100.64.0.0/10`
- 使用 `last_routes` 做路由差异同步和清理
- 使用 wireguard-go UAPI 无中断切换 AllowedIPs 并上报路径状态

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
- Relay 节点启用与业务流量模式完全分离
- 自动模式按“客户端—主站点”链路独立 fallback
- 可使用 `deploy/install/install-relay.sh` 安装 Linux amd64 Agent 和 systemd 服务；WireGuard 接口、路由和防火墙仍需按部署文档预先配置

## 使用权限

所有正常账户均可使用子网路由和自建 Relay，无需开通套餐。设备配额、路由审批和 Relay 健康检查继续生效。旧订阅数据保留，仅用于兼容和回滚。

## 网络模式

### Hub 模式

默认模式。

- 同账号设备通过控制端下发 peer 信息互联。
- 主节点设备可以发布 LAN 子网路由。
- 控制台审批后，客户端收到对应子网路由。
- 主节点机器本身仍需要开启系统转发和必要 NAT。

### 自动 Relay fallback

`v1.2.0` 支持三种账户路径模式：

- `direct`：只使用主站点与客户端直连。
- `auto`：优先直连；连续 30 秒无有效握手后切 Relay，Relay 最少驻留 60 秒，直连连续恢复 30 秒后切回。
- `relay`：强制所有客户端与主站点通过活动 Relay 通信。

切换按客户端独立执行。直连和 Relay Peer 会同时保留用于探测，但同一业务 AllowedIPs 始终只属于当前目标路径。自动和强制模式要求活动 Relay 最近 15 秒内有心跳，并要求所有活动客户端至少为 `v1.2.0`。

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
8
```

主要迁移：

- `000001_init`：基础用户、设备、endpoint、订阅等表结构
- `000002_add_device_site_role`：设备站点角色
- `000003_add_subscription_free_upgrade`：免费升级订阅字段
- `000004_add_relay_mode`：Relay 模式和 Relay 节点表
- `000005_add_email_verifications`：注册邮箱验证码
- `000006_add_path_fallback`：按客户端路径状态机、peer stats 和 applied path
- `000007_subnet_gateway_status`：子网网关状态上报
- `000008_relay_health_path_quality`：Relay 健康字段和客户端本地路径质量表
- `000009_subnet_service_probes`：主站点 LAN TCP 服务探测状态

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
  --lan-cidr 192.168.50.0/24
```

`--out-interface` 可省略，Agent 会根据到 LAN 探测目标的系统路由自动推断；也可显式指定，例如 `--out-interface eth0`。默认探测目标为 LAN CIDR 的第一个可用地址，也可用 `--lan-target 192.168.50.1` 指定网关或稳定在线的内网主机。

查看或关闭：

```bash
sudo sdwan-agent subnet-gateway status --lan-cidr 192.168.50.0/24
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
- Linux Agent 安装脚本默认使用 `SDWAN_VERSION=v1.2.2`
- Controller 需要配置强随机 `ADMIN_JWT_SECRET`
- Bootstrap 和 Relay Token 必须独立生成并妥善保存

## 已知限制

- 暂无 ACL，账号内设备默认互通。
- 暂无 MagicDNS。
- 暂无 Exit Node。
- 自动 fallback 当前只支持单主站点和单活动自建 Relay。
- Relay 健康检查基于心跳；尚无容量、地域和多 Relay 调度。
- Windows 端暂未实现子网网关能力。
- 支付逻辑暂未接入真实支付渠道。
- 审计日志字段预留但未完整产品化。
