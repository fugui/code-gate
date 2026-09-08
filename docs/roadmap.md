# CodeGate（码界）工程实施路线图与里程碑规划

---

## 一、实施策略与原则

为确保项目快速落地见效并与公司 `code-*` 系列基础架构深度融合，CodeGate 采取 **“核心先行、敏捷迭代、协议感知、精准计费”** 的建设原则：
1. **统一数据底座先行**：接入公司标准 PostgreSQL 数据库，搭建 GORM 数据模型与基础迁移。
2. **打通 OpenAI 核心代理与 Credits 计费**：跑通 `/v1/chat/completions` 与 `/v1/responses` 代理，尽早支持内部研发工具（Claude Code、OpenCode）及代码扫描服务（`code-shield`）。
3. **渐进增强协议感知与高可用调度**：自动标识后端协议支持能力，严格按协议能力直通路由；实现多后端负载均衡、默认模型 Fallback 容灾及 HRW KV Cache 亲和加速。
4. **安全治理与精细化运营**：落地日/周双周期 Credits 配额（周配额为日配额 4 倍）、企业 SSO 单点登录与 4 阶段原始报文转储。

---

## 二、里程碑阶段规划全览

```
[Phase 1: 核心代理与 PG 底座]  ──>  [Phase 2: 协议感知与高可用治理]
  • 共享 PostgreSQL (GORM v2)        • 后端协议能力自动探针 (Chat/Responses)
  • OpenAI 兼容代理 (/v1/chat)       • 协议感知智能直通路由 (严格匹配后端)
  • Credits 基础计费 (1 / 0.1 / 5)   • 双周期配额管控 (周配额为日配额 4倍)
  • SSE 长连接心跳保活               • 多后端加权轮询 / 最少连接 / 自动熔断
        │                                  │
        ▼                                  ▼
[Phase 3: 高级调度与企业安全]  ──>  [Phase 4: 全功能门户与协同赋能]
  • HRW KV Cache 亲和加速 (结合优惠) • 现代化 Web 监控大屏 (Credits 走势)
  • 实例级原子 CAS 并发控制          • 原生极速 Chat (折叠思维链)
  • 企业 SSO (OIDC/Azure) 登录       • 外部网关一键批量导入
  • 流式响应还原 & 4阶段报文转储     • 全面赋能 code-shield / code-pipeline
```

---

## 三、各阶段详细任务拆解

### 阶段一：核心代理引擎与 PostgreSQL 基础底座（Phase 1: MVP）
> **核心目标**：完成共享 PostgreSQL 建模，跑通 OpenAI 核心协议代理转发与 Credits 基础费率核算。

- [ ] **项目骨架与 PostgreSQL 接入**：
  - 初始化 Go 模块与分层结构（`cmd/`, `internal/api/`, `internal/config/`, `internal/service/`, `internal/models/`）。
  - 集成 PostgreSQL 驱动（`gorm.io/driver/postgres`），实现连接池配置（`max_open_conns`, `max_idle_conns`）与数据库自动迁移（AutoMigrate）。
  - 创建核心表结构：`users`, `quota_policies`, `credits_wallets`, `models`, `backends`, `api_keys`, `access_logs`。
- [ ] **OpenAI 兼容接口与直通转发**：
  - 实现 `/v1/chat/completions`（支持流式 SSE 打字机与非流式聚合返回）。
  - 实现 SSE 长连接心跳保活（Keep-Alive Ping 协程，防前置代理超时）。
  - 实现 `/v1/models` 端点，支持动态模型目录过滤。
- [ ] **真实 Token 提取与 Credits 基础核算**：
  - 从后端服务商流式结束事件或响应体中提取官方真实 Usage（输入、缓存命中、输出 Token）；
  - 实现基础 Credits 公式核算：$\text{Cost} = \frac{\text{Input} \times 1.0 + \text{CacheHit} \times 0.1 + \text{Output} \times 5.0}{1000} \times \text{Multiplier}$；
  - 异步将调用日志与 Credits 消耗落库至 PostgreSQL。
- [ ] **基础 API Key 鉴权中间件**：
  - 支持 Header `Authorization: Bearer <sk-...>` 校验，辅以本地内存高速缓存。

---

### 阶段二：协议感知调度、双周期配额与高可用治理（Phase 2: Protocol-Aware & HA）
> **核心目标**：实现后端协议能力自动探测、协议感知精准路由、日/周 Credits 配额治理与多后端容灾。

- [ ] **后端协议能力自动标识（Capabilities Probing）**：
  - 后台健康探针协程定期探测各物理实例对 `/v1/chat/completions` 和 `/v1/responses` 的支持状态；
  - 探测到可用或参数错误状态码时自动打标对应能力（`chat` / `responses`），并在内存动态维护。
