# 默认管理员账号创建设计

**日期**: 2026-06-16  
**状态**: 待实现

## 需求

在后端服务启动时自动创建默认的管理员账号，默认账号密码均为 `admin`。如果账号已存在则不再创建。

## 目标

- 确保系统始终有至少一个可用的管理员账号
- 新部署的系统可以立即使用默认账号登录
- 即使默认账号被误删，重启服务后自动恢复
- 保持幂等性，多次启动不会重复创建

## 设计方案

### 方案选择

**选定方案：启动时检查并创建**

在 `main.go` 的启动流程中，数据库迁移完成后立即检查并创建默认管理员账号。

**替代方案及弃用原因**：
- 通过数据库迁移创建：如果账号被删除不会自动恢复，且密码哈希需硬编码在 SQL 中
- 混合方案：代码冗余

### 架构设计

#### 1. 新增方法

在 `internal/auth/repository.go` 中添加方法：

```go
func (r *Repository) EnsureDefaultAdmin(ctx context.Context) error
```

**职责**：
- 检查 email 为 "admin" 的用户是否存在
- 如果不存在，创建默认管理员账号
- 如果已存在，直接返回（幂等操作）

**默认账号规格**：
- ID: `"admin"`
- Email: `"admin"`
- Password: `"admin"` (通过 `auth.HashPassword` 哈希)
- Role: `"admin"`

#### 2. 调用位置

在 `cmd/agentforge-api/main.go` 的 `run()` 函数中：

```go
func run() error {
    // ... 现有代码：加载配置、打开数据库 ...
    
    // 执行迁移
    if err := db.Migrate(ctx, database, migrationsDir); err != nil {
        return err
    }
    
    // 新增：确保默认管理员存在
    authRepo := auth.NewRepository(database)
    if err := authRepo.EnsureDefaultAdmin(ctx); err != nil {
        return fmt.Errorf("ensure default admin: %w", err)
    }
    
    // ... 继续现有代码：创建服务、启动 HTTP 服务器 ...
}
```

**时机**：数据库迁移完成后、创建其他服务前。

### 实现细节

#### 查询逻辑

```go
// 1. 查询是否存在
_, err := r.FindUserByEmail(ctx, "admin")
if err == nil {
    // 用户已存在
    return nil
}
if !errors.Is(err, ErrUserNotFound) {
    // 其他错误
    return err
}

// 2. 用户不存在，创建
```

#### 插入逻辑

```go
hash, err := HashPassword("admin")
if err != nil {
    return fmt.Errorf("hash password: %w", err)
}

_, err = r.database.ExecContext(ctx, `
    INSERT INTO users (id, email, password_hash, role)
    VALUES (?, ?, ?, ?);
`, "admin", "admin", hash, RoleAdmin)

// 处理唯一约束冲突（并发场景）
if isUniqueConstraint(err) {
    return nil
}
return err
```

**并发安全**：
- 如果多个服务实例同时启动（不太可能但理论存在），可能同时尝试插入
- 通过捕获唯一约束冲突并视为成功来处理

### 错误处理

#### 成功场景

- 用户不存在 → 创建成功 → 返回 `nil`
- 用户已存在 → 跳过创建 → 返回 `nil`
- 并发插入导致唯一约束冲突 → 返回 `nil`

#### 失败场景

- 密码哈希失败 → 返回错误 → 服务启动失败
- 数据库插入失败（非唯一约束） → 返回错误 → 服务启动失败

**失败影响**：如果无法创建默认管理员，服务将拒绝启动。这是合理的，因为没有管理员账号，系统无法管理。

### 测试策略

在 `internal/auth/auth_test.go` 中添加测试用例：

#### 测试 1：首次创建

```go
func TestEnsureDefaultAdmin_FirstTime(t *testing.T) {
    // 空数据库
    // 调用 EnsureDefaultAdmin
    // 验证：用户被创建，email 为 "admin"，role 为 "admin"
    // 验证：使用 "admin"/"admin" 可以登录
}
```

#### 测试 2：幂等性

```go
func TestEnsureDefaultAdmin_Idempotent(t *testing.T) {
    // 调用 EnsureDefaultAdmin 两次
    // 验证：第二次调用不报错
    // 验证：数据库中只有一个 admin 用户
}
```

#### 测试 3：已存在时跳过

```go
func TestEnsureDefaultAdmin_AlreadyExists(t *testing.T) {
    // 手动创建 email 为 "admin" 的用户
    // 调用 EnsureDefaultAdmin
    // 验证：不报错，用户数据未改变
}
```

### 安全考虑

#### 默认密码风险

- 默认密码 `admin` 存在明显的安全风险
- 任何能访问系统的人都可以使用默认凭据登录
- **缓解措施**（不在本次实现范围）：
  - 在文档中添加"首次登录后立即修改密码"的警告
  - 未来可添加"强制修改初始密码"功能
  - 未来可在日志中记录默认账号的使用情况

#### 密码哈希

- 使用现有的 `auth.HashPassword` 函数
- 基于 bcrypt，与系统其他部分一致
- 每次服务启动时重新哈希（如果需要创建），不会硬编码哈希值

## 影响范围

### 修改的文件

1. **`internal/auth/repository.go`**: 添加 `EnsureDefaultAdmin` 方法
2. **`cmd/agentforge-api/main.go`**: 在启动流程中调用该方法
3. **`internal/auth/auth_test.go`**: 添加测试用例

### 对现有代码的影响

- **无破坏性变更**：仅添加新功能，不修改现有逻辑
- **启动时间**：增加一次数据库查询（通常 < 1ms）
- **兼容性**：如果数据库中已有 email 为 "admin" 的用户（手动创建的），将保持不变

## 验收标准

1. 空数据库启动后，可以使用 `admin/admin` 登录
2. 删除默认管理员后重启服务，账号自动恢复
3. 多次重启服务，不会创建重复的管理员账号
4. 测试全部通过
5. 启动日志中无错误信息

## 未来增强

（不在本次需求范围内）

- 支持通过环境变量配置默认管理员的用户名和密码
- 首次登录强制修改密码
- 记录默认账号使用情况到审计日志
- 支持禁用默认账号创建（生产环境安全考虑）
