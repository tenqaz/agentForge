# AgentForge MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付 AgentForge MVP：管理员在平台内维护并发布 Agent 模板；普通用户只能基于已发布模板创建 Agent；后端为每个 Agent 创建独立且持久化的 Hermes home，并启动独立 Hermes Docker 容器；用户在 Agent 运行成功后通过微信二维码连接，扫码的微信用户就是后续给 Agent 发私信的微信用户。

**Architecture:** 简化 monorepo：`web/` 是 Next.js 控制台，`services/api/` 是 Go API 和同进程 worker，`var/` 是本地持久化数据目录，SQLite WAL 保存元数据。Go 后端负责 REST API、RBAC、SQLite 迁移、文件系统、Docker 生命周期、Hermes `config.yaml`、Hermes `.env` 和 Go 原生 iLink 微信扫码流程。Hermes 是运行时边界，不通过 `hermes gateway setup` 做后台配置。

**Tech Stack:** Next.js、TypeScript、Go、SQLite WAL、Docker CLI、Hermes 镜像 `nousresearch/hermes-agent:v2026.6.5`、Go 原生 HTTP iLink client、Playwright。

---

## 文件结构

实现时创建并维护以下结构：

```text
AgentForge/
├── web/
│   ├── app/
│   │   ├── login/
│   │   ├── templates/
│   │   ├── agents/
│   │   └── admin/templates/
│   ├── components/
│   ├── lib/api/
│   └── tests/
├── services/
│   └── api/
│       ├── cmd/agentforge-api/main.go
│       ├── internal/
│       │   ├── agents/
│       │   ├── auth/
│       │   ├── channels/
│       │   ├── config/
│       │   ├── db/
│       │   ├── http/
│       │   ├── jobs/
│       │   ├── runtime/
│       │   ├── templates/
│       │   └── weixin/
│       ├── migrations/
│       ├── tests/
│       └── .env.example
├── var/
│   ├── templates/
│   ├── agents/
│   └── logs/
└── docs/
```

职责边界：

- `web/`：只做浏览器控制台，不写 Hermes 文件，不执行 Docker，只调用 Go REST API。
- `services/api/internal/http`：REST 路由、middleware、请求解析、响应格式。
- `services/api/internal/auth`：session、密码、当前用户、RBAC。
- `services/api/internal/db`：SQLite 连接、WAL、迁移、事务。
- `services/api/internal/templates`：管理员模板、`SOUL.md`、`USER.md`、skills。skills 只支持新增和删除。
- `services/api/internal/agents`：Agent 创建、归属校验、状态查询。
- `services/api/internal/runtime`：Hermes home 生成、`config.yaml`、`.env`、Docker runner。
- `services/api/internal/weixin`：Go iLink client、二维码状态解释。
- `services/api/internal/channels`：微信渠道资源、状态机、渠道策略。
- `services/api/internal/jobs`：`runtime_jobs`、`channel_jobs`、任务 claim、worker 执行、重试。

## 阶段 1：仓库骨架

- [ ] 创建目录和基础工程。

  命令：

  ```bash
  mkdir -p web services/api/cmd/agentforge-api services/api/internal/{agents,auth,channels,config,db,http,jobs,runtime,templates,weixin} services/api/migrations services/api/tests var/{templates,agents,logs}
  cd services/api && go mod init agentforge.local/services/api
  cd ../../web && npm create next-app@latest . -- --typescript --eslint --app --src-dir=false --import-alias="@/*"
  ```

  期望结果：

  ```text
  services/api/go.mod exists
  web/package.json exists
  ```

- [ ] 创建后端服务配置示例。

  文件：`services/api/.env.example`

  ```dotenv
  AGENTFORGE_HTTP_ADDR=:8080
  AGENTFORGE_PUBLIC_BASE_URL=http://localhost:8080
  AGENTFORGE_DATA_DIR=../../var
  AGENTFORGE_SESSION_SECRET=dev-change-me
  AGENTFORGE_HERMES_IMAGE=nousresearch/hermes-agent:v2026.6.5
  AGENTFORGE_HERMES_MEMORY=500m
  AGENTFORGE_HERMES_CPUS=0.5
  ```

