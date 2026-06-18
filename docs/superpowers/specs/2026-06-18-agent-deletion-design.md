# Agent 删除功能设计

- **状态**: Draft
- **日期**: 2026-06-18
- **作者**: zhengwenfeng（与 Claude 协作）

## 1. 背景与目标

AgentForge 当前提供创建、查询、重启 agent 的能力，但**缺少删除能力**。一个 agent 一旦创建就无法清理，磁盘上的 hermes-home 目录、容器、数据库记录会持续累积。本设计为系统补齐 agent 删除功能。

### 删除需要清理的资源

一个 agent 在系统中关联以下副作用：

1. **数据库**（`agents` 表 + 通过外键 CASCADE 关联的子表）：
   - `agent_runtime_events` — 运行时事件流
   - `agent_channels` — 渠道（→ 进而 CASCADE 清理 `channel_pairing_sessions`、`channel_jobs`）
   - `runtime_jobs` — 运行时任务历史
2. **Docker 容器**：命名 `agentforge-hermes-{agentID}`（见 `runtime.DefaultContainerName`）
3. **本地文件系统**：`{DATA_DIR}/agents/{agentID}/hermes-home/` 目录及其全部内容

删除必须把这三类资源都清理干净，否则会留下孤儿容器/文件。

### 设计目标

- **同步执行**：HTTP 请求直接驱动整个删除，返回 204 即真正删除完成
- **幂等可重试**：任一中间步失败，agent 进入 `error` 状态，用户可再次发起删除
- **并发安全**：通过状态白名单避免与正在跑的 RuntimeWorker 冲突
- **路径安全**：拒绝意外删除非 hermes-home 路径

## 2. 关键设计决策

| 决策点 | 选择 | 理由 |
|------|------|------|
| 中间态（`provisioning`/`starting`）能否删除 | **拒绝删除，返回 409** | 避免与 RuntimeWorker 并发；状态稳定后用户可重试 |
| 中途失败的处理策略 | **顺序删除，失败即中止，agent 进入 `error` 状态** | `error` 状态在删除白名单中 → 天然支持重试；不强求外部副作用的原子性 |
| 权限模型 | **owner + admin 可删** | 与现有 `authorizeAgent` 一致 |
| 微信渠道处理 | **不主动通知微信侧**，容器被杀后渠道经 DB CASCADE 清理 | 简单可靠；微信侧的"掉线"信号本就是离线设备的正常表现 |
| 删除前是否检查 runtime_jobs | **是，存在 pending/running 的 job 拒绝删除** | 防御性兜底，避免 worker 在 agent 删除后仍尝试操作其资源 |
| API 同步 vs 异步 | **同步 DELETE，返回 204** | 操作耗时可控（< 15s）；与 `template.Delete` 一致；避免引入新 job 类型和 `deleting` 状态 |
| 二次确认 | 前端弹框输入 agent 名称确认 | 后端 API 不做名称二次校验（前端 UX 职责） |

## 3. 架构与组件

### 3.1 调用链

```
HTTP DELETE /api/agents/:id
  │
  └─> http.AgentHandlers.Delete                      [新增 handler]
        │
        ├─ requireAuthenticatedUser                   [复用]
        ├─ authorizeAgent (owner/admin 校验)          [复用]
        └─ agents.Service.Delete(ctx, agentID)        [新增 service 方法]
              │
              ├─ 阶段 1: 校验
              │    ├─ Repository.Get                  [复用]
              │    ├─ Status.CanDelete()              [新增]
              │    └─ runtimeJobs.HasUnfinished       [新增]
              │
              ├─ 阶段 2: 容器清理（幂等）
              │    ├─ runtime.Runner.Inspect          [复用]
              │    ├─ runtime.Runner.Stop             [复用]
              │    └─ runtime.Runner.Remove           [复用]
              │
              ├─ 阶段 3: 文件清理（幂等）
              │    └─ runtime.DestroyHome             [新增]
              │
              └─ 阶段 4: 数据库清理（CASCADE）
                   └─ Repository.Delete               [新增]
```

### 3.2 改动文件清单

