# Agent 删除功能实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 AgentForge 后端 API 增加 agent 删除功能，同步执行、按顺序清理 Docker 容器、本地 hermes-home 目录与数据库记录，支持失败重试。

**Architecture:** 在 `agents.Service` 上新增 `Delete` 方法编排四阶段顺序清理（校验 → 容器 → 文件 → 数据库），通过状态白名单 + unfinished job 检查避免与 RuntimeWorker 并发冲突。任意外部副作用阶段失败时把 agent 置为 `error` 状态，因 `error` 在白名单内 + 每步幂等，用户可重试到完全清理。HTTP 同步返回，无新增 job 类型与状态机变更。

**Tech Stack:** Go 1.22+, gin, modernc.org/sqlite, slog, Docker CLI

## Global Constraints

- 设计文档：`docs/superpowers/specs/2026-06-18-agent-deletion-design.md`
- 错误处理标准：`golang-error-handling` skill —— Service 层只 wrap+return 不打日志，handler 层是唯一的 slog 触发点；sentinel 用 `errors.Is` 匹配，wrap 用 `%w`
- 错误消息小写、无尾部标点
- 不新增数据库迁移（依赖现有外键 CASCADE）
- 不引入新 job 类型，不改 agent 状态机 transitions 表
- TDD：每个 Task 先写测试再写实现，每个 Task 结束都要 `cd services/api && go test ./...` 通过
- 工作目录：所有 `go` 命令在 `services/api` 子目录内运行
- 提交消息使用 conventional commit 风格

---

## File Structure

新建文件：
- `services/api/internal/agents/errors.go` — `DeleteFailure*` 业务错误码常量
- `services/api/internal/agents/service_delete_test.go` — `Service.Delete` 单元测试
- `services/api/internal/agents/repository_delete_test.go` — `Repository.Delete`/`MarkDeleteFailed` 单测
- `services/api/internal/jobs/runtime_repository_unfinished_test.go` — `HasUnfinishedByAgent` 单测
- `services/api/tests/agent_delete_integration_test.go` — 端到端集成测试

修改文件：
- `services/api/internal/agents/model.go` — 新增 `ErrCannotDelete`、`ErrHasUnfinishedJobs`、`Status.CanDelete()`
- `services/api/internal/agents/repository.go` — 新增 `Delete`、`MarkDeleteFailed` 方法
- `services/api/internal/agents/service.go` — `NewService` 注入 `runtime.Runner`；新增 `Delete` 方法
- `services/api/internal/jobs/runtime_repository.go` — 新增 `HasUnfinishedByAgent` 方法
- `services/api/internal/runtime/docker.go` — `Remove` 把 "No such container" 转为 `ErrContainerNotFound`
- `services/api/internal/runtime/home.go` — 新增 `DestroyHome` 函数（含路径安全校验）
- `services/api/internal/runtime/home_test.go` — 追加 `DestroyHome` 测试用例
- `services/api/internal/http/agent_handlers.go` — 新增 `Delete` handler 与路由
- `services/api/internal/http/errors.go` — `writeAgentError` 扩展映射 `ErrCannotDelete`/`ErrHasUnfinishedJobs`
- `services/api/cmd/agentforge-api/main.go` — `agents.NewService` 调用增加 `runner` 参数
- `docs/api.md` — Agents 章节追加 DELETE 端点文档

---

## Task 1: runtime.DestroyHome（路径安全删除）

**Files:**
- Modify: `services/api/internal/runtime/home.go`
- Test: `services/api/internal/runtime/home_test.go`

**Interfaces:**
- Produces: `func DestroyHome(homePath string) error` — 删除 hermes-home 目录；空路径/根目录/非 hermes-home 后缀路径返回错误；目录已不存在返回 nil（幂等）

- [ ] **Step 1: 在 home_test.go 末尾追加测试用例**

打开 `services/api/internal/runtime/home_test.go`，在文件末尾添加：

```go
func TestDestroyHomeRemovesExistingDirectory(t *testing.T) {
	root := t.TempDir()
	homePath := filepath.Join(root, "agents", "agent-x", "hermes-home")
	if err := os.MkdirAll(filepath.Join(homePath, "memories"), 0o755); err != nil {
		t.Fatalf("mkdir hermes-home: %v", err)
	}
	if err := os.WriteFile(filepath.Join(homePath, "USER.md"), []byte("payload"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	if err := DestroyHome(homePath); err != nil {
		t.Fatalf("DestroyHome returned error: %v", err)
	}

	if _, err := os.Stat(homePath); !os.IsNotExist(err) {
		t.Fatalf("expected hermes home removed, got err=%v", err)
	}
}

func TestDestroyHomeIsIdempotentWhenMissing(t *testing.T) {
	root := t.TempDir()
	homePath := filepath.Join(root, "agents", "agent-x", "hermes-home")

	if err := DestroyHome(homePath); err != nil {
		t.Fatalf("DestroyHome on missing dir returned error: %v", err)
	}
}

func TestDestroyHomeRefusesEmptyPath(t *testing.T) {
	if err := DestroyHome(""); err == nil {
		t.Fatal("DestroyHome(\"\") returned nil, want error")
	}
}

func TestDestroyHomeRefusesNonHermesHome(t *testing.T) {
	root := t.TempDir()
	bad := filepath.Join(root, "agents", "agent-x", "skills")
	if err := os.MkdirAll(bad, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := DestroyHome(bad); err == nil {
		t.Fatal("DestroyHome on non-hermes-home path returned nil, want error")
	}
	if _, err := os.Stat(bad); err != nil {
		t.Fatalf("non-hermes-home path was deleted: %v", err)
	}
}

func TestDestroyHomeRefusesShallowPath(t *testing.T) {
	if err := DestroyHome("/hermes-home"); err == nil {
		t.Fatal("DestroyHome(\"/hermes-home\") returned nil, want error")
	}
}

func TestDestroyHomeAcceptsRelativePath(t *testing.T) {
	root := t.TempDir()
	rel, err := filepath.Rel(must(t, os.Getwd()), filepath.Join(root, "agents", "agent-x", "hermes-home"))
	if err != nil {
		t.Fatalf("rel: %v", err)
	}
	abs := filepath.Join(root, "agents", "agent-x", "hermes-home")
	if err := os.MkdirAll(abs, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := DestroyHome(rel); err != nil {
		t.Fatalf("DestroyHome on relative path returned error: %v", err)
	}
	if _, err := os.Stat(abs); !os.IsNotExist(err) {
		t.Fatalf("expected dir removed, err=%v", err)
	}
}

func must(t *testing.T, s string, err error) string {
	t.Helper()
	if err != nil {
		t.Fatalf("must helper: %v", err)
	}
	return s
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd services/api && go test ./internal/runtime/ -run TestDestroyHome -v
```

预期输出：FAIL，错误为 `undefined: DestroyHome`。

- [ ] **Step 3: 在 home.go 中实现 DestroyHome**

打开 `services/api/internal/runtime/home.go`，在文件末尾追加：

```go
// DestroyHome removes the agent's hermes-home directory. It is idempotent:
// a missing directory is treated as success. The path is validated to end
// with "hermes-home" and to be at least three levels deep, refusing root or
// other shallow paths to avoid accidental destruction.
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
	parent := filepath.Dir(cleaned)
	grandparent := filepath.Dir(parent)
	separator := string(filepath.Separator)
	if grandparent == separator || grandparent == "." || grandparent == filepath.VolumeName(grandparent)+separator {
		return fmt.Errorf("refuse to destroy shallow path: %s", cleaned)
	}
	if err := os.RemoveAll(cleaned); err != nil {
		return fmt.Errorf("remove hermes home: %w", err)
	}
	return nil
}
```

注意：`home.go` 已经 import 了 `errors`、`fmt`、`os`、`path/filepath`、`strings`，不需要添加新 import。

- [ ] **Step 4: 运行测试确认通过**

```bash
cd services/api && go test ./internal/runtime/ -run TestDestroyHome -v
```

预期所有 6 个 TestDestroyHome* 用例 PASS。再跑全包测试确认没破坏其他测试：

```bash
cd services/api && go test ./internal/runtime/
```

预期 PASS。

- [ ] **Step 5: 提交**

