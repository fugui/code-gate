# CodeGate（码界）系统技术架构设计

---

## 一、总体架构分层设计

CodeGate 深度融入公司 `code-*` 系列技术体系，底层持久化接入统一的 **PostgreSQL** 数据库，全面集成 **`code-common` 前后端公共框架**，并共享 **CodeBench** 统一用户管理。

```
                          ┌────────────────────────┐
                          │  客户端 / Web / Agent  │
                          └───────────┬────────────┘
                                      │ HTTP / HTTPS (Chat / Responses)
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 1. 公共接入与脚手架层 (基于 code-common/backend/server)                     │
│    • Gin Engine 统一脚手架            • CORS 跨域治理                       │
│    • Read/Write/Idle 超时控制 (30m+)  • MaxHeaderBytes 安全防御             │
│    • 优雅停机信号捕获 (Graceful Shutdown)                                   │
└─────────────────────────────────────┬───────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 2. 用户认证与独立配额角色层 (Auth & Quota Engine)                           │
│    • 共享 CodeBench 用户认证体系 (code-common/backend/auth 中间件)          │
│    • 用户专属配额映射表 (GateUserQuota)                                     │
│      - 默认角色: guest (初始体验配额，低日/周上限)                          │
│      - 管理员可赋权: developer / vip / 自定义配额策略                       │
│    • API Key 内存高速缓存鉴权         • User-Agent 客户端黑名单拦截         │
│    • Credits 算力点数治理体系         • 每日上限 (Daily) & 每周总量 (Weekly)│
│    • Token 差异费率 (1 / 0.1 / 5)     • 模型专属倍率系数 (0.5x ~ 10x)       │
└─────────────────────────────────────┬───────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 3. 协议感知调度与流量治理引擎 (Routing Engine)                              │
│    • 后端协议能力自动探针 (Capabilities: Chat / Responses)                 │
│    • 协议感知过滤 (Protocol Filter: 仅分发至具备该协议能力的 Backend)       │
│    • 会话特征提取 (Header / Body Session ID)                                │
│    • HRW (Rendezvous) 亲和性路由      • 加权最少连接 (WLC) / 轮询 (WRR)     │
│    • 实例级原子 CAS 并发占槽/释放     • 主动健康探测探针 & 自动熔断剔除     │
│    • 默认模型 Fallback 降级调度       • 参数静默重写 & __header__ 注入      │
└─────────────────────────────────────┬───────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 4. 代理通信与直通核心 (Proxy Core)                                          │
│    • 纯直通代理 (Passthrough Mode)    • 原始载荷字节无损转发                │
│    • SSE 流式打字机透传               • Keep-Alive Ping 保活协程            │
│    • 官方真实 Usage 提取 (Input / CacheHit / Output)                        │
└─────────────────────────────────────┬───────────────────────────────────────┘
                                      │
                   ┌──────────────────┴──────────────────┐
                   ▼                                     ▼
┌─────────────────────────────────────┐ ┌─────────────────────────────────────┐
│ 5. 存储与公共基座 (Persistence)     │ │ 6. 观测审计与前端交互 (Web & Audit) │
│    • PostgreSQL (code-common/gormdb)│ │    • 全链路 Access Log (敏感头脱敏) │
│    • 连接池高并发调优 (Pool Tuning) │ │    • 流式响应内容异步还原重组       │
│    • 内存 LRU Cache (高速鉴权)      │ │    • 4 阶段原始报文转储 (Raw Dumps) │
│    • 异步批量落盘 Credits 账单流水  │ │    • 前端基于 @code/common 组件库   │
│    • embed.FS 前端静态资源内嵌      │ │    • 深浅主题 Design Tokens 规范    │
└─────────────────────────────────────┘ └─────────────────────────────────────┘
```

---

## 二、关键技术选型与公共框架复用

| 模块 / 需求 | 传统/原型方案 | CodeGate 生产决策 | 框架复用与架构收益 |
| :--- | :--- | :--- | :--- |
| **基础脚手架** | 独立搭建 Gin | **`code-common/backend/server`** | 直接复用公共基础配置、中间件流水线与优雅关机生命周期调度。 |
| **认证与凭证** | 自建注册/独立认证 | **`code-common/backend/auth`** | 直接共享 CodeBench 用户数据库与统一 JWT/SSO 鉴权，免去独立维护账号密码。 |
| **持久化连接** | 独立初始化 DB | **`code-common/backend/gormdb`** | 复用团队统一连接池配置规范、慢日志监听与 PostgreSQL 驱动。 |
| **配额管理** | 与用户耦合的简单字段 | **CodeGate 独立配额角色模型** | 用户首次访问默认打标 **`guest`** 角色，管理员可在后台灵活赋予 `developer`/`vip` 等高阶角色与自定义配额。 |
| **前端样式与组件** | 随意手写样式 / 纯 AntD | **`@code/common` 公共前端规范** | 全面遵循团队 Design Tokens 语义颜色变量、`theme.css` 深浅双模主题，复用 `Pagination`、`Drawer` 等成熟组件。 |
| **计费模型** | 单纯请求频次（RPD） | **Credits 算力点数体系（日/周双周期）** | 精准区分输入/缓存命中/输出成本；周配额为日配额 4 倍，兼顾工作日弹性与周预算可控。 |
| **协议适配** | 盲目全量转发 | **协议能力自动识别 + 协议感知直通** | 自动探测并打标后端对 `/v1/responses` 的支持能力，确保仅向兼容后端转发，杜绝 404/405 报错。 |
| **前端交付** | 独立 Nginx 托管 React | **Go `embed.FS` 原生内嵌 React 产物** | 单二进制部署，解耦独立静态服务器，降低运维部署复杂度。 |

