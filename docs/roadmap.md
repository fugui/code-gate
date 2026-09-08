# CodeGate（码界）工程实施路线图与里程碑规划

---

## 一、实施策略与原则

为确保项目快速落地见效并与公司 `code-*` 系列基础架构深度融合，CodeGate 采取 **“核心先行、复用公共框架、协议感知、自治配额”** 的建设原则：
1. **深度复用公共框架（code-common）**：直接集成 `code-common/backend` 的数据库连接、身份认证中间件与基础 Server 脚手架；前端全面依托 `@code/common` 样式体系与组件库。
2. **共享 CodeBench 用户认证，独立配额管理**：无需重复开发注册与用户基础信息，用户初次进入自动绑定为 `guest` 默认低配额角色，管理员按需提权。
3. **协议感知与精准直通**：自动探测物理后端对 `/v1/responses` 等协议的支持能力，严格按协议能力直通路由；实现多后端负载均衡与容灾降级。
4. **精细化 Credits 算力治理**：落地日/周双周期 Credits 配额（周配额为日配额 4 倍）、Token 差异化费率与 4 阶段原始报文转储。

---

## 二、里程碑阶段规划全览

```
[Phase 1: 核心代理与公共基座]  ──>  [Phase 2: 协议感知与配额引擎]
  • 集成 code-common/backend         • 后端协议能力自动探针 (Chat/Responses)
  • 共享 CodeBench 用户认证          • 协议感知智能直通路由 (严格匹配后端)
  • OpenAI 兼容代理 (/v1/chat)       • 独立配额表 (用户默认 guest 角色)
  • Credits 基础计费 (1 / 0.1 / 5)   • 双周期配额管控 (周配额为日配额 4倍)
        │                                  │
        ▼                                  ▼
[Phase 3: 高级调度与运维排障]  ──>  [Phase 4: 全功能控制台与协同赋能]
  • 管理员用户配额角色赋权管理       • 基于 @code/common 构建 Web 控制台
  • HRW KV Cache 亲和加速 (结合优惠) • 实时监控大屏 (Credits 走势)
  • 实例级原子 CAS 并发控制          • 原生极速 Chat (折叠思维链)
  • 流式响应还原 & 4阶段报文转储     • 全面赋能 code-shield / code-pipeline
```

---

## 三、各阶段详细任务拆解

### 阶段一：核心代理引擎与公共基座整合（Phase 1: MVP）
> **核心目标**：集成 `code-common/backend`，打通共享 CodeBench 用户认证与 PostgreSQL 数据库，跑通 OpenAI 核心代理转发与 Credits 基础费率。

- [ ] **集成 `code-common/backend` 公共库**：
  - 在 `go.mod` 中引入 `code-common/backend`（支持本地 replace 或模块引用）。
  - 基于 `code-common/backend/gormdb` 初始化 PostgreSQL 连接池，基于 `code-common/backend/server` 初始化标准 Gin 引擎与优雅停机。
  - 基于 `code-common/backend/auth` 接入统一 JWT 认证中间件，共享 `models.User` 身份体系。
- [ ] **CodeGate 专属数据表初始化**：
  - 创建并自动迁移：`gate_user_quotas`, `quota_policies`, `credits_wallets`, `models`, `backends`, `api_keys`, `access_logs`。
- [ ] **OpenAI 兼容接口与直通转发**：
  - 实现 `/v1/chat/completions`（支持流式 SSE 打字机与非流式聚合返回）。
  - 实现 SSE 长连接心跳保活（Keep-Alive Ping 协程，防前置代理超时）。
  - 实现 `/v1/models` 端点，支持动态模型目录过滤。
- [ ] **真实 Token 提取与 Credits 基础核算**：
  - 从后端服务商流式结束事件或响应体中提取官方真实 Usage（输入、缓存命中、输出 Token）；
  - 实现基础 Credits 公式核算：$\text{Cost} = \frac{\text{Input} \times 1.0 + \text{CacheHit} \times 0.1 + \text{Output} \times 5.0}{1000} \times \text{Multiplier}$；
  - 异步将调用日志与 Credits 消耗落库至 PostgreSQL。
- [ ] **API Key 鉴权与缓存加速**：
  - 支持 Header `Authorization: Bearer <sk-...>` 校验，结合内存 LRU 缓存加速。

---

### 阶段二：协议感知调度、配额引擎与高可用治理（Phase 2: Protocol-Aware & Quota）
> **核心目标**：实现后端协议能力自动探测、协议感知精准路由、用户默认 `guest` 角色与日/周 Credits 配额治理。

- [ ] **后端协议能力自动标识（Capabilities Probing）**：
  - 后台健康探针协程定期探测各物理实例对 `/v1/chat/completions` 和 `/v1/responses` 的支持状态；
  - 探测到可用或参数错误状态码时自动打标对应能力（`chat` / `responses`），并在内存动态维护。
- [ ] **协议感知直通路由（Protocol-Aware Routing）**：
  - 收到 `/v1/responses` 直通请求时，**仅调度到已被标识/声明支持 `responses` 协议的后端节点**；
  - 若无可用后端支持该协议，返回标准 501 状态码或触发具备该能力的备选模型 Fallback，杜绝盲目转发导致的 404/405 报错。
- [ ] **CodeGate 独立配额引擎与 `guest` 默认角色**：
  - 接入拦截逻辑：当合法的 CodeBench 用户初次发起请求时，自动创建 `GateUserQuota` 记录，默认绑定 **`guest` 配额角色**；
  - 为 `guest` 角色分配基础体验额度与基础模型白名单；
  - 实现每日限额（Daily）与每周限额（Weekly）两级校验，**默认周配额为日配额的 4 倍**；
  - 请求前内存快速预检，超限即时返回 429 提示；定时任务自动完成日/周结转。
