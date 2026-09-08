# CodeGate（码界）系统技术架构设计

---

## 一、总体架构分层设计

CodeGate 采用高内聚、低耦合的轻量级模块化分层架构。系统深度融入公司 `code-*` 系列技术体系，底层持久化接入统一的 **PostgreSQL** 数据库。

```
                          ┌────────────────────────┐
                          │  客户端 / Web / Agent  │
                          └───────────┬────────────┘
                                      │ HTTP / HTTPS (Chat / Responses)
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 1. 网络接入层 (Gin Engine)                                                  │
│    • Read/Write/Idle 超时控制 (30m+)  • MaxHeaderBytes 攻击防御             │
│    • CORS 跨域治理                    • 优雅停机信号捕获 (Graceful Shutdown) │
└─────────────────────────────────────┬───────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 2. 安全、鉴权与 Credits 配额层 (Security & Quota Engine)                    │
│    • User-Agent 客户端黑名单拦截      • 企业 SSO (OIDC/Azure AD) / JWT 校验 │
│    • API Key 内存高速缓存鉴权         • 模型白名单 & 跨午夜可用时段校验     │
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
│ 5. 存储与缓存层 (Persistence)       │ │ 6. 观测审计与排障 (Observability)   │
│    • 企业级 PostgreSQL (GORM v2)    │ │    • 全链路 Access Log (敏感头脱敏) │
│    • 连接池高并发调优 (Pool Tuning) │ │    • 流式响应内容异步还原重组       │
│    • 内存 LRU Cache (高速鉴权)      │ │    • 4 阶段原始报文转储 (Raw Dumps) │
│    • 异步批量落盘 Credits 账单流水  │ │    • 历史数据自动分区与清理 (7天)   │
│    • embed.FS 前端静态资源内嵌      │ │                                     │
└─────────────────────────────────────┘ └─────────────────────────────────────┘
```

---

## 二、关键技术选型与决策

| 模块 / 需求 | 既有考量 | CodeGate 选型决策 | 决策收益与理由 |
| :--- | :--- | :--- | :--- |
| **持久化存储** | 单文件纯 Go SQLite | **PostgreSQL (共享 `code-*` 基础库)** | 与团队整体技术栈统一（遵循 GORM v2 标准），具备高并发事务、成熟连接池和企业级备份能力，完全无需单文件锁的妥协。 |
| **编程语言** | Python / Node.js | **Go 1.22+** | 高并发性能卓越、天然原生 Goroutine、内存开销极低。 |
| **Web 框架** | Kong / APISIX / FastAPI | **Gin Web Framework** | 成熟稳定，中间件生态健全，路由性能与流式代理能力出众。 |
| **缓存架构** | 外部单独部署 Redis | **内存滑动窗口 + LRU 缓存 + PG 异步持久化** | 避免引入额外 Redis 运维负担，鉴权与限流全部纳秒级内存完成，账单异步写入 PostgreSQL。 |
| **计费模型** | 单纯请求频次（RPD） | **Credits 算力点数体系（日/周双周期）** | 精准区分输入/缓存命中/输出成本；周配额为日配额 4 倍，兼顾工作日弹性与周预算可控。 |
| **协议适配** | 盲目全量转发 | **协议能力自动识别 + 协议感知直通** | 自动探测并打标后端对 `/v1/responses` 的支持能力，确保仅向兼容后端转发，杜绝 404/405 报错。 |
| **前端交付** | 独立 Nginx 托管 React | **Go `embed.FS` 原生内嵌 React 产物** | 单二进制部署，解耦独立静态服务器，降低运维部署复杂度。 |

---

## 三、协议感知与后端能力自动标识机制

### 1. 协议现状与核心痛点
- **OpenAI Chat 协议 (`/v1/chat/completions`)**：业界最通用的大模型接口，所有推理后端（开源推理引擎 vLLM/TGI、主流商业 API 等）均原生支持。
- **OpenAI Responses 协议 (`/v1/responses`)**：专为智能编程 Agent（如 OpenCode、Codex CLI）设计的现代化端点，仅部分特定供应商或针对 Agent 深度优化的后端才提供支持。
- **痛点**：若网关不加甄别地将 `/v1/responses` 请求以轮询方式打到普通后端，会频繁遭遇下游 HTTP 404 Not Found 或 405 Method Not Allowed，导致终端编码工具不可用。

### 2. 后端能力自动探查（Capability Probing）流程

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