- [ ] 增加忽略规则。

  文件：`.gitignore`

  ```gitignore
  var/
  services/api/.env
  web/.env.local
  ```

- [ ] 验证。

  ```bash
  cd services/api && go test ./...
  cd ../../web && npm run lint
  ```

- [ ] 提交。

  ```bash
  git add . && git commit -m "Bootstrap AgentForge monorepo"
  ```

## 阶段 2：配置、SQLite WAL 和迁移

- [ ] 实现后端配置加载。

  文件：

  - `services/api/internal/config/config.go`
  - `services/api/internal/config/config_test.go`

  必须满足：

  - 存在 `.env` 时读取 `.env`。
  - Docker binary 默认是 `docker`，不暴露为常规配置项。
  - worker 第一版默认随 API 进程启动。
  - SQLite 路径由 `AGENTFORGE_DATA_DIR/agentforge.db` 推导。
  - 启动时把 `AGENTFORGE_DATA_DIR` 转成绝对路径。

  核心类型：

  ```go
  type Config struct {
      HTTPAddr      string
      PublicBaseURL string
      DataDir       string
      SQLitePath    string
      SessionSecret string
      HermesImage   string
      HermesMemory  string
      HermesCPUs    string
      DockerBin     string
  }
  ```

- [ ] 实现 SQLite 打开和迁移。

  文件：

  - `services/api/internal/db/db.go`
  - `services/api/internal/db/migrate.go`
  - `services/api/internal/db/db_test.go`

  连接后执行：

  ```sql
  PRAGMA journal_mode=WAL;
  PRAGMA busy_timeout=5000;
  PRAGMA foreign_keys=ON;
  ```

  测试断言：

  - `PRAGMA journal_mode` 返回 `wal`。
  - 外键约束生效。
  - 迁移重复执行不会失败。

- [ ] 创建初始迁移。

  文件：`services/api/migrations/001_initial.sql`

  表：

  - `users`
  - `agent_templates`
  - `template_skills`
  - `agents`
  - `agent_runtime_events`
  - `agent_channels`
  - `channel_pairing_sessions`
  - `runtime_jobs`
  - `channel_jobs`

  关键约束：

  ```sql
  CHECK (role IN ('admin', 'user'));
  CHECK (status IN ('draft', 'published', 'archived'));
  CHECK (type IN ('provision_agent', 'start_runtime', 'stop_runtime', 'restart_runtime'));
  CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled'));
  ```

  活跃任务唯一索引：

  ```sql
  CREATE UNIQUE INDEX idx_runtime_jobs_one_active
  ON runtime_jobs(agent_id)
  WHERE status IN ('queued', 'running');

  CREATE UNIQUE INDEX idx_channel_jobs_one_active
  ON channel_jobs(agent_channel_id)
  WHERE status IN ('queued', 'running');

  CREATE UNIQUE INDEX idx_pairing_one_active
  ON channel_pairing_sessions(agent_channel_id)
  WHERE status = 'pending';
  ```

- [ ] 验证。

  ```bash
  cd services/api && go test ./internal/config ./internal/db
  ```

- [ ] 提交。

  ```bash
  git add . && git commit -m "Add backend config and SQLite migrations"
  ```

## 阶段 3：认证、Session 和 RBAC

- [ ] 实现密码和 session。

  文件：

  - `services/api/internal/auth/password.go`
  - `services/api/internal/auth/session.go`
  - `services/api/internal/auth/repository.go`
  - `services/api/internal/auth/auth_test.go`

  Cookie 要求：

  - 名称：`agentforge_session`
  - `HttpOnly`
  - `SameSite=Lax`
  - 使用 `AGENTFORGE_SESSION_SECRET` 签名。

