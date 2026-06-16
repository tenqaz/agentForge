# 默认管理员账号创建 - 实现计划

**设计规范**: `docs/superpowers/specs/2026-06-16-default-admin-account-design.md`  
**日期**: 2026-06-16

## 实现步骤

### 步骤 1: 添加 EnsureDefaultAdmin 方法

**文件**: `services/api/internal/auth/repository.go`

**任务**:
1. 在 `Repository` 结构体添加 `EnsureDefaultAdmin(ctx context.Context) error` 方法
2. 实现逻辑：
   - 调用 `r.FindUserByEmail(ctx, "admin")` 检查用户是否存在
   - 如果返回 `nil`（用户存在），直接返回 `nil`
   - 如果返回 `ErrUserNotFound`，继续创建流程
   - 如果返回其他错误，返回该错误
   - 调用 `HashPassword("admin")` 生成密码哈希
   - 执行 SQL 插入：`INSERT INTO users (id, email, password_hash, role) VALUES (?, ?, ?, ?)`
   - 处理唯一约束错误（使用现有的 `isUniqueConstraint` 辅助函数）

**需要检查**: `isUniqueConstraint` 函数是否存在于 auth 包中，如果不存在，需要从其他包导入或创建

### 步骤 2: 在启动流程中调用

**文件**: `services/api/cmd/agentforge-api/main.go`

**任务**:
1. 在 `run()` 函数中，定位到 `db.Migrate(ctx, database, migrationsDir)` 调用之后
2. 在创建 `authRepo := auth.NewRepository(database)` 之前，创建一个临时的 authRepo 实例
3. 调用 `authRepo.EnsureDefaultAdmin(ctx)`
4. 如果返回错误，使用 `fmt.Errorf("ensure default admin: %w", err)` 包装并返回

**注意**: 当前代码在迁移后才创建 authRepo，需要提前创建以调用新方法

### 步骤 3: 添加测试

**文件**: `services/api/internal/auth/auth_test.go`

**任务**:
1. 添加 `TestEnsureDefaultAdmin_FirstTime`:
   - 创建空的测试数据库
   - 调用 `EnsureDefaultAdmin`
   - 验证返回 `nil`
   - 使用 `FindUserByEmail("admin")` 验证用户被创建
   - 验证用户的 role 为 `RoleAdmin`
   - 使用 `PasswordHashForUser` 获取哈希，使用 `CheckPassword` 验证密码为 "admin"

2. 添加 `TestEnsureDefaultAdmin_Idempotent`:
   - 调用 `EnsureDefaultAdmin` 两次
   - 验证两次都返回 `nil`
   - 查询数据库验证只有一个用户

3. 添加 `TestEnsureDefaultAdmin_AlreadyExists`:
   - 手动插入一个 email 为 "admin" 的用户（使用不同的密码哈希）
   - 调用 `EnsureDefaultAdmin`
   - 验证返回 `nil`
   - 验证用户的密码哈希未被修改

**测试辅助**: 复用现有的测试数据库设置代码（参考 `auth_test.go` 中的其他测试）

### 步骤 4: 运行测试

**命令**:
```bash
cd services/api
go test ./internal/auth -v -run TestEnsureDefaultAdmin
```

**验证**: 所有三个测试用例通过

### 步骤 5: 手动验证

**任务**:
1. 删除现有的 SQLite 数据库（如果存在）
2. 启动服务：`cd services/api && go run cmd/agentforge-api/main.go`
3. 验证服务启动成功，无错误日志
4. 使用 `admin/admin` 通过 API 登录（POST `/api/sessions`）
5. 验证登录成功并返回 admin 角色

### 步骤 6: 幂等性验证

**任务**:
1. 不删除数据库，重启服务
2. 验证服务启动成功
3. 再次使用 `admin/admin` 登录，验证账号仍然有效

## 实现细节

### isUniqueConstraint 函数

**需要确认**: 该函数是否已存在。根据 CodeGraph 输出，`jobs/channel_repository.go` 中使用了该函数，需要检查是否在公共位置或需要复制实现。

**如果不存在**，在 `internal/auth/repository.go` 中添加：

```go
func isUniqueConstraint(err error) bool {
    if err == nil {
        return false
    }
    return strings.Contains(err.Error(), "UNIQUE constraint failed")
}
```

### SQL 插入语句

```go
_, err = r.database.ExecContext(ctx, `
    INSERT INTO users (id, email, password_hash, role)
    VALUES (?, ?, ?, ?);
`, "admin", "admin", hash, RoleAdmin)
```

**注意**: 不需要指定 `created_at` 和 `updated_at`，它们有默认值

## 风险和注意事项

1. **并发启动**: 虽然不太可能，但如果多个实例同时启动，唯一约束冲突处理会确保幂等性
2. **密码哈希失败**: bcrypt 理论上可能失败（如系统资源不足），会导致服务启动失败，这是预期行为
3. **现有 admin 用户**: 如果数据库中已有手动创建的 admin 用户，不会被覆盖

## 验收检查清单

- [ ] `EnsureDefaultAdmin` 方法实现并通过代码审查
- [ ] 三个测试用例全部通过
- [ ] 空数据库启动后可以用 `admin/admin` 登录
- [ ] 重启服务不会创建重复账号
- [ ] 删除 admin 用户后重启，账号自动恢复
- [ ] 代码已提交到 git

## 估算时间

- 步骤 1-2: 15 分钟（代码实现）
- 步骤 3: 20 分钟（测试编写）
- 步骤 4-6: 10 分钟（验证）
- **总计**: 约 45 分钟