- **显式配置与自动探查结合**：
  - 配置文件支持显式定义 `protocols: ["chat", "responses"]`；
  - 若未配置或开启 `auto_detect_protocols: true`，探针在定期探活时自动探测，并动态沉淀至 Backend 状态中。

### 3. 协议感知直通调度算法（Protocol-Aware Routing）

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

## 四、Credits 算力点数模型与高并发扣减架构

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
- **高并发数据一致性**：
  - 请求前：在内存中完成日/周剩余 Credits 的快速预检；
  - 请求后：流式连接正常结束或异常关闭时，提取服务商真实 Usage，计算出精确点数；
  - 异步通道批量同步：通过缓冲 Channel 将扣费流水聚合批量写入 PostgreSQL，大幅减轻数据库并发事务压力。

---

## 五、流量调度算法深度剖析

### 1. KV Cache 亲和性会话粘性路由（HRW 哈希）

#### HRW (Rendezvous Hashing) 原理
在通过协议过滤后的健康后端集合中，调度器利用会话特征计算每个后端的权重评分：
$$\text{Score}(S, B_i) = \text{Hash}(S \parallel B_i.\text{ID}) \times B_i.\text{Weight}$$
- 选取最高评分节点作为本次多轮对话的承载节点，最大化命中显存中的 Prompt KV Cache；
- 配合 0.1 Credits 的缓存低费率，既让响应首字耗时（TTFT）降低 50%~80%，又为团队直接节省 90% 的输入点数。

#### 溢出保护（Spillover Protection）
- 若首选亲和节点达到 `max_concurrency` 上限或突发健康检查失败，流量自动顺延至评分次高的健康后端，保证吞吐与高可用优先。

---

### 2. 实例级原子 CAS 无锁并发反压控制

针对每个物理 Backend 实例实行独立的并发占槽保护：

```go
// 占槽原子操作
func (b *Backend) AcquireSlot() bool {
    for {
        current := atomic.LoadInt32(&b.activeConnections)
        if current >= b.MaxConcurrency {
            return false // 实例当前已饱和
        }
        if atomic.CompareAndSwapInt32(&b.activeConnections, current, current+1) {
            return true  // 成功占槽
        }
    }
}

// 释放槽位
func (b *Backend) ReleaseSlot() {
    atomic.AddInt32(&b.activeConnections, -1)
}
```

---

## 六、数据库设计规范（PostgreSQL + GORM v2）

遵循团队 `code-*` 系列规范，数据库模型统一管理在 `models` 目录下，主要实体表结构包括：

1. **`users`**：用户基本信息、角色（Admin / User）、状态（Pending / Active / Disabled）、关联配额策略 ID。
2. **`quota_policies`**：配额策略定义，包含 `daily_credits_limit`、`weekly_credits_limit`（默认为日限额 4 倍）、`rate_limit_rpm`、可用时段区间（支持跨午夜）及模型权限白名单。
3. **`credits_wallets`**：用户日/周 Credits 实时消耗台账（每日/每周自动按自然周期结转重置）。
4. **`models`**：逻辑模型定义，包含模型名称、默认降级模型（`default_model`）、模型倍率乘数（`multiplier`，如 0.5、1.0、10.0）、静默注入参数（`model_params`）。
5. **`backends`**：物理实例定义，包含所属模型 ID、Base URL、真实 API Key、权重、`max_concurrency`、声明及探测到的能力集（`protocols: ["chat", "responses"]`）、健康状态。
6. **`api_keys`**：用户自助创建的 API Key 凭据，包含哈希签名、过期时间、限定模型白名单、状态。
7. **`access_logs`**：全链路访问日志，包含用户、客户端 IP、路径、协议、模型、输入 Token、缓存命中 Token、输出 Token、扣减 Credits、耗时、TTFT 及敏感脱敏字段。
8. **`raw_dumps`**（可选存储）：针对错误请求的 4 阶段原始报文转储记录。

---

## 七、单二进制与热重载机制

1. **嵌入式静态前端（`go:embed`）**：
   - 前端 React 18 产物编译到后端工程目录，通过 Go 1.16+ 原生 `embed.FS` 静态打入二进制；
   - 生产部署仅需一个二进制可执行文件与 PostgreSQL 连接配置即可运行，运维干净纯粹。
2. **配置动态热更新（Zero-Downtime）**：
   - 管理后台对模型、后端实例权重、协议能力声明或配额策略的变更，通过配置管理器发布至内部事件总线；
   - 调度器即时重载内存路由表与限流器，正在传输的超长流式连接不断开，服务完全平滑。
