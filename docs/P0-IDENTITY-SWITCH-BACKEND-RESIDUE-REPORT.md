# P0 身份切换后端 Residue Report

## 1. 工作区残留

- `node_modules/`：为隔离 worktree 执行前端 typecheck/build 而安装，已被 `.gitignore` 忽略，不进入 git。
- `apps/miniapp/dist/`、`apps/admin/dist/`：构建产物，被 `.gitignore` 忽略，不进入 git。
- 临时 patch：已删除。

## 2. 数据库残留

当前环境未提供 `P0_IDENTITY_MYSQL_DSN` / `MYSQL_DSN` / `.env`，因此没有连接真实 MySQL，也没有写入数据库残留。

## 3. 云资源残留

本次未调用 COS、微信内容审核、腾讯 IM 或支付接口；无云资源残留。

## 4. 已知未完成项

- 真实 MySQL 8.4 migration 029 apply/rollback/apply 待跑。
- 真实 HTTP+MySQL 身份切换验收待跑。

## 5. 边界残留

- 未修改 delivery/community/trade 核心域。
- 新增 takeout 资格 adapter 位于 `backend/cmd/server` composition root，不改变 G/D 域核心依赖。
- `users.role` 未改语义，仍只用于后台权限。
