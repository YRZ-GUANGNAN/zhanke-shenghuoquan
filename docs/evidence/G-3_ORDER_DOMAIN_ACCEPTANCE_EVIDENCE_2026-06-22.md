# G-③ Order Domain Acceptance Evidence — 2026-06-22

实现提交：`95bdabbe411428e06eae14e107c825083c1ebc80`

## 1. Git 与门禁

```text
branch: feat/g3-order
base: f03e8140
required tags: g1-merchant-baseline, g2-catalog-baseline, media-purpose-takeout, infra-automigrate-retired
pre-development migration 017 exists: true
pre-development migration 018 exists: true
pre-development migration 019 exists: false
pre-development maximum migration: 018
backend/cmd/server db.AutoMigrate count: 0
main worktree: clean on master
```

## 2. 真实 MySQL 8.4

执行方式（DSN 已脱敏且未输出）：

```powershell
cd backend
$env:TAKEOUT_G3_MYSQL_DSN = '<redacted real MySQL DSN>'
go test ./internal/module/takeout -run '^Test00OrderMigrationRollbackApplyAgainstRealMySQL84$' -count=1 -v
go test ./internal/module/takeout -run '^(TestOrderLifecycleSnapshotsFeesAndSettlementAgainstRealMySQL84|TestOrderCompetingCASAgainstRealMySQL84|TestZZOrderAcceptanceResidueAgainstRealMySQL84)$' -count=1 -v
```

结果：

```text
=== RUN   Test00OrderMigrationRollbackApplyAgainstRealMySQL84
real_mysql_version=8.4.0
migration_019_apply_rollback_apply=PASS tables=4 foreign_keys=0 fee_seed=global:100:50
--- PASS

=== RUN   TestOrderLifecycleSnapshotsFeesAndSettlementAgainstRealMySQL84
multi_dish_snapshot=PASS server_amount_recalculation=PASS lifecycle_all_paths=PASS stale_CAS=REJECTED terminal_fail_closed=PASS
complete_earning_same_tx=PASS repeated_complete_earning_count=1 fee_snapshot=120:60 settlement=PASS
--- PASS

=== RUN   TestOrderCompetingCASAgainstRealMySQL84
competing_order_CAS=PASS success=1 conflict=1 final_status=accepted
--- PASS

=== RUN   TestZZOrderAcceptanceResidueAgainstRealMySQL84
order_residue orders=0 items=0 earnings=0 catalog_test_dishes=0 fee_config=global:100:50:rv1
--- PASS
```

竞争测试允许最终态为 `accepted` 或 `refunded`，不依赖 goroutine 调度顺序，只要求恰好一个成功且另一个返回 `ErrConcurrentUpdate`。

## 3. 数据库契约证据

```text
tables=4
takeout_orders columns=26
takeout_order_items columns=8
takeout_rider_earning_records columns=11
takeout_fee_config columns=6
all required public IDs=CHAR(26)
all money columns=BIGINT UNSIGNED
quantity=INT UNSIGNED
order status CHECK=9 frozen values
settlement status CHECK=2 frozen values
required indexes present
cross-domain/all foreign keys=0
fee seed=global,100,50,row_version=1
```

迁移 SHA-256：

```text
019_takeout_order.sql
ffb8d7071a896c690e1f0813169c8758f6d148233e52ea16208ba7f503fa6275

rollback/019_takeout_order.sql
049d73b617fd74c65150f214fee7d4530c848755ed0def2462efb1021bea1d42
```

## 4. 金额、快照与幂等证据

```text
dish snapshots: Rice 1200×2=2400; Soup 500×3=1500
items_amount=3900
delivery_fee=100+50=150
total_amount=4050
catalog update after order: stored item snapshot unchanged
draft dish / wrong merchant / missing dish: rejected
overflow multiplication/addition: rejected
completion fee snapshot after config update: rider=120 platform=60
repeated CompleteDelivery: earning row count remains 1
repeated MarkSettled: row_version remains 2
```

## 5. CAS 与事务证据

```text
order stale row_version: ErrConcurrentUpdate
fee config stale row_version: ErrConcurrentUpdate
same-version Accept vs Refund: one success, one conflict
order + all items: one transaction
completed status + earning record: one transaction
earning UNIQUE(order_public_id): present and exercised
target-state repeats: idempotent
different pay_no on paid order: fail-closed
```

## 6. 边界证据

```text
Order production A-F/Media imports: 0
Order production Gin/net/http/route imports: 0
Order production float/decimal types: 0
migration 019 SQL FOREIGN KEY/REFERENCES: 0
migration 019 SQL FLOAT/DOUBLE/DECIMAL: 0
backend/cmd/server db.AutoMigrate count: 0
HTTP routes/controllers added: 0
apps/** changes: 0
001-018 migration changes: 0
```

`repository.go` 仅从原第 244 行后追加 G-③ 方法；G-①/G-② 既有部分未修改。Catalog 交互仅通过同包只读 `CatalogReader.FindDish`。

## 7. 全量回归与构建

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

MiniApp 仅报告既有 `vendors.js` 体积提示；G-③ 没有前端代码变更。

## 8. 安全说明

证据不包含数据库口令、DSN、token 或云凭证。测试结束后订单、明细、应结、Catalog 测试菜品及临时构建资源残留均为 0，费率种子恢复到冻结值。