- [ ] **协议感知直通路由（Protocol-Aware Routing）**：
  - 收到 `/v1/responses` 直通请求时，**仅调度到已被标识/声明支持 `responses` 协议的后端节点**；
  - 若无可用后端支持该协议，返回标准 501 状态码或触发具备该能力的备选模型 Fallback，杜绝盲目转发导致的 404/405 报错。
- [ ] **双周期弹性 Credits 配额引擎**：
  - 实现每日限额（Daily）与每周限额（Weekly）两级校验；
  - **默认将周配额配置为日配额的 4 倍**，实现工作日弹性用量与总预算严格锁死；
  - 请求前内存快速预检，超限即时返回 429 提示；
  - 定时重置任务（每日 00:00 与每周一 00:00 自动结转）。
- [ ] **多后端高可用与熔断降级**：
  - 实现加权轮询（Weighted Round-Robin）与加权最少连接（Weighted Least-Connections）；
  - 连续失败超过阈值自动熔断移出路由池，恢复后秒级归入；
  - **默认模型 Fallback 容灾**：当主模型全部后端异常时，自动平滑转接至备选模型。

---

### 阶段三：高级调度、企业安全与排障诊断（Phase 3: Advanced Routing & Audit）
> **核心目标**：面向 AI Coding Agent 落地 KV Cache 亲和性加速，完善企业 SSO 与 4 阶段原始报文诊断。

- [ ] **KV Cache 亲和性会话路由（HRW 算法）**：
  - 提取多源会话特征（`X-Session-ID`、Body 中的 `session_id` 等），使用 Rendezvous Hashing 锁定承载后端；
  - 配合 0.1 Credits 的缓存优惠费率，实现响应延迟与计算点数双重降低；
  - 实现溢出保护（Spillover Protection）：亲和节点达并发上限或故障时顺延至次优节点。
- [ ] **实例级原子 CAS 并发控制**：
  - 使用 `atomic.CompareAndSwapInt32` 对单后端实例进行高并发安全占槽与释放，严防物理算力集群被打爆。
- [ ] **模型参数透明注入与 Header 定制**：
  - 支持网关层静默补充参数（如 `enable_thinking: false`）与 `__header__` 上游定制标头注入。
- [ ] **企业身份认证与全生命周期安全**：
  - 接入公司统一 OIDC / Azure AD 单点登录（SSO）；
  - 支持用户自主注册与管理员审核激活流；
  - 实现 API Key 限定到期日与限定模型白名单。
- [ ] **全链路审计与 4 阶段原始报文转储（Raw Dumps）**：
  - 全结构化脱敏 Access Log；
  - 异步将流式 SSE Chunk 聚合还原为完整对话记录；
  - 实现错误请求时的 4 阶段原始报文转储（客户端输入 -> 转给后端 -> 后端响应 -> 返回客户端），秒级定位故障根因；
  - 自动清理与分区策略（默认保留 7 天）。

---

### 阶段四：现代化控制大屏与生态协同赋能（Phase 4: Portal & Ecosystem）
> **核心目标**：交付内嵌深浅双模前端控制中心与原生 Chat 对话台，全面联动赋能内部产品矩阵。

- [ ] **Web 控制中心开发**（React 18 + TS + AntD）：
  - 遵循团队样式规范（Vanilla CSS，支持深浅双模主题，无硬编码颜色）；
  - **普通员工端**：个人日/周剩余 Credits 仪表盘、API Key 自助创建/吊销、历史流水明细（Token / Credits 拆解）。
  - **管理控制大屏**：全局实时指标卡（Recharts 走势）、后端协议能力透视卡片、模型/后端/配额策略可视化编排。
  - **外部网关一键批量导入**：支持拉取第三方 OpenAI 兼容网关模型列表，一键批量初始化。
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
| **Phase 1** | 核心代理服务、PostgreSQL 数据库表、基础 API Key | • `/v1/chat/completions` 流式透传延迟 < 5ms<br>• GORM 连接池稳定，Usage 提取与 Credits 基础核算准确<br>• 数据库自动迁移成功 |
| **Phase 2** | 协议自动探针、协议感知路由器、双周期配额引擎 | • 探针准确识别 Backend 的 `responses` 协议能力<br>• `/v1/responses` 请求 100% 仅分发至兼容后端，零 404 报错<br>• 日/周配额超限拦截准确，周限额严格为日限额 4 倍<br>• 后端故障自动熔断与默认模型 Fallback 成功率 100% |
| **Phase 3** | HRW 会话亲和调度器、SSO、报文转储引擎 | • 多轮长对话锁定同一节点，TTFT 显著降低，计费自动享受 0.1 优惠<br>• 单后端并发达到上限时平滑溢出至次优节点<br>• 异常时精准产出 4 阶段原始报文转储 |
| **Phase 4** | 嵌入式 Web 控制台、原生 Chat 界面、监控大屏 | • 浏览器开箱即用，支持深浅双模主题<br>• 动态热更新配置不断连接<br>• 与 code-shield 联动测试 0 报错 |