---

## 三、用户管理共享与独立配额角色设计

### 1. 架构解耦原理
CodeGate 遵循**“用户认证归一，业务配额自治”**的设计思想：
- **用户认证（Authentication）**：委托给 CodeBench 与 `code-common`，用户身份信息（User ID、姓名、邮箱、部门、全局角色）完全由统一用户系统判定；
- **配额授权（Authorization & Quota）**：CodeGate 在本地 PostgreSQL 维护用户专属的配额绑定表 `gate_user_quotas`。

```
                     CodeBench 统一用户体系
                  (code-common/models.User)
                             │
                             ▼ 首次访问 CodeGate
              ┌──────────────────────────────┐
              │   自动检测并初始化配额记录   │
              │   GateUserQuota              │
              │   Role: "guest" (默认低配额) │
              └──────────────┬───────────────┘
                             │
            ┌────────────────┴────────────────┐
            ▼                                 ▼
   普通用户以 guest 体验运行           管理员在控制台调优提权
   (基础轻量模型，受控额度)           (赋予 developer / vip / 定制策略)
```

### 2. 配额角色定义
1. **`guest`（访客 / 默认角色）**：
   - 任何 CodeBench 合法用户初次访问 CodeGate 时自动生成；
   - 默认分配安全保底额度（例如每日 50 Credits、每周 200 Credits），仅开放基础模型白名单，防止未报备产生大额消耗。
2. **`developer`（主力研发）**：
   - 管理员在控制台为研发工程师赋予；
   - 开放较高日/周 Credits 额度，授权主流代码大模型（如 DeepSeek-V3、Claude 系列）。
3. **`team_lead` / `vip`（架构与核心业务）**：
   - 享受大额或不限额算力，开放全部旗舰模型。
4. **`custom`（自定义策略）**：
   - 管理员可针对特定项目或团队直接覆盖指定个性化的日/周 Credits 阈值与专属模型白名单。

---

## 四、协议感知与后端能力自动标识机制

### 1. 协议能力自动探查（Capability Probing）流程

```
[后台健康与能力探针协程 (Health & Capability Prober)]
                    │
                    ▼ (每隔 30s 探测)
        向 Backend 实例发送轻量探针请求
                    │
        ┌───────────┴───────────┐
        ▼                       ▼
探测 /v1/chat/completions    探测 /v1/responses
        │                       │
        ▼                       ▼
  状态码 200/400/422       状态码 200/400/422
  [确认支持 Chat]         [确认支持 Responses]
        │                       │
        └───────────┬───────────┘
                    ▼
  动态更新该 Backend 内存能力位:
  b.Capabilities = ["chat", "responses"] (或仅 ["chat"])
```

### 2. 协议感知直通调度算法（Protocol-Aware Routing）

```go
// 调度前基于协议进行候选池过滤
func SelectBackendsByProtocol(model *Model, requestedProtocol string) []*Backend {
    var candidates []*Backend
    for _, b := range model.Backends {
        if b.IsHealthy() && b.SupportsProtocol(requestedProtocol) {
            candidates = append(candidates, b)
        }
    }
    return candidates
}
```

- 当接收到 `/v1/responses` 请求：
  1. 调度器仅在声明或探查到支持 `responses` 的健康实例子集中进行选路；
  2. 若该模型下无可用后端支持该协议，系统立即根据配置判定：
     - 若配置了具备该协议的 `default_model`，无缝降级转接；
     - 否则返回规范的 HTTP 501 Not Implemented，并明确告知“当前模型暂无支持 Responses 协议的可用算力节点”，杜绝下游解析畸变。

---

## 五、Credits 算力点数模型与高并发扣减架构

### 1. Credits 费用计算公式

$$\text{Credits} = \left( \frac{\text{Input} \times 1.0 + \text{CacheHit} \times 0.1 + \text{Output} \times 5.0}{1000} \right) \times \text{ModelMultiplier}$$

- **Token 差异费率**：
  - 输入 Token：$1.0$ 点 / 1k Tokens；
  - 缓存命中 Token（Prompt Cache Hit）：$0.1$ 点 / 1k Tokens（立省 90%）；
  - 输出 Token（Completion）：$5.0$ 点 / 1k Tokens。
