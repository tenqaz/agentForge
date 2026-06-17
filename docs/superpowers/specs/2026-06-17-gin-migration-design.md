# Gin 迁移设计

日期：2026-06-17
主题：将 `services/api` 的 HTTP 层从 `net/http` 迁移到 Gin

## 目标

将 `services/api` 中当前基于 `net/http` + `ServeMux` 的后端 HTTP 栈迁移为 Gin。

核心目标：

- 在整个 HTTP 层统一采用 Gin 的 `Context`、路由分组和中间件模型。
- 借助这次迁移整理 handler 结构、请求绑定和错误处理。
- 保持应用行为和用户功能不变，同时允许为适配 Gin 对相关层做联动重构和 API 调整。

非目标：

- 不改变产品流程，也不新增终端用户功能。
- 不做会改变业务语义的重构。

## 当前状态

后端入口位于 `services/api/cmd/agentforge-api/main.go`，负责构建依赖、通过 `internal/http.NewRouter` 创建 HTTP 路由，并交给 `http.Server` 提供服务。

当前 HTTP 层特征如下：

- 使用 `http.NewServeMux()`，并采用 `GET /api/health` 这类带方法信息的模式。
- 通过多个 handler 组注册路由：health、registration、session、templates、agents 和 weixin channels。
- 通过原生 `http.Handler` 中间件处理 session 解析、panic recover 和 request ID 注入。
- 多个 handler 手工完成 JSON 解码。
- 通过共享的 JSON/错误响应辅助函数输出 `{ error, message?, requestId? }` 结构的错误体。

前端当前依赖：

- `/api/...` 下的稳定 API 路径。
- 现有的 session、template、agent、runtime job 和 weixin 流程。
- `web/lib/api/client.ts` 中基于错误码的处理逻辑。
- 在 Playwright 和 API client 测试中写死的路径、方法和响应预期。

## 设计原则

1. 允许为适配 Gin 跨层重构，但重构必须直接服务于迁移目标，而不是顺带做无关架构翻修。
2. 优先保证路由、中间件、上下文传递和错误处理模型的一致性，而不是最小改动量。
3. 在不影响业务语义的前提下，service、repository、database、runtime、worker 都可以按需调整。
4. 避免无意义的 API 重设计；只有在能提升一致性且前端可同步调整时，才允许修改 HTTP 行为。
5. 除非路径调整能明显改善结构，否则保持现有路由路径不变。
6. 即使允许跨层改造，也应尽量保持清晰边界，避免把 Gin 细节无序扩散到所有实现中。

## 目标架构

### HTTP 入口

`internal/http.NewRouter` 将返回 Gin engine，而不再返回泛化的 `http.Handler`。`main.go` 中的服务启动逻辑仍然继续使用 `http.Server`，只是将 Gin engine 作为其 handler。

这样可以保留：

- 现有启动流程
- 现有优雅关闭逻辑
- 现有依赖组装方式

### 分层边界

Gin 可以进入整个 `services/api`，但任何被 Gin 影响到的改动都必须保持功能和业务语义不变。

以下层可以根据需要进行适配性重构：

- `internal/http`
- `internal/auth`
- `internal/agents`
- `internal/templates`
- `internal/channels`
- `internal/jobs`
- `internal/runtime`
- `internal/weixin`

重构后，各层之间仍应尽量保持清晰边界；service/repository/database/runtime/worker 可以调整实现和协作方式，但不应把 Gin 类型向下泄漏到业务核心逻辑里。

## 路由组织

### Engine 组装

Gin engine 将在 `router.go` 中集中组装：

- 显式创建 engine 并配置中间件。
- 安装全局中间件，用于 request ID、panic recover、日志相关元数据和 session 注水。
- 按领域挂载 `/api` 路由分组。
- 如有必要，可同步调整业务层初始化、上下文传递和辅助组件，以适配新的 HTTP 入口模型。

### 路由分组

路由将按资源域组织，而不再通过分散的 `ServeMux` 注册调用来装配：

- `/api/health`
- `/api/users`
- `/api/sessions` 和 `/api/session`
- `/api/templates`
- `/api/admin/templates`
- `/api/agents`
- agent 之下的 runtime jobs 和 weixin channels 嵌套路由

