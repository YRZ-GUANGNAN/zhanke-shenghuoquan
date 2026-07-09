# P0 身份切换后端 Acceptance Report

## 1. 结论

工程门禁已通过；真实 MySQL 验收未完成，原因是当前环境没有 `P0_IDENTITY_MYSQL_DSN`、`MYSQL_DSN` 或 `.env` 数据库配置。

因此本次结论不是完整 PASS，而是：

```text
LOCAL ENGINEERING GATES PASS
REAL MYSQL ACCEPTANCE PENDING
```

## 2. 已通过命令

```powershell
go test ./internal/module/user ./cmd/server
```

结果：

```text
ok campus-circle/backend/internal/module/user
ok campus-circle/backend/cmd/server
```

```powershell
npm run check:boundaries
```

结果：

```text
ok A-G boundaries plus independent delivery and isolated P0-4 media domains
```

```powershell
go test ./...
```

结果：全部 Go package 通过；真实外部环境测试按既有 env-gated 模式 skip。

```powershell
npm run typecheck
```

结果：admin + miniapp TypeScript 通过。

```powershell
npm run build:miniapp
```

结果：Taro build 成功。存在既有 Sass `@import` deprecation 与 vendors 体积 warning，非本次新增阻断。

```powershell
npm run verify
```

结果：`check:boundaries` + `typecheck` + `test:backend` 全通过。

```powershell
npm run build:admin
```

结果：Vite build 成功。

## 3. 真实 MySQL 待验收项

已新增 env-gated 测试：

```powershell
go test ./cmd/server -run TestP0Identity -count=1 -v
```

当前结果：

```text
SKIP TestP0IdentityMigrationApplyRollbackApply
SKIP TestP0IdentityRealHTTPMySQLEligibilitySwitch
reason: set P0_IDENTITY_MYSQL_DSN to run P0 identity real HTTP/MySQL acceptance
```

待具备真实库 DSN 后必须补跑：

- migration 029 apply / rollback / apply。
- `GET /api/v1/auth/identity` 默认 student 摘要。
- 无 rider/merchant 资格切换 403。
- active rider 切换成功。
- resting/suspended rider 切换 403。
- active merchant 切换成功。
- pending/resting/rejected/suspended merchant 切换 403。
- `users.role` 保持 `user/admin/superadmin` 后台权限语义不变。

## 4. 验收边界

- 已验证：代码编译、单元测试、HTTP handler 编译、边界扫描、前端类型、前端构建。
- 未验证：当前机器真实 MySQL 8.4 上的迁移实际 apply/rollback/apply 与 HTTP+MySQL 全链路。
- 未运行原因：当前环境没有数据库 DSN；未构造临时库、未伪造真实验收。

## 5. 风险

- 在真实库未补跑前，不应声明 P0 身份切换正式 PASS。
- 迁移 029 会给 `users` 表新增字段；上线前必须先在真实库执行并验证。
- rollback 会删除当前身份字段；如已有非 student 身份事实，应先评估是否允许丢弃当前视图状态。