- **模型倍率乘数（Multiplier）**：
  - 例如 `qwen-2.5-coder-7b` 配置为 `0.5`，`deepseek-v3` 为 `1.0`，`deepseek-r1` / `claude-3-7-sonnet` 为 `5.0` 或 `10.0`。

### 2. 双周期配额管控设计（Daily & Weekly Quota）

```
                     用户请求到达
                          │
                          ▼
            ┌───────────────────────────┐
            │   内存高速配额预检        │
            │   DailyRemaining > 0 ?    │
            │   WeeklyRemaining > 0 ?   │
            └─────────────┬─────────────┘
                          │ YES
                          ▼
            ┌───────────────────────────┐
            │   执行协议感知代理转发    │
            │   (SSE 流式 / 真实 Usage) │
            └─────────────┬─────────────┘
                          │ 完成响应 (defer 钩子)
                          ▼
            ┌───────────────────────────┐
            │   计算单次精确 Credits    │
            │   Cost = f(Token, Multi)  │
            └─────────────┬─────────────┘
                          │
                          ▼
            ┌───────────────────────────┐
            │   内存原子扣减剩余额度    │
            └─────────────┬─────────────┘
                          │ 异步投递 Channel
                          ▼
            ┌───────────────────────────┐
            │   PostgreSQL 批量事务入库 │
            │   (更新日/周累计 & 写流水)│
            └───────────────────────────┘
```

- **配额比例原则**：系统默认且推荐 $\text{WeeklyQuota} = 4 \times \text{DailyQuota}$。
  - **弹性平衡**：一周 5 个工作日内，研发人员在关键攻坚日可打满单日额度，但单周整体总用量被 4 倍日额度锚定，防止出现“周一就把整周算力刷爆”或“持续超高负荷消耗公司算力”的情况。
- **异步通道批量同步**：通过缓冲 Channel 将扣费流水聚合批量写入 PostgreSQL，大幅减轻数据库并发事务压力。

---

## 六、数据库设计规范（PostgreSQL + GORM v2）

遵循团队 `code-*` 系列规范，用户表直接复用 `code-common`，CodeGate 专属模型统一放置在 `internal/models/` 目录下：

1. **`users`**（直接复用 `code-common/backend/models.User`）：
   - 存储全局用户身份，包含 `ID`, `EmployeeID`, `Email`, `Username`, `Name`, `Roles` 等。
2. **`gate_user_quotas`**（CodeGate 用户配额映射表）：
   - `user_id`（外键关联 `users.id`，唯一索引）
   - `role`：配额角色（默认为 `'guest'`，可调整为 `'developer'`, `'vip'` 等）
   - `policy_id`：关联的配额策略 ID（可选）
   - `custom_daily_credits` / `custom_weekly_credits`：个性化覆盖额度（若设置则优先于角色默认值）
   - `created_at`, `updated_at`
3. **`quota_policies`**（配额策略模板表）：
   - `name`：策略名称（如 guest_policy, dev_policy）
   - `daily_credits_limit` / `weekly_credits_limit`（周配额默认为日配额 4 倍）
   - `rate_limit_rpm`：每分钟请求速率限制
   - `time_ranges`：允许可用时间段（JSON 数组，如 `["09:00-18:00"]`，支持跨午夜）
   - `model_whitelist`：允许模型白名单（JSON 数组或 `["*"]`）
4. **`credits_wallets`**（用户日/周算力台账表）：
   - `user_id`、`daily_consumed`、`weekly_consumed`、`last_daily_reset`、`last_weekly_reset`
5. **`models`**（逻辑模型表）：
   - `name`、`default_model`、`multiplier`（倍率乘数）、`model_params`（静默覆盖参数）
6. **`backends`**（物理实例表）：
   - `model_id`、`base_url`、`api_key`、`weight`、`max_concurrency`、`declared_protocols`、`detected_protocols`、`is_healthy`
7. **`api_keys`**（用户专属 API Key 表）：
   - `user_id`、`key_hash`、`expires_at`、`allowed_models`、`is_active`
8. **`access_logs`**（全链路访问审计日志表）：
   - `user_id`、`protocol`、`model`、`input_tokens`、`cache_hit_tokens`、`output_tokens`、`cost_credits`、`duration_ms`、`ttft_ms`、`status_code`

---

## 七、前端架构与 Design Tokens 规范

前端管理控制台完全融入 **`code-common/frontend`** 规范：
1. **单一真实源（SSOT）**：全局色彩以 `theme.css` 中的 `--color-*` 语义变量为基准，严格避免硬编码 `#fff`、`#000` 或固定 hex 颜色。
2. **深浅主题双模适配**：卡片与主表面统一使用 `var(--color-bg-surface)`，文字统一使用 `var(--color-text-primary)`，确保在明亮模式与暗色模式下均获得极致质感。
3. **通用分页与导航规范**：全量复用 `@code/common` 的 `Pagination` 组件与 `useSearchParams` URL 历史同步，中间仅平滑展示 5 个连续数字滑动窗口。
4. **统一命名空间**：采用扁平化 BEM 命名，组件统一以 `.code-gate-*` 命名空间组织。