- [ ] 实现 RBAC。

  文件：`services/api/internal/auth/rbac.go`

  函数：

  ```go
  func RequireAdmin(u User) error
  func RequireAgentOwner(u User, ownerUserID string) error
  func CanViewTemplate(u User, status string) bool
  ```

  规则：

  - 管理员可以管理模板。
  - 普通用户只能看已发布模板。
  - 普通用户只能访问自己的 Agent、渠道和 pairing session。
  - 普通用户不能修改模板文件或 skills。

- [ ] 实现 session REST 接口。

  文件：

  - `services/api/internal/http/router.go`
  - `services/api/internal/http/session_handlers.go`
  - `services/api/internal/http/middleware.go`
  - `services/api/internal/http/session_handlers_test.go`

  接口：

  - `POST /api/sessions`
  - `GET /api/session`
  - `DELETE /api/session`

- [ ] 验证。

  ```bash
  cd services/api && go test ./internal/auth ./internal/http
  ```

- [ ] 提交。

  ```bash
  git add . && git commit -m "Add sessions and RBAC"
  ```

## 阶段 4：管理员模板管理

- [ ] 实现模板 repository 和文件存储。

  文件：

  - `services/api/internal/templates/model.go`
  - `services/api/internal/templates/repository.go`
  - `services/api/internal/templates/store.go`
  - `services/api/internal/templates/service.go`
  - `services/api/internal/templates/service_test.go`

  存储路径：

  ```text
  {data_dir}/templates/{template_id}/versions/{version}/SOUL.md
  {data_dir}/templates/{template_id}/versions/{version}/USER.md
  {data_dir}/templates/{template_id}/versions/{version}/skills/{skill_name}/SKILL.md
  ```

  发布规则：

  - `SOUL.md` 必须非空。
  - `USER.md` 必须存在。
  - 每个 skill 目录必须包含 `SKILL.md`。
  - 已发布版本不可变。
  - 编辑已发布模板时创建新的草稿版本。

- [ ] 实现 skill 只新增和删除。

  API 语义：

  - `POST /api/admin/templates/{id}/skills` 创建完整 skill。
  - `GET /api/admin/templates/{id}/skills/{skillId}` 查看 skill。
  - `DELETE /api/admin/templates/{id}/skills/{skillId}` 删除完整 skill。
  - 不实现 skill 的 `PUT` 或 `PATCH`。

  测试：

  - 重复 `skill_name` 返回 `409`。
  - 删除 skill 后移除数据库记录和目录。
  - 请求不存在的 skill 编辑路由返回 `404` 或 method not allowed。

- [ ] 实现模板 REST 接口。

  文件：

  - `services/api/internal/http/template_handlers.go`
  - `services/api/internal/http/template_handlers_test.go`

  接口：

  - `GET /api/templates`
  - `GET /api/templates/{id}`
  - `POST /api/admin/templates`
  - `PUT /api/admin/templates/{id}`
  - `DELETE /api/admin/templates/{id}`
  - `GET /api/admin/templates/{id}/soul`
  - `PUT /api/admin/templates/{id}/soul`
  - `GET /api/admin/templates/{id}/user`
  - `PUT /api/admin/templates/{id}/user`
  - `GET /api/admin/templates/{id}/skills`
  - `POST /api/admin/templates/{id}/skills`
  - `GET /api/admin/templates/{id}/skills/{skillId}`
  - `DELETE /api/admin/templates/{id}/skills/{skillId}`
  - `PUT /api/admin/templates/{id}/publication`
  - `DELETE /api/admin/templates/{id}/publication`

- [ ] 验证。

  ```bash
  cd services/api && go test ./internal/templates ./internal/http
  ```

- [ ] 提交。

  ```bash
  git add . && git commit -m "Add admin template management"
  ```

## 阶段 5：Agent 创建和 runtime job

- [ ] 实现 Agent repository 和状态机。

  文件：

  - `services/api/internal/agents/model.go`
  - `services/api/internal/agents/repository.go`
  - `services/api/internal/agents/service.go`
  - `services/api/internal/agents/state_test.go`

  状态流转：

  ```text
  creating -> provisioning -> starting -> running
  creating -> error
  provisioning -> error
  starting -> error
  running -> stopped
  running -> error
  stopped -> starting
  error -> provisioning
  error -> starting
  ```