```bash
git add services/api/internal/runtime/home.go services/api/internal/runtime/home_test.go
git commit -m "feat(runtime): add DestroyHome with path safety guards"
```

---

## Task 2: dockerRunner.Remove 转换 NotFound 错误

**Files:**
- Modify: `services/api/internal/runtime/docker.go:105-111`
- Test: `services/api/internal/runtime/docker_test.go`

**Interfaces:**
- Modifies: `dockerRunner.Remove` 在 stderr 包含 "No such container"/"No such object" 时返回 `ErrContainerNotFound`

- [ ] **Step 1: 在 docker_test.go 末尾追加测试用例**

打开 `services/api/internal/runtime/docker_test.go`，在文件末尾追加（如果文件结尾没有 helpers，先确认 `shellQuote` 等 helper 存在；这些 helpers 在文件中已有定义）：

```go
func TestDockerRunnerRemoveReturnsNotFoundWhenContainerMissing(t *testing.T) {
	ctx := context.Background()
	workdir := t.TempDir()
	stubPath := filepath.Join(workdir, "docker")
	script := "#!/bin/sh\nprintf 'Error: No such container: %s' \"$3\" >&2\nexit 1\n"
	if err := os.WriteFile(stubPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub docker: %v", err)
	}
	runner := NewDockerRunner(stubPath)

	err := runner.Remove(ctx, "agentforge-hermes-missing")
	if !errors.Is(err, ErrContainerNotFound) {
		t.Fatalf("Remove err = %v, want ErrContainerNotFound", err)
	}
}

func TestDockerRunnerRemoveWrapsOtherErrors(t *testing.T) {
	ctx := context.Background()
	workdir := t.TempDir()
	stubPath := filepath.Join(workdir, "docker")
	script := "#!/bin/sh\nprintf 'Error: permission denied' >&2\nexit 1\n"
	if err := os.WriteFile(stubPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub docker: %v", err)
	}
	runner := NewDockerRunner(stubPath)

	err := runner.Remove(ctx, "agentforge-hermes-x")
	if err == nil {
		t.Fatal("Remove returned nil, want error")
	}
	if errors.Is(err, ErrContainerNotFound) {
		t.Fatalf("Remove err = %v, did not expect ErrContainerNotFound", err)
	}
}
```

如果 `docker_test.go` 中尚未 import `errors`，需要添加。

- [ ] **Step 2: 运行测试确认失败**

```bash
cd services/api && go test ./internal/runtime/ -run TestDockerRunnerRemove -v
```

预期 `TestDockerRunnerRemoveReturnsNotFoundWhenContainerMissing` FAIL（`errors.Is` 返回 false），`TestDockerRunnerRemoveWrapsOtherErrors` PASS。

- [ ] **Step 3: 修改 dockerRunner.Remove**

打开 `services/api/internal/runtime/docker.go`，找到第 105-111 行的 `Remove` 方法，替换为：

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

- [ ] **Step 4: 运行测试确认通过**

```bash
cd services/api && go test ./internal/runtime/ -run TestDockerRunnerRemove -v
```

预期两个用例都 PASS。然后跑整个 runtime 包：

```bash
cd services/api && go test ./internal/runtime/
```

预期 PASS。

- [ ] **Step 5: 提交**

```bash
git add services/api/internal/runtime/docker.go services/api/internal/runtime/docker_test.go
git commit -m "feat(runtime): map docker rm not-found to ErrContainerNotFound"
```

---

## Task 3: agents.Status.CanDelete + sentinel errors

**Files:**
- Modify: `services/api/internal/agents/model.go`
- Test: `services/api/internal/agents/state_test.go`

**Interfaces:**
- Produces:
  - `var ErrCannotDelete = errors.New("agent cannot be deleted in current state")`
  - `var ErrHasUnfinishedJobs = errors.New("agent has unfinished runtime jobs")`
  - `func (s Status) CanDelete() bool`

- [ ] **Step 1: 在 state_test.go 末尾追加 CanDelete 测试**

打开 `services/api/internal/agents/state_test.go`，在文件末尾追加：

```go
func TestStatusCanDelete(t *testing.T) {
	t.Parallel()

	cases := []struct {
		status Status
		want   bool
	}{
		{StatusCreating, true},
		{StatusRunning, true},
		{StatusStopped, true},
		{StatusError, true},
		{StatusProvisioning, false},
		{StatusStarting, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.status), func(t *testing.T) {
			t.Parallel()
			if got := tc.status.CanDelete(); got != tc.want {
				t.Fatalf("%s.CanDelete() = %t, want %t", tc.status, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd services/api && go test ./internal/agents/ -run TestStatusCanDelete -v
```

预期 FAIL：`undefined: CanDelete`。

- [ ] **Step 3: 在 model.go 中添加 sentinel 与 CanDelete**

打开 `services/api/internal/agents/model.go`，在 `var (...)` 错误声明块（约第 16-23 行）末尾追加：

```go
	ErrCannotDelete      = errors.New("agent cannot be deleted in current state")
	ErrHasUnfinishedJobs = errors.New("agent has unfinished runtime jobs")
```

最终该块形如：

```go
var (
	ErrNotFound               = errors.New("agent not found")
	ErrTemplateNotFound       = errors.New("agent template not found")
	ErrConflict               = errors.New("agent conflict")
	ErrInvalidInput           = errors.New("invalid agent input")
	ErrInvalidStateTransition = errors.New("invalid agent status transition")
	ErrRuntimeUnavailable     = errors.New("agent runtime unavailable")
	ErrCannotDelete           = errors.New("agent cannot be deleted in current state")
	ErrHasUnfinishedJobs      = errors.New("agent has unfinished runtime jobs")
)
```

然后在文件末尾追加 `CanDelete` 方法：

```go
// CanDelete reports whether an agent in this status is eligible to be
// deleted. Only stable states are eligible; provisioning/starting are
// rejected to avoid races with RuntimeWorker. error is included to allow
// retries after a partially-completed deletion.
func (s Status) CanDelete() bool {
	switch s {
	case StatusCreating, StatusRunning, StatusStopped, StatusError:
		return true
	default:
		return false
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd services/api && go test ./internal/agents/ -run TestStatusCanDelete -v
```

预期 6 个子测试全部 PASS。再跑全包：

```bash
cd services/api && go test ./internal/agents/
```

预期 PASS。

- [ ] **Step 5: 提交**

```bash
git add services/api/internal/agents/model.go services/api/internal/agents/state_test.go
git commit -m "feat(agents): add Status.CanDelete and delete sentinel errors"
```

---

## Task 4: agents/errors.go 业务错误码常量

**Files:**
- Create: `services/api/internal/agents/errors.go`

**Interfaces:**
- Produces:
  - `const DeleteFailureInspect = "delete_inspect_failed"`
  - `const DeleteFailureStop    = "delete_stop_failed"`
  - `const DeleteFailureRemove  = "delete_remove_failed"`
  - `const DeleteFailureHome    = "delete_home_failed"`

- [ ] **Step 1: 创建 errors.go**

新建 `services/api/internal/agents/errors.go`：

```go
package agents

// Delete failure codes are written to agent.last_error_code when a
// deletion attempt fails at a specific stage. Frontend clients use these
// to surface targeted error messages and suggest retries.
const (
	DeleteFailureInspect = "delete_inspect_failed"
	DeleteFailureStop    = "delete_stop_failed"
	DeleteFailureRemove  = "delete_remove_failed"
	DeleteFailureHome    = "delete_home_failed"
)
```

注意：这些是字符串常量（DB 字段值），不是 Go error 值，故无 `Err` 前缀。

- [ ] **Step 2: 运行编译确认无语法错误**

```bash
cd services/api && go build ./internal/agents/
```

预期无输出（成功）。无独立测试，常量在后续 Task 5/6 间接被使用。

- [ ] **Step 3: 提交**

```bash
git add services/api/internal/agents/errors.go
git commit -m "feat(agents): add delete failure code constants"
```

---

## Task 5: Repository.Delete + Repository.MarkDeleteFailed

**Files:**
- Modify: `services/api/internal/agents/repository.go`
- Test: `services/api/internal/agents/repository_delete_test.go` (new)

