# AgentForge MVP 设计规格

日期：2026-06-12

## 摘要

AgentForge MVP 提供一个完整闭环：用户从管理员维护的 Agent 模板创建自己的 Agent，等待 Hermes 运行时启动成功，然后通过微信扫码连接该 Agent，并在微信中与 Agent 聊天。

MVP 明确不包含普通用户上传 skills、普通用户编辑 `SOUL.md` 或 `USER.md`、团队空间、模板市场，以及 QQ、Telegram、WeCom 等其他渠道。这些能力可以在 Hermes + 微信运行链路稳定后再加入。

## 已确认范围

- 前端使用 Next.js。
- 后端使用 Go。
- 元数据数据库使用 SQLite，并开启 WAL。
- Agent 运行时复用 Hermes。
- 每个用户创建的 Agent 都有独立 Hermes 容器和独立 `HERMES_HOME`。
- 第一版通讯渠道是个人微信扫码登录。
- 平台是轻量多用户产品。
- 只有管理员可以创建和发布 Agent 模板。
- 普通用户只能基于已发布模板创建 Agent。
- 普通用户不能编辑模板提供的 `SOUL.md`、`USER.md` 或 skills。
- 普通用户在 MVP 中不能上传或选择自定义 skills。
- 渠道配置只能在 Agent 运行时启动成功后开放。

## Hermes 约束

Hermes 官方文档说明了以下能力：

- `SOUL.md` 用于人格配置。
- `USER.md` 和记忆相关文件用于持久用户上下文。
- skills 系统基于 skill 目录和 `SKILL.md`。
- 消息网关支持 Weixin、WeCom、QQ Bot、Telegram 等平台。
- `config.yaml` 是非密钥配置的主配置文件，`.env` 是 API key、token、密码等密钥配置位置。
- `hermes gateway setup` 是面向人工操作的交互式向导，不适合作为平台后端的主集成方式。
- gateway 运行时由平台适配器接收消息，通过聊天会话路由，并分发给 Agent 处理。

AgentForge 应把 Hermes 作为运行时边界。Go 后端负责生命周期、文件系统准备、`config.yaml` 和 `.env` 生成、容器操作和状态跟踪；Hermes 进程负责真实 Agent 执行、gateway 运行和聊天处理。

## 架构

系统分为三个主要边界。

### Next.js 控制台

控制台提供：

- 登录和会话 UI。
- 普通用户可见的模板列表。
- 基于模板创建 Agent。
- Agent 列表和详情页。
- 运行时状态展示。
- 微信渠道配置页。
- 二维码展示和连接状态。
- 管理员模板管理页面。

控制台不直接操作 Hermes 文件或容器。所有运行时动作都通过 Go API 完成。

### Go API 与 Worker

Go 后端提供：

- 认证和授权。
- `admin` 与 `user` 的角色权限控制。
- 开启 WAL 的 SQLite 持久化。
- 模板元数据和版本管理。
- Agent 实例创建。
- Hermes home 目录准备。
- Hermes 容器生命周期管理。
- 微信渠道配置编排。
- 运行时和渠道状态同步。
- 创建、启动和排障事件日志。

长耗时操作必须由 worker 任务执行，不能在请求处理器里阻塞完成。API 请求只负责创建记录、校验权限、投递任务并返回当前状态。

### Hermes 运行时

每个 Agent 对应一个独立 Hermes 容器。容器接收：

- 专用 `HERMES_HOME`。
- 模板提供的 `SOUL.md`、`USER.md`、Hermes 配置片段和 skills。
- 模型或 provider 访问所需的运行时配置。
- 用户启动微信渠道配置后生成的 gateway 配置。

平台不能在不同 Agent 或不同用户之间共享 Hermes home 目录。

## Hermes 配置策略

平台后端不直接驱动 `hermes gateway setup`。该命令是交互式向导，适合人工初始化和排障，但不适合稳定的后台编排。

