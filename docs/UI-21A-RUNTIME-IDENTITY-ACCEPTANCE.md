# UI-21a 运行时登录与身份地基 Acceptance Report

## 真实 MySQL 8.4 与 HTTP

验收只从主项目 `backend/.env` 读取 `MYSQL_DSN` 到进程环境，未在命令输出、报告或提交中打印凭证。

```text
TestUI21AIdentityMigrationApplyRollbackApply PASS
migration_029_apply_rollback_apply=PASS users.current_identity foreign_keys=0

TestUI21AIdentityRealHTTPMySQL PASS
ui21a_identity_http=PASS auth_401+rider_403+active_switch+merchant_delivery_policy+disabled_gate+persistence+role_unchanged
```

覆盖事实：

- 未登录身份端点返回 401；
- 无骑手资格切 rider 返回 403；
- active rider 可切换；
- Delivery 开启时 merchant 可用且不依赖旧外卖商家；
- Delivery 关闭时 rider/merchant fail-closed；
- 非法 `admin` 身份返回 400；
- 重复切换同一身份幂等成功；
- 重建 Service 后仍读取持久化身份；
- `users.role` 在身份切换前后不变；
- 029 apply→rollback→apply 通过，`users` 新增外键数为 0；测试夹具已 scoped 清理。

## 自动化验证

为了避免 C 盘缓存干扰，在 `D:\codex-temp\ui21a-verify` 对当前分支提交执行全新 `npm ci`（1167 packages），验证结果：

| 验收项 | 结果 |
|---|---|
| `npm --workspace apps/miniapp run typecheck` | PASS |
| `npm run check:miniapp-static-routes` | PASS，25 routes |
| `npm run check:boundaries` | PASS |
| `npm run build` | PASS（miniapp + admin） |
| `go test ./... -count=1` | PASS |
| `npm run verify` | PASS |
| `rg "db\.AutoMigrate\(" backend/cmd/server` | 0 命中 |
| `git diff --check master -- .` | PASS |

构建只有既有 `publish-post/index.scss` 的 Sass `@import` 弃用警告，不是 UI-21a 新增错误。

## 边界证据

- `backend/internal/module/takeout/**`：零业务 diff。
- `backend/internal/module/admin/**`：零 diff。
- `apps/admin/**`：零 diff。
- 迁移 `001–028`：零 diff，仅新增 029 与 rollback 029。
- `package.json`、lockfile：零 diff，无新依赖。
- `db.AutoMigrate(`：0。
- `feat/p0-identity-switch`：仍存在且未被操作。

## Conditional 手工项

自动化已证明会话、HTTP、资格、持久化、路由表、typecheck 与构建正确。以下视觉/设备行为需 owner 在微信开发者工具或真机做最终肉眼确认，当前不冒充已完成：

1. 无 token 冷启动只触发一次微信登录；
2. 持久化 rider/merchant 冷启动进入对应首页；
3. 快速重复点击切换只发送一次请求；
4. 断网/5xx 显示重试且不删除 token；
5. 非法深链短暂不泄露骑手/商家业务内容。

因此自动化验收为 **PASS**，设备级视觉验收保持 **CONDITIONAL**，待 owner 手工确认后再合并和打 tag。
