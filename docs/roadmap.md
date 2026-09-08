# CodeGate（码界）工程实施路线图与里程碑规划

---

## 一、实施策略与原则

为确保项目快速落地见效并可控演进，CodeGate 采取 **“核心先行、敏捷迭代、场景牵引、渐进增强”** 的建设原则：
1. **优先保障研发核心通道**：先跑通最关键的 OpenAI 协议代理与 API Key 鉴权，尽早向内部研发工具（Claude Code、OpenCode）开放测试。
2. **渐进式高可用加固**：逐步引入多后端负载均衡、主动探针熔断、默认模型降级与 KV Cache 会话亲和加速。
3. **安全审计与企业级治理**：分步接入企业 SSO、细粒度配额策略、流式内容聚合还原与 4 阶段原始报文转储。
4. **前端交互与极致交付体验**：同步打磨 Web 控制台、原生 Chat 体验与 Go 单二进制自包含交付体系。

---

## 二、里程碑阶段规划全览

```
[Phase 1: 核心代理 MVP]       ──>  [Phase 2: 高可用与并发治理]
  • OpenAI 兼容代理 (/v1/chat)       • 多后端加权轮询 / 最少连接
  • 纯 Go SQLite + API Key 鉴权      • 主动健康检查与熔断自愈
  • SSE 长连接保活心跳               • 默认模型 Fallback 降级
  • 单二进制嵌入编译原型             • 实例级原子 CAS 并发控制
        │                                  │
        ▼                                  ▼
[Phase 3: 高级调度与企业安全] ──>  [Phase 4: 全功能门户与协同赋能]
  • HRW KV Cache 会话亲和性调度      • 现代化 Web 监控大屏
  • Responses 直通代理 (Agent专属)   • 原生极速 Chat (折叠思维链)
  • 企业 SSO (OIDC/Azure) 登录       • 外部网关一键批量导入
  • 流式响应还原 & 4阶段报文转储     • 全面赋能 code-shield/pipeline
```

---

## 三、各阶段详细任务拆解

### 阶段一：核心代理引擎与单二进制原型（Phase 1: MVP）
> **核心目标**：实现 OpenAI 基础协议代理转发与鉴权，打通纯 Go 单二进制交付闭环。

- [ ] **项目骨架与构建体系**：
  - 初始化 Go 模块与目录分层（`cmd/`, `internal/api/`, `internal/config/`, `internal/service/`, `internal/store/`）。
  - 配置 `Makefile`，支持后端编译、前端静态资源嵌入与跨平台构建。
- [ ] **纯 Go SQLite 数据模型初始化**：
  - 集成 `modernc.org/sqlite` 驱动，创建 Users、APIKeys、Models、Backends、AccessLogs 基础数据表结构。
- [ ] **OpenAI 兼容接口实现**：
  - 实现 `/v1/chat/completions`（支持流式 SSE 打字机与非流式聚合返回）。
  - 实现 SSE 长连接心跳保活（Keep-Alive Ping 协程，防超时断连）。
  - 实现 `/v1/models` 接口，根据可用模型返回列表。
- [ ] **真实 Token 提取与落盘**：
  - 从后端服务商的流式结束事件或响应体中提取官方精确 Usage 并异步落盘。
- [ ] **基础 API Key 鉴权中间件**：
  - 支持 Header `Authorization: Bearer <sk-...>` 校验，支持内存缓存加速。

---

### 阶段二：多后端高可用与并发反压治理（Phase 2: HA & Concurrency）
> **核心目标**：实现一个模型挂载多个物理后端实例，实现动态负载均衡、容灾熔断与并发保护。

- [ ] **多后端实例模型（1:N）架构**：
  - 支持一个逻辑 Model 关联多个 Backend 实例，各实例拥有独立的 Base URL、真实 Key、权重与最大并发限制。
- [ ] **负载均衡算法实现**：
  - 实现加权轮询（Weighted Round-Robin）。
  - 实现加权最少连接（Weighted Least-Connections）。
- [ ] **主动健康检查与自动熔断**：
  - 后台探测协程周期性发送轻量探活请求；
  - 连续失败超过阈值自动熔断并移出路由池；探测恢复后秒级自动归入。
- [ ] **默认模型 Fallback 容灾降级**：
  - 当目标逻辑模型全部后端离线或未配置时，自动降级至备选模型（`default_model`），保障下游调用不断摆。
- [ ] **实例级原子 CAS 并发控制**：
  - 使用 `atomic.CompareAndSwapInt32` 实现细粒度并发占槽与安全释放，杜绝瞬间高并发击穿单实例。
- [ ] **基础速率限制（RPM）**：
  - 基于内存滑动窗口计数器，超出上限响应 HTTP 429。