- [ ] 实现 runtime job repository。

  文件：

  - `services/api/internal/jobs/runtime_repository.go`
  - `services/api/internal/jobs/runtime_repository_test.go`

  行为：

  - `POST /api/agents` 创建 `agents` 记录，状态为 `creating`。
  - 同一事务创建 `runtime_jobs`，`type=provision_agent`。
  - 同一 Agent 同一时间只能有一个活跃 runtime job。
  - worker 使用事务 claim job 并写入 `locked_until`。

  Claim SQL 形状：

  ```sql
  UPDATE runtime_jobs
  SET status = 'running',
      locked_by = ?,
      locked_until = ?,
      started_at = COALESCE(started_at, CURRENT_TIMESTAMP),
      updated_at = CURRENT_TIMESTAMP
  WHERE id = (
      SELECT id FROM runtime_jobs
      WHERE status = 'queued'
        AND (locked_until IS NULL OR locked_until < CURRENT_TIMESTAMP)
      ORDER BY priority DESC, created_at ASC
      LIMIT 1
  )
  RETURNING *;
  ```

- [ ] 实现 Agent REST 接口。

  文件：

  - `services/api/internal/http/agent_handlers.go`
  - `services/api/internal/http/agent_handlers_test.go`

  接口：

  - `POST /api/agents`
  - `GET /api/agents`
  - `GET /api/agents/{id}`
  - `GET /api/agents/{id}/runtime`
  - `GET /api/agents/{id}/runtime-jobs`
  - `POST /api/agents/{id}/runtime-jobs`
  - `GET /api/agents/{id}/runtime-jobs/{jobId}`

  runtime job 请求体：

  ```json
  {
    "type": "restart_runtime"
  }
  ```

- [ ] 验证。

  ```bash
  cd services/api && go test ./internal/agents ./internal/jobs ./internal/http
  ```

- [ ] 提交。

  ```bash
  git add . && git commit -m "Add Agent creation and runtime jobs"
  ```

## 阶段 6：Hermes home 和 Docker runner

- [ ] 实现 Hermes home builder。

  文件：

  - `services/api/internal/runtime/home.go`
  - `services/api/internal/runtime/home_test.go`

  生成布局：

  ```text
  {data_dir}/agents/{agent_id}/hermes-home/
  ├── config.yaml
  ├── .env
  ├── SOUL.md
  ├── memories/
  │   └── USER.md
  ├── skills/
  ├── sessions/
  ├── logs/
  └── weixin/
      └── accounts/
  ```

  `config.yaml` 模型片段：

  ```yaml
  model:
    default: deepseek-v4-flash
    provider: custom
    base_url: https://api.deepseek.com
    api_key: xxx
    api_mode: chat_completions
  ```

  测试：

  - 复制 `SOUL.md`。
  - 复制 `USER.md` 到 `memories/USER.md`。
  - 复制 skills。
  - 重复 provision 不删除 `sessions/`。
  - provider `api_key` 不出现在 API 响应和普通日志。

- [ ] 实现 Agent `.env` 写入。

  文件：`services/api/internal/runtime/env.go`

  连接前默认值：

  ```dotenv
  WEIXIN_DM_POLICY=allowlist
  WEIXIN_GROUP_POLICY=disabled
  WEIXIN_GROUP_ALLOWED_USERS=
  ```

  连接后写入：

  ```dotenv
  WEIXIN_ACCOUNT_ID={account_id}
  WEIXIN_TOKEN={bot_token}
  WEIXIN_BASE_URL={base_url}
  WEIXIN_DM_POLICY=allowlist
  WEIXIN_GROUP_POLICY=disabled
  WEIXIN_ALLOWED_USERS={ilink_user_id}
  WEIXIN_GROUP_ALLOWED_USERS=
  ```

