# 注册功能设计

**日期**: 2026-06-16  
**状态**: 待实现

## 需求

系统需要新增公开注册能力。未登录访客可以使用邮箱和密码创建账号。注册成功后不自动登录，而是跳转到登录页由用户手动登录。

同时，服务启动时自动创建的默认管理员账号需要从 `admin` 调整为 `admin@123.com`。

## 已确认范围

- 注册面向所有未登录访客开放
- 注册信息只有 `email` 和 `password`
- 注册成功后不创建 session
- 注册成功后前端跳转到登录页
- 密码规则为至少 8 位，且必须同时包含字母和数字
- 默认管理员邮箱改为 `admin@123.com`
- 默认管理员密码保持现状 `admin`

## 目标

- 为系统提供最小可用的自助注册闭环
- 复用现有登录、密码哈希和用户表结构
- 保持默认管理员初始化逻辑幂等
- 让前端注册流程与现有登录页风格和 API 客户端保持一致

## 非目标

- 邮箱验证码或邮箱真实性校验
- 找回密码和修改密码
- 邀请码、关闭注册或白名单注册
- 管理员后台创建普通用户
- 注册后自动登录

## 方案选择

### 选定方案：在现有认证链路上扩展注册

本次直接在现有 `auth.Repository`、HTTP handler 和 Next.js 登录页面附近扩展注册能力，不额外引入新的认证 service。

这样做的原因：

- 当前项目认证能力较薄，注册逻辑规模不大，直接扩展现有边界改动最小
- 可以复用现有 `users` 表、bcrypt 哈希、session 和 API client 模式
- 测试可以沿用现有 `auth` 与 `session_handlers` 的测试组织方式

### 备选方案及放弃原因

- 新增独立 `auth.Service`
  - 边界更清晰，但对当前需求偏重，会引入额外样板
- 只做后端接口，暂不补前端页面
  - 功能不闭环，不满足“新增注册功能”的直接目标

## 架构设计

### 1. 后端接口

新增公开注册接口：

```http
POST /api/users
Content-Type: application/json
```

请求体：

```json
{
  "email": "user@example.com",
  "password": "abc12345"
}
```

成功响应：

```json
{
  "user": {
    "id": "generated-id",
    "email": "user@example.com",
    "role": "user"
  }
}
```

状态码：

- `201 Created`：注册成功
- `400 Bad Request`：邮箱格式非法、密码不满足规则或请求体非法
- `409 Conflict`：邮箱已存在
- `500 Internal Server Error`：数据库或其它未预期错误

接口是公开的，不要求 session，也不会在成功后设置 cookie。

### 2. 认证仓储扩展

在 `services/api/internal/auth/repository.go` 中扩展创建用户能力，新增一个面向注册使用的方法，例如：

```go
func (r *Repository) CreateUser(ctx context.Context, params CreateUserParams) (User, error)
```

其中 `CreateUserParams` 包含：

- `Email string`
- `Password string`
- `Role Role`

该方法负责：

- 规范化邮箱输入，例如去掉首尾空格并转为小写
- 校验邮箱格式
- 校验密码规则
- 检查邮箱是否已存在
- 对密码做 bcrypt 哈希
- 写入 `users` 表，默认角色为 `user`

不建议把这些逻辑拆散到 handler 中，否则注册规则会分布在 HTTP 层和 repository 层，后续维护会变脆。

### 3. 默认管理员初始化调整

现有 `EnsureDefaultAdmin` 保留，但默认账号规格改为：

- `ID`: `admin`
- `Email`: `admin@123.com`
- `Password`: `admin`
- `Role`: `admin`

启动时仍然在数据库迁移后执行默认管理员检查与创建，并保持幂等：

- 已存在 `admin@123.com` 时直接返回成功
- 创建时遇到唯一约束冲突仍视为成功，用于处理并发启动场景

本次只修改默认管理员邮箱，不新增管理员密码规则，因为该路径是系统初始化逻辑，不是公开注册入口。

## 数据流

注册请求的处理流程：

1. 前端注册页提交邮箱和密码到 `POST /api/users`
2. handler 解析 JSON 并拒绝多余 payload
3. handler 调用 `auth.Repository.CreateUser`
4. repository 完成邮箱规范化、校验、去重、哈希和插入
5. handler 返回 `201` 和用户公开字段
6. 前端显示注册成功提示并跳转到 `/login`