---

### 阶段三：高级调度、企业安全与审计排障（Phase 3: Advanced Routing & Audit）
> **核心目标**：针对 AI Agent 专属优化，实现 KV Cache 亲和性加速、企业 SSO 及生产级排障转储。

- [ ] **KV Cache 亲和性会话路由（HRW 算法）**：
  - 提取 `X-Session-ID` 等多源特征，基于 Rendezvous Hashing 精准锁定多轮对话后端节点；
  - 实现溢出保护（Spillover Protection），节点饱和或异常时自动降级至次优节点。
- [ ] **Responses 直通代理端点（`/v1/responses`）**：
  - 专为 OpenCode、Codex CLI 等现代代码编写 Agent 定制，请求体载荷原始字节直通，杜绝参数畸变。
- [ ] **模型参数静默覆盖与 Header 注入**：
  - 支持通过配置静默注入或覆盖参数（如 `enable_thinking: false`），支持 `__header__` 注入上游定制标头。
- [ ] **企业级身份与认证体系**：
  - 集成 OIDC / Azure AD 单点登录（SSO）；
  - 支持用户自助注册 + 管理员审核激活流；
  - 实现 JWT 滑动会话刷新（Sliding Session Refresh）。
- [ ] **多维配额策略引擎（QuotaPolicy）**：
  - 支持每日请求额度（RPD）管控与跨午夜可用时段限制（如 `22:00-06:00`）。
- [ ] **全链路审计与 4 阶段原始报文转储（Raw Dumps）**：
  - 全链路 Access Log 脱敏记录；
  - 异步将流式 SSE Chunk 还原聚合为完整对话记录；
  - 实现故障时捕获四阶段报文转储，秒级定位是供应商故障还是网关参数问题；
  - 7 天历史日志定时自动轮转清理。

---

### 阶段四：现代化控制中心与生态协同赋能（Phase 4: Portal & Ecosystem）
> **核心目标**：交付现代化深浅双模前端控制台与原生 Chat 对话台，全面联动赋能内部产品矩阵。

- [ ] **Web 管理与控制中心开发**（React 18 + TS + AntD）：
  - 严格遵循团队设计规范（Vanilla CSS，支持深浅双模主题，无硬编码颜色）；
  - **普通用户工作台**：个人用量大盘、API Key 自助创建/吊销、历史调用流水抽屉。
  - **管理控制大屏**：全局实时监控大屏（Recharts 图表）、用户审批、模型/后端/策略 CRUD。
  - **外部网关一键批量导入**：支持拉取第三方 OpenAI 兼容网关的模型列表，一键批量初始化。
- [ ] **原生极速 Chat 交互台**：
  - 多模型下拉即用，支持 Markdown 渲染与语法高亮；
  - 深度思考模型思维链（Reasoning）气泡折叠展示；
  - 支持流式生成随时主动“停止生成”（AbortController）。
- [ ] **系统运行时配置热重载（Hot-Reloading）**：
  - 实现配置发布-订阅机制，控制台变更即时热生效，不断开已有长连接。
- [ ] **内部系统协同联动与验收**：
  - 联动 `code-shield`：将其 AI 检视逻辑配置为指向 CodeGate，验证多实例高可用分流与并发反压效果；
  - 联动 `code-pipeline`：为流水线分配独立 API Key，验证成本计费核算；
  - 团队公测：向研发团队开放，接入 Claude Code、OpenCode、Cursor 与 IDE 插件。

---

## 四、各阶段验收标准

| 阶段 | 交付物清单 | 核心验收指标 |
| :--- | :--- | :--- |
| **Phase 1** | 单二进制可执行程序、示例配置、基础 API Key | • `/v1/chat/completions` 流式透传延迟 < 5ms<br>• 官方 Token 精确提取无误<br>• 支持无 CGO 交叉编译 |
| **Phase 2** | 多后端负载均衡器、健康探针、降级引擎 | • 单后端模拟故障 30s 内自动剔除并容灾切换<br>• 后端并发达到上限时平滑等待或流转次优<br>• 默认模型 Fallback 成功率 100% |
| **Phase 3** | HRW 会话亲和调度器、SSO、报文转储引擎 | • 多轮长对话锁定同一节点，TTFT 显著降低<br>• OpenCode 工具调用直通无字段损耗<br>• 异常时精准产出 4 阶段原始报文转储 |
| **Phase 4** | 嵌入式 Web 控制台、原生 Chat 界面、监控大屏 | • 浏览器开箱即用，支持深浅主题无缝切换<br>• 动态热更新配置不断连接<br>• 与 code-shield 联动测试 0 报错 |