每组 handler 将注册到 `*gin.RouterGroup` 或 engine 本身，例如：

- `RegisterPublicRoutes`
- `RegisterAdminTemplateRoutes`
- `RegisterAgentRoutes`
- `RegisterWeixinRoutes`

函数命名可以结合仓库现有风格微调，但设计要求是停止传递 `*http.ServeMux`，并将 HTTP 注册方式全面迁移为 Gin 原生路由分组。

## Handler 模型

### 签名

`internal/http` 中的所有 HTTP handler 将从：

- `func (h *XHandlers) Method(w http.ResponseWriter, r *http.Request)`

改为：

- `func (h *XHandlers) Method(c *gin.Context)`

### 参数访问

所有路径参数访问都将从 `r.PathValue(...)` 迁移为 `c.Param(...)`。

### 请求绑定

手写 JSON 解码将替换为共享的、面向 Gin 的请求绑定辅助层。

绑定层必须满足：

- 输出一致的 JSON 错误响应
- 显式处理非法 JSON
- 在相关场景中显式处理多余 JSON 内容或不支持的 body 结构
- 为各 handler 提供可复用的请求 DTO 解码能力

该辅助层内部可以使用 Gin 的 binding 机制，但必须保留现有测试依赖的关键 API 行为。如果迁移中有意调整这些语义，则必须同步更新后端测试、前端 client 逻辑和浏览器测试。

### 响应输出

handler 不再直接通过 `http.ResponseWriter` 写 JSON，而是统一使用共享的 Gin 响应辅助函数。

响应层需要统一：

- JSON 成功响应
- 无内容响应
- 错误响应
- request ID 在错误体中的透传

## 中间件设计

### Request ID

当前 `X-Request-ID` 的行为语义将被保留，但改为 Gin 中间件实现。

要求：

- 确保每个请求都有 request ID。
- 在响应头中暴露该 request ID。
- 让下游 handler 和错误辅助函数都能访问该 request ID。
- 在 JSON 错误体中继续提供 `requestId`。

### Panic Recovery

当前 panic recover 中间件将替换为 Gin 中间件，职责包括：

- 捕获 panic
- 记录 panic 上下文
- 返回结构化的内部错误响应
- 在可用时把 request ID 注入错误体

可以基于 Gin 内建 recover 能力扩展，但如果要保持现有响应语义，也允许采用自定义封装。

### Session 注水与鉴权上下文

当前 session 中间件会从 session cookie 解析并加载当前认证用户。

迁移后：

- session 解析和当前认证用户查找将改为 Gin 中间件和/或 Gin 辅助函数。
- 解析出的用户将存入 `gin.Context`。
- 鉴权辅助函数从 `gin.Context` 读取用户，而不是从自定义 request context 中读取。

它需要支持：

- 登录用户校验
- 管理员权限校验
- agent 及其子资源的归属校验

## 授权与共享 HTTP 辅助层

这次迁移也是收敛重复 HTTP 逻辑的合适时机。

设计中会包含一层共享辅助逻辑，用于：

- 获取当前认证用户
- 校验管理员权限
- 校验 agent 访问权限
- 将 domain/service 错误映射为 HTTP 响应
- 输出统一 JSON 结构

目标是减少每个 handler 内部的分支判断，让路由行为更容易审计和维护。

## 错误处理策略

### 稳定的错误体结构

默认错误响应结构保持为：

```json
{
  "error": "code",
  "message": "可选的人类可读消息",
  "requestId": "可选的请求 ID"
}
```

保留这一结构的原因是前端 client 和测试已经依赖基于错误码的行为。

### 收敛方式

现有的 `writeError`、`writeInternalError`、`writeAgentError`、`writeWeixinError` 等辅助函数在概念上会保留，但会重组为面向 Gin 的统一错误输出通道。

目标结果：

- 只有一条一致的错误输出路径
- domain 错误到 HTTP 状态码与错误码的映射清晰
- 日志元数据保持一致
- 减少不同 handler 之间的重复代码

### 允许的 API 调整

迁移过程中允许为了提升一致性而调整 HTTP 行为，但前提是所有受影响的消费方必须一并更新。

