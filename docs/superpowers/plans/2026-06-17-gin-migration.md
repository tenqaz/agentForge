# Gin Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `services/api` 及其联动前端从 `net/http` 迁移到 Gin，同时保持功能、业务语义和数据语义不变。

**Architecture:** 先把 Gin 引入 API 入口和路由层，再按领域迁移 handler、中间件和共享辅助函数；随后按需重构 service / repository / runtime / worker 的适配代码，避免 Gin 类型下沉到业务核心。最后同步更新前端 API client、页面和端到端测试，确保契约与功能都不回退。

**Tech Stack:** Go, Gin, SQLite, `net/http` server bootstrap, Next.js, Playwright, Go test

---

### Task 1: 引入 Gin 并建立新的路由入口

**Files:**
- Modify: `services/api/cmd/agentforge-api/main.go:1-166`
- Modify: `services/api/internal/http/router.go:1-61`
- Modify: `services/api/internal/http/health_handlers.go:1-17`
- Modify: `services/api/internal/http/health_handlers_test.go:1-20`

- [ ] **Step 1: 写一个会失败的路由测试**

```go
func TestHealthRouteUsesGinEngine(t *testing.T) {
	router := NewRouter(Dependencies{})
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}
```

- [ ] **Step 2: 运行测试确认当前实现还没切到 Gin**

Run: `cd services/api && go test ./internal/http -run TestNewRouterReturnsGinEngine -v`
Expected: fail 或者需要先改代码才能通过。

- [ ] **Step 3: 实现 Gin engine 和 main.go 启动路径**

```go
// router.go
func NewRouter(deps Dependencies) *gin.Engine {
    r := gin.New()
    return r
}

// main.go
router := httpapi.NewRouter(httpapi.Dependencies{
    AuthRepository:       authRepo,
    SessionManager:       sessionManager,
    TemplateService:      templateService,
    AgentService:         agentService,
    RuntimeJobRepository: runtimeJobs,
    ChannelService:       channelService,
    ChannelRepository:    channelRepo,
    ChannelJobRepository: channelJobs,
})
server := &http.Server{Addr: cfg.HTTPAddr, Handler: router}
```

- [ ] **Step 4: 运行后端测试确认通过**

Run: `cd services/api && go test ./...`
Expected: `ok`。

- [ ] **Step 5: 提交这一步**

```bash
git add services/api/cmd/agentforge-api/main.go services/api/internal/http/router.go services/api/internal/http/health_handlers.go
git commit -m "refactor(api): bootstrap gin router"
```

### Task 2: 把 HTTP 中间件迁移到 Gin

**Files:**
- Modify: `services/api/internal/http/middleware.go:1-30`
- Modify: `services/api/internal/http/middleware_request_id.go:1-20`
- Modify: `services/api/internal/http/middleware_recover.go:1-14`
- Modify: `services/api/internal/http/middleware_recover_test.go:1-40`
- Modify: `services/api/internal/http/router.go:1-61`

- [ ] **Step 1: 写 request-id / recover 的失败测试**

```go
func TestRequestIDMiddlewareSetsHeader(t *testing.T) {
	router := gin.New()
	router.Use(RequestIDMiddleware())
	router.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	router.ServeHTTP(recorder, req)
	if recorder.Header().Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID is empty")
	}
}

func TestRecoverMiddlewareReturnsJSONError(t *testing.T) {
	router := gin.New()
	router.Use(RequestIDMiddleware(), RecoverMiddleware())
	router.GET("/panic", func(c *gin.Context) { panic("boom") })
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}
```

- [ ] **Step 2: 运行测试确认旧中间件还不满足 Gin 语义**

Run: `cd services/api && go test ./internal/http -run 'TestRequestIDMiddlewareSetsHeader|TestRecoverMiddlewareReturnsJSONError' -v`
Expected: fail。

- [ ] **Step 3: 用 Gin 中间件重写 request-id、recover、session 装配**