MVP 采用文件生成方式配置 Hermes：

- Worker 为每个 Agent 创建独立 `HERMES_HOME`。
- Worker 将模板提供的 `SOUL.md`、`USER.md`、skills 和配置片段复制到该目录。
- Worker 生成或合并 `$HERMES_HOME/config.yaml`，写入模型、terminal、display、gateway、Weixin 平台配置等非密钥配置。
- Worker 生成或更新 `$HERMES_HOME/.env`，写入 provider key、gateway token、Weixin 适配器凭据等密钥配置。
- Worker 启动或重启该 Agent 的 Hermes gateway 进程。
- 对于微信扫码，Worker 通过 Hermes gateway 或 Weixin 适配器产生的二维码事件、日志、状态文件或后续可用的非交互接口获取二维码，并同步到 `channel_pairing_sessions`。

`hermes gateway setup` 只作为人工排障路径保留，不进入 MVP 的正常后台任务链路。

## 数据模型

### `users`

保存账号身份和角色。

重要字段：

- `id`
- `email`
- `password_hash` 或外部认证 subject
- `role`：`admin` 或 `user`
- `created_at`
- `updated_at`

### `agent_templates`

保存管理员维护的模板元数据。

重要字段：

- `id`
- `name`
- `description`
- `status`：`draft`、`published`、`archived`
- `version`
- `template_path`
- `content_checksum`
- `created_by`
- `created_at`
- `updated_at`
- `published_at`

已发布模板应保持不可变。编辑已发布模板时创建新版本，保证已有 Agent 可复现。

### `template_skills`

保存模板包含的 skills。

重要字段：

- `id`
- `template_id`
- `skill_name`
- `skill_path`
- `checksum`
- `created_at`

只有管理员可以通过模板管理流程修改这张表。

### `agents`

保存用户创建的 Agent 实例。

重要字段：

- `id`
- `owner_user_id`
- `template_id`
- `template_version`
- `name`
- `status`：`creating`、`provisioning`、`starting`、`running`、`stopped`、`error`
- `runtime_id`
- `hermes_home_path`
- `last_error_code`
- `last_error_message`
- `created_at`
- `updated_at`

### `agent_runtime_events`

保存运行时生命周期事件。

重要字段：

- `id`
- `agent_id`
- `event_type`
- `status_before`
- `status_after`
- `message`
- `metadata_json`
- `created_at`

这张表用于支持排障、调试和面向用户的摘要日志。

### `agent_channels`

保存渠道配置和状态。

重要字段：

- `id`
- `agent_id`
- `channel_type`：MVP 中为 `weixin`
- `status`：`not_configured`、`qr_pending`、`connected`、`error`、`disconnected`
- `external_account_id`
- `last_error_code`
- `last_error_message`
- `created_at`
- `updated_at`

### `channel_pairing_sessions`

保存二维码配对会话。

重要字段：

- `id`
- `agent_channel_id`
- `status`：`pending`、`connected`、`expired`、`failed`
- `qr_payload`
- `qr_image_path`
- `expires_at`
- `attempt_count`
- `last_error_code`
- `last_error_message`
- `created_at`
- `updated_at`

二维码内容和图片路径可以返回前端。provider key、gateway token、微信凭据和 Hermes secret 不能返回前端。

## 状态机

### Agent 运行时

正常流转：

```text
creating -> provisioning -> starting -> running
```

失败和恢复流转：

```text
creating -> error
provisioning -> error
starting -> error
running -> stopped
running -> error
stopped -> starting
error -> provisioning
error -> starting
```

重试从哪个阶段恢复取决于失败阶段。后端通过 `last_error_code` 和 `agent_runtime_events` 记录失败阶段。

### 微信渠道

正常流转：

```text
not_configured -> qr_pending -> connected
```

失败和恢复流转：

```text
qr_pending -> error
qr_pending -> not_configured
connected -> disconnected
disconnected -> qr_pending
error -> qr_pending
```

