# CodeGate 管控运维与配置治理系统设计文档

---

## 一、背景与设计动机 (Background & Motivation)

### 1.1 现状与对标背景
随着 CodeGate（码界）核心代理（OpenAI Chat 原生兼容、Responses 直通代理、Credits 算力点数计费与双周期配额）的逐步落地，系统在底层调度与数据面上已具备了生产级能力。然而，在**运维管控（Control Plane）与系统配置治理**维度，网关仍需一套更加直观、灵活、易于维护的管理后台体系。

通过深入剖析成熟 AI 网关系统 `/home/fugui/modelgate` 中的「配置管理」模块，我们学习总结了其在多 Tab 架构下的 7 大核心功能：
1. **用户管理 (`UserTab`)**：用户多维检索、生命周期审核与策略绑定。
2. **模型管理 (`ModelTab`)**：1:N 逻辑模型与物理实例嵌套树形展现、模型参数可视化注入，以及亮眼的**上游网关批量自动导入模型（Gateway Auto-Import）**。
3. **配额策略 (`PolicyTab`)**：独立的配额模板池、速率限制、模型白名单与跨午夜可用时段动态配置。
4. **健康监控 (`HealthTab`)**：后端节点健康探活矩阵、实时网络延迟与细粒度并发水位报警。
5. **系统配置 (`SystemTab`)**：核心服务超时配置、系统基础信息，以及**客户端 User-Agent 动态拦截规则表**。
6. **全员日志 (`AdminLogs`)**：全链路访问日志检索与用户身份友好反查映射。
7. **7天TOP用户 (`AdminTopUsers`)**：按日透视团队算力消耗与 Token 支出矩阵。

### 1.2 CodeGate 差异化定位与“有所为、有所不为”
结合 CodeGate 的企业级定位，我们明确以下架构取舍原则：
- **坚决复用统一用户基座，不造重复轮子**：
  `modelgate` 内置了独立的用户注册、登录与密码修改系统。而 **CodeGate 深度融入 `code-bench` 统一用户生态与 `code-common` 架构**，因此**坚决不重复开发本地用户注册与密码维护模块**；系统聚焦于用户在网关内部的**配额角色映射（`GateUserQuota`）与策略赋权**。
- **强化 Credits 算力核算优势**：
  `modelgate` 仅支持粗放的每日请求次数（RPD）限制。CodeGate 将其全面升维为**精准 Credits 算力点数体系（区分输入、缓存命中、输出费率及模型倍率）**，并支持**每日限额与每周总量（4倍自动联动）双周期管控**。
- **协议感知直通与并发治理赋能**：
  深度结合网关独有的 `/v1/responses` 协议识别能力与物理后端原子 CAS 无锁并发控制，将底层探针与并发状态完整透传至前端大屏。

---

## 二、管控运维总体架构与模块全景 (System Architecture)

