# AgentForge

AgentForge 是一个多智能体平台，用于从可复用的模板创建、供应并运行智能体，并通过微信等渠道对外提供对话能力。

平台由三部分组成：

- **后端 API**（`services/api`）：Go 服务，负责智能体与模板管理、运行时供应编排、微信渠道集成。
- **Web 控制台**（`web`）：Next.js 应用，提供管理员与普通用户界面。
- **运行时**：基于 Docker 的 Hermes 容器，执行智能体本身。

## 技术栈

| 层 | 技术 |
| --- | --- |
| 后端 | Go 1.25、[Gin](https://github.com/gin-gonic/gin)、[modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)（纯 Go，无 CGo） |
| 前端 | Next.js 16（App Router）、React 19、Tailwind CSS 4、TypeScript |
| 测试 | 后端 `go test`；前端 Vitest（单元）+ Playwright（E2E） |
| 运行时 | Docker、[Hermes Agent](https://github.com/NousResearch) 镜像 |

## 目录结构

```
.
├── services/api/            # Go 后端 API
│   ├── cmd/agentforge-api/  # 入口 main.go
│   ├── internal/            # 分层领域代码（agents / auth / channels / config / db / http / jobs / runtime / templates / weixin）
│   ├── migrations/          # SQL 迁移文件
│   └── tests/               # 集成测试
├── web/                     # Next.js 前端
│   ├── app/                 # App Router 路由（admin / agents / login / register / templates / 首页营销页）
│   ├── components/          # 复用组件
│   └── tests/               # Playwright E2E 与 Vitest 单测
├── docs/                    # 本地文档（api.md / security.md / runbooks，未纳入版本控制）
└── var/                     # 运行时数据目录（DATA_DIR 默认指向此处）
```

## 快速开始

### 前置依赖

- Go ≥ 1.25
- Node.js ≥ 20 与 npm
- Docker（用于运行 Hermes 智能体容器）
- 一个 LLM 提供商 API Key（默认配置为 DeepSeek）

### 1. 配置后端环境

```bash
cd services/api
cp .env.example .env
# 编辑 .env，至少填入 AGENTFORGE_MODEL_API_KEY
```

关键配置项（见 `.env.example`）：

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `AGENTFORGE_HTTP_ADDR` | API 监听地址 | `:8080` |
| `AGENTFORGE_PUBLIC_BASE_URL` | 对外可访问的基础 URL | `http://localhost:8080` |
| `AGENTFORGE_DATA_DIR` | 数据目录（SQLite、智能体 home 等） | `../../var` |
| `AGENTFORGE_SESSION_SECRET` | 会话签名密钥（生产环境务必修改） | `dev-change-me` |
| `AGENTFORGE_HERMES_IMAGE` | Hermes 容器镜像 | `nousresearch/hermes-agent:v2026.6.5` |
| `AGENTFORGE_HERMES_MEMORY` / `AGENTFORGE_HERMES_CPUS` | 容器资源限制 | `500m` / `0.5` |
| `AGENTFORGE_MODEL_*` | LLM 模型、提供商、Base URL、API Key、API 模式 | DeepSeek / `chat_completions` |
| `AGENTFORGE_WEIXIN_BASE_URL` | 微信 iLink Bot API 地址 | `https://ilinkai.weixin.qq.com` |

### 2. 启动后端

```bash
cd services/api
go run cmd/agentforge-api/main.go
# API: http://localhost:8080
```

启动时会自动执行数据库迁移并开启运行时与渠道后台 worker。

### 3. 启动前端

```bash
cd web
npm install
npm run dev
# Web: http://localhost:3000
```

首页 `/` 为公开营销着陆页；管理员可在 `/admin` 管理模板，用户在 `/agents` 管理智能体。

## 核心概念

### Template 与 Agent

- **Template（模板）**：可复用的智能体定义，包含 `SOUL.md`（人设）、`USER.md`（用户画像）和若干 skill（每个 skill 是一个含 `SKILL.md` 的 ZIP）。模板有版本，发布后生成不可变版本号。
- **Agent（智能体）**：从某个已发布模板版本创建的运行时实例，创建时锁定到该版本。

### 运行时供应（多阶段后台任务）

智能体供应由 `RuntimeWorker` 编排，状态流转：

```
Creating → Provisioning → Starting → Running
```

- **Provisioning**：复制模板文件到智能体的 Hermes home 目录
- **Starting**：写入 Hermes 配置、启动 Docker 容器
- 任意阶段失败进入 `error` 状态，查看 `last_error_code` / `last_error_message`

Docker 容器命名：`agentforge-agent-{agentID}`；Hermes home 路径：`{DATA_DIR}/agents/{agentID}/hermes-home/`。

状态转换通过 `WHERE status = ?` 实现**乐观锁**，并发更新会返回 `409 conflict`。

### 渠道（Channels）

智能体可绑定渠道（目前仅微信）。渠道供应由独立的 `ChannelWorker` 管理，与运行时供应解耦。微信渠道通过扫码配对会话（pairing session）将智能体与微信账号绑定。

## API 概览

完整接口规范见 [`docs/api.md`](docs/api.md)。主要路由：

- **会话**：`POST /api/sessions`、`GET /api/session`、`DELETE /api/session`
- **公共模板**：`GET /api/templates`、`GET /api/templates/{id}`
- **管理员模板**：`/api/admin/templates/*`（需管理员会话，含 SOUL/USER/skill 文件管理与发布）
- **智能体**：`/api/agents/*`（创建、列表、运行时状态、运行时任务、删除）
- **微信渠道**：`/api/agents/{id}/channels/weixin/*`（含配对会话）

> 安全策略：API 响应绝不暴露提供商密钥、`WEIXIN_TOKEN`、`bot_token`、密码哈希等敏感信息。

## 测试

### 后端

```bash
cd services/api
go test ./...                              # 全部单元测试
go test ./tests -v -run TestMVPIntegration # 端到端集成测试
```

### 前端

```bash
cd web
npm run test        # Vitest 单元测试
npm run test:e2e    # Playwright E2E（需先启动前后端）
```

## 文档

- [`CLAUDE.md`](CLAUDE.md) — 仓库工作指南（架构、命令、约定）
- [`docs/api.md`](docs/api.md) — API 接口规范
- [`docs/security.md`](docs/security.md) — 安全策略
- [`docs/runbooks/`](docs/runbooks/) — 运维手册

> `docs/` 目录默认未纳入版本控制（见 `.gitignore`），作为本地工作文档存在。

## 开发约定

- 面向用户的文档与说明默认使用中文，代码标识、路径、命令保留原文。
- 后端遵循 `internal/` 分层架构，领域逻辑与 HTTP/存储解耦。
- 数据库迁移位于 `services/api/migrations/`，启动时自动发现并执行。