```go
func RequestIDMiddleware() gin.HandlerFunc
func RecoverMiddleware() gin.HandlerFunc
func SessionMiddleware(sessionManager *auth.SessionManager, authRepo AuthRepository) gin.HandlerFunc
```

- [ ] **Step 4: 运行中间件测试**

Run: `cd services/api && go test ./internal/http -run 'TestRequestIDMiddlewareSetsHeader|TestRecoverMiddlewareReturnsJSONError' -v`
Expected: `ok`。

- [ ] **Step 5: 提交这一步**

```bash
git add services/api/internal/http/middleware.go services/api/internal/http/middleware_request_id.go services/api/internal/http/middleware_recover.go services/api/internal/http/middleware_recover_test.go services/api/internal/http/router.go
git commit -m "refactor(api): move middleware to gin"
```

### Task 3: 按资源域迁移 HTTP handler

**Files:**
- Modify: `services/api/internal/http/registration_handlers.go:1-59`
- Modify: `services/api/internal/http/session_handlers.go:1-98`
- Modify: `services/api/internal/http/agent_handlers.go:1-260`
- Modify: `services/api/internal/http/template_handlers.go:1-420`
- Modify: `services/api/internal/http/weixin_handlers.go:1-260`
- Modify: `services/api/internal/http/health_handlers.go:1-17`
- Modify: `services/api/internal/http/router.go:1-61`
- Modify: `services/api/internal/http/agent_handlers_test.go:1-260`
- Modify: `services/api/internal/http/registration_handlers_test.go:1-220`
- Modify: `services/api/internal/http/session_handlers_test.go:1-260`
- Modify: `services/api/internal/http/template_handlers_test.go:1-700`
- Modify: `services/api/internal/http/weixin_handlers_test.go:1-220`

- [ ] **Step 1: 为一个代表性接口写失败测试**