```
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│                           CodeGate 管控运维体系 (Control Plane)                          │
├─────────────────────────────────────────────────────────────────────────────────────────┤
│ [展现层]  CodeGate Admin Web 控制台 (@code/common / Vanilla CSS / 深浅双模 Design Tokens)│
├──────────────┬──────────────┬──────────────┬──────────────┬──────────────┬──────────────┤
│ 1.模型后端   │ 2.配额策略   │ 3.健康与并发 │ 4.系统与安全 │ 5.算力透视   │ 6.用户与日志 │
│   层次治理   │   可视化编排 │   水位监控   │   动态规则   │   消耗大账   │   全景审计   │
├──────────────┴──────────────┴──────────────┴──────────────┴──────────────┴──────────────┤
│ [接口层]  /v1/admin/* RESTful APIs (Gin Engine + RequireAdmin 角色鉴权中间件)            │
├─────────────────────────────────────────────────────────────────────────────────────────┤
│ [核心引擎]                                                                              │
│ • 模型与后端 1:N 映射引擎   • Credits 双周期配额引擎       • Prober 协议与健康定时探针   │
│ • Gateway 批量导入解析器    • 跨午夜时间段校验算法         • 原子 CAS 并发计数器         │
│ • 客户端 UA 过滤热重载器    • 7天算力逐日交叉透视聚合器   • CodeBench 真实用户身份关联器 │
├─────────────────────────────────────────────────────────────────────────────────────────┤
│ [持久与缓存] PostgreSQL 数据库 (GORM v2 连接池)  |  本地原子无锁并发计数器 (sync/atomic)   │
└─────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 三、五大核心治理功能模块详细设计

### 模块一：模型与后端 1:N 层次化治理与一键批量导入 (Model & Backend Orchestration)

#### 1. 1:N 逻辑模型与物理后端解耦设计
- **逻辑模型（Model）**：面向客户端暴露的统一接入标识（如 `deepseek-v3`、`claude-3-7-sonnet`）。
  - 支持配置 **算力倍率乘数（`Multiplier`）**，如轻量模型 `0.5x`，旗舰模型 `10.0x`；
  - 支持配置 **默认容灾降级模型（`DefaultModel`）**，当前模型所有实例全军覆没时自动无缝切换；
  - 支持配置 **`ModelParams`（JSON 字符串）**，用于网关透明注入参数（如 `enable_thinking: false` 或注入固定请求头）。
- **物理后端（Backend）**：实际提供推理算力的服务节点。
  - 挂载在指定的 `ModelID` 下，拥有独立 `BaseURL`、私有 `APIKey`（脱敏存储与展示）、调度权重（`Weight`）、最大连接并发（`MaxConcurrency`）；
  - 自动打标支持的协议标签：`["chat"]` 或 `["chat", "responses"]`。
- **前端交互形式**：采用模型折叠树卡片，折叠面板为主模型基础信息与费率乘数，展开后为该模型下挂载的所有物理实例列表，支持直接在模型行下“添加实例”。

#### 2. 上游网关批量自动导入模型 (Gateway Auto-Import)
- **业务场景**：新接入一家大模型供应商（如 OpenAI 官方、Azure、DeepSeek 或内网部署的 vLLM / Ollama 集群）时，手动逐个录入数十个模型极为低效且易出错。
- **导入流程规范**：
  ```
  [管理员] ──输入 (供应商前缀如 'deepseek', BaseURL, APIKey)──> [CodeGate 后端]
                                                                        │
  [上游 /v1/models 接口] <──GET /v1/models (带 Authorization)───────────┤
           │
           └──返回模型列表 JSON: { "data": [{"id": "deepseek-chat"}, ...] }
                                                                        │
  [CodeGate 后端] ──解析模型 ID 并执行幂等写入 (事务保护)────────────────┘
           ├── 1. 若逻辑模型不存在: 创建 Model (Name=model_id, Multiplier=1.0)
           └── 2. 创建关联 Backend (Name=prefix-model_id-1, BaseURL=baseURL, APIKey=key)
  ```
- **参数规格**：
  - `prefix`：字母与数字组合（如 `deepseek`, `vllm`, `google`）；
  - `base_url`：上游接口根路径（如 `https://api.deepseek.com`）；
  - `api_key`：可选上游密钥。

---

### 模块二：独立配额策略池与时段可视化编排 (Quota Policy Management)

