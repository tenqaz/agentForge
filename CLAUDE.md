# CLAUDE.md

本文件为 Claude Code (claude.ai/code) 在此仓库中工作时提供指导。

## 项目概述

AgentForge 是一个多智能体平台，包含：
- **后端**: Go API 服务 (`services/api`) 管理智能体、模板、运行时供应和微信渠道集成
- **前端**: Next.js 16 Web 控制台 (`web`) 提供管理员和用户界面
- **运行时**: 基于 Docker 的 Hermes 容器执行智能体

## 开发命令

### 后端 (Go API)

```bash
# 在 services/api 目录下运行
cd services/api

# 运行 API 服务器（需要 .env 配置）
go run cmd/agentforge-api/main.go

# 运行所有测试
go test ./...

# 运行特定测试
go test ./internal/auth -v
go test ./tests -v -run TestMVPIntegration

# 构建二进制文件
go build -o agentforge-api cmd/agentforge-api/main.go
```

### 前端 (Next.js)

```bash
# 在 web 目录下运行
cd web

# 开发服务器 (http://localhost:3000)
npm run dev

# 生产构建
npm run build

# 启动生产服务器
npm run start

# 代码检查
npm run lint

# 运行单元测试 (Vitest)
npm run test

# 运行 E2E 测试 (Playwright)
npm run test:e2e
```

## 架构

### 后端结构

Go API 遵循清晰的分层架构：

- **`cmd/agentforge-api/main.go`**: 入口点，连接依赖并启动 HTTP 服务器和任务调度器
- **`internal/http/`**: HTTP 处理器和路由
- **`internal/agents/`**: 智能体领域逻辑（service、repository、models）
- **`internal/templates/`**: 模板管理（service、repository、file store）
- **`internal/auth/`**: 认证（会话、RBAC、密码哈希）
- **`internal/runtime/`**: Docker 运行器和 Hermes home 供应
- **`internal/jobs/`**: 后台任务系统，包含运行时和渠道 workers
- **`internal/channels/`**: 渠道管理（微信集成）
- **`internal/weixin/`**: 微信 API 客户端
- **`internal/db/`**: 数据库连接和迁移
- **`migrations/`**: SQL 迁移文件

### 智能体运行时供应

智能体供应是由 `RuntimeWorker` 编排的**多阶段后台任务**：

1. **Creating** → **Provisioning**: 复制模板文件到智能体的 Hermes home 目录
2. **Provisioning** → **Starting**: 写入 Hermes 配置并准备运行时环境
3. **Starting** → **Running**: 启动带有 Hermes 运行时的 Docker 容器
4. **错误状态**: 任务可能在任何阶段失败；检查 `last_error_code` 和 `last_error_message`

关键文件：
- `internal/jobs/runtime_worker.go`: 任务处理逻辑
- `internal/runtime/docker_runner.go`: Docker 容器管理
- `internal/runtime/home_builder.go`: Hermes home 目录设置

### 前端结构

Next.js App Router 结构：
- **`app/page.tsx`**: 首页（模板列表）
- **`app/admin/`**: 管理员路由（模板 CRUD、技能管理）
- **`app/agents/`**: 智能体管理路由
- **`app/templates/`**: 公共模板路由
- **`app/login/`**: 认证

### 数据库

- **SQLite** 使用 modernc.org/sqlite（纯 Go，无 CGo）
- 迁移文件位于 `services/api/migrations/`
- 迁移发现会检查多个路径（见 `resolveMigrationsDir()`）

## 核心概念

### Templates vs Agents

- **Template（模板）**: 可重用的智能体定义（SOUL.md、USER.md、skills）
- **Agent（智能体）**: 从已发布模板版本创建的运行时实例
- 模板有版本；智能体在创建时锁定到特定版本

### Runtime Jobs（运行时任务）

用于供应和重启操作的异步任务系统：
- `provision_agent`: 初始智能体设置和容器启动
- `restart_runtime`: 停止并重启现有容器

任务在 `runtime_jobs` 表中跟踪，状态包括：`pending`、`running`、`succeeded`、`failed`

### Channels（渠道）

智能体可以拥有渠道（目前仅支持微信）。渠道供应独立于运行时供应，由 `ChannelWorker` 管理。

## API 文档

完整 API 规范见 `docs/api.md`：
- 公共路由：`/api/sessions`、`/api/templates`
- 管理员路由：`/api/admin/templates/*`（需要管理员角色）
- 智能体路由：`/api/agents/*`
- 渠道路由：`/api/agents/{id}/channels/weixin/*`

## 环境配置

后端需要在 `services/api/` 中配置 `.env`

参考 `services/api/.env.example`

## 测试策略

- **单元测试**: 测试文件与实现文件放在一起（`*_test.go`）
- **集成测试**: `services/api/tests/` 目录
  - `mvp_integration_test.go`: 端到端 API 工作流
  - `runtime_persistence_test.go`: 运行时状态持久化

## 重要注意事项

- 智能体状态转换通过 `WHERE status = ?` 检查实现**乐观锁** — 并发更新会返回 `ErrConflict`
- Docker 容器使用命名约定：`agentforge-agent-{agentID}`
- Hermes home 路径：`{DATA_DIR}/agents/{agentID}/hermes-home/`
- 模板文件存储：`{DATA_DIR}/templates/{templateID}/v{version}/`
- Session cookies 在生产环境使用安全设置（见 `SessionManager`）
- API 响应绝不能暴露敏感信息（提供商密钥、令牌、密码哈希）— 见 `docs/api.md` 安全策略