默认管理员初始化流程：

1. 服务启动
2. 数据库迁移完成
3. 调用 `EnsureDefaultAdmin`
4. 查找 `admin@123.com`
5. 若不存在则插入默认管理员账号

## 前端设计

### 1. 页面与入口

新增页面：

- `web/app/register/page.tsx`

页面内容保持最小化：

- 邮箱输入框
- 密码输入框
- 提交按钮
- 注册失败提示
- “已有账号，去登录”链接

登录页增加“去注册”入口，注册页增加“去登录”入口，形成双向导航。

### 2. 提交行为

前端通过现有 API client 新增注册方法，例如：

```ts
register(input: { email: string; password: string })
```

提交成功后：

- 不更新当前 session 状态
- 跳转到 `/login`
- 显示成功提示，例如“注册成功，请登录”

提交失败后：

- `409` 映射为“邮箱已注册”
- `400` 中密码或邮箱问题统一映射为用户可理解的提示
- `500` 映射为通用失败提示

## 错误处理

### 后端错误语义

为避免前端依赖字符串匹配，注册接口应输出稳定错误码，建议包括：

- `invalid_json`
- `invalid_email`
- `invalid_password`
- `email_already_exists`
- `internal_error`

这样前端可以做确定性提示，后端也能保持与现有 session handler 接近的返回风格。

### 密码规则

密码必须同时满足：

- 长度不少于 8
- 至少包含一个字母
- 至少包含一个数字

密码不要求特殊字符，也不区分必须大小写混合。这保持规则足够简单，避免过早增加注册摩擦。

### 邮箱规则

邮箱使用“实用型校验”，即：

- 非空
- 符合常见邮箱格式
- 存储前统一转为小写

这样可以避免 `User@Example.com` 和 `user@example.com` 被视为两个账号。

## 测试策略

### 后端测试

在现有认证和 HTTP 测试中补充以下覆盖：

- 注册成功后返回 `201`，用户角色为 `user`
- 重复邮箱注册返回 `409`
- 非法邮箱返回 `400`
- 弱密码返回 `400`
- 注册成功后不写 session cookie
- `EnsureDefaultAdmin` 创建的管理员邮箱为 `admin@123.com`
- 现有默认管理员登录测试更新为 `admin@123.com` / `admin`

### 前端测试

补充前端页面或 API client 测试：

- 注册页可以提交成功并跳转到登录页
- 注册失败时展示正确提示
- 登录页包含跳转到注册页的入口

不要求本次加入完整的端到端浏览器回归，只需要覆盖新入口和关键跳转。

## 影响范围

预计涉及文件：

1. `services/api/internal/auth/repository.go`
2. `services/api/internal/http/router.go`
3. `services/api/internal/http/session_handlers.go` 或新增独立注册 handler 文件
4. `services/api/internal/auth/auth_test.go`
5. `services/api/internal/http/session_handlers_test.go` 或对应新 handler 测试
6. `services/api/cmd/agentforge-api/main.go`（如果默认管理员初始化调用位置需要同步）
7. `web/app/login/page.tsx`
8. `web/app/register/page.tsx`
9. `web/lib/api/client.ts`
10. 相关前端测试文件

## 验收标准

1. 未登录访客可以使用邮箱和密码成功注册普通用户
2. 重复邮箱不能重复注册
3. 弱密码会被拒绝，并给出可理解的错误提示
4. 注册成功后用户被引导到登录页，且不会自动登录
5. 服务启动后默认管理员账号为 `admin@123.com`，密码为 `admin`
6. 相关后端和前端测试通过

## 风险与后续增强

当前方案的已知限制：

- 公开注册会带来账号滥用风险，但这是当前确认范围内的设计选择
- 没有邮箱验证，用户可使用不可达邮箱注册
- 默认管理员密码仍为 `admin`，存在明显安全风险

后续可考虑但不属于本次实现：

- 通过环境变量关闭公开注册
- 引入邮箱验证或邀请码
- 支持修改密码和找回密码
- 首次使用默认管理员时强制修改密码