#### 1. 策略池与用户角色解耦
告别单用户固定配额角色的生硬逻辑，建立企业级统一策略模板库：
- **策略模型定义 (`models.QuotaPolicy`)**：
  ```go
  type QuotaPolicy struct {
      ID                 uint           `gorm:"primaryKey" json:"id"`
      Name               string         `gorm:"size:64;uniqueIndex;not null" json:"name"`
      Description        string         `gorm:"size:255;default:''" json:"description"`
      DailyCreditsLimit  float64        `gorm:"not null" json:"daily_credits_limit"`
      WeeklyCreditsLimit float64        `gorm:"not null" json:"weekly_credits_limit"` // 默认约为日限额 4 倍
      RateLimitRPM       int            `gorm:"default:60" json:"rate_limit_rpm"`
      TimeRanges         datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"time_ranges"`
      ModelWhitelist     datatypes.JSON `gorm:"type:jsonb;default:'[\"*\"]'" json:"model_whitelist"`
      DefaultModel       string         `gorm:"size:128;default:''" json:"default_model"`
  }
  ```

#### 2. 双周期 Credits 限额联动规范
- 前端在创建/编辑策略时，输入每日 Credits 限额后，**每周限额默认自动乘以 4 进行联动填充**（例如每日 500 Credits，每周自动填入 2000 Credits）；
- 支持管理员按需手动微调周限额，但界面给出提示：“建议维持每周限额为每日限额的 4 倍，以兼顾工作日弹性与企业周度预算红线”。

#### 3. 跨午夜可用时段可视化配置 (Time Ranges Control)
- 界面提供动态时段行列表，支持添加多个时段区间；
- 单行控件组合为：`[开始时间 HH:MM] 至 [结束时间 HH:MM] [删除按钮]`；
- 允许配置跨午夜时段（例如开始 `22:00`，结束 `06:00`，代表夜间批处理闲时可用）；
- 为空或包含 `*` 时代表全天 24 小时可用。

---

### 模块三：物理后端实例全景健康与并发水位监控 (Health & Concurrency Matrix)

#### 1. 监控大盘关键指标
- **顶部指标卡**：
  - 总接入实例数（Total Backends）；
  - 健康在线实例数（Healthy Nodes，绿色指示灯）；
  - 异常熔断实例数（Unhealthy Nodes，红色警告灯）；
  - 平均网络延迟（Average Latency ms）。

#### 2. 细粒度并发实时水位监控（Concurrency Gauge）
- 依据 `models.Backend` 的原子并发计数器（`active_connections`）与配置的 `max_concurrency` 实时计算当前负载率：
  $$\text{Utilization Ratio} = \frac{\text{Active Connections}}{\text{Max Concurrency}}$$
- **三级色彩告警机制**：
  - **正常绿色（Safe）**：水位 $< 70\%$；
  - **负载预警（Warning 橙色）**：$70\% \le \text{水位} < 90\%$；
  - **高危拥塞（Danger 红色闪烁）**：水位 $\ge 90\%$（接近或已触发原子 CAS 反压，可能产生排队或请求溢出）。

#### 3. 协议支持与探针状态
- 每一行实例展示协议支持徽标：`[chat]` 绿色胶囊、`[responses]` 蓝色胶囊；
- 记录连续探活失败次数（`fail_count`）以及最后一次探活时间戳；
- 支持 30 秒自动轮询静默刷新，并提供“立即手动探测”按钮。

---

### 模块四：动态系统配置与客户端安全访问控制 (System Settings & Dynamic Client Filter)

#### 1. 客户端 User-Agent 黑名单在线管理
- **业务痛点**：恶意自动化扫描器（如 `sqlmap`、`nikto` 等）或非授权脚本频繁探测网关，传统静态配置修改需重新编译或重启服务，存在运维窗口期。
- **在线规则表设计**：
  - 列表展示当前激活的拦截规则（规则名称、UA 匹配关键词、启用开关、添加时间、操作）；
  - 支持即时在表单行中输入新规则并点击“添加”；
  - **后端内存热重载机制**：更新数据库或配置后，通过读写锁安全刷新全局 `ClientFilterMiddleware` 内部的黑名单切片，实现 **0 停机、秒级生效**。

#### 2. 核心传输超时可视化维护
- 展示当前 HTTP Server 核心超时参数：
  - 读取请求超时（`read_timeout`，如 `60s`）；
  - 写入响应超时（`write_timeout`，大模型长文本流式生成推荐 `30m`）；
  - 空闲保活超时（`idle_timeout`，如 `300s`）。

---

### 模块五：多维算力透视与团队消耗大账 (Top Consumers Matrix)

#### 1. 7 天算力消耗透视表设计
- **列维度**：
  `[排名]` `[用户姓名/部门]` `[D-6]` `[D-5]` `[D-4]` `[D-3]` `[D-2]` `[昨天]` `[今天]` `[7天总计]`
- **单元格复合展示**：
  - 主指标：请求总次数（次）；
  - 次指标（微缩字号）：$\uparrow$ 输入 Tokens / $\downarrow$ 输出 Tokens；
  - 算力结算：消耗的 Credits 点数。
- **表尾总计行（Grand Total）**：
  自动聚合过去 7 天每天的全局总请求数、总吞吐 Token 以及总消耗点数，为团队预算复盘提供宏观支撑。

#### 2. CodeBench 统一用户身份反查联动
- 针对日志与配额表中的 `user_id`，后端通过关联 CodeBench 核心 `users` 表：
  - 提取用户真实姓名（`name`）；
  - 提取企业邮箱（`email`）；
  - 提取所属部门（`department`）。
- 彻底摒弃前端生硬的 `UID-1` 占位符，全面呈现人性化、可识别的真实人员身份。

---

## 四、前端规范与设计系统适配 (Design Tokens Compliance)

所有新增界面与组件严格遵守 `code-common/rules/GEMINI.md` 的规范：
1. **深浅双模主题无缝适配**：
   - 统一引用 `theme.css` 中的 `--color-bg-surface`、`--color-bg-muted`、`--color-text-primary`、`--color-border-primary` 等语义 Token；
   - 严禁硬编码任何绝对颜色值（如 `#fff`、`#000`、`#111827` 等）。