如果 Agent 不是 `running`，API 必须拒绝渠道配置请求。

## 用户流程

### 管理员模板流程

1. 管理员登录。
2. 管理员创建草稿 Agent 模板。
3. 管理员上传或编辑模板包，模板包包含 `SOUL.md`、`USER.md`、Hermes 配置片段和 skills。
4. 后端校验必需文件和 skill 结构。
5. 管理员发布模板。
6. 已发布模板出现在普通用户模板列表中。

### 普通用户创建 Agent 流程

1. 用户登录。
2. 用户查看已发布模板。
3. 用户选择模板并输入 Agent 名称。
4. API 创建 `agents` 记录，状态为 `creating`。
5. API 投递 `ProvisionAgent` 任务。
6. Worker 将模板包复制到新的 `HERMES_HOME`。
7. Worker 创建 Hermes 容器。
8. Worker 启动容器。
9. Agent 状态变为 `running`。
10. 控制台开放微信配置入口。

### 微信配置流程

1. 用户打开 Agent 详情页。
2. 控制台确认 Agent 状态为 `running`。
3. 用户启动微信配置。
4. API 创建或复用有效的 `channel_pairing_sessions` 记录。
5. Worker 更新该 Agent 的 `config.yaml` 和 `.env`，写入微信渠道所需配置。
6. Worker 启动或重启该 Agent 的 Hermes gateway。
7. Worker 从 gateway 或 Weixin 适配器的二维码输出中提取二维码内容或图片引用。
8. 控制台展示二维码。
9. 用户用微信扫码。
10. Worker 观察到连接成功。
11. 渠道状态变为 `connected`。
12. 用户在微信中与 Agent 聊天。

## API 设计

### 认证

- `POST /api/auth/login`
- `POST /api/auth/logout`
- `GET /api/auth/me`

### 模板

- `GET /api/templates`
- `GET /api/templates/{id}`
- `POST /api/admin/templates`
- `PUT /api/admin/templates/{id}`
- `POST /api/admin/templates/{id}/publish`
- `POST /api/admin/templates/{id}/archive`

### Agents

- `POST /api/agents`
- `GET /api/agents`
- `GET /api/agents/{id}`
- `POST /api/agents/{id}/start`
- `POST /api/agents/{id}/stop`
- `POST /api/agents/{id}/retry`

### 微信渠道

- `POST /api/agents/{id}/channels/weixin/setup`
- `GET /api/agents/{id}/channels/weixin`
- `POST /api/agents/{id}/channels/weixin/retry`

渠道接口返回当前状态，不阻塞等待用户扫码完成。

## Worker 任务

### `ProvisionAgent`

职责：

- 获取 Agent 级运行时锁。
- 将 Agent 状态从 `creating` 推进到 `provisioning`。
- 创建 Agent 的 `HERMES_HOME`。
- 复制模板文件和 skills。
- 写入 Hermes 配置。
- 创建 Hermes 容器。
- 启动 Hermes 容器。
- 将 Agent 状态推进到 `running`。
- 在每个主要步骤记录事件。

任务必须幂等。重复执行时应识别已有文件系统和容器状态，并从最后一个安全点继续。

### `SetupWeixinChannel`

职责：

- 校验 Agent 为 `running`。
- 获取 Agent 渠道锁。
- 创建或复用未过期的配对会话。
- 生成或更新该 Agent 的 `$HERMES_HOME/config.yaml` 和 `$HERMES_HOME/.env`。
- 启动、重启或重载该 Agent 的 Hermes gateway。
- 从 Hermes gateway、Weixin 适配器输出、状态文件或后续非交互接口中读取二维码内容或图片引用。
- 将渠道状态更新为 `qr_pending`。
- 观察连接成功、二维码过期或失败。
- 将渠道状态更新为 `connected`、`not_configured` 或 `error`。

重复 setup 请求应返回当前未过期的配对会话，不能创建互相竞争的二维码会话。

## 并发控制