**Interfaces:**
- Consumes: `requireAffected`（已存在于 repository.go），`agents.ErrNotFound`（来自 Task 3）
- Produces:
  - `func (r *Repository) Delete(ctx context.Context, id string) error` — 物理删除 agents 行，行不存在返回 `ErrNotFound`；外键 CASCADE 自动清理子表
  - `func (r *Repository) MarkDeleteFailed(ctx context.Context, id, code, msg string) error` — 绕过状态机直接将 status 置为 `error`，写 `last_error_code`、`last_error_message`；行不存在返回 `ErrNotFound`

- [ ] **Step 1: 创建 repository_delete_test.go 写测试**

新建 `services/api/internal/agents/repository_delete_test.go`：

```go
package agents

import (
	"context"
	"errors"
	"testing"

	_ "modernc.org/sqlite"
)

func TestRepositoryDeleteRemovesRow(t *testing.T) {
	database := newAgentsTestDB(t)
	repository := NewRepository(database)
	ctx := context.Background()
	insertAgentFixture(t, database, "agent-1", "user-1", StatusRunning)

	if err := repository.Delete(ctx, "agent-1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	if _, err := repository.Get(ctx, "agent-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after Delete err = %v, want ErrNotFound", err)
	}
}

func TestRepositoryDeleteCascadesRuntimeJobs(t *testing.T) {
	database := newAgentsTestDB(t)
	repository := NewRepository(database)
	ctx := context.Background()
	insertAgentFixture(t, database, "agent-1", "user-1", StatusRunning)
	if _, err := database.ExecContext(ctx, `
		INSERT INTO runtime_jobs (id, agent_id, type, status)
		VALUES ('job-1', 'agent-1', 'restart_runtime', 'succeeded');
	`); err != nil {
		t.Fatalf("seed runtime job: %v", err)
	}

	if err := repository.Delete(ctx, "agent-1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	var count int
	if err := database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM runtime_jobs WHERE agent_id = ?;`, "agent-1").Scan(&count); err != nil {
		t.Fatalf("count runtime_jobs: %v", err)
	}
	if count != 0 {
		t.Fatalf("runtime_jobs count after Delete = %d, want 0", count)
	}
}

func TestRepositoryDeleteReturnsNotFoundForMissingAgent(t *testing.T) {
	database := newAgentsTestDB(t)
	repository := NewRepository(database)
	ctx := context.Background()

	if err := repository.Delete(ctx, "agent-missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete err = %v, want ErrNotFound", err)
	}
}

func TestRepositoryMarkDeleteFailedWritesErrorState(t *testing.T) {
	database := newAgentsTestDB(t)
	repository := NewRepository(database)
	ctx := context.Background()
	insertAgentFixture(t, database, "agent-1", "user-1", StatusStopped)

	if err := repository.MarkDeleteFailed(ctx, "agent-1", DeleteFailureStop, "docker stop failed: timeout"); err != nil {
		t.Fatalf("MarkDeleteFailed returned error: %v", err)
	}

	agent, err := repository.Get(ctx, "agent-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if agent.Status != StatusError {
		t.Fatalf("agent.Status = %s, want %s", agent.Status, StatusError)
	}
	if agent.LastErrorCode != DeleteFailureStop {
		t.Fatalf("agent.LastErrorCode = %q, want %q", agent.LastErrorCode, DeleteFailureStop)
	}
	if agent.LastErrorMessage != "docker stop failed: timeout" {
		t.Fatalf("agent.LastErrorMessage = %q, unexpected", agent.LastErrorMessage)
	}
}

func TestRepositoryMarkDeleteFailedReturnsNotFound(t *testing.T) {
	database := newAgentsTestDB(t)
	repository := NewRepository(database)
	ctx := context.Background()

	err := repository.MarkDeleteFailed(ctx, "agent-missing", DeleteFailureStop, "x")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("MarkDeleteFailed err = %v, want ErrNotFound", err)
	}
}