2. **扁平化 BEM 命名规范**：
   - 样式类统一采用 `.gate-admin-{block}__{element}--{modifier}`。
3. **公共组件深度复用**：
   - 分页器全量复用 `@code/common` 的 `Pagination` 组件，遵守 URL 状态同步、5 页连续数字滑动窗口规范；
   - 弹层全量复用 `@code/common` 的 `Drawer` 抽屉组件。

---

## 五、演进与实施排期路线图 (Roadmap)

```
┌────────────────────────────────────────────────────────────────────────────────┐
│                           配置管理与治理落地里程碑                              │
├────────────────────────────────────────────────────────────────────────────────┤
│ 阶段一：核心治理配置暴露 (Phase 1)                                              │
│ • 新增配额策略管理 (PolicyTab)，支持日/周限额、跨午夜时段与模型白名单可视化编排   │
│ • 新增客户端 UA 访问控制在线规则表 (Client Filter UI)，实现拦截规则动态热更新  │
├────────────────────────────────────────────────────────────────────────────────┤
│ 阶段二：资源接入与健康大盘 (Phase 2)                                            │
│ • 实现上游网关批量自动导入模型 (Gateway Auto-Import)，自动调用 /v1/models 同步 │
│ • 构建物理后端实例全景健康与 CAS 并发水位大盘 (Health Monitor)                 │
├────────────────────────────────────────────────────────────────────────────────┤
│ 阶段三：算力精细化透视与运营 (Phase 3)                                          │
│ • 实现最近 7 天 TOP 算力消费者交叉矩阵大账 (Top Consumers Matrix)              │
│ • 打通 CodeBench 统一用户表，实现全链路日志与台账的真实用户画像反查展示         │
└────────────────────────────────────────────────────────────────────────────────┘
```

---

## 六、检视补遗：交叉对标后的遗漏项清单 (Gap Review Addendum)

> 以下遗漏项通过逐模块交叉比对 `modelgate` 全量源码（前端 7 个 Admin Tab + 4 个普通用户页面 + 后端 domain 层全部 Handler/Service）与 CodeGate 现有代码及设计文档后发现，按重要程度排序。

### 遗漏 1：管理后台 REST API 接口完整性 — 策略与模型生命周期管理接口

**现状**：
设计文档中定义了配额策略池的 CRUD 与模型/后端的层次化治理前端交互，但**未系统化梳理后端需要新增哪些 API 端点**。

**补充设计**：
管理后台需在 `/v1/admin/` 路由组下补齐以下 RESTful 接口（均需挂载 `RequireAdmin` 中间件）：