允许的调整示例：

- 统一错误消息
- 统一 bad request 处理方式
- 统一不同接口的 JSON 响应格式细节

未经额外范围确认，不允许：

- 改变核心产品流程
- 改变认证模型
- 改变数据语义
- 仅仅为了“看起来更整齐”而重设计资源 URL

## 前端影响

前端属于本次兼容性改造范围的一部分。

可能受影响的区域包括：

- `web/lib/api/client.ts`
- `web/lib/api/types.ts` 中的 API 响应类型
- 假定特定响应细节的 server actions 或组件
- `web/tests` 下的 Playwright 测试

默认兼容原则：

- 除非有明确收益，否则保持现有路由路径不变
- 如果有意调整响应结构或错误细节，必须同步更新前端解析和测试
- 保持所有用户可见流程功能不变

## 测试策略

### 后端

本次迁移必须保持或恢复 HTTP 层与现有集成流程的完整后端测试覆盖。

必须执行的后端验证：

- 在 `services/api` 下运行 `go test ./...`
- `services/api/internal/http` 中的 HTTP handler 测试
- `services/api/tests` 中的集成测试

现有测试辅助代码可能需要从基于泛化 `http.Handler` 的假设调整为面向 Gin engine，但测试场景本身仍应聚焦行为，而不是框架细节。

### 前端

前端验证需要覆盖 API client 行为，以及所有受 HTTP 响应调整影响的用户流程。

预期验证至少包括：

- API client 行为相关单元测试
- 注册、登录、模板、agent、weixin channel 相关 Playwright 流程

具体命令可以在实现计划阶段，结合仓库脚本进一步明确。

## 迁移范围拆解

实现阶段至少需要覆盖以下内容：

1. 引入 Gin 依赖并接入 API 服务。
2. 用 Gin engine 和路由分组替换 `ServeMux` 路由装配。
3. 将所有 HTTP handler 改为 `gin.Context` 模型。
4. 用 Gin 原生中间件替换现有中间件实现。
5. 将 auth/session 辅助流程改为基于 `gin.Context`。
6. 重做请求绑定与 JSON 响应辅助层。
7. 更新后端测试，使其运行在 Gin 路由之上。
8. 对任何有意调整的 HTTP 行为，同步更新前端 client 与 UI 测试。

## 风险

### 行为漂移

请求绑定和中间件栈切换后，容易意外改变：

- JSON 解析失败行为
- header 写入时机
- cookie 行为
- 路径参数提取
- unauthorized 与 forbidden 的边界

缓解方式：

- 保持 URL 结构稳定
- 使用现有测试作为行为安全网
- 对 Gin 容易引入歧义的点补充定向测试

### 业务层与 Gin 耦合过深

迁移过程中存在把 Gin 类型带入 service 或 repository 层的风险。

缓解方式：

- 严格限制 Gin 只出现在 `internal/http`
- 继续向 domain service 传递标准 `context.Context`

### 前后端契约不一致

如果在清理 API 时调整了行为，但没有同步前端，会导致功能回归。

缓解方式：

- 将前端联动更新视为同一次迁移的一部分
- 在同一个变更集中同时更新 API client 测试和流程测试

## 成功标准

当以下条件全部满足时，迁移才算完成：

- `services/api` 中所有与请求处理相关的入口、路由和中间件都已改为 Gin 体系
- 后端代码中不再保留基于 `ServeMux` 的路由装配
- 允许业务层和支撑层重构，但功能、业务语义和数据语义保持不变
- 后端测试通过
- 受影响的前端测试通过
- 应用对用户的既有功能流程保持正常

## 实现计划注意事项

实现计划应按“一次性完成迁移”的思路制定，而不是长期维持混合框架状态。

但在这次整体迁移内部，仍应按风险递减顺序组织工作：

1. 先搭好 Gin engine 与共享中间件/辅助层
2. 再按领域迁移路由注册与 handler
3. 然后按需重构 service/repository/database/runtime/worker 相关适配
4. 再适配后端测试
5. 再联动适配前端契约使用方
6. 最后做全量验证

实现计划必须显式列出哪些 endpoint 语义是“有意调整”的，以便在合并前复核。