| 文件 | 改动类型 | 说明 |
|------|---------|------|
| `internal/agents/model.go` | 新增 | sentinel `ErrCannotDelete`、`ErrHasUnfinishedJobs`；方法 `Status.CanDelete()` |
| `internal/agents/errors.go` | 新建文件 | 业务错误码常量（DB 字段值，非 Go error） |
| `internal/agents/repository.go` | 新增方法 | `Delete(ctx, id)`、`MarkDeleteFailed(ctx, id, code, msg)` |
| `internal/agents/service.go` | 修改构造 + 新增方法 | `NewService` 增加 `runtime.Runner` 参数；新增 `Delete()` |
| `internal/jobs/runtime_repository.go` | 新增方法 | `HasUnfinishedByAgent(ctx, agentID)` |
| `internal/runtime/docker.go` | 修改 `Remove` | 把 "No such container" 转为 `ErrContainerNotFound`，与 `Inspect` 风格一致 |
| `internal/runtime/home.go` | 新增函数 | `DestroyHome(homePath)`（含路径安全校验） |
| `internal/http/agent_handlers.go` | 新增 handler | `Delete()` + 路由注册 `DELETE /agents/:id` |
| `internal/http/agent_handlers.go` | 扩展 `writeAgentError` | 映射 `ErrCannotDelete`/`ErrHasUnfinishedJobs` 到 409 |
| `cmd/agentforge-api/main.go` | 修改 wiring | 把 `runtime.Runner` 注入到 `agents.Service` |

**不需要数据库 migration**：现有外键的 `ON DELETE CASCADE` 已经覆盖所有子表清理。

## 4. 详细数据流

### 4.1 删除流程时序

```
请求                               响应
DELETE /api/agents/:id
Cookie: session=...
  │
  ├─ 401 unauthorized              （未登录）
  ├─ 403 forbidden                 （非 owner 也非 admin）
  ├─ 404 agent_not_found
  ├─ 409 agent_cannot_delete       （状态在白名单外）
  ├─ 409 agent_has_unfinished_jobs （存在未完成 job）
  ├─ 500 internal_error            （容器/文件操作失败 → agent 转 error 状态）
  └─ 204 No Content                （成功）
```

### 4.2 状态白名单

```go
func (s Status) CanDelete() bool {
    switch s {
    case StatusRunning, StatusStopped, StatusError, StatusCreating:
        return true
    default: // StatusProvisioning, StatusStarting
        return false
    }
}
```

`StatusCreating` 被允许进入白名单的理由：从 agent 行写入到 RuntimeWorker 拿走 provision job 之间存在窗口期，此时容器还未创建、文件还未生成。但白名单只是第一道闸门——紧接着的 `HasUnfinishedByAgent` 检查会发现 status=creating 时一定有一个 pending 的 provision job（这正是 `Service.Create` 在事务里一同写入的），从而拒绝删除。换句话说：creating 状态在实际运行中**总是会被未完成 job 检查挡下**，但保留它在白名单里有两个好处：(a) 让"白名单"代表"agent 自身允许删除"的纯粹语义，未完成 job 检查独立表达"作业系统视角的允许"；(b) 如果将来 provision job 模型变化（例如不再立即写入），这层逻辑自动适应。

`StatusError` 必须在白名单里——这是失败重试的基础。删除部分失败 → status 转 error → 用户重试时 error 通过白名单 → 进入幂等清理流程。

### 4.3 Service.Delete 实现骨架

```go
func (s *Service) Delete(ctx context.Context, agentID string) error {
    // ── 阶段 1: 校验 ─────────────────────────────────────────
    agent, err := s.repository.Get(ctx, agentID)
    if err != nil {
        return fmt.Errorf("get agent for delete: %w", err)
    }
    if !agent.Status.CanDelete() {
        return fmt.Errorf("%w: status=%s", ErrCannotDelete, agent.Status)
    }
    hasUnfinished, err := s.runtimeJobs.HasUnfinishedByAgent(ctx, agentID)
    if err != nil {
        return fmt.Errorf("check unfinished jobs: %w", err)
    }
    if hasUnfinished {
        return ErrHasUnfinishedJobs
    }

    // ── 阶段 2: 容器清理（幂等） ──────────────────────────────
    containerName := runtime.DefaultContainerName(agentID)
    status, inspectErr := s.runner.Inspect(ctx, containerName)
    if inspectErr != nil && !errors.Is(inspectErr, runtime.ErrContainerNotFound) {
        return s.failWith(ctx, agentID, DeleteFailureInspect,
            fmt.Errorf("inspect container: %w", inspectErr))
    }
    if inspectErr == nil { // 容器存在
        if status.Running {
            if err := s.runner.Stop(ctx, containerName); err != nil {
                return s.failWith(ctx, agentID, DeleteFailureStop,
                    fmt.Errorf("stop container: %w", err))
            }
        }
        if err := s.runner.Remove(ctx, containerName); err != nil {
            // 并发情形：另一个删除流程已 remove 了容器
            if !errors.Is(err, runtime.ErrContainerNotFound) {
                return s.failWith(ctx, agentID, DeleteFailureRemove,
                    fmt.Errorf("remove container: %w", err))
            }
        }
    }

    // ── 阶段 3: 文件清理（幂等） ──────────────────────────────
    if err := runtime.DestroyHome(agent.HermesHomePath); err != nil {
        return s.failWith(ctx, agentID, DeleteFailureHome,
            fmt.Errorf("destroy hermes home: %w", err))
    }

    // ── 阶段 4: 数据库清理（CASCADE） ─────────────────────────
    if err := s.repository.Delete(ctx, agentID); err != nil {
        // 容器和文件已删除；DB 删除失败属于罕见情况，不调 markError
        // （markError 也走 DB，多半也会失败）。返回错误让上层观测到。
        return fmt.Errorf("delete agent from database: %w", err)
    }
    return nil
}

// failWith 将 agent 置 error 状态并组合错误返回。markError 失败时用 errors.Join。
func (s *Service) failWith(ctx context.Context, agentID, code string, original error) error {
    msg := original.Error()
    if markErr := s.repository.MarkDeleteFailed(ctx, agentID, code, msg); markErr != nil {
        return errors.Join(original, fmt.Errorf("mark agent delete failed: %w", markErr))
    }
    return original
}
```