- [ ] 实现 Docker runner 抽象。

  文件：

  - `services/api/internal/runtime/docker.go`
  - `services/api/internal/runtime/docker_test.go`

  接口：

  ```go
  type Runner interface {
      EnsureRunning(ctx context.Context, spec ContainerSpec) error
      Stop(ctx context.Context, containerName string) error
      Remove(ctx context.Context, containerName string) error
      Inspect(ctx context.Context, containerName string) (ContainerStatus, error)
  }
  ```

  必须生成的 `docker run` 参数：

  ```bash
  docker run -d \
    --name agentforge-hermes-{agent_id} \
    --restart unless-stopped \
    -v {absolute_hermes_home}:/opt/data \
    -e HERMES_HOME=/opt/data \
    --memory=500m \
    --cpus=0.5 \
    nousresearch/hermes-agent:v2026.6.5 \
    gateway run
  ```

  测试：

  - 容器名使用内部 Agent ID，不使用用户输入。
  - volume 使用绝对路径。
  - 包含 `-e HERMES_HOME=/opt/data`。
  - 命令末尾是 `gateway run`。

- [ ] 实现 `ProvisionAgent` worker 任务。

  文件：

  - `services/api/internal/jobs/runtime_worker.go`
  - `services/api/internal/jobs/runtime_worker_test.go`

  行为：

  - Agent 状态依次进入 `provisioning`、`starting`、`running`。
  - 记录 `agent_runtime_events`。
  - 模板复制失败记录 `copy_template_failed`。
  - 配置写入失败记录 `config_write_failed`。
  - Docker 失败记录 `container_start_failed`。
  - 失败后重试保留 `hermes-home/sessions`。

- [ ] 验证。

  ```bash
  cd services/api && go test ./internal/runtime ./internal/jobs
  ```

- [ ] 提交。

  ```bash
  git add . && git commit -m "Add Hermes runtime provisioning"
  ```

## 阶段 7：微信 iLink 和渠道任务

- [ ] 实现 Go iLink client。

  文件：

  - `services/api/internal/weixin/client.go`
  - `services/api/internal/weixin/client_test.go`

  方法：

  ```go
  type Client interface {
      GetBotQRCode(ctx context.Context, req QRCodeRequest) (QRCodeResponse, error)
      GetQRCodeStatus(ctx context.Context, req QRStatusRequest) (QRStatusResponse, error)
  }
  ```

  支持状态：

  - `wait`
  - `scaned`
  - `scaned_but_redirect`
  - `expired`
  - `confirmed`

  测试：

  - 获取二维码返回 `qrcode` 和 `qrcode_img_content`。
  - `scaned_but_redirect` 切换 base URL。
  - `confirmed` 必须包含 `ilink_bot_id`、`bot_token`、`baseurl`、`ilink_user_id`。
  - `confirmed` 缺字段返回稳定错误码。

- [ ] 实现渠道 repository 和状态机。

  文件：

  - `services/api/internal/channels/model.go`
  - `services/api/internal/channels/repository.go`
  - `services/api/internal/channels/service.go`
  - `services/api/internal/channels/state_test.go`

  状态流转：

  ```text
  not_configured -> qr_pending -> connected
  qr_pending -> error
  qr_pending -> not_configured
  connected -> disconnected
  disconnected -> qr_pending
  error -> qr_pending
  ```

  Agent 不是 `running` 时，微信配置请求必须返回 `409 agent_not_running`。

- [ ] 实现 channel job repository。

  文件：

  - `services/api/internal/jobs/channel_repository.go`
  - `services/api/internal/jobs/channel_repository_test.go`

  任务类型：

  - `connect_weixin`
  - `disconnect_weixin`
  - `refresh_weixin_pairing`

  约束：

  - 每个 `agent_channel_id` 只能有一个活跃 channel job。
  - 每个微信渠道只能有一个活跃 pairing session。
  - 重复创建 pairing session 返回当前活跃 session。

