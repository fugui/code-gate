# 码界（CodeGate） - 企业大模型统一接入网关

[![Go Version](https://img.shields.io/badge/Go-1.22+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

**CodeGate（码界）** 是为公司内部团队构建的统一大模型接入网关与算力调度平台。它作为内部研发辅助（Claude Code、OpenCode、Cursor 等）、代码安全分析（Code-Shield 等）以及全员日常 AI 交互的统一门户，提供多模型统一协议代理、动态负载均衡与容灾、KV Cache 会话亲和性加速、精细化配额限流与全链路审计。

---

## 🌟 核心特性概览

- **统一协议标准兼容**：
  - 原生兼容 OpenAI `/v1/chat/completions` 与 `/v1/models` 标准协议。
  - 专为现代 AI 编码 Agent（如 OpenCode、Codex CLI）提供专属原生直通代理 `/v1/responses`。
  - 流式（SSE）增量输出与内置心跳保活（Keep-Alive Ping），杜绝大段代码生成超时中断。
  - 从服务商官方流中提取真实 `input_tokens` 与 `output_tokens`，精准计量。
- **智能流量调度与高可用**：
  - 逻辑模型与物理实例解耦（1:N 映射），支持加权轮询（WRR）与加权最少连接（WLC）。
  - 基于 HRW 哈希的 **Prompt KV Cache 会话粘性路由**，复用显存上下文，降低首字延迟（TTFT）达 50%~80%。
  - 后端主动健康检查探针 + 自动故障熔断剔除与自愈。
  - **默认模型 Fallback 容灾降级**，后端全离线时平滑切换备选模型，保障业务 SLA。
- **细粒度配额与安全治理**：
  - 基于内存滑动窗口的速率限制（RPM）与每日额度（RPD）管控。
  - 原生支持跨午夜可用时间段策略（如 `22:00-06:00` 闲时运行），引导合理用量。
  - 面向后端实例的原子 CAS 无锁并发控制，严防私有显卡或上游账号被瞬间打爆。
  - 基于 User-Agent 的黑名单拦截过滤规则，动态防范恶意请求。
- **组织凭证与全链路审计**：
  - 支持企业 SSO 单点登录（OIDC / Azure AD）与用户自助注册审核流。
  - 用户自助申请与管理多场景 API Key（支持到期日与模型权限白名单）。
  - 全链路脱敏 Access Log、流式 SSE 响应异步聚合还原、4 阶段原始报文转储（Raw Dumps）。
- **极简交付与运维**：
  - **零外部中间件依赖**：无需部署 Redis / MySQL / Nginx。
  - **单二进制交付**：内嵌纯 Go SQLite 驱动（无 CGO 依赖）与前端 React 构建产物（`go:embed`），一个二进制 + 一个配置文件即跑。
  - **运行时热重载**：配置修改毫秒级热生效，长连接完全不中断。

---

## 📚 规划与设计文档

本项目完整的需求设计与架构规划位于 [docs/](docs/) 目录中：

| 文档名称 | 路径 | 内容简介 |
| :--- | :--- | :--- |
| **功能特性全景规划** | [docs/features.md](docs/features.md) | 业务背景、用户画像、核心功能矩阵及典型应用场景详细说明 |
| **系统技术架构设计** | [docs/architecture.md](docs/architecture.md) | 系统总体架构、调度算法剖析、无锁并发设计与存储模型 |
| **实施路线图与里程碑** | [docs/roadmap.md](docs/roadmap.md) | 四阶段（MVP -> 高可用 -> 调度安全 -> 完整门户）工程排期与验收标准 |

---

## 🛠️ 核心技术栈

- **后端开发**：Go 1.22+，Gin Web Framework
- **持久化与缓存**：内置纯 Go SQLite（`modernc.org/sqlite`，无 CGO 编译依赖），内存滑动窗口计数器与 LRU 缓存
- **前端控制台**：React 18，Vite 5，TypeScript，Ant Design，Vanilla CSS（深浅主题双模适配）
- **打包交付**：Go `embed.FS` 嵌入式单可执行文件交付，支持全平台交叉编译

---

## 🚀 协同赋能价值

- **`code-shield`（代码质量与安全网关）**：为其 AI 检视逻辑提供稳定、多实例负载均衡的高可用推理算力支撑，避免 429 报错中断流水线。
- **`code-pipeline`（CI/CD 流水线）**：统一配置项目专属密钥，精准核算各业务线构建时的 AI 成本。
- **研发工程师协同**：为团队使用 Claude Code / OpenCode / Cursor / VSCode 插件提供统一算力充值通道与审计底座。