### 4.4 Repository.MarkDeleteFailed

不能复用 `TransitionStatus`——它依赖现有状态机表，而 `stopped → error` 不在转移图里。新增一个**绕过状态机校验**的专用方法：

```go
func (r *Repository) MarkDeleteFailed(ctx context.Context, id, code, msg string) error {
    result, err := r.database.ExecContext(ctx, `
        UPDATE agents
        SET status = 'error',
            last_error_code = ?,
            last_error_message = ?,
            updated_at = datetime('now')
        WHERE id = ?;
    `, code, msg, id)
    if err != nil {
        return fmt.Errorf("update agent to error: %w", err)
    }
    return requireAffected(result)
}
```

### 4.5 Repository.Delete

```go
func (r *Repository) Delete(ctx context.Context, id string) error {
    result, err := r.database.ExecContext(ctx,
        `DELETE FROM agents WHERE id = ?;`, id)
    if err != nil {
        return fmt.Errorf("delete agent row: %w", err)
    }
    return requireAffected(result)
}
```

外键 CASCADE 自动清理：`agent_runtime_events`、`agent_channels`（→ `channel_pairing_sessions`、`channel_jobs`）、`runtime_jobs`。

### 4.6 RuntimeRepository.HasUnfinishedByAgent

```go
func (r *RuntimeRepository) HasUnfinishedByAgent(ctx context.Context, agentID string) (bool, error) {
    var count int
    err := r.database.QueryRowContext(ctx, `
        SELECT COUNT(*)
        FROM runtime_jobs
        WHERE agent_id = ? AND status IN ('pending', 'running');
    `, agentID).Scan(&count)
    if err != nil {
        return false, fmt.Errorf("count unfinished runtime jobs: %w", err)
    }
    return count > 0, nil
}
```

### 4.7 dockerRunner.Remove 调整

`Service.Delete` 在阶段 2 的并发吞错依赖 `errors.Is(err, runtime.ErrContainerNotFound)` 命中。但当前 `dockerRunner.Remove`（`internal/runtime/docker.go:105`）失败时仅返回 `fmt.Errorf("docker rm failed: %w: %s", ...)`，永远不会返回 `ErrContainerNotFound`。需要改为与 `Inspect` 同样的处理风格：

```go
func (r *dockerRunner) Remove(ctx context.Context, containerName string) error {
    output, err := exec.CommandContext(ctx, r.dockerBin, "rm", "-f", containerName).CombinedOutput()
    if err != nil {
        trimmed := strings.TrimSpace(string(output))
        if strings.Contains(trimmed, "No such object") || strings.Contains(trimmed, "No such container") {
            return ErrContainerNotFound
        }
        return fmt.Errorf("docker rm failed: %w: %s", err, trimmed)
    }
    return nil
}
```

这是一个**对外行为兼容**的小修改：现有调用方（如重启逻辑）若没有专门处理 NotFound，新行为只是把"docker rm 不存在的容器返回错误"变成"返回更具体的 ErrContainerNotFound"，调用方仍可作为通用错误处理；新增的 `Service.Delete` 则可以借此实现并发幂等。