- [ ] 实现 `ConnectWeixinChannel` worker 任务。

  文件：

  - `services/api/internal/jobs/channel_worker.go`
  - `services/api/internal/jobs/channel_worker_test.go`

  流程：

  1. 校验 Agent 为 `running`。
  2. 创建或复用 pairing session。
  3. 调用 `GetBotQRCode`。
  4. 保存二维码内容或二维码图片路径。
  5. 轮询 `GetQRCodeStatus`。
  6. `scaned` 时保持 pending，并暴露“等待微信内确认”的状态。
  7. `scaned_but_redirect` 时切换 iLink base URL 并继续轮询。
  8. `expired` 时刷新二维码，达到次数上限后标记 `expired`。
  9. `confirmed` 时写入账号文件和 Hermes `.env`。
  10. 启动或重启 Hermes gateway。
  11. 渠道状态变为 `connected`。

  Hermes 账号文件：

  ```text
  {hermes_home}/weixin/accounts/{account_id}.json
  ```

  JSON 格式：

  ```json
  {
    "account_id": "ilink_bot_id",
    "token": "bot_token",
    "base_url": "baseurl",
    "user_id": "ilink_user_id"
  }
  ```

  测试：

  - `wait -> scaned -> confirmed` 成功。
  - `confirmed` 的 `ilink_user_id` 写入 `WEIXIN_ALLOWED_USERS`。
  - 默认私信策略是 `allowlist`。
  - 默认群组策略是 `disabled`。
  - 二维码过期产生 `qr_expired`。
  - Agent 未运行产生 `agent_not_running`。
  - API 响应结构不包含 token。

- [ ] 实现微信 REST 接口。

  文件：

  - `services/api/internal/http/weixin_handlers.go`
  - `services/api/internal/http/weixin_handlers_test.go`

  接口：

  - `GET /api/agents/{id}/channels/weixin`
  - `PUT /api/agents/{id}/channels/weixin`
  - `DELETE /api/agents/{id}/channels/weixin`
  - `GET /api/agents/{id}/channels/weixin/pairing-sessions`
  - `POST /api/agents/{id}/channels/weixin/pairing-sessions`
  - `GET /api/agents/{id}/channels/weixin/pairing-sessions/{sessionId}`

  pairing session 响应可以包含：

  ```json
  {
    "id": "pairing_id",
    "status": "pending",
    "qrPayload": "string",
    "qrImageContent": "base64-or-data-url",
    "expiresAt": "2026-06-14T12:00:00Z"
  }
  ```

  响应不能包含：

  - `WEIXIN_TOKEN`
  - `bot_token`
  - provider API key
  - gateway secret

- [ ] 验证。

  ```bash
  cd services/api && go test ./internal/weixin ./internal/channels ./internal/jobs ./internal/http
  ```

- [ ] 提交。

  ```bash
  git add . && git commit -m "Add Weixin channel pairing"
  ```

## 阶段 8：后端入口和 worker loop

- [ ] 实现启动入口。

  文件：`services/api/cmd/agentforge-api/main.go`

  启动顺序：

  1. 加载配置。
  2. 创建 `DataDir`。
  3. 打开 SQLite WAL。
  4. 执行迁移。
  5. 创建 repositories 和 services。
  6. 启动 worker goroutine。
  7. 启动 HTTP server。

- [ ] 实现 worker supervisor。

  文件：

  - `services/api/internal/jobs/supervisor.go`
  - `services/api/internal/jobs/supervisor_test.go`

  行为：

  - 轮询 `runtime_jobs` 和 `channel_jobs`。
  - 事务化 claim job。
  - 长时间微信二维码轮询时续租锁。
  - 失败时写稳定错误码。
  - context 取消时干净退出。

- [ ] 增加健康检查。

  接口：

  - `GET /api/health`

  响应：

  ```json
  {
    "ok": true
  }
  ```

- [ ] 本地验证。

  命令：

  ```bash
  cd services/api
  cp .env.example .env
  go run ./cmd/agentforge-api
  curl -s http://localhost:8080/api/health
  ```

  期望输出：

  ```json
  {"ok":true}
  ```

- [ ] 提交。

  ```bash
  git add . && git commit -m "Wire backend server and worker loop"
  ```

## 阶段 9：Next.js 控制台

