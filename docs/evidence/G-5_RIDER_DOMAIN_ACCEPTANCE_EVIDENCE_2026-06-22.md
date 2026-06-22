# G-⑤ Rider Domain Acceptance Evidence — 2026-06-22

实现提交：`745b633c981ccd092a52b507258d79296dbc5492`

## 1. Git 与门禁

```text
branch: feat/g5-rider
base: ae83ef88 docs(discipline): v1.2
g3-order-baseline: 4f0981a4 (ancestor of base)
required tags: g1-merchant-baseline, g2-catalog-baseline, g3-order-baseline, media-purpose-takeout, infra-automigrate-retired
migrations 016/017/018/019: present
pre-development migration 020: absent
backend/cmd/server db.AutoMigrate count: 0
main worktree: clean on master
```

## 2. 真实 MySQL 8.4

执行方式（DSN 已脱敏）：

```powershell
cd backend
$env:TAKEOUT_G5_MYSQL_DSN = '<redacted real MySQL DSN>'
go test ./internal/module/takeout -run '^Test00RiderMigrationRollbackApplyAgainstRealMySQL84$' -count=1 -v
go test ./internal/module/takeout -run '^(TestRiderLifecycleClaimLimitsAndDeliveryAgainstRealMySQL84|TestRiderConcurrentClaimAgainstRealMySQL84|TestRiderCrossPointRollbackAgainstRealMySQL84|TestZZRiderAcceptanceResidueAgainstRealMySQL84)$' -count=1 -v
```

脱敏结果：

```text
=== RUN   Test00RiderMigrationRollbackApplyAgainstRealMySQL84
real_mysql_version=8.4.0
migration_020_apply_rollback_apply=PASS tables=3 foreign_keys=0 rider_config=global:1000 g3_order_unchanged=PASS
--- PASS

=== RUN   TestRiderLifecycleClaimLimitsAndDeliveryAgainstRealMySQL84
rider_lifecycle=PASS fulfillment_linear=PASS stale_CAS=REJECTED idempotency=PASS
claim_order_same_tx=PASS ready_pool=PASS configurable_limit=1_reject_then_1000_accept delivery_crosspoint=PASS earning_count=1
--- PASS

=== RUN   TestRiderConcurrentClaimAgainstRealMySQL84
same_rider_two_orders_lock=PASS success=1 in_flight_limit=1
multi_rider_same_order=PASS success=1 already_claimed=1 winning_rider=<redacted test rider>
--- PASS

=== RUN   TestRiderCrossPointRollbackAgainstRealMySQL84
inactive_rider_claim=REJECTED order=ready_for_pickup fulfillment_count=0
complete_crosspoint_failure_rollback=PASS fulfillment=TO_USER order=ready_for_pickup
--- PASS

=== RUN   TestZZRiderAcceptanceResidueAgainstRealMySQL84
rider_residue riders=0 fulfillments=0 orders=0 items=0 earnings=0 dishes=0 rider_config=global:1000:rv1
--- PASS
```

## 3. 数据库契约

```text
takeout_riders columns=8
takeout_fulfillments columns=13
takeout_rider_config columns=5
rider status CHECK=active/resting/suspended
fulfillment status CHECK=ASSIGNED/TO_STORE/PICKED/TO_USER/DELIVERED
uq_takeout_rider_user present
uq_takeout_fulfillment_order present
idx_takeout_fulfillment_rider_status=(rider_user_id,status)
cross-domain/all foreign keys=0
rider config seed=global,1000,row_version=1
G-3 order status CHECK unchanged
```

迁移 SHA-256：

```text
020_takeout_rider.sql
b426c985b37c06cfdcc5a4512b0fc07f4a6a790967b901e901e50013f8d3ac1d

rollback/020_takeout_rider.sql
4ea912ca2a65a91a9ecedf9ee2b44735c20919ce49aa5bba1cf652ffd0cc6304
```

## 4. 并发与原子性

```text
same rider + two ready orders + limit=1:
  SELECT rider FOR UPDATE serializes both claims
  result: one success, one ErrInFlightLimit

two riders + same ready order:
  independent rider locks
  G-3 order CAS selects winner
  result: one success, one ErrOrderAlreadyClaimed
  fulfillment count=1; order rider matches winner

CompleteFulfillment failure injection:
  fulfillment CAS attempted TO_USER->DELIVERED
  G-3 CompleteDelivery rejects ready_for_pickup order
  outer transaction rollback
  final fulfillment=TO_USER; final order=ready_for_pickup
```

## 5. 配置治理与边界

```text
production hardcoded max 1000 count=0
config update stale row_version=REJECTED
limit=1 rejects second active fulfillment
limit raised to 1000 allows subsequent claim
existing fulfillment history unchanged

production Rider A-F/Media imports=0
production Rider HTTP/route imports=0
production Rider float/decimal usage=0
migration 020 SQL foreign keys=0
apps/** changes=0
001-019 migration changes=0
AutoMigrate count=0
```

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

MiniApp 仅保留既有 `vendors.js` 体积提示；G-⑤ 没有前端变更。证据不包含 DSN、口令、token 或云凭证。