### 4.8 runtime.DestroyHome（含路径安全校验）

```go
// DestroyHome 删除 agent 的 hermes-home 目录。已不存在视为成功（幂等）。
// 强制路径必须以 "/hermes-home" 结尾且足够深，防御误删根目录或父目录。
func DestroyHome(homePath string) error {
    trimmed := strings.TrimSpace(homePath)
    if trimmed == "" {
        return errors.New("hermes home path is empty")
    }
    abs, err := filepath.Abs(trimmed)
    if err != nil {
        return fmt.Errorf("resolve hermes home path: %w", err)
    }
    cleaned := filepath.Clean(abs)
    if filepath.Base(cleaned) != "hermes-home" {
        return fmt.Errorf("refuse to destroy non-hermes-home path: %s", cleaned)
    }
    // 至少要在 .../agents/<id>/hermes-home 这种深度
    parent := filepath.Dir(cleaned)
    grandparent := filepath.Dir(parent)
    if grandparent == "/" || grandparent == "." || grandparent == filepath.VolumeName(grandparent)+string(filepath.Separator) {
        return fmt.Errorf("refuse to destroy shallow path: %s", cleaned)
    }
    if err := os.RemoveAll(cleaned); err != nil {
        return fmt.Errorf("remove hermes home: %w", err)
    }
    return nil
}
```

`os.RemoveAll` 在路径不存在时本来就不报错，幂等性自动具备。

## 5. 错误处理

本节遵循 `golang-error-handling` skill。

### 5.1 错误类型

```go
// internal/agents/model.go — sentinel errors（用 errors.Is 识别）
var (
    ErrCannotDelete      = errors.New("agent cannot be deleted in current state")
    ErrHasUnfinishedJobs = errors.New("agent has unfinished runtime jobs")
)
```

### 5.2 业务错误码常量（写入 `agent.last_error_code`）

```go
// internal/agents/errors.go
const (
    DeleteFailureInspect = "delete_inspect_failed"
    DeleteFailureStop    = "delete_stop_failed"
    DeleteFailureRemove  = "delete_remove_failed"
    DeleteFailureHome    = "delete_home_failed"
)
```

这些是字符串常量供前端识别错误类型，不是 Go error 值，故无 `Err` 前缀。

### 5.3 错误处理铁律

**Service 层**：

- 永远不调用 `slog.*` 记录日志
- 每一层都用 `fmt.Errorf("{action}: %w", err)` 包装下层错误，保留错误链
- sentinel 直接返回不要 wrap（让调用方 `errors.Is` 命中）；带额外上下文时用 `fmt.Errorf("%w: ...", sentinel, ...)`
- `failWith` 中如果 markError 自身失败，用 `errors.Join` 组合返回——不掩盖原始错误，也不丢失 markError 失败的信息

**Handler 层**（**唯一的日志触发点**）：

```go
func (h *AgentHandlers) Delete(c *gin.Context) {
    agent, ok := h.authorizeAgent(c)
    if !ok {
        return
    }
    user, _ := UserFromContext(c)

    if err := h.service.Delete(c.Request.Context(), agent.ID); err != nil {
        slog.ErrorContext(c.Request.Context(), "agent delete failed",
            "agent_id", agent.ID,
            "actor_user_id", user.ID,
            "error", err)
        writeAgentError(c, err)
        return
    }
    slog.InfoContext(c.Request.Context(), "agent delete succeeded",
        "agent_id", agent.ID,
        "actor_user_id", user.ID)
    c.Status(http.StatusNoContent)
}
```

### 5.4 HTTP 错误映射

扩展 `writeAgentError`：

| Service 返回（`errors.Is` 匹配） | HTTP 状态 | error code |
|------|------|------|
| `ErrNotFound` | 404 | `agent_not_found` |
| `ErrCannotDelete` | 409 | `agent_cannot_delete` |
| `ErrHasUnfinishedJobs` | 409 | `agent_has_unfinished_jobs` |
| 其他 | 500 | `internal_error`（不暴露技术细节） |

### 5.5 路由注册

在 `AgentHandlers.Register` 里加：

```go
router.DELETE("/agents/:id", h.Delete)
```

## 6. 并发与幂等

### 6.1 与 RuntimeWorker 的并发

- 状态白名单（拒绝 provisioning/starting）+ unfinished jobs 检查 → 双重防御
- worker 拿到 job 后会先 `TransitionStatus(creating → provisioning)`；如果删除发起在 worker 拿走 job 之前，job 仍在 pending → unfinished 检查命中 → 拒绝删除
- 如果 worker 已开始 → status=provisioning → 白名单命中 → 拒绝删除
- 用户消息："请稍后重试"