```go
func TestSessionCreateWithGinBinding(t *testing.T) {
	router := NewRouter(Dependencies{AuthRepository: newAuthRepo(t), SessionManager: newSessionManager(t)})
	body := bytes.NewBufferString(`{"email":"user@example.com","password":"secret-password"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/sessions", body)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}
```

- [ ] **Step 2: 运行该测试确认需要迁移 handler**

Run: `cd services/api && go test ./internal/http -run TestSessionCreateWithGinBinding -v`
Expected: fail。

- [ ] **Step 3: 把各 handler 改成 `func(c *gin.Context)` 并统一路径参数、绑定和响应输出**

```go
func (h *SessionHandlers) Create(c *gin.Context)
func (h *AgentHandlers) GetRuntimeJob(c *gin.Context)
```

- [ ] **Step 4: 运行 `internal/http` 全量测试**

Run: `cd services/api && go test ./internal/http ./tests`
Expected: `ok`。

- [ ] **Step 5: 提交这一步**

```bash
git add services/api/internal/http
git commit -m "refactor(api): migrate http handlers to gin"
```

### Task 4: 让业务层配合新的请求入口做适配性重构

**Files:**
- Modify: `services/api/internal/auth/session.go:1-220`
- Modify: `services/api/internal/auth/repository.go:1-240`
- Modify: `services/api/internal/agents/service.go:1-135`
- Modify: `services/api/internal/channels/service.go:1-43`
- Modify: `services/api/internal/jobs/runtime_worker.go:1-260`
- Modify: `services/api/internal/jobs/channel_worker.go:1-260`
- Modify: `services/api/internal/runtime/docker.go:1-120`
- Modify: `services/api/internal/runtime/env.go:1-120`
- Modify: `services/api/internal/runtime/home.go:1-120`
- Modify: `services/api/internal/templates/service.go:1-120`
- Modify: `services/api/internal/weixin/client.go:1-220`

- [ ] **Step 1: 为一个需要跨层适配的行为写失败测试**

```go
func TestAgentCreateStillQueuesProvisionJob(t *testing.T) {
	fixture := newFixture(t)
	resp := doJSON(t, fixture.router, http.MethodPost, "/api/agents", `{"templateId":"template-1","name":"Support Agent"}`, fixture.userCookie)
	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusCreated)
	}
}
```

- [ ] **Step 2: 运行测试确认业务层还没配合完成**

Run: `cd services/api && go test ./internal/agents ./internal/jobs ./internal/runtime ./internal/auth -v`
Expected: 至少有一部分失败或需要调整。

- [ ] **Step 3: 重构业务层接口以适配 Gin 驱动的调用链**

```go
// 保持业务语义不变，只调整初始化、上下文传递、错误映射和辅助依赖
```

- [ ] **Step 4: 运行业务层测试确认行为不变**

Run: `cd services/api && go test ./internal/auth ./internal/agents ./internal/channels ./internal/jobs ./internal/runtime ./internal/templates ./internal/weixin ./tests`
Expected: `ok`。

- [ ] **Step 5: 提交这一步**

```bash
git add services/api/internal/auth services/api/internal/agents services/api/internal/channels services/api/internal/jobs services/api/internal/runtime services/api/internal/templates services/api/internal/weixin
git commit -m "refactor(api): adapt core services for gin"
```

### Task 5: 更新前端 API 契约和交互测试

**Files:**
- Modify: `web/lib/api/client.ts:1-430`
- Modify: `web/lib/api/types.ts:1-220`
- Modify: `web/app/login/actions.ts:1-120`
- Modify: `web/app/register/actions.ts:1-120`
- Modify: `web/components/app-shell.tsx:1-140`
- Modify: `web/tests/api-client.test.ts:1-220`
- Modify: `web/tests/register-flow.spec.ts:1-140`
- Modify: `web/tests/agent-flow.spec.ts:1-260`
- Modify: `web/tests/admin-template-flow.spec.ts:1-320`

- [ ] **Step 1: 为一个会受响应变化影响的前端路径写失败测试**

```ts
test("api client still maps backend error codes", async () => {
  const client = createApiClient({ baseUrl: "http://example.test" });
  const result = await client.post("/api/sessions", { email: "user@example.com", password: "bad" });
  expect(result.ok).toBe(false);
  if (!result.ok) {
    expect(result.error.code).toBe("invalid_credentials");
  }
});
```

- [ ] **Step 2: 运行前端相关测试确认当前契约需要同步**

Run: `cd web && npm test -- --runInBand`
Expected: 至少有一部分测试需要更新。

- [ ] **Step 3: 调整 API client、类型和页面逻辑以匹配新的 Gin 后端契约**

```ts
// 保持功能不变，只同步新的状态码、错误体或 header 行为
```

- [ ] **Step 4: 运行前端测试确认通过**

Run: `cd web && npm test -- --runInBand && npx playwright test`
Expected: `ok`。

- [ ] **Step 5: 提交这一步**

```bash
git add web/lib/api/client.ts web/lib/api/types.ts web/app/login/actions.ts web/app/register/actions.ts web/components/app-shell.tsx web/tests
git commit -m "refactor(web): sync api contract with gin backend"
```

### Task 6: 全量验证并收尾

**Files:**
- Modify: `docs/superpowers/plans/2026-06-17-gin-migration.md`
- Modify: `docs/superpowers/specs/2026-06-17-gin-migration-design.md`（如有微调）

- [ ] **Step 1: 运行后端全量测试**

Run: `cd services/api && go test ./...`
Expected: `ok`。

- [ ] **Step 2: 运行前端全量测试**

Run: `cd web && npm test -- --runInBand && npx playwright test`
Expected: `ok`。

- [ ] **Step 3: 检查是否还有旧的 ServeMux 入口**

Run: `cd /Users/zhengwenfeng/work/projs/AgentForge && rg -n "ServeMux|HandleFunc\\(\"GET /api|HandleFunc\\(\"POST /api" services/api`
Expected: 只保留与历史无关的注释或测试痕迹，代码中不再使用旧路由装配。

- [ ] **Step 4: 如果有必要，补一轮最终 commit**

```bash
git add -A
git commit -m "refactor(api): complete gin migration"
```