- [ ] 实现 API client 和 session 状态。

  文件：

  - `web/lib/api/client.ts`
  - `web/lib/api/types.ts`
  - `web/components/app-shell.tsx`

  要求：

  - 使用 cookie session。
  - 对 `401`、`403`、`409` 展示稳定 UI 状态。
  - 不在浏览器状态里保存密钥。

- [ ] 实现登录页。

  文件：

  - `web/app/login/page.tsx`
  - `web/app/login/actions.ts`

  用户可以：

  - 输入邮箱和密码。
  - 创建 session。
  - 看到认证错误。

- [ ] 实现普通用户模板和 Agent 流程。

  文件：

  - `web/app/templates/page.tsx`
  - `web/app/templates/[id]/page.tsx`
  - `web/app/agents/page.tsx`
  - `web/app/agents/[id]/page.tsx`
  - `web/components/agent-runtime-status.tsx`
  - `web/components/weixin-channel-panel.tsx`

  要求：

  - 只列出已发布模板。
  - 用户可以基于模板创建 Agent。
  - 轮询 Agent，直到 `running` 或 `error`。
  - Agent 不是 `running` 时禁用微信配置入口。
  - 展示 pairing 二维码和状态。
  - 配对成功后展示 `connected`。

- [ ] 实现管理员模板页面。

  文件：

  - `web/app/admin/templates/page.tsx`
  - `web/app/admin/templates/new/page.tsx`
  - `web/app/admin/templates/[id]/page.tsx`
  - `web/components/template-editor.tsx`
  - `web/components/template-skill-list.tsx`

  要求：

  - 管理员可以创建模板草稿。
  - 管理员可以编辑 `SOUL.md`。
  - 管理员可以编辑 `USER.md`。
  - 管理员可以新增完整 skill，输入 `skill_name` 和 `SKILL.md` 内容。
  - 管理员可以删除完整 skill。
  - 已存在 skill 没有编辑按钮，也没有文件级编辑 UI。
  - 管理员可以发布和取消发布模板。

- [ ] 增加前端 E2E。

  文件：

  - `web/tests/agent-flow.spec.ts`
  - `web/tests/admin-template-flow.spec.ts`

  覆盖：

  - 用户登录。
  - 已发布模板列表。
  - 创建 Agent。
  - runtime 到达 `running`。
  - 微信入口在 `running` 前禁用。
  - 创建 pairing 后显示二维码。
  - mocked pairing 后状态变为 `connected`。
  - 管理员 skill 只支持新增和删除。

- [ ] 验证。

  ```bash
  cd web && npm run lint
  cd web && npm test
  cd web && npm run test:e2e
  ```

- [ ] 提交。

  ```bash
  git add . && git commit -m "Add Next.js console"
  ```

## 阶段 10：端到端运行验证

- [ ] 增加 fake Hermes runner 集成测试。

  文件：`services/api/tests/mvp_integration_test.go`

  场景：

  1. 管理员创建包含 `SOUL.md`、`USER.md` 和一个 skill 的模板。
  2. 管理员发布模板。
  3. 普通用户创建 Agent。
  4. worker 生成 Hermes home。
  5. fake runner 记录 Docker spec。
  6. Agent 变为 `running`。
  7. 用户创建微信 pairing session。
  8. fake iLink 返回 `wait`、`scaned`、`confirmed`。
  9. 渠道变为 `connected`。
  10. 断言 `.env` 包含 `WEIXIN_DM_POLICY=allowlist`、`WEIXIN_GROUP_POLICY=disabled`、`WEIXIN_ALLOWED_USERS={ilink_user_id}`。
  11. 断言 API 响应不暴露 token。

- [ ] 增加容器删除重建持久化测试。

  文件：`services/api/tests/runtime_persistence_test.go`

  场景：

  1. provision Agent。
  2. 在 `sessions/` 和 `weixin/accounts/{account_id}.json` 写入文件。
  3. 模拟容器删除。
  4. 创建 `restart_runtime` job。
  5. 断言宿主机 `hermes-home` 中的文件仍存在。