### 6.2 两个删除请求并发

- 第一个进入 → 阶段 2 stop+remove 容器
- 第二个进入 → Inspect 可能：(a) 容器已删 → ErrContainerNotFound → 跳过；(b) 容器存在但即将被删 → Remove 时返回 NotFound → 在 Service 层吞掉
- 第一个进入阶段 4 删 DB 行；第二个进入阶段 4 时 `requireAffected` 返回 `ErrNotFound`
- 一个 204、另一个 404，都符合预期语义

### 6.3 失败后的重试

- 任一外部副作用阶段失败 → agent.status='error'、last_error_code 设置
- 用户再次发起 DELETE：
  - status=error 通过白名单
  - 容器/文件如果上次部分删除完成 → Inspect 返回 NotFound 跳过、`os.RemoveAll` 不报错
  - 接续完成剩余清理

## 7. 测试策略

### 7.1 Service 层单测

文件：`internal/agents/service_delete_test.go`

mock `runtime.Runner`（接口已存在），数据库用 in-memory SQLite + 真实迁移。

**A. 状态白名单**

| Case | 初始状态 | 期望 |
|------|---------|------|
| `TestDelete_Running` | running | 成功 |
| `TestDelete_Stopped` | stopped | 成功 |
| `TestDelete_Error` | error | 成功（**关键 — 支持失败重试**） |
| `TestDelete_Creating` | creating | 成功（容器还没起来） |
| `TestDelete_Provisioning` | provisioning | `ErrCannotDelete` |
| `TestDelete_Starting` | starting | `ErrCannotDelete` |
| `TestDelete_NotFound` | （不存在） | `ErrNotFound` |

**B. 未完成 job 防御**

| Case | 描述 | 期望 |
|------|------|------|
| `TestDelete_HasPendingJob` | running + pending restart job | `ErrHasUnfinishedJobs` |
| `TestDelete_HasRunningJob` | running + running job | `ErrHasUnfinishedJobs` |
| `TestDelete_OnlyHasFinishedJobs` | running + 已 succeeded/failed jobs | 成功 |

**C. 容器清理分支**

| Case | 容器状态 | 期望 |
|------|---------|------|
| `TestDelete_ContainerRunning` | exists & running | Stop 调用 1 次, Remove 调用 1 次 |
| `TestDelete_ContainerStopped` | exists, not running | Stop 不调用，Remove 调用 1 次 |
| `TestDelete_ContainerNotFound` | 不存在 | Stop/Remove 都不调用 |
| `TestDelete_ConcurrentRemoveRace` | Inspect 时存在；Remove 返回 NotFound | 视为成功 |

**D. 失败路径**

| Case | 失败阶段 | 期望 |
|------|---------|------|
| `TestDelete_InspectFails` | 非 NotFound | 返回 wrapped error；status=error；code=`delete_inspect_failed` |
| `TestDelete_StopFails` | Stop | 同上，code=`delete_stop_failed` |
| `TestDelete_RemoveFails` | 非 NotFound | 同上，code=`delete_remove_failed` |
| `TestDelete_DestroyHomeFails` | RemoveAll 失败（chmod 只读模拟） | 同上，code=`delete_home_failed` |
| `TestDelete_DBDeleteFails` | repository.Delete | 返回 wrapped error；**不** markError |
| `TestDelete_MarkErrorAlsoFails` | Stop 失败且 markError 失败 | 返回 `errors.Join`，两 err 都能被 `errors.Is` 命中 |

**E. 文件副作用**

| Case | 描述 | 期望 |
|------|------|------|
| `TestDelete_HomeDirectoryRemoved` | 创建临时 hermes-home | 删除后 `os.Stat` 返回 IsNotExist |
| `TestDelete_HomeDirectoryAlreadyGone` | 不创建目录 | 不报错（幂等） |
| `TestDelete_DBCascadeWorks` | 创建 agent + runtime_jobs(succeeded) + agent_channels | 删除后子表行数=0 |

**F. 重试场景**

| Case | 描述 | 期望 |
|------|------|------|
| `TestDelete_RetryAfterStopFailure` | 第一次 Stop 失败；第二次 Stop OK | 第二次返回成功 |

### 7.2 Handler 层单测

文件：`internal/http/agent_handlers_test.go`（追加）

