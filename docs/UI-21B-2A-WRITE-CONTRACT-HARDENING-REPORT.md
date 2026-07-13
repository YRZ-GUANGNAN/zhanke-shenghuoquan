# UI-21b-2a 写契约加固验收报告

## Development

本次在 `feat/ui21b2a-write-contract` worktree 内完成社区/二手写契约加固，范围保持在授权内：

- 新增迁移 `030_community_trade_write_contract.sql` 与 rollback，补齐社区/二手写入幂等、目标状态、outbox 与查询索引；未新增外键。
- `review.EnsureSubmitted` 改为幂等提交语义，保留既有合法状态，非法状态 fail-closed。
- 社区/二手点赞、收藏、关注、售出等写操作改为目标状态幂等语义，避免重复写导致计数漂移。
- 社区/二手创建补充 `Idempotency-Key` 契约：首次创建返回 201，重复同 payload 返回 200 replay，key 相同但 payload 不同返回 409。
- 新增社区/二手写 outbox processor，具备 lease、CAS、backoff、deadletter。
- IM 会话稳定化，避免重复创建同一会话。
- 旧 media 补充提交幂等保护测试。
- `main.go` 只做必要 wiring；未恢复 `AutoMigrate`。

## Acceptance Evidence

### 真实 MySQL 8.4 迁移

使用主项目 `backend/.env` 中真实 `MYSQL_DSN`，未复制或提交 `.env`。执行：

```powershell
go test ./migrations -run TestMigration030AgainstRealMySQL84 -count=1 -v
```

结果：

- `migration_030_apply_apply_rollback_apply=PASS`
- `mysql=8.4.0`
- `foreign_keys=0`
- PASS

### 真实 MySQL 定向测试

执行：

```powershell
go test ./internal/module/admin/review ./internal/module/community ./internal/module/trade ./internal/module/im ./migrations -count=1
```

结果：PASS。

补充说明：`internal/pkg/media` 的既有外部 MySQL 测试在复用主库并注入 `TEST_MYSQL_DSN` 时触发既有 `AutoMigrate`/数据形状问题；本次新增 media 幂等测试单独执行通过：

```powershell
go test ./internal/pkg/media -run TestEnsureCommitted -count=1
```

结果：PASS。

### 全量与前端构建

执行并通过：

```powershell
go test ./... -count=1
npm run check:boundaries
npm run typecheck
npm run verify
npm run build
```

其中 `npm run verify` 与 worktree `npm run build` 因 C 盘空间不足，临时使用 `D:\codex-tmp\ui21b2a*` 作为 Go/TMP cache，并临时通过 junction 复用主仓库 `node_modules`。验收后已删除这些临时目录和 junction。

### Boundary

- `backend/cmd/server` 中 `db.AutoMigrate(` 搜索结果为 0。
- 未改动旧外卖封存开关。
- 未改动小程序静态 UI 壳。
- 未新增外键。
- 未提交 `.env` 或临时缓存。

## Residue

- 临时 `D:\codex-tmp\ui21b2a*` 已清理。
- 临时 worktree `node_modules` junction 已清理。
- 测试使用真实 MySQL scoped 数据，未留下临时库残留。
- 当前报告随本次提交入库；最终 commit hash 见交付回复。