- [ ] 增加可选手动微信 smoke 文档。

  文件：`services/api/tests/manual/weixin_smoke.md`

  内容：

  ```markdown
  # Manual Weixin Smoke Test

  1. 启动 Docker。
  2. 在 `services/api` 执行 `go run ./cmd/agentforge-api`。
  3. 在 `web` 执行 `npm run dev`。
  4. 管理员创建并发布模板。
  5. 普通用户创建 Agent。
  6. 等待 Agent 进入 `running`。
  7. 创建微信 pairing session。
  8. 用后续要给 Agent 发消息的微信账号扫码。
  9. 在微信内确认。
  10. 给 Agent 发私信。
  11. 确认微信中收到 Agent 回复。
  ```

- [ ] 全量验证。

  ```bash
  cd services/api && go test ./...
  cd web && npm run lint && npm run test:e2e
  ```

- [ ] 提交。

  ```bash
  git add . && git commit -m "Add MVP integration coverage"
  ```

## 阶段 11：文档和运维说明

- [ ] 增加后端 runbook。

  文件：`docs/runbooks/backend.md`

  包含：

  - Docker 权限要求。
  - 后端 `.env` 配置。
  - SQLite WAL 文件：`agentforge.db`、`agentforge.db-wal`、`agentforge.db-shm`。
  - 备份目标：整个 `var/`。
  - 如何查看 runtime job。
  - 如何通过 REST 创建 job 进行重试。

- [ ] 增加 REST API 文档。

  文件：`docs/api.md`

  要写入本规格确认过的全部接口。不要记录 skill 编辑或 skill 替换接口。

- [ ] 增加安全清单。

  文件：`docs/security.md`

  包含：

  - 密钥不返回前端。
  - 每个 Agent 只挂载自己的 Hermes home。
  - 容器名由内部 ID 生成。
  - 微信私信 allowlist 使用扫码确认得到的 `ilink_user_id`。
  - 群聊默认禁用。

- [ ] 验证文档和代码没有动词式 REST 路径、没有 skill 编辑接口。

  命令：

  ```bash
  rg "/api/.*/start|/api/.*/stop|/api/.*/setup|/api/.*/publish" docs services/api web
  rg "PUT /api/admin/templates/.*/skills|PATCH /api/admin/templates/.*/skills" docs services/api web
  ```

  期望结果：

  ```text
  no matches
  ```

- [ ] 提交。

  ```bash
  git add . && git commit -m "Document MVP operations and API"
  ```

## 最终验收清单

- [ ] `cd services/api && go test ./...` 通过。
- [ ] `cd web && npm run lint` 通过。
- [ ] `cd web && npm run test:e2e` 通过。
- [ ] 管理员可以创建并发布 Agent 模板。
- [ ] 管理员可以编辑 `SOUL.md` 和 `USER.md`。
- [ ] 管理员只能新增和删除完整 skill。
- [ ] 普通用户可以看到已发布模板。
- [ ] 普通用户可以基于模板创建 Agent。
- [ ] 每个 Agent 有专用宿主机 `hermes-home`。
- [ ] Hermes 容器使用：

  ```bash
  -v {host_hermes_home}:/opt/data -e HERMES_HOME=/opt/data
  ```

- [ ] 删除并重建容器不会删除 `config.yaml`、`.env`、`sessions/` 或 `weixin/accounts/{account_id}.json`。
- [ ] Agent 到达 `running` 前，微信渠道配置入口禁用。
- [ ] 微信二维码流程使用 Go 原生 iLink client。
- [ ] 扫码确认得到的微信用户 ID 写入 `WEIXIN_ALLOWED_USERS`。
- [ ] `WEIXIN_DM_POLICY=allowlist`。
- [ ] `WEIXIN_GROUP_POLICY=disabled`。
- [ ] API 响应不暴露 provider API key、`WEIXIN_TOKEN` 或 `bot_token`。
- [ ] 用户可以用扫码的微信账号给 Agent 发私信并收到回复。
