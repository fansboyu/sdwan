# 多 Hub 与安全升级路线

## 当前限制

当前数据模型以一个账号一个 `main_site` 为中心：账号汇总只选一个主站点，`peer_paths` 以 Client-Hub 设备对为键，子网发布和 Relay AllowedIPs 都围绕该 Hub 生成。因此把第二台设备简单设成 `main_site`，会覆盖既有主站点角色/路由语义，并不能得到多 Hub。

## 建议的多 Hub 模型

采用“企业网络 -> 站点 -> 设备”的层次，不让 Hub 直接等同于某一台设备：

```text
Enterprise Network
  ├─ Site: HQ (Hub)
  │    └─ hub device(s), LAN routes
  ├─ Site: Region-East (Hub)
  │    └─ hub device(s), LAN routes
  └─ Site: Branch-17 (Spoke)
       └─ clients / LAN routes
```

推荐分阶段实现：

1. 新增 `sites` 和 `site_devices`，设备可从旧 `site_role=main_site` 映射到一个默认 HQ Hub Site。保持旧字段一段兼容期，不直接删除。
2. 将 `subnet_routes` 的归属从“设备”提升为“站点出口”，记录 CIDR、发布站点、下一跳 Hub、审批人和优先级；同企业 CIDR 重叠继续拒绝，支持例外策略前必须先定义最长前缀/优先级行为。
3. 将 `peer_paths` 从 Client-Hub 专用关系改成 `path_links`（两站点之间的链路），关联 direct/Relay 候选、generation、双端应用确认和业务探测结果。
4. 第一版先做分层 Hub-Spoke：每个 Spoke 配置一个首选区域 Hub 和一个备份 Hub，路由只向必要站点传播。不要立即做全量 Hub Mesh，否则 Peer 数量和 AllowedIPs 冲突/传播复杂度会快速增长。
5. 定义跨 Hub LAN 路由的来源、下一跳、环路检测、重复路由和故障收敛规则。区域 Hub 学习下属站点路由，HQ Hub 只接收批准的路由汇总。
6. UI 增加站点列表、站点拓扑、Hub 主备、路由来源/下一跳和冲突定位；现有“一个主站点”总览不能直接扩充成多 Hub 管理页。

## 升级与兼容建议

当前 Controller 会启动时自动执行数据库 migration；Agent 有版本上报和升级提示，但这不是完整的自动更新/回滚系统。生产升级建议采用 expand-migrate-contract：

1. 发布前备份数据库和部署配置，验证恢复流程；记录当前镜像/tag、数据库 migration 版本及 Agent 版本分布。
2. 先发兼容性版本：新增字段允许 NULL/默认值，旧 Controller/Agent 忽略未知字段；不要在同一次发布中删除旧字段或改变 API 语义。
3. 逐台更新 Relay Agent 和 Hub Agent，检查心跳、同步、握手、子网路由及客户端版本覆盖率。Relay 健康升级期间显示 `awaiting_data_plane`，不能误判为已可承载业务。
4. 再滚动更新 Controller/Web。启动迁移后检查 migration 版本、`/readyz`、数据库连接、账号 Netmap 和路由状态；小租户 canary 成功后再扩大。
5. 按客户端平台分批升级 Linux/Windows Agent，确保新协议功能只对达到最低版本的设备启用。保留 direct 模式及显式恢复开关。
6. 稳定运行一个观察周期后，下一版本再移除兼容旧字段/旧 API 的代码。schema 收缩不要与第一阶段代码发布捆绑。

回滚规则：应用代码可以回滚到仍兼容扩展 schema 的版本；数据库 migration 默认前滚修复，不建议生产直接执行 down migration。只有在确认新增数据可丢弃、旧应用兼容且已演练时才回滚 schema。Relay/客户端升级应支持分批回退；任何版本切换都不能自动把 `relay` 强制策略悄悄改成 direct。

## 本次数据库变化

Migration 11/12 增加 Relay 同步与真实握手状态、子网路由近期诊断历史、CIDR 冲突事件及开放冲突唯一索引。迁移为增量字段/表/索引；旧设备仍可注册和直连，Relay Agent 升级后才能提供新的数据面健康信号。