func TestRepositoryMarkDeleteFailedBypassesStateMachine(t *testing.T) {
	// stopped -> error is NOT in transitions table; MarkDeleteFailed must
	// still succeed because deletion failures bypass the normal state machine.
	database := newAgentsTestDB(t)
	repository := NewRepository(database)
	ctx := context.Background()
	insertAgentFixture(t, database, "agent-1", "user-1", StatusStopped)

	if err := repository.MarkDeleteFailed(ctx, "agent-1", DeleteFailureHome, "rm failed"); err != nil {
		t.Fatalf("MarkDeleteFailed should bypass state machine, got err: %v", err)
	}
	agent, _ := repository.Get(ctx, "agent-1")
	if agent.Status != StatusError {
		t.Fatalf("status = %s, want error", agent.Status)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd services/api && go test ./internal/agents/ -run "TestRepositoryDelete|TestRepositoryMarkDeleteFailed" -v
```

预期所有用例 FAIL：`undefined: Repository.Delete` 和 `Repository.MarkDeleteFailed`。

- [ ] **Step 3: 在 repository.go 末尾追加 Delete 与 MarkDeleteFailed**

打开 `services/api/internal/agents/repository.go`，在 `HasAgentsForTemplate` 之后（即文件末尾）追加：

```go
// Delete physically removes the agents row by id. Foreign-key CASCADE on
// agent_runtime_events, agent_channels, runtime_jobs cleans up children.
// Returns ErrNotFound if no row was affected.
func (r *Repository) Delete(ctx context.Context, id string) error {
	result, err := r.database.ExecContext(ctx,
		`DELETE FROM agents WHERE id = ?;`, id)
	if err != nil {
		return fmt.Errorf("delete agent row: %w", err)
	}
	return requireAffected(result)
}

// MarkDeleteFailed forces the agent into the 'error' state and records
// the failure code and message. It bypasses the state-machine transitions
// table because some legitimate sources (e.g. stopped) cannot otherwise
// reach error. Returns ErrNotFound if no row was affected.
func (r *Repository) MarkDeleteFailed(ctx context.Context, id, code, message string) error {
	result, err := r.database.ExecContext(ctx, `
		UPDATE agents
		SET status = ?,
		    last_error_code = ?,
		    last_error_message = ?,
		    updated_at = datetime('now')
		WHERE id = ?;
	`, StatusError, code, message, id)
	if err != nil {
		return fmt.Errorf("update agent to error: %w", err)
	}
	return requireAffected(result)
}
```

注意：`repository.go` 顶部当前 import 不包含 `"fmt"`。需要在顶部 import 块加 `"fmt"`：

```go
import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd services/api && go test ./internal/agents/ -run "TestRepositoryDelete|TestRepositoryMarkDeleteFailed" -v
```

预期所有用例 PASS。再跑全包：

```bash
cd services/api && go test ./internal/agents/
```

预期 PASS。

- [ ] **Step 5: 提交**

```bash
git add services/api/internal/agents/repository.go services/api/internal/agents/repository_delete_test.go
git commit -m "feat(agents): add Repository.Delete and MarkDeleteFailed"
```

---

## Task 6: RuntimeRepository.HasUnfinishedByAgent

**Files:**
- Modify: `services/api/internal/jobs/runtime_repository.go`
- Test: `services/api/internal/jobs/runtime_repository_unfinished_test.go` (new)

**Interfaces:**
- Produces: `func (r *RuntimeRepository) HasUnfinishedByAgent(ctx context.Context, agentID string) (bool, error)` — 当存在 status 为 `queued` 或 `running` 的 runtime_job 时返回 true

- [ ] **Step 1: 查看 runtime_repository_test.go 的测试 helpers**

```bash
grep -n "func newRuntimeRepoTestDB\|func insertRuntimeJob\|func newJobsTestDB" /Users/zhengwenfeng/work/projs/AgentForge/services/api/internal/jobs/runtime_repository_test.go | head -10
```

记录 helper 函数名，后面测试要用。如果 helper 名为 `newJobsTestDB`，则下方测试代码引用对应名字。

- [ ] **Step 2: 创建 runtime_repository_unfinished_test.go 写测试**

新建 `services/api/internal/jobs/runtime_repository_unfinished_test.go`。先用以下模板，**根据 Step 1 查到的 helper 名替换** `newJobsTestDB`：

```go
package jobs

import (
	"context"
	"testing"

	_ "modernc.org/sqlite"
)

func TestHasUnfinishedByAgentReturnsTrueForQueued(t *testing.T) {
	database := newJobsTestDB(t)
	repo := NewRuntimeRepository(database)
	ctx := context.Background()

	if _, err := database.ExecContext(ctx, `
		INSERT INTO runtime_jobs (id, agent_id, type, status)
		VALUES ('job-1', 'agent-1', 'provision_agent', 'queued');
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	has, err := repo.HasUnfinishedByAgent(ctx, "agent-1")
	if err != nil {
		t.Fatalf("HasUnfinishedByAgent: %v", err)
	}
	if !has {
		t.Fatal("expected has=true for queued job, got false")
	}
}

func TestHasUnfinishedByAgentReturnsTrueForRunning(t *testing.T) {
	database := newJobsTestDB(t)
	repo := NewRuntimeRepository(database)
	ctx := context.Background()

	if _, err := database.ExecContext(ctx, `
		INSERT INTO runtime_jobs (id, agent_id, type, status)
		VALUES ('job-1', 'agent-1', 'restart_runtime', 'running');
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	has, err := repo.HasUnfinishedByAgent(ctx, "agent-1")
	if err != nil {
		t.Fatalf("HasUnfinishedByAgent: %v", err)
	}
	if !has {
		t.Fatal("expected has=true for running job, got false")
	}
}

func TestHasUnfinishedByAgentReturnsFalseForFinishedJobs(t *testing.T) {
	database := newJobsTestDB(t)
	repo := NewRuntimeRepository(database)
	ctx := context.Background()

	if _, err := database.ExecContext(ctx, `
		INSERT INTO runtime_jobs (id, agent_id, type, status) VALUES
			('job-1', 'agent-1', 'provision_agent', 'succeeded'),
			('job-2', 'agent-1', 'restart_runtime', 'failed'),
			('job-3', 'agent-1', 'restart_runtime', 'cancelled');
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	has, err := repo.HasUnfinishedByAgent(ctx, "agent-1")
	if err != nil {
		t.Fatalf("HasUnfinishedByAgent: %v", err)
	}
	if has {
		t.Fatal("expected has=false when only finished jobs exist")
	}
}

func TestHasUnfinishedByAgentScopesByAgentID(t *testing.T) {
	database := newJobsTestDB(t)
	repo := NewRuntimeRepository(database)
	ctx := context.Background()

	if _, err := database.ExecContext(ctx, `
		INSERT INTO runtime_jobs (id, agent_id, type, status)
		VALUES ('job-1', 'agent-other', 'provision_agent', 'queued');
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	has, err := repo.HasUnfinishedByAgent(ctx, "agent-1")
	if err != nil {
		t.Fatalf("HasUnfinishedByAgent: %v", err)
	}
	if has {
		t.Fatal("expected has=false when only other agent's jobs are queued")
	}
}
```

注意：如果现有 `runtime_repository_test.go` 中的 fixture 因为 `idx_runtime_jobs_one_active` 唯一约束（每 agent 仅一个 queued/running）拒绝插入两条 queued 行的并发测试，注意不要在同一 agent 同时 seed 两条未完成 job。

- [ ] **Step 3: 运行测试确认失败**

```bash
cd services/api && go test ./internal/jobs/ -run TestHasUnfinishedByAgent -v
```

预期 FAIL：`undefined: HasUnfinishedByAgent`。

- [ ] **Step 4: 在 runtime_repository.go 中追加方法**

打开 `services/api/internal/jobs/runtime_repository.go`，在 `MarkFailed`（约 :183）方法之后（或文件中已有方法之间合适位置）追加：

```go
// HasUnfinishedByAgent reports whether the agent has any runtime job that
// is still queued or running (i.e. not in a terminal state).
func (r *RuntimeRepository) HasUnfinishedByAgent(ctx context.Context, agentID string) (bool, error) {
	var count int
	err := r.database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM runtime_jobs
		WHERE agent_id = ? AND status IN (?, ?);
	`, agentID, StatusQueued, StatusRunning).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("count unfinished runtime jobs: %w", err)
	}
	return count > 0, nil
}
```

`runtime_repository.go` 顶部 import 已包含 `"fmt"`，无需添加。

- [ ] **Step 5: 运行测试确认通过**

```bash
cd services/api && go test ./internal/jobs/ -run TestHasUnfinishedByAgent -v
```

预期 4 个用例全部 PASS。再跑全包：

```bash
cd services/api && go test ./internal/jobs/
```

预期 PASS。

- [ ] **Step 6: 提交**

```bash
git add services/api/internal/jobs/runtime_repository.go services/api/internal/jobs/runtime_repository_unfinished_test.go
git commit -m "feat(jobs): add HasUnfinishedByAgent on runtime repository"
```

---

## Task 7: agents.Service.Delete + 注入 runtime.Runner

**Files:**
- Modify: `services/api/internal/agents/service.go`
- Test: `services/api/internal/agents/service_delete_test.go` (new)

**Interfaces:**
- Consumes:
  - `runtime.Runner`（来自 `internal/runtime/docker.go`，`Inspect`/`Stop`/`Remove` 已存在）
  - `runtime.DefaultContainerName(agentID) string`（已存在）
  - `runtime.ErrContainerNotFound`（已存在 + Task 2 增强）
  - `runtime.DestroyHome(homePath) error`（Task 1 新增）
  - `Repository.Get`/`Repository.Delete`/`Repository.MarkDeleteFailed`（Task 5 新增）
  - `RuntimeRepository.HasUnfinishedByAgent`（Task 6 新增）
  - `Status.CanDelete()`（Task 3 新增）
  - `ErrCannotDelete`、`ErrHasUnfinishedJobs`（Task 3 新增）
  - `DeleteFailureInspect`/`DeleteFailureStop`/`DeleteFailureRemove`/`DeleteFailureHome`（Task 4 新增）
- Produces:
  - `func NewService(database *sql.DB, repository *Repository, runtimeJobs *jobs.RuntimeRepository, runner runtime.Runner, dataDir string) *Service` — **签名新增 `runner` 参数**
  - `func (s *Service) Delete(ctx context.Context, agentID string) error`

- [ ] **Step 1: 创建 service_delete_test.go 写测试 — fakeRunner 与基础用例**

新建 `services/api/internal/agents/service_delete_test.go`：

```go
package agents

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"agentforge.local/services/api/internal/jobs"
	"agentforge.local/services/api/internal/runtime"

	_ "modernc.org/sqlite"
)

// fakeRunner is a controllable mock of runtime.Runner.
type fakeRunner struct {
	mu             sync.Mutex
	inspectStatus  runtime.ContainerStatus
	inspectErr     error
	stopErr        error
	removeErr      error
	stopCalls      int
	removeCalls    int
	inspectCalls   int
	// programmable: returned errors can be lists where each call pops one.
	stopErrSeq   []error
	removeErrSeq []error
}

func (r *fakeRunner) EnsureRunning(ctx context.Context, spec runtime.ContainerSpec) error {
	return errors.New("not implemented in fakeRunner")
}

func (r *fakeRunner) Inspect(ctx context.Context, containerName string) (runtime.ContainerStatus, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.inspectCalls++
	return r.inspectStatus, r.inspectErr
}

func (r *fakeRunner) Stop(ctx context.Context, containerName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopCalls++
	if len(r.stopErrSeq) > 0 {
		err := r.stopErrSeq[0]
		r.stopErrSeq = r.stopErrSeq[1:]
		return err
	}
	return r.stopErr
}

func (r *fakeRunner) Remove(ctx context.Context, containerName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removeCalls++
	if len(r.removeErrSeq) > 0 {
		err := r.removeErrSeq[0]
		r.removeErrSeq = r.removeErrSeq[1:]
		return err
	}
	return r.removeErr
}

// newServiceForDelete builds a Service against an in-memory DB pre-seeded
// with one agent in the requested state. dataDir is set to a temp dir so
// the agent's hermes_home_path resolves under it.
func newServiceForDelete(t *testing.T, agentID string, status Status, runner runtime.Runner) (*Service, *sql.DB, string) {
	t.Helper()
	database := newAgentsTestDB(t)
	repository := NewRepository(database)
	dataDir := t.TempDir()
	homePath := filepath.Join(dataDir, "agents", agentID, "hermes-home")
	insertAgentFixtureWithHome(t, database, agentID, "user-1", status, homePath)
	runtimeJobs := jobs.NewRuntimeRepository(database)
	svc := NewService(database, repository, runtimeJobs, runner, dataDir)
	return svc, database, homePath
}

// insertAgentFixtureWithHome is a helper similar to insertAgentFixture but
// allows setting hermes_home_path explicitly.
func insertAgentFixtureWithHome(t *testing.T, database *sql.DB, agentID, ownerUserID string, status Status, homePath string) {
	t.Helper()
	if _, err := database.ExecContext(context.Background(), `
		INSERT INTO agents (id, owner_user_id, template_id, template_version, name, status, hermes_home_path)
		VALUES (?, ?, 'template-1', 3, 'Test Agent', ?, ?);
	`, agentID, ownerUserID, status, homePath); err != nil {
		t.Fatalf("insert agent fixture: %v", err)
	}
}

func TestDeleteRefusesProvisioning(t *testing.T) {
	runner := &fakeRunner{inspectErr: runtime.ErrContainerNotFound}
	svc, _, _ := newServiceForDelete(t, "agent-1", StatusProvisioning, runner)

	err := svc.Delete(context.Background(), "agent-1")
	if !errors.Is(err, ErrCannotDelete) {
		t.Fatalf("Delete err = %v, want ErrCannotDelete", err)
	}
	if runner.stopCalls != 0 || runner.removeCalls != 0 {
		t.Fatalf("runner should not be called when refused; stop=%d remove=%d", runner.stopCalls, runner.removeCalls)
	}
}

func TestDeleteRefusesStarting(t *testing.T) {
	runner := &fakeRunner{}
	svc, _, _ := newServiceForDelete(t, "agent-1", StatusStarting, runner)

	err := svc.Delete(context.Background(), "agent-1")
	if !errors.Is(err, ErrCannotDelete) {
		t.Fatalf("Delete err = %v, want ErrCannotDelete", err)
	}
}

func TestDeleteRefusesWhenUnfinishedJobExists(t *testing.T) {
	runner := &fakeRunner{}
	svc, db, _ := newServiceForDelete(t, "agent-1", StatusRunning, runner)
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO runtime_jobs (id, agent_id, type, status)
		VALUES ('job-1', 'agent-1', 'restart_runtime', 'running');
	`); err != nil {
		t.Fatalf("seed running job: %v", err)
	}

	err := svc.Delete(context.Background(), "agent-1")
	if !errors.Is(err, ErrHasUnfinishedJobs) {
		t.Fatalf("Delete err = %v, want ErrHasUnfinishedJobs", err)
	}
}

func TestDeleteReturnsNotFoundForMissingAgent(t *testing.T) {
	runner := &fakeRunner{}
	database := newAgentsTestDB(t)
	repository := NewRepository(database)
	runtimeJobs := jobs.NewRuntimeRepository(database)
	svc := NewService(database, repository, runtimeJobs, runner, t.TempDir())

	err := svc.Delete(context.Background(), "agent-missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete err = %v, want ErrNotFound", err)
	}
}
```

注意：测试文件需 `import "database/sql"`（已经被 `*sql.DB` 引入会被 Go 自动检测）。如果保存时编辑器没自动加入 `database/sql`，手动加上。

- [ ] **Step 2: 运行测试确认失败**

```bash
cd services/api && go test ./internal/agents/ -run TestDelete -v
```

预期编译失败：`NewService` 签名只接受 4 个参数，传入 5 个参数报错；`svc.Delete` 未定义。

- [ ] **Step 3: 修改 Service 结构与 NewService 签名**

打开 `services/api/internal/agents/service.go`：

(a) 顶部 import 增加：

```go
"agentforge.local/services/api/internal/runtime"
```

(b) 修改 `Service` 结构与 `NewService`（约第 14-28 行）：

```go
type Service struct {
	database    *sql.DB
	repository  *Repository
	runtimeJobs *jobs.RuntimeRepository
	runner      runtime.Runner
	dataDir     string
}

func NewService(database *sql.DB, repository *Repository, runtimeJobs *jobs.RuntimeRepository, runner runtime.Runner, dataDir string) *Service {
	return &Service{
		database:    database,
		repository:  repository,
		runtimeJobs: runtimeJobs,
		runner:      runner,
		dataDir:     dataDir,
	}
}
```

(c) 在文件末尾追加 `Delete` 与辅助 `failWith`：

```go
// Delete cleans up an agent's container, hermes-home directory, and
// database row in that order. Each external side-effect stage is
// idempotent, so a partially-completed deletion can be retried safely
// (the agent will be in StatusError, which CanDelete allows).
//
// This method follows the single-handling rule: it never logs; the HTTP
// handler is the sole logging point.
func (s *Service) Delete(ctx context.Context, agentID string) error {
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

	containerName := runtime.DefaultContainerName(agentID)
	status, inspectErr := s.runner.Inspect(ctx, containerName)
	if inspectErr != nil && !errors.Is(inspectErr, runtime.ErrContainerNotFound) {
		return s.failWith(ctx, agentID, DeleteFailureInspect,
			fmt.Errorf("inspect container: %w", inspectErr))
	}
	if inspectErr == nil {
		if status.Running {
			if err := s.runner.Stop(ctx, containerName); err != nil {
				return s.failWith(ctx, agentID, DeleteFailureStop,
					fmt.Errorf("stop container: %w", err))
			}
		}
		if err := s.runner.Remove(ctx, containerName); err != nil {
			if !errors.Is(err, runtime.ErrContainerNotFound) {
				return s.failWith(ctx, agentID, DeleteFailureRemove,
					fmt.Errorf("remove container: %w", err))
			}
		}
	}

	if err := runtime.DestroyHome(agent.HermesHomePath); err != nil {
		return s.failWith(ctx, agentID, DeleteFailureHome,
			fmt.Errorf("destroy hermes home: %w", err))
	}

	if err := s.repository.Delete(ctx, agentID); err != nil {
		return fmt.Errorf("delete agent from database: %w", err)
	}
	return nil
}

// failWith records the deletion failure on the agent row and returns the
// original error. If recording itself fails, both errors are joined so
// neither is lost.
func (s *Service) failWith(ctx context.Context, agentID, code string, original error) error {
	msg := original.Error()
	if markErr := s.repository.MarkDeleteFailed(ctx, agentID, code, msg); markErr != nil {
		return errors.Join(original, fmt.Errorf("mark agent delete failed: %w", markErr))
	}
	return original
}
```

(d) 在 import 块加入 `"errors"`（如果尚未引入）。`service.go` 当前 import 为：

```go
import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	"agentforge.local/services/api/internal/jobs"
	"github.com/google/uuid"
)
```

修改为：

```go
import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"agentforge.local/services/api/internal/jobs"
	"agentforge.local/services/api/internal/runtime"
	"github.com/google/uuid"
)
```

- [ ] **Step 4: 修改 main.go 调用点**

打开 `services/api/cmd/agentforge-api/main.go`，找到第 78 行：

```go
agentService := agents.NewService(database, agentRepo, runtimeJobs, cfg.DataDir)
```

第 82 行 `runner := runtime.NewDockerRunner(cfg.DockerBin)` 在第 78 行之后。需要把 `runner` 的初始化**提前**到第 78 行之前，并传入 `agents.NewService`：

```go
runtimeJobs := jobs.NewRuntimeRepository(database)
agentRepo := agents.NewRepository(database)
templateService := templates.NewService(templateRepo, templateStore, agentRepo)
runner := runtime.NewDockerRunner(cfg.DockerBin)
agentService := agents.NewService(database, agentRepo, runtimeJobs, runner, cfg.DataDir)
channelRepo := channels.NewRepository(database)
channelService := channels.NewService(database, channelRepo)
channelJobs := jobs.NewChannelRepository(database)
```

注意：`runner` 之前是在 line 82 创建，被 `runtimeWorker` 使用。提前到 line 78 之前不影响后续 `runtimeWorker` 引用——后面的 `Runner: runner` 引用同一个变量。删除原 line 82 的 `runner := runtime.NewDockerRunner(cfg.DockerBin)` 重复定义。

- [ ] **Step 5: 运行测试确认基础用例通过 + 编译通过**

```bash
cd services/api && go build ./...
```

预期无输出（成功）。然后：

```bash
cd services/api && go test ./internal/agents/ -run TestDelete -v
```

预期 `TestDeleteRefusesProvisioning`/`TestDeleteRefusesStarting`/`TestDeleteRefusesWhenUnfinishedJobExists`/`TestDeleteReturnsNotFoundForMissingAgent` 全部 PASS。

- [ ] **Step 6: 追加成功路径测试用例**

继续在 `service_delete_test.go` 末尾追加：

```go
func TestDeleteSucceedsForRunningAgent(t *testing.T) {
	runner := &fakeRunner{
		inspectStatus: runtime.ContainerStatus{Exists: true, Running: true},
	}
	svc, db, homePath := newServiceForDelete(t, "agent-1", StatusRunning, runner)
	if err := os.MkdirAll(homePath, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	if err := os.WriteFile(filepath.Join(homePath, "USER.md"), []byte("payload"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	if err := svc.Delete(context.Background(), "agent-1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	if runner.stopCalls != 1 {
		t.Errorf("stop calls = %d, want 1", runner.stopCalls)
	}
	if runner.removeCalls != 1 {
		t.Errorf("remove calls = %d, want 1", runner.removeCalls)
	}
	if _, err := os.Stat(homePath); !os.IsNotExist(err) {
		t.Errorf("hermes home should be removed, stat err = %v", err)
	}
	var count int
	if err := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM agents WHERE id = ?;`, "agent-1").Scan(&count); err != nil {
		t.Fatalf("count agents: %v", err)
	}
	if count != 0 {
		t.Errorf("agents count after Delete = %d, want 0", count)
	}
}

func TestDeleteSkipsStopForStoppedContainer(t *testing.T) {
	runner := &fakeRunner{
		inspectStatus: runtime.ContainerStatus{Exists: true, Running: false},
	}
	svc, _, _ := newServiceForDelete(t, "agent-1", StatusStopped, runner)

	if err := svc.Delete(context.Background(), "agent-1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if runner.stopCalls != 0 {
		t.Errorf("stop should not be called when container is not running, got %d calls", runner.stopCalls)
	}
	if runner.removeCalls != 1 {
		t.Errorf("remove calls = %d, want 1", runner.removeCalls)
	}
}

func TestDeleteSucceedsWhenContainerAlreadyMissing(t *testing.T) {
	runner := &fakeRunner{inspectErr: runtime.ErrContainerNotFound}
	svc, _, _ := newServiceForDelete(t, "agent-1", StatusError, runner)

	if err := svc.Delete(context.Background(), "agent-1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if runner.stopCalls != 0 || runner.removeCalls != 0 {
		t.Errorf("expected no stop/remove calls; got stop=%d remove=%d", runner.stopCalls, runner.removeCalls)
	}
}

func TestDeleteToleratesRemoveRace(t *testing.T) {
	runner := &fakeRunner{
		inspectStatus: runtime.ContainerStatus{Exists: true, Running: false},
		removeErr:     runtime.ErrContainerNotFound,
	}
	svc, _, _ := newServiceForDelete(t, "agent-1", StatusRunning, runner)

	if err := svc.Delete(context.Background(), "agent-1"); err != nil {
		t.Fatalf("Delete should swallow concurrent remove race, got: %v", err)
	}
}

func TestDeleteSucceedsForCreatingAgent(t *testing.T) {
	runner := &fakeRunner{inspectErr: runtime.ErrContainerNotFound}
	svc, _, _ := newServiceForDelete(t, "agent-1", StatusCreating, runner)

	if err := svc.Delete(context.Background(), "agent-1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
}

func TestDeleteSucceedsWhenHomeDirectoryAlreadyMissing(t *testing.T) {
	runner := &fakeRunner{inspectErr: runtime.ErrContainerNotFound}
	svc, _, homePath := newServiceForDelete(t, "agent-1", StatusError, runner)

	if _, err := os.Stat(homePath); !os.IsNotExist(err) {
		t.Fatalf("expected home dir absent before test, err=%v", err)
	}

	if err := svc.Delete(context.Background(), "agent-1"); err != nil {
		t.Fatalf("Delete with missing home returned error: %v", err)
	}
}
```

- [ ] **Step 7: 运行测试确认成功路径通过**

```bash
cd services/api && go test ./internal/agents/ -run TestDelete -v
```

预期所有 TestDelete* 用例 PASS。

- [ ] **Step 8: 追加失败路径测试用例**

继续在 `service_delete_test.go` 末尾追加：

```go
func TestDeleteFailsAndMarksErrorWhenInspectFails(t *testing.T) {
	runner := &fakeRunner{inspectErr: errors.New("docker inspect failed: bad output")}
	svc, _, _ := newServiceForDelete(t, "agent-1", StatusRunning, runner)
	repository := NewRepository(svc.database)

	err := svc.Delete(context.Background(), "agent-1")
	if err == nil {
		t.Fatal("Delete should return error when Inspect fails")
	}
	agent, getErr := repository.Get(context.Background(), "agent-1")
	if getErr != nil {
		t.Fatalf("Get after failed delete: %v", getErr)
	}
	if agent.Status != StatusError {
		t.Errorf("agent status = %s, want error", agent.Status)
	}
	if agent.LastErrorCode != DeleteFailureInspect {
		t.Errorf("last_error_code = %q, want %q", agent.LastErrorCode, DeleteFailureInspect)
	}
}

func TestDeleteFailsAndMarksErrorWhenStopFails(t *testing.T) {
	runner := &fakeRunner{
		inspectStatus: runtime.ContainerStatus{Exists: true, Running: true},
		stopErr:       errors.New("docker stop failed: signal denied"),
	}
	svc, _, _ := newServiceForDelete(t, "agent-1", StatusRunning, runner)
	repository := NewRepository(svc.database)

	if err := svc.Delete(context.Background(), "agent-1"); err == nil {
		t.Fatal("Delete should fail when Stop fails")
	}
	agent, _ := repository.Get(context.Background(), "agent-1")
	if agent.Status != StatusError || agent.LastErrorCode != DeleteFailureStop {
		t.Errorf("agent state = (%s, %q), want (error, %s)", agent.Status, agent.LastErrorCode, DeleteFailureStop)
	}
}

func TestDeleteFailsAndMarksErrorWhenRemoveFailsNonNotFound(t *testing.T) {
	runner := &fakeRunner{
		inspectStatus: runtime.ContainerStatus{Exists: true, Running: false},
		removeErr:     errors.New("docker rm failed: permission denied"),
	}
	svc, _, _ := newServiceForDelete(t, "agent-1", StatusStopped, runner)
	repository := NewRepository(svc.database)

	if err := svc.Delete(context.Background(), "agent-1"); err == nil {
		t.Fatal("Delete should fail when Remove fails")
	}
	agent, _ := repository.Get(context.Background(), "agent-1")
	if agent.LastErrorCode != DeleteFailureRemove {
		t.Errorf("last_error_code = %q, want %q", agent.LastErrorCode, DeleteFailureRemove)
	}
}

func TestDeleteRetriesAfterStopFailure(t *testing.T) {
	// First call: Stop fails. Second call: container already gone, Inspect
	// returns NotFound and Delete completes the cleanup.
	runner := &fakeRunner{
		inspectStatus: runtime.ContainerStatus{Exists: true, Running: true},
		stopErr:       errors.New("transient stop failure"),
	}
	svc, _, _ := newServiceForDelete(t, "agent-1", StatusRunning, runner)

	if err := svc.Delete(context.Background(), "agent-1"); err == nil {
		t.Fatal("first Delete should fail")
	}

	// Simulate operator (or natural state) cleaning the container.
	runner.mu.Lock()
	runner.inspectErr = runtime.ErrContainerNotFound
	runner.inspectStatus = runtime.ContainerStatus{}
	runner.stopErr = nil
	runner.mu.Unlock()

	if err := svc.Delete(context.Background(), "agent-1"); err != nil {
		t.Fatalf("retry Delete returned error: %v", err)
	}
}
```

- [ ] **Step 9: 运行全部测试确认通过**

```bash
cd services/api && go test ./internal/agents/ -v
```

预期所有用例 PASS。再跑全仓：

```bash
cd services/api && go test ./...
```

预期全部 PASS。如果其他包对 `agents.NewService` 的调用因签名改变而编译失败，需要逐个修正：

```bash
cd services/api && grep -rn "agents.NewService" --include="*.go"
```

把每处调用更新为新的 5 参数签名（增加 `runner` 参数）。如有测试中以 mock/nil 形式调用 NewService 的，传入 `nil` 作为 `runner` 即可（不调用 Delete 时不会被解引用）。

- [ ] **Step 10: 提交**

```bash
git add services/api/internal/agents/service.go \
        services/api/internal/agents/service_delete_test.go \
        services/api/cmd/agentforge-api/main.go
git commit -m "feat(agents): add Service.Delete with sequential resource cleanup"
```

---

## Task 8: writeAgentError 扩展映射

**Files:**
- Modify: `services/api/internal/http/errors.go:199-212`

**Interfaces:**
- Modifies: `writeAgentError` 在 `errors.Is(err, agents.ErrCannotDelete)` 或 `errors.Is(err, agents.ErrHasUnfinishedJobs)` 时返回 409 + 对应 error code

- [ ] **Step 1: 编辑 errors.go**

打开 `services/api/internal/http/errors.go`，找到 `writeAgentError`（约 :199）。在 `case errors.Is(err, agents.ErrConflict):` 之后、`case errors.Is(err, agents.ErrInvalidInput)` 之前插入两个新分支：

修改后的 `writeAgentError` 完整形态：

```go
func writeAgentError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, agents.ErrNotFound), errors.Is(err, agents.ErrTemplateNotFound):
		writeAPIError(c, http.StatusNotFound, "not_found", err.Error(), errWithRequest(c, err))
	case errors.Is(err, agents.ErrConflict):
		writeAPIError(c, http.StatusConflict, "conflict", err.Error(), errWithRequest(c, err))
	case errors.Is(err, agents.ErrCannotDelete):
		writeAPIError(c, http.StatusConflict, "agent_cannot_delete", err.Error(), errWithRequest(c, err))
	case errors.Is(err, agents.ErrHasUnfinishedJobs):
		writeAPIError(c, http.StatusConflict, "agent_has_unfinished_jobs", err.Error(), errWithRequest(c, err))
	case errors.Is(err, agents.ErrInvalidInput), errors.Is(err, agents.ErrInvalidStateTransition):
		writeAPIError(c, http.StatusBadRequest, "invalid_request", err.Error(), errWithRequest(c, err))
	case errors.Is(err, agents.ErrRuntimeUnavailable):
		writeAPIError(c, http.StatusConflict, "runtime_unavailable", err.Error(), errWithRequest(c, err))
	default:
		writeInternalError(c, http.StatusInternalServerError, "internal_error", "", err)
	}
}
```

- [ ] **Step 2: 验证编译**

```bash
cd services/api && go build ./internal/http/
```

预期无输出。

- [ ] **Step 3: 提交**

```bash
git add services/api/internal/http/errors.go
git commit -m "feat(http): map agent delete sentinel errors to 409"
```

---

## Task 9: AgentHandlers.Delete + 路由注册

**Files:**
- Modify: `services/api/internal/http/agent_handlers.go`

**Interfaces:**
- Consumes: `agents.Service.Delete`（Task 7）、`writeAgentError`（Task 8 扩展）、`authorizeAgent`/`requireAuthenticatedUser`（已有）
- Produces: `DELETE /agents/:id` 路由 + `(h *AgentHandlers) Delete(c *gin.Context)` handler

- [ ] **Step 1: 修改 Register 注册新路由**

打开 `services/api/internal/http/agent_handlers.go`，找到 `Register` 方法（约 :21-29），在末尾添加 `router.DELETE("/agents/:id", h.Delete)`：

```go
func (h *AgentHandlers) Register(router gin.IRoutes) {
	router.POST("/agents", h.Create)
	router.GET("/agents", h.List)
	router.GET("/agents/:id", h.Get)
	router.GET("/agents/:id/runtime", h.GetRuntime)
	router.GET("/agents/:id/runtime-jobs", h.ListRuntimeJobs)
	router.POST("/agents/:id/runtime-jobs", h.CreateRuntimeJob)
	router.GET("/agents/:id/runtime-jobs/:jobId", h.GetRuntimeJob)
	router.DELETE("/agents/:id", h.Delete)
}
```

- [ ] **Step 2: 在 agent_handlers.go 添加 Delete handler**

在 `GetRuntimeJob`（约 :138-149）之后、`type agentResponse` 声明（约 :151）之前插入：

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

- [ ] **Step 3: 添加 slog import**

`agent_handlers.go` 顶部 import 当前为：

```go
import (
	"net/http"

	"agentforge.local/services/api/internal/agents"
	"agentforge.local/services/api/internal/auth"
	"agentforge.local/services/api/internal/jobs"
	"github.com/gin-gonic/gin"
)
```

修改为：

```go
import (
	"log/slog"
	"net/http"

	"agentforge.local/services/api/internal/agents"
	"agentforge.local/services/api/internal/auth"
	"agentforge.local/services/api/internal/jobs"
	"github.com/gin-gonic/gin"
)
```

- [ ] **Step 4: 验证编译**

```bash
cd services/api && go build ./...
```

预期无输出。

- [ ] **Step 5: 跑包内全部测试**

```bash
cd services/api && go test ./internal/http/
```

预期 PASS（现有测试不应被破坏）。

- [ ] **Step 6: 提交**

```bash
git add services/api/internal/http/agent_handlers.go
git commit -m "feat(http): add DELETE /agents/:id handler"
```

---

## Task 10: 集成测试 + API 文档

**Files:**
- Create: `services/api/tests/agent_delete_integration_test.go`
- Modify: `docs/api.md`

**Interfaces:**
- 端到端验证：HTTP DELETE → DB CASCADE → 文件系统清理 → mock runner 调用

- [ ] **Step 1: 检查现有集成测试 helpers**

```bash
grep -n "func setupTestServer\|func newTestApp\|func loginAs\|httptest.NewServer" /Users/zhengwenfeng/work/projs/AgentForge/services/api/tests/*.go | head -20
```

记录现有的 helper 函数名（如 `newMVPTestEnv`、`loginAdmin` 等），下方测试代码中相应替换。

- [ ] **Step 2: 创建集成测试文件**

新建 `services/api/tests/agent_delete_integration_test.go`。**先复用与 `mvp_integration_test.go` 相同的 setup helpers**（同一 `tests` 包内可直接复用未导出函数）。

最简集成测试骨架（如下示例假定存在 `newMVPEnv(t)` 类似 helper 返回 `*httptest.Server` 与 `*sql.DB`；如名字不同则替换）：

```go
package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAgentDeleteHappyPath verifies the full delete flow:
// owner can delete a running agent, container Stop+Remove are invoked,
// hermes-home is wiped, DB row is gone, child runtime_jobs are cascaded.
func TestAgentDeleteHappyPath(t *testing.T) {
	env := newMVPEnv(t) // existing helper from mvp_integration_test.go
	defer env.Close()

	// 1. login as a regular user
	cookie := loginAs(t, env, "user@example.com", "user-password")

	// 2. create published template (admin) and agent (user)
	templateID := createPublishedTemplate(t, env)
	agentID := createAgent(t, env, cookie, templateID, "test-agent")

	// 3. Force agent into 'running' state and mark provision job succeeded.
	// State machine forbids creating -> running directly, so we bypass it.
	if _, err := env.DB.ExecContext(context.Background(), `
		UPDATE agents SET status = 'running' WHERE id = ?;
	`, agentID); err != nil {
		t.Fatalf("force running: %v", err)
	}
	if _, err := env.DB.ExecContext(context.Background(), `
		UPDATE runtime_jobs SET status = 'succeeded' WHERE agent_id = ?;
	`, agentID); err != nil {
		t.Fatalf("mark provision job succeeded: %v", err)
	}

	// 4. Materialize hermes-home directory with a dummy file
	homePath := filepath.Join(env.DataDir, "agents", agentID, "hermes-home")
	if err := os.MkdirAll(homePath, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	if err := os.WriteFile(filepath.Join(homePath, "USER.md"), []byte("payload"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	// 5. DELETE request
	req, _ := http.NewRequest(http.MethodDelete, env.URL("/api/agents/"+agentID), nil)
	req.Header.Set("Cookie", cookie)
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE status = %d, want 204", resp.StatusCode)
	}

	// 6. GET should return 404
	getReq, _ := http.NewRequest(http.MethodGet, env.URL("/api/agents/"+agentID), nil)
	getReq.Header.Set("Cookie", cookie)
	getResp, err := env.Client.Do(getReq)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET after Delete status = %d, want 404", getResp.StatusCode)
	}

	// 7. DB rows are gone
	var agentCount, jobCount int
	_ = env.DB.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM agents WHERE id = ?;`, agentID).Scan(&agentCount)
	_ = env.DB.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM runtime_jobs WHERE agent_id = ?;`, agentID).Scan(&jobCount)
	if agentCount != 0 {
		t.Errorf("agents count = %d, want 0", agentCount)
	}
	if jobCount != 0 {
		t.Errorf("runtime_jobs count = %d, want 0 (CASCADE)", jobCount)
	}

	// 8. hermes home directory is gone
	if _, err := os.Stat(homePath); !os.IsNotExist(err) {
		t.Errorf("hermes home still exists, stat err = %v", err)
	}

	// 9. mock runner Stop + Remove were both called once
	if env.Runner.StopCalls() < 1 {
		t.Errorf("runner.Stop was not called")
	}
	if env.Runner.RemoveCalls() < 1 {
		t.Errorf("runner.Remove was not called")
	}
}

func TestAgentDeleteRecoversFromStopFailure(t *testing.T) {
	env := newMVPEnv(t)
	defer env.Close()

	cookie := loginAs(t, env, "user@example.com", "user-password")
	templateID := createPublishedTemplate(t, env)
	agentID := createAgent(t, env, cookie, templateID, "retry-agent")
	_, _ = env.DB.ExecContext(context.Background(), `UPDATE agents SET status='running' WHERE id=?;`, agentID)
	_, _ = env.DB.ExecContext(context.Background(), `UPDATE runtime_jobs SET status='succeeded' WHERE agent_id=?;`, agentID)

	homePath := filepath.Join(env.DataDir, "agents", agentID, "hermes-home")
	_ = os.MkdirAll(homePath, 0o755)

	// Make first Stop fail
	env.Runner.SetStopError(fmt.Errorf("transient docker error"))

	req, _ := http.NewRequest(http.MethodDelete, env.URL("/api/agents/"+agentID), nil)
	req.Header.Set("Cookie", cookie)
	resp, _ := env.Client.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("first DELETE status = %d, want 500", resp.StatusCode)
	}

	// Verify state is error with delete_stop_failed code
	var status, errCode string
	_ = env.DB.QueryRowContext(context.Background(),
		`SELECT status, last_error_code FROM agents WHERE id=?;`, agentID).Scan(&status, &errCode)
	if status != "error" || errCode != "delete_stop_failed" {
		t.Fatalf("agent state = (%s, %s), want (error, delete_stop_failed)", status, errCode)
	}

	// Clear stop error and retry
	env.Runner.SetStopError(nil)
	req2, _ := http.NewRequest(http.MethodDelete, env.URL("/api/agents/"+agentID), nil)
	req2.Header.Set("Cookie", cookie)
	resp2, _ := env.Client.Do(req2)
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusNoContent {
		t.Fatalf("retry DELETE status = %d, want 204", resp2.StatusCode)
	}
}

func TestAgentDeleteRefusesProvisioning(t *testing.T) {
	env := newMVPEnv(t)
	defer env.Close()

	cookie := loginAs(t, env, "user@example.com", "user-password")
	templateID := createPublishedTemplate(t, env)
	agentID := createAgent(t, env, cookie, templateID, "provisioning-agent")

	// Force into provisioning
	_, _ = env.DB.ExecContext(context.Background(),
		`UPDATE agents SET status='provisioning' WHERE id=?;`, agentID)

	req, _ := http.NewRequest(http.MethodDelete, env.URL("/api/agents/"+agentID), nil)
	req.Header.Set("Cookie", cookie)
	resp, _ := env.Client.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("DELETE status = %d, want 409", resp.StatusCode)
	}
	var body struct {
		Code string `json:"code"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body.Code != "agent_cannot_delete" {
		t.Errorf("error code = %q, want agent_cannot_delete", body.Code)
	}
}

// suppress unused import warnings if some helpers aren't ultimately needed
var _ = bytes.NewReader
var _ = strings.TrimSpace
```

**重要适配说明**：

1. 上面的 `newMVPEnv`/`env.Runner`/`env.DB`/`env.URL`/`loginAs`/`createPublishedTemplate`/`createAgent` **可能在现有 `mvp_integration_test.go` 中名字不同**。Step 1 的 grep 命令结果决定真实名字。
2. 如果现有集成测试**用的是真实 Docker** 而不是 mock runner，需要在测试 setup 中替换为 fakeRunner。一种做法：在 `tests/` 包内创建一个 `fakeRunner` 类型（与 Task 7 中的相同），并在 `newMVPEnv` 里允许注入。如果现有 setup 不便注入，最简方案是**新建一个独立的 `newDeleteTestEnv(t)` helper**，专门为删除测试构造一个使用 fakeRunner 的服务器实例。
3. 如果现有 helpers 无法快速复用，**降级方案**：把这些集成测试改写为针对 `agents.Service` 的"半集成"测试（直接调 service，不走 HTTP）——本质等价于 Task 7 中的单元测试。这种情况下，仅保留 `TestAgentDeleteHappyPath` 一个端到端用例并接受其复杂度。

**先尝试复用现有 helpers**；如果复用受阻则采用降级方案，并在提交说明中注明。

- [ ] **Step 3: 运行集成测试**

```bash
cd services/api && go test ./tests/ -run TestAgentDelete -v
```

预期所有 3 个用例 PASS。如果失败，根据 Step 2 的适配说明调整 helpers。

- [ ] **Step 4: 跑全仓回归测试**

```bash
cd services/api && go test ./...
```

预期 PASS。

- [ ] **Step 5: 更新 docs/api.md**

打开 `docs/api.md`，在 Agents 章节末尾追加：

```markdown
### DELETE /api/agents/:id

物理删除 agent：停止并移除其 Docker 容器、删除 hermes-home 目录、删除数据库记录（关联的 runtime_jobs/agent_channels/agent_runtime_events 通过外键 CASCADE 自动清理）。

**Auth**: required（owner 或 admin）。

**Responses**:

| 状态码 | error code | 说明 |
|------|------|------|
| 204 No Content | — | 删除成功 |
| 401 | unauthorized | 未登录 |
| 403 | forbidden | 不是 owner 也不是 admin |
| 404 | not_found | agent 不存在 |
| 409 | agent_cannot_delete | 当前状态（provisioning/starting）不允许删除，请等状态稳定后重试 |
| 409 | agent_has_unfinished_jobs | 存在未完成的运行时任务，请稍后重试 |
| 500 | internal_error | 删除过程内部错误（agent 转 error 状态，可重试同一接口） |

**注意**：500 错误后 agent 进入 `error` 状态并写入 `last_error_code`（`delete_inspect_failed` / `delete_stop_failed` / `delete_remove_failed` / `delete_home_failed`）。再次调用 DELETE 会从中断处接续完成清理（每一步都幂等）。
```

- [ ] **Step 6: 提交**

```bash
git add services/api/tests/agent_delete_integration_test.go docs/api.md
git commit -m "test(agents): add delete integration tests; docs: api delete endpoint"
```

注意：如果 `docs/api.md` 也被 .gitignore 忽略（参考 specs 目录的规则），需要 `git add -f`。

---

## 完成检查

- [ ] `cd services/api && go test ./...` 全部通过
- [ ] `cd services/api && go vet ./...` 无警告
- [ ] `cd services/api && go build ./...` 成功
- [ ] 所有 10 个 Task 的提交都进入 main 分支
- [ ] 设计文档：`docs/superpowers/specs/2026-06-18-agent-deletion-design.md`
- [ ] API 文档：`docs/api.md` 已更新

如果一切就绪，本计划完成。
