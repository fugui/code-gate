# 码界（CodeGate） - 企业大模型统一接入网关

[![Go Version](https://img.shields.io/badge/Go-1.22+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

**CodeGate（码界）** 是为公司内部团队构建的统一大模型接入网关与算力调度平台。作为 `code-*` 系列基础设施的核心成员，它深度集成 **`code-common` 前后端公共框架**，共享企业级 **PostgreSQL** 数据库与 **CodeBench** 统一用户认证体系，作为内部研发辅助（Claude Code、OpenCode、Cursor 等）、代码安全分析（Code-Shield 等）以及全员日常 AI 交互的统一门户，提供协议感知直通代理、Credits 算力点数治理、动态负载均衡与容灾、KV Cache 会话亲和性加速与全链路审计。

---

## 🌟 核心特性概览

- **公共框架深度集成（`code-common`）**：
  - 后端复用 `code-common/backend`（`gormdb` 统一连接池、`auth` JWT 鉴权中间件、`server` 脚手架与优雅停机）。
  - 前端全量遵循 `@code/common` 设计规范（Design Tokens 语义颜色、`theme.css` 深浅双模主题）并复用成熟 UI 组件库。
- **共享 CodeBench 用户，独立配额自治**：
  - 零重复开发账号注册与密码管理，直接共享 CodeBench 用户认证体系。
  - 用户初次使用默认自动绑定 **`guest` 配额角色**（受保底体验额度与基础模型保护）。
  - CodeGate 管理员可在控制台灵活为用户分配高阶角色（如 `developer`、`vip`）或定制专属配额策略。
- **协议感知直通代理**：
  - 原生兼容 OpenAI `/v1/chat/completions` 与 `/v1/models` 标准协议。
  - 专为现代 AI 编码 Agent（如 OpenCode、Codex CLI）提供专属原生直通代理 `/v1/responses`。
  - **后端能力自动探查**：后台探针自动探测物理后端实例对 `chat` 与 `responses` 协议的支持能力并动态打标。
  - **严格协议感知选路**：只转发消息至具备该协议支持能力的后端实例，杜绝盲目转发导致的 404/405 报错。
  - 流式（SSE）增量输出与内置心跳保活（Keep-Alive Ping），杜绝大段代码生成超时中断。
- **Credits 算力点数与双周期弹性配额**：
  - **差异化 Token 费率**：输入 Token 为 1.0，缓存命中（Prompt Cache Hit）为 0.1（立省 90%），输出 Token 为 5.0。
  - **模型专属倍率系数**：轻量模型 0.5x、标准模型 1.0x、旗舰/推理模型 5.0x ~ 10.0x。
  - **双周期配额联动**：支持每日限额（Daily）与每周配额（Weekly），**每周总量默认为每日限额的 4 倍**，兼顾工作日弹性与周度预算可控。
- **智能流量调度与高可用**：
  - 逻辑模型与物理实例解耦（1:N 映射），支持加权轮询（WRR）与加权最少连接（WLC）。
  - 基于 HRW 哈希的 **Prompt KV Cache 会话粘性路由**，复用显存上下文，降低首字延迟（TTFT）达 50%~80% 且享受低至 0.1 的缓存点数计费。
  - 后端主动健康检查探针 + 自动故障熔断剔除与自愈。
  - **默认模型 Fallback 容灾降级**，后端全离线时平滑切换备选模型，保障业务 SLA。
- **细粒度并发与安全防护**：
  - 面向后端实例的原子 CAS 无锁并发控制，严防私有显卡或上游账号被瞬间打爆。
  - 基于 User-Agent 的黑名单拦截过滤规则，动态防范恶意请求。
  - 支持跨午夜可用时间段策略（如 `22:00-06:00` 闲时运行），引导合理用量。
- **全链路审计与运维排障**：
  - 全链路脱敏 Access Log、流式 SSE 响应异步聚合还原、4 阶段原始报文转储（Raw Dumps）。
  - 配置修改毫秒级热生效（Hot-Reload），长连接完全不中断。

---

## 📚 规划与设计文档

本项目完整的需求设计与架构规划位于 [docs/](docs/) 目录中：

| 文档名称 | 路径 | 内容简介 |
| :--- | :--- | :--- |
| **功能特性全景规划** | [docs/features.md](docs/features.md) | 业务背景、用户画像、核心特性矩阵、CodeBench 用户与独立配额、Credits 计费模型及应用场景 |
| **系统技术架构设计** | [docs/architecture.md](docs/architecture.md) | 架构分层、code-common 模块整合、协议自动探测与路由算法、配额角色模型与 PG 设计 |
| **实施路线图与里程碑** | [docs/roadmap.md](docs/roadmap.md) | 四阶段（MVP -> 协议感知与配额 -> 调度运维 -> 完整门户）工程排期与验收标准 |

---

## 🛠️ 核心技术栈

- **后端开发**：Go 1.22+，基于 `code-common/backend`（`server`, `auth`, `gormdb`, `models`）
- **持久化与缓存**：PostgreSQL（共享 `code-*` 数据库，GORM v2），内存滑动窗口计数器与 LRU 缓存
- **前端控制台**：React 18，Vite 5，TypeScript，Ant Design，基于 `@code/common` 样式规范与组件系统
- **打包交付**：Go `embed.FS` 嵌入式单可执行文件交付，支持全平台交叉编译

---

## 🚀 协同赋能价值

- **`code-shield`（代码质量与安全网关）**：为其 AI 检视逻辑提供稳定、多实例负载均衡的高可用推理算力支撑，避免 429 报错中断流水线。
- **`code-pipeline`（CI/CD 流水线）**：统一配置项目专属密钥，精准按 Credits 费率核算各业务线构建时的 AI 成本。
- **研发工程师协同**：直接使用已有 CodeBench 账号登录，为使用 Claude Code / OpenCode / Cursor / VSCode 插件提供统一算力充值通道与审计底座。
