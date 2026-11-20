# 客户端：自动 Relay fallback 本地执行器

当前自动 Relay fallback 采用“Controller 策略 + Agent 本地快速执行”的模型。

## 1. 职责划分

Controller 负责：

- 判断账户是否允许 `auto` / `relay` 模式。
- 校验是否存在健康的活动 Relay。
- 下发 `relay_policy`、`relay_candidates`、直连 peer 和 Relay peer。
- 接收 Agent 上报的 `device_path_quality`。
- 在管理台展示路径状态。

Agent 负责：

- 保存上一份 Controller 原始 Netmap 到本地 `agent.json` 的 `last_netmap`。
- 每轮采集 WireGuard peer stats。
- 维护本地 `path_qualities`。
- 在 `auto` 模式下快速切换业务 `AllowedIPs` 的归属。
- 重新渲染并应用 WireGuard 配置。

## 2. 本地切换规则

仅当 `relay_policy=auto` 时启用本地执行器。

当前默认规则：

- 直连 peer 连续失败后切到 Relay。
- Relay 至少驻留 60 秒，避免来回抖动。
- 直连恢复并稳定 30 秒后切回 direct。
- `AllowedIPs` 始终只归属一个当前业务路径。
- 非当前路径仍保留 peer 和 endpoint，用于持续探测。

## 3. WireGuard 配置变化

直连正常时：

```text
direct peer:
  AllowedIPs = 对端 /32 + 已批准子网

relay peer:
  无业务 AllowedIPs，仅用于探测
```

直连失败并 fallback 后：

```text
direct peer:
  无业务 AllowedIPs，仅用于探测

relay peer:
  AllowedIPs = 对端 /32 + 已批准子网
```

这样切换时不需要 Controller 立即 bump netmap version，Agent 可以在本地完成快速恢复。

## 4. 本地状态

Agent 在配置文件中保存：

```text
last_netmap
path_qualities
```

`path_qualities` 包含：

```text
peer_device_id
path_type: direct / relay
current_path
endpoint
latest_handshake_at
rx_bytes / tx_bytes
last_rx_at
fail_count
healthy_since
switched_at
local_reason
```

这些状态会通过 `POST /api/v1/devices/poll` 上报给 Controller。

## 5. 当前限制

- 当前只支持单主站点和单活动自建 Relay。
- 本地执行器只移动 `AllowedIPs`，不做多 Relay 选择。
- Relay 故障时还没有多 Relay 兜底。
- 路径质量目前基于 WireGuard 握手时间、收包变化和失败次数，不包含主动 RTT 探测。

后续可以在这个基础上增加 direct/relay RTT、丢包率、Relay 容量、地域和多 Relay 调度。
