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