- [ ] **多后端高可用与熔断降级**：
  - 实现加权轮询（Weighted Round-Robin）与加权最少连接（Weighted Least-Connections）；
  - 连续失败超过阈值自动熔断移出路由池，恢复后秒级归入；
  - **默认模型 Fallback 容灾**：当主模型全部后端异常时，自动平滑转接至备选模型。

---

### 阶段三：高级调度、配额角色赋权与排障诊断（Phase 3: Advanced Routing & Ops）
> **核心目标**：面向 AI Coding Agent 落地 KV Cache 亲和性加速，实现管理员配额角色赋权与 4 阶段原始报文转储。

- [ ] **管理员用户配额角色赋权管理**：
  - 提供 API 与管理接口，支持管理员检索 CodeBench 用户并调整其配额角色（提权为 `developer`, `team_lead`, `vip` 等）或直接指定自定义日/周额度。
- [ ] **KV Cache 亲和性会话路由（HRW 算法）**：
  - 提取多源会话特征（`X-Session-ID`、Body 中的 `session_id` 等），使用 Rendezvous Hashing 锁定承载后端；
  - 配合 0.1 Credits 的缓存优惠费率，实现响应延迟与计算点数双重降低；
  - 实现溢出保护（Spillover Protection）：亲和节点达并发上限或故障时顺延至次优节点。
- [ ] **实例级原子 CAS 并发控制**：
  - 使用 `atomic.CompareAndSwapInt32` 对单后端实例进行高并发安全占槽与释放，严防物理算力集群被打爆。
- [ ] **模型参数透明注入与 Header 定制**：
  - 支持网关层静默补充参数（如 `enable_thinking: false`）与 `__header__` 上游定制标头注入。
- [ ] **全链路审计与 4 阶段原始报文转储（Raw Dumps）**：
  - 全结构化脱敏 Access Log；
  - 异步将流式 SSE Chunk 聚合还原为完整对话记录；
  - 实现错误请求时的 4 阶段原始报文转储，秒级定位故障根因；
  - 历史数据定时自动轮转清理（默认保留 7 天）。

---

### 阶段四：现代控制大屏与生态协同赋能（Phase 4: Portal & Ecosystem）
> **核心目标**：基于 `@code/common` 交付深浅双模前端控制中心与原生 Chat 对话台，全面联动赋能内部产品矩阵。

- [ ] **Web 控制中心开发（全面基于 `@code/common`）**：
  - 严格遵循团队 Design Tokens 语义颜色变量与 Vanilla CSS 规范，实现极致的深浅双模主题；
  - 复用 `@code/common` 的 `Pagination`、`Drawer`、`StatusBadge` 等组件；
  - **普通员工端**：个人日/周剩余 Credits 仪表盘、API Key 自助创建/吊销、历史流水明细（Token / Credits 拆解）。
  - **管理控制大屏**：全局实时指标卡（Recharts 走势）、后端协议能力透视卡片、用户配额角色提权管理面板、模型/后端/策略 CRUD。
- [ ] **原生极速 Chat 交互台**：
  - 多模型下拉即切，支持完整 Markdown 渲染与代码高亮；
  - 深度思考模型思维链（Reasoning）气泡折叠展示；
  - 支持流式生成中随时点击“停止生成”（AbortController）。
- [ ] **单二进制自包含交付与配置热重载**：
  - Go `embed.FS` 将前端构建产物打入二进制；
  - 实现配置发布-订阅机制，控制台修改模型、费率或实例时毫秒级热生效，长连接完全不中断。
- [ ] **内部系统协同联动与验收**：
  - 联动 `code-shield`：将其 AI 代码检视流量切入 CodeGate，验证多实例高可用与并发反压稳定性；
  - 联动 `code-pipeline`：为流水线分配独立 API Key，核算 Credits 消耗；
  - 团队公测：向研发团队开放，接入 Claude Code、OpenCode、Cursor 与 IDE 插件。

---

## 四、各阶段验收标准

| 阶段 | 核心交付物 | 关键验收指标 |
| :--- | :--- | :--- |
| **Phase 1** | 集成 `code-common` 代理服务、PostgreSQL 表结构、基础 Key 鉴权 | • 成功接入 `code-common/backend`（gormdb / auth / server）<br>• `/v1/chat/completions` 流式透传延迟 < 5ms<br>• Usage 提取与 Credits 基础核算准确 |
| **Phase 2** | 协议自动探针、协议感知路由器、`guest` 配额初始化逻辑 | • 探针准确识别 Backend 的 `responses` 协议能力<br>• `/v1/responses` 请求 100% 仅分发至兼容后端，零 404 报错<br>• CodeBench 用户首访自动打标 `guest` 角色并受基础额度约束<br>• 周配额严格为日配额 4 倍，超限拦截准确 |
| **Phase 3** | 管理员配额赋权接口、HRW 会话亲和调度器、4阶段报文转储 | • 管理员可成功为用户变更配额角色与个性化上限<br>• 多轮长对话锁定同一节点，计费自动享受 0.1 优惠<br>• 异常时精准产出 4 阶段原始报文转储 |
| **Phase 4** | 基于 `@code/common` 的嵌入式 Web 控制台、原生 Chat 界面 | • 浏览器开箱即用，深浅双模主题无缝切换且无硬编码色值<br>• 动态热更新配置不断连接<br>• 与 code-shield 联动测试 0 报错 |