| 接口 | 方法 | 说明 |
| :--- | :--- | :--- |
| `/v1/admin/policies` | `GET` | 获取所有配额策略列表 |
| `/v1/admin/policies` | `POST` | 创建新配额策略 |
| `/v1/admin/policies/:id` | `PUT` | 更新指定策略（日/周限额、时段、模型白名单等） |
| `/v1/admin/policies/:id` | `DELETE` | 删除指定策略（需校验无用户绑定或提供强制选项） |
| `/v1/admin/models` | `GET` | 获取所有逻辑模型（含 `backend_count` 统计） |
| `/v1/admin/models` | `POST` | 创建逻辑模型（含可选的首个后端自动创建） |
| `/v1/admin/models/:id` | `PUT` | 更新模型属性（倍率、描述、`model_params`、`default_model`） |
| `/v1/admin/models/:id` | `DELETE` | 删除模型及其关联后端（级联或要求先清空后端） |
| `/v1/admin/models/:id/backends` | `GET` | 获取指定模型下的所有物理后端实例 |
| `/v1/admin/backends` | `POST` | 创建物理后端实例（指定 `model_id`） |
| `/v1/admin/backends/:id` | `PUT` | 更新后端属性（`BaseURL`、权重、并发、API Key 等） |
| `/v1/admin/backends/:id` | `DELETE` | 删除物理后端实例 |
| `/v1/admin/backends/:id/toggle` | `PATCH` | 启用/禁用指定后端（不删除，仅从路由池摘除） |
| `/v1/admin/models/import` | `POST` | 批量从上游网关导入模型 |
| `/v1/admin/config/system` | `GET` | 获取当前系统运行时配置快照 |
| `/v1/admin/config/system` | `PUT` | 更新系统配置并触发内存热重载 |
| `/v1/admin/health` | `GET` | 获取所有后端实例的实时健康与并发状态 |
| `/v1/admin/health/probe` | `POST` | 手动触发一次全量后端探活 |
| `/v1/admin/top-consumers` | `GET` | 获取最近 7 天 TOP 算力消费者交叉矩阵 |

---

### 遗漏 2：配额策略删除的安全性校验

**现状**：
设计文档仅讨论了策略的创建与编辑，**未考虑删除策略时的安全约束**。

**补充设计**：
- 删除策略前，后端需检查是否仍有 `GateUserQuota` 记录关联该策略（`policy_id = ?`）；
- 若有关联用户：
  - 方案 A（推荐）：拒绝删除并返回错误提示 `"该策略仍有 N 位用户绑定，请先为用户更换策略"`；
  - 方案 B（可选）：前端弹窗二次确认"将同时重置关联用户至 guest 默认策略"，后端在事务中批量更新 `policy_id = NULL, role = 'guest'`。

---

### 遗漏 3：模型与后端的启用/禁用操作（软删除）

**现状**：
`modelgate` 的 `ModelTab` 和 `HealthTab` 均支持对模型和后端实例的**启用/禁用开关**（`enabled: boolean`），禁用后实例从路由池摘除但数据不删除，可随时一键恢复。CodeGate 数据库已有 `is_enabled` 字段但设计文档未提及前端可视化交互。

**补充设计**：
- **后端实例启用/禁用**：
  - 在后端卡片/行上增加 `Switch` 开关组件；
  - 禁用时从内存路由表中摘除该实例（不影响探针持续监控），启用时自动重新纳管；
  - 禁用状态在 HealthTab 中以 **灰色 `[已禁用]` 徽标** 展示，区别于健康/异常。
- **逻辑模型启用/禁用**：
  - 禁用的模型不出现在 `/v1/models` 列表中，客户端请求该模型将得到 `model_not_found` 错误；
  - 在模型管理树中以 **半透明样式** 呈现，提醒管理员该模型已下线。

---

### 遗漏 4：管理后台前端控制台入口架构 — 菜单与路由规划

**现状**：
设计文档着重讨论了每个功能模块的交互细节，但**未说明这些模块在整体前端控制台中的挂载位置、菜单结构和路由组织**。

**补充设计**：
管控运维功能在 CodeGate 前端中的组织方式需遵循现有 `menu.ts` 的 `MenuGroup` 架构：

```
侧边栏菜单（管控运维 Group, adminOnly: true）
├── /admin/dashboard     → 监控大屏 (已实现)
├── /admin/users         → 用户配额台账 (已实现)
├── /admin/backends      → 模型与后端 (已实现，需升级为 1:N 树状)
├── /admin/policies      → 配额策略管理 (NEW)
├── /admin/health        → 后端健康监控 (NEW)
├── /admin/settings      → 系统配置与安全 (NEW)
├── /admin/top-consumers → 算力消费透视 (NEW)
└── /logs                → 全链路审计日志 (已实现)
```

路由在 `App.tsx` 中按 `/:tab` 动态匹配，或拆分为独立的 `<Route>` 组件；所有新增页面均需在 `menu.ts` 的 `gateMenuConfig.groups[1].items` 中注册 SVG 图标路径与路由。

---

