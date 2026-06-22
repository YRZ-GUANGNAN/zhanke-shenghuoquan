# G-② Catalog Domain Acceptance Evidence — 2026-06-22

实现提交：`9a021c88`

## 1. Git 与门禁

```text
branch: feat/g2-catalog
base: 9489cc49
required tags: g1-merchant-baseline, media-purpose-takeout
pre-development migration 017 exists: false
pre-development maximum migration: 018
backend/cmd/server db.AutoMigrate count: 0
main worktree: clean on master
```

## 2. 真实 MySQL 8.4

执行方式（DSN 已脱敏且未输出）：

```powershell
cd backend
$env:TAKEOUT_G2_MYSQL_DSN = '<redacted real MySQL DSN>'
go test ./internal/module/takeout -run '^(Test00CatalogMigrationRollbackApplyAgainstRealMySQL84|TestCatalogCategoryAndDishLifecycleAgainstRealMySQL84|TestCatalogCompetingDishCASAgainstRealMySQL84|TestZZCatalogAcceptanceResidueAgainstRealMySQL84)$' -count=1 -v
```

结果：

```text
=== RUN   Test00CatalogMigrationRollbackApplyAgainstRealMySQL84
real_mysql_version=8.4.0
migration_017_apply_rollback_apply=PASS tables=2 foreign_keys=0 media_018_unchanged=PASS
--- PASS

=== RUN   TestCatalogCategoryAndDishLifecycleAgainstRealMySQL84
category_create_rename_reorder_find=PASS dish_integer_cents=PASS lifecycle_all_paths=PASS stale_CAS=REJECTED idempotency=PASS
--- PASS

=== RUN   TestCatalogCompetingDishCASAgainstRealMySQL84
competing_dish_CAS=PASS success=1 conflict=1 final_status=off_shelf
--- PASS

=== RUN   TestZZCatalogAcceptanceResidueAgainstRealMySQL84
catalog_residue categories=0 dishes=0
--- PASS
```

## 3. 迁移断言

```text
takeout_dish_categories: schema/index/default contract PASS
takeout_dishes: schema/index/default/status CHECK contract PASS
price_cents: bigint unsigned NOT NULL PASS
public IDs: char(26), unique PASS
foreign keys across both tables: 0
017 rollback removes both tables in dependency-safe order
018 media constraint snapshot before/after 017: unchanged
```

迁移 SHA-256：

```text
017_takeout_catalog.sql
7f9962e7fbfa010b5e8cf01ff1cdc5304283022a1215d7849c9ce806acf8a798

rollback/017_takeout_catalog.sql
61103685bd8d5873b738bb47993c6b92ef93cbdfbbf519f77927210a277aad03
```

## 4. CAS 与状态机

```text
category stale row_version: ErrConcurrentUpdate
dish stale row_version/status: ErrConcurrentUpdate
same-version competing writes: one success, one conflict
target-state repeat: idempotent, row_version unchanged
same-detail repeat: idempotent, row_version unchanged
illegal transition: fail-closed
removed: terminal
all seven legal directed edges: PASS
```

## 5. 边界证据

```text
Catalog production media imports: 0
Catalog production Gin/net/http/route imports: 0
migration 017 FOREIGN KEY/REFERENCES: 0
backend/cmd/server db.AutoMigrate count: 0
HTTP routes/controllers added: 0
apps/** changes: 0
001-016 and 018 migration changes: 0
```

既有 `repository.go` 的 G-① 内容未改；仅从文件末尾追加 Catalog 的 `GormRepository` 方法，以保持既有 Repository 单一数据库边界与冻结边界测试。

## 6. 全量回归与构建

```text
go test ./... -count=1    PASS
go vet ./...              PASS
go build ./...            PASS
npm run verify            PASS
  check:boundaries        PASS
  admin typecheck         PASS
  miniapp typecheck       PASS
  backend tests           PASS
npm run build             PASS
  MiniApp Taro build      PASS
  Admin Vite build        PASS
```

MiniApp 报告既有 `vendors.js` 体积提示，未影响构建，且 G-② 没有前端变更。

## 7. 安全说明

本证据不包含数据库口令、DSN、token 或云凭证。测试结束后 Catalog 数据、构建目录和临时依赖链接均为 0。

