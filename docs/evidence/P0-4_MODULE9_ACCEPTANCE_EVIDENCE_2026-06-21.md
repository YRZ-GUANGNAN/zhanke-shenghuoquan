# P0-4 Module ⑨ Acceptance Evidence — 2026-06-21

## Environment

- Database: real MySQL 8.4.0
- Backend: Go 1.25 module
- Admin: TypeScript + Vite
- Date: 2026-06-21

## Permission model

Test: `TestMediaConsolePermissionModel`

```text
Unauthenticated -> 401
RoleUser       -> 403
RoleAdmin      -> 200
RoleSuperAdmin -> 200
All Module 9 routes -> GET only
```

## Real MySQL 8.4

```powershell
cd backend
go test ./internal/module/admin/mediaconsole -count=1 -v
```

Result: PASS。

覆盖：

- Asset、Reference、Ticket、Completion；
- Projection、Decision；
- Outbox、Inbox、Checkpoint；
- Object/Auxiliary GC Job；
- 死信查询；
- 过滤与分页；
- 敏感字段排除；
- 查询前后 status/row_version 不变；
- 数据库残留为 0。

## Full regression

```powershell
cd backend
go test ./... -count=1
```

Result: PASS。

```powershell
npm run verify
```

Result:

```text
check:boundaries PASS
admin typecheck PASS
miniapp typecheck PASS
backend tests PASS
```

```powershell
npm --workspace apps/admin run build
```

Result: PASS，Vite production bundle generated successfully。

## Read-only proof

- `query_repository.go` 无 Create/Save/Update/Updates/Delete/Exec。
- `handler.go` 无 POST/PUT/PATCH/DELETE。
- 真实查询前后：Asset、Projection、Inbox、GC Job status 与 row_version 相同。
- Module ⑨无 migration。

## Frozen boundary

- 009～014 SHA-256 保持冻结值；
- Module ②～⑧ production files 未写入；
- Module ⑧文件最后修改时间均早于 Module ⑨启动；
- 本次新增代码仅位于 Domain F、composition root 和 Admin frontend。

## Residue

- MySQL Module ⑨ fixture assets: 0
- Dependent references/tickets/completions/projections/decisions/events/checkpoints/GC jobs: 0
- New HTTP write routes: 0
- New migrations: 0
- Module ⑩ artifacts: 0