| Case | 期望 |
|------|------|
| `TestDelete_Unauthenticated` | 没有 session → 401 |
| `TestDelete_NotOwnerNotAdmin` | 删别人的 → 403 |
| `TestDelete_Owner` | owner 删自己的 → 转给 service |
| `TestDelete_Admin` | admin 删别人的 → 转给 service |
| `TestDelete_ServiceReturnsCannotDelete` | 409 + code=`agent_cannot_delete` |
| `TestDelete_ServiceReturnsHasUnfinishedJobs` | 409 + code=`agent_has_unfinished_jobs` |
| `TestDelete_ServiceReturnsNotFound` | 404 + code=`agent_not_found` |
| `TestDelete_ServiceReturnsInternalError` | 500，不暴露错误细节 |
| `TestDelete_ServiceSuccess` | 204；ERROR 日志未触发；INFO 日志触发一次 |

日志断言用 slog 的 testHandler 捕获事件。

### 7.3 路径安全单测

文件：`internal/runtime/home_test.go`

| Case | 输入 | 期望 |
|------|------|------|
| `TestDestroyHome_NormalPath` | `/data/agents/abc-123/hermes-home`（存在） | 删除成功 |
| `TestDestroyHome_NotExists` | 同上但不存在 | 不报错 |
| `TestDestroyHome_RefusesEmpty` | `""` | 错误，不调用 RemoveAll |
| `TestDestroyHome_RefusesRoot` | `/` | 错误 |
| `TestDestroyHome_RefusesShallow` | `/hermes-home` | 错误 |
| `TestDestroyHome_RefusesNonHermesHome` | `/data/agents/abc/skills` | 错误 |
| `TestDestroyHome_AcceptsRelativePath` | `./tmp/agents/x/hermes-home` | 转绝对后通过 |

### 7.4 集成测试

文件：`tests/agent_delete_integration_test.go`（新建）

主流程（mock docker runner，真实 SQLite + 真实文件系统）：

```
1. 创建用户 + 登录
2. 创建模板 + 发布
3. POST /api/agents 创建 agent
4. 直接用 db.Exec UPDATE agents SET status='running'
   并把对应 provision job 标为 succeeded（绕过 RuntimeWorker，
   因为状态机不允许直接 creating → running，且本测试不验证 worker）
5. 在 hermes-home 路径下放 dummy 文件，验证存在
6. DELETE /api/agents/:id → 204
7. GET /api/agents/:id → 404
8. 验证 agents、runtime_jobs(agent_id)、agent_channels 行数都为 0
9. 验证 hermes-home 目录已不存在
10. 验证 mock runner.Stop 和 Remove 都被调用
```

失败重试集成测试 `AgentDelete_StopFailure_Recovers`：

```
1. 创建 agent，按上面方式直接置 running
2. 注入 mock runner.Stop 第一次返回错误
3. DELETE → 500
4. GET → status=error, last_error_code=delete_stop_failed
5. mock runner.Stop 改为成功
6. 再次 DELETE → 204
7. 验证彻底清理
```

## 8. API 文档更新

`docs/api.md` 中的 Agents 章节追加：

```
DELETE /api/agents/:id
  Auth: required (owner 或 admin)
  Returns:
    204 No Content                   删除成功
    401 unauthorized                 未登录
    403 forbidden                    无权操作此 agent
    404 agent_not_found              agent 不存在
    409 agent_cannot_delete          当前状态不允许删除（provisioning/starting）
    409 agent_has_unfinished_jobs    存在未完成的运行时任务
    500 internal_error               删除过程内部错误（agent 转 error 状态，可重试）
```

## 9. 不在本设计范围内

- 软删除/回收站（当前是物理删除）
- 批量删除
- 前端确认框 UX 实现（前端单独完成）
- 微信账号主动登出（依赖 hermes 容器停止后微信侧自然掉线）
- 删除审计日志的持久化表（slog 日志已覆盖运行时记录）

## 10. 实施顺序建议

1. `runtime.DestroyHome` + 路径安全单测
2. `Status.CanDelete()` + sentinel errors + `internal/agents/errors.go`
3. `Repository.Delete` + `Repository.MarkDeleteFailed`
4. `RuntimeRepository.HasUnfinishedByAgent`
5. `Service.Delete` + 修改 `NewService`（注入 Runner）
6. `main.go` wiring 调整
7. `AgentHandlers.Delete` + `writeAgentError` 扩展 + 路由注册
8. 全部单测
9. 集成测试
10. 更新 `docs/api.md`

逐步提交，每一步保证 `go test ./...` 通过。