SQLite 必须启用 WAL 模式和 busy timeout。

后端必须阻止同一 Agent 的运行时操作并发执行。例如：

- start 和 stop 不能同时执行。
- provision 和 retry 不能同时执行。
- 微信 setup 和渠道 retry 不能同时执行。

实现方式可以是数据库锁表，也可以是事务化任务状态检查。实现计划应选择第一版里最简单可靠的方案。

## 安全

授权规则：

- 管理员可以管理模板。
- 普通用户只能查看已发布模板。
- 普通用户只能为自己创建 Agent。
- 普通用户只能访问自己的 Agent、渠道和面向用户的事件摘要。
- 普通用户不能修改模板文件或 skills。

运行时隔离：

- 每个 Hermes 容器只挂载自己的 `HERMES_HOME`。
- 容器不能挂载共享用户 home 目录。
- 容器名称、路径和 runtime ID 应基于内部 ID 生成，不能直接使用用户输入。

密钥处理：

- provider API key、微信凭据、gateway token 和 Hermes secret 只能保存在服务端。
- 密钥不能出现在 API 响应、日志或前端状态中。
- 面向用户的错误只暴露简短摘要和稳定错误码。

## 错误处理

Agent 错误应记录失败阶段：

- `copy_template_failed`
- `config_write_failed`
- `container_create_failed`
- `container_start_failed`
- `container_healthcheck_failed`

微信错误应记录渠道相关原因：

- `agent_not_running`
- `qr_generation_failed`
- `qr_expired`
- `scan_rejected`
- `gateway_disconnected`
- `setup_timeout`

Agent 创建失败不会创建已连接渠道。微信渠道失败不会删除或回滚 Agent。

## 测试策略

### 后端单元测试

覆盖：

- RBAC 规则。
- 模板发布规则。
- Agent 状态机流转。
- 微信渠道状态机流转。
- SQLite repository 行为。
- 运行时锁行为。

### Worker 集成测试

默认使用 fake Hermes runner，而不使用真实容器。

覆盖：

- `ProvisionAgent` 成功。
- 模板复制失败。
- 容器启动失败。
- provision 失败后的重试。
- `SetupWeixinChannel` 成功。
- 二维码过期。
- 重复 setup 请求复用活跃配对会话。

### 前端 E2E 测试

覆盖：

- 用户登录。
- 用户选择已发布模板。
- 用户创建 Agent。
- Agent 推进到 `running`。
- 微信 setup 只在 `running` 后可用。
- 用户看到二维码。
- 模拟配对后状态变为 `connected`。

### 手动或可选外部测试

真实 Hermes + 微信扫码登录应作为可选测试，通过环境变量和手动账号可用性控制。它不应进入默认 CI。

## MVP 后续事项

- 用户上传 skills。
- 模板变量，替代用户直接编辑 `SOUL.md` 或 `USER.md`。
- QQ、Telegram、WeCom 和其他渠道。
- 模板市场和评分。
- 组织或团队支持。
- Agent 分享。
- 运行时指标和成本跟踪。
- 如果 Hermes 当前 Weixin 适配器没有稳定二维码状态输出，需要补充一个非交互式二维码状态接口或轻量 wrapper。

## 验收标准

- 管理员可以创建并发布 Agent 模板。
- 普通用户可以看到已发布模板。
- 普通用户可以基于模板创建 Agent。
- 后端为 Agent 创建专用 Hermes home。
- 后端为 Agent 启动专用 Hermes 容器。
- 控制台展示 Agent 生命周期状态。
- Agent 到达 `running` 前，微信 setup 禁用。
- Agent 到达 `running` 后，用户可以启动微信 setup。
- 控制台展示微信 setup 二维码。
- 扫码成功后，渠道标记为 `connected`。
- 用户可以从微信中与 Agent 聊天。
- Agent provisioning 失败和微信 setup 失败都会进入可恢复状态，并提供面向用户的错误信息。