### 遗漏 5：全局数据看板（DashboardStats）与 modelgate 的对标差距

**现状**：
`modelgate` 拥有独立的 **`DashboardStats.tsx` 数据看板** 页面（非管理员可见），展示：
- 全局请求趋势（5 分钟粒度柱状图，Recharts）；
- 并发数与平均时延的双 Y 轴复合图（ComposedChart: Area + Line）；
- **各后端实例的请求数与时延对比图**（可按模型筛选，堆叠柱状图 + 线图叠加）；
- 今日 TOP 模型与 TOP 用户排行。

CodeGate 已有 `Dashboard` 页面（监控大屏），但当前仅展示了**指标卡和 24 小时走势**。对比 modelgate，以下维度是缺失的：
1. **各后端实例维度的请求量与时延对比图**：按 `backend_id` 分组的堆叠柱状图，可以精准看出哪个实例流量过大或响应变慢；
2. **并发数实时走势曲线**：结合后端的原子 CAS 并发计数器，以 5 分钟为粒度展示全局并发峰值走势。

**补充设计**：
在监控大屏页面中新增以下图表区域：
- **后端实例请求分布与时延对比图**（可下钻筛选指定模型）；
- **全局并发水位走势（5 分钟粒度面积图）**；
- 需后端新增 `/v1/admin/dashboard/backend-metrics` 接口，返回最近 24 小时每 5 分钟粒度的各后端请求数与平均时延数据。

---

### 遗漏 6：配置动态热重载机制的完整设计

**现状**：
设计文档在客户端 UA 过滤部分提到了"后端内存热重载"，但**未系统化描述整个配置热重载架构**。`modelgate` 在 `SystemTab` 中提供了完整的"保存所有配置并生效"功能（`PUT /api/v1/admin/config/system`），一次性更新超时配置、前端配置和客户端规则，后端在内存中即时生效。

**补充设计**：
CodeGate 的配置热重载应覆盖以下维度，且均需在后端实现**事件发布-订阅机制**：
1. **模型与后端变更事件**：新增/修改/删除模型或后端后，自动通知路由引擎刷新内存路由表，无需重启服务；
2. **配额策略变更事件**：修改策略后，配额引擎内存缓存自动失效并重新加载最新策略值；
3. **客户端过滤规则变更事件**：读写锁保护的黑名单切片原子替换；
4. **系统超时参数**：对于 `read_timeout` / `write_timeout` / `idle_timeout`，因涉及 `http.Server` 底层参数，**仅在服务重启后生效**，前端需在保存时提示用户"超时配置变更将在服务下次重启后生效"。

---

### 遗漏 7：模型描述（Description）与参数（model_params）的可视化编辑体验

**现状**：
`modelgate` 的 `ModelTab` 在模型创建/编辑弹窗中提供了完善的交互：
- `description` 多行文本域，支持模型用途描述；
- `model_params` 提供 JSON TextArea 编辑器，带占位符示例（如 `{ "enable_thinking": false, "max_tokens": 4096 }`）；
- `context_window` 上下文窗口长度数字输入框；
- 创建模型时可**同时填入首个后端的 BaseURL 和 API Key**，一步完成模型+后端联合创建。

CodeGate 数据库 `Model` 表已定义了 `Description`、`ModelParams` 和 `DefaultModel` 字段，但设计文档未详细描述编辑表单的交互细节。

**补充设计**：
模型创建/编辑弹窗（Modal 或 Drawer）应包含以下控件：
- **模型标识 ID（Name）**：创建时必填，编辑时只读禁用；
- **显示描述**：多行文本域（`<textarea rows={3}>`），用于记录模型能力与适用场景；
- **算力倍率乘数**：数字输入框（`min=0.1, step=0.1`），默认 `1.0`，带提示说明"旗舰模型建议 5.0~10.0, 轻量模型建议 0.5"；
- **默认降级模型**：模型下拉选择器（排除自身），可选，用于配置容灾 Fallback；
- **模型参数注入（JSON）**：多行代码编辑文本域，带语法提示 `{ "enable_thinking": false }`；
- **（仅创建时显示）快捷关联首个后端**：分隔线下方展示可选的 `BaseURL` 和 `API Key` 输入框，填写后同时创建模型与首个后端实例。
