# G-⑥a-1 Acceptance Evidence

## 基线与门禁

```text
master=87de169a docs(G-5): record rider domain acceptance
required_tags=PASS
migration_020=present
migration_021_before_development=absent
automigrate_count=0
main_worktree_clean=PASS
```

## Boundary 早期通路验证

```text
> npm run check:boundaries
ok A-G boundaries plus isolated P0-4 media domain
```

结论：`admin/takeoutconsole` 导入 `takeout` 符合既有 Admin 跨域组合范式，无需修改白名单。

## 真实 MySQL 迁移

```text
=== RUN   Test00WithdrawMigrationRollbackApplyAgainstRealMySQL84
real_mysql_version=8.4.0
migration_021_apply_rollback_apply=PASS table=takeout_withdraw_requests foreign_keys=0
--- PASS
```

## 提现 CAS、原子结算与故障回滚

```text
withdraw_CAS=PASS settlement_atomic=PASS repeated_paid=idempotent
withdraw_failure_injection=PASS transaction_rollback=PASS partial_settlement=0
withdraw_residue withdrawals=0 earnings=0
```

故障注入位置：第二条 `takeout_rider_earning_records` UPDATE。失败后：

```text
withdraw.status=pending
withdraw.row_version=1
withdraw.paid_at=NULL
settled earning records=0
```

## 真实 HTTP 与鉴权

测试方法：`httptest.NewServer` + 真实 Gin Router + `AuthRequired` + `AdminRequired` + 真实 MySQL 8.4。

```text
real_http=PASS
auth=401/403/200
paginated_lists=PASS
config_CAS=PASS
withdraw_payment_rejection=PASS
repeat_paid=idempotent
```

分页结果逐项核对真实种子公共 ID/骑手 ID：

```text
merchants=PASS
categories=PASS
dishes=PASS
orders(status=pending_payment)=PASS
riders(status=active)=PASS
settlement-report=PASS
withdraw-requests(status=pending)=PASS
```

## 回归

```text
go test ./... -count=1        PASS
go vet ./...                  PASS
go build ./...                PASS
npm run check:boundaries      PASS
Admin typecheck               PASS
MiniApp typecheck             PASS
npm run verify                PASS
npm run build                 PASS
```

MiniApp 构建仅出现既有 `vendors.js` 体积提示；本模块没有 Apps 变更。

## 最终边界与残留

```text
takeout_http_crossdomain=0
new_F_review_refs=0
automigrate=0
float_decimal=0
old_migration_changes=0
apps_or_boundary_changes=0
http_residue users=0 merchants=0 categories=0 dishes=0 orders=0 riders=0 withdrawals=0 earnings=0 configs=baseline
```

证据未包含 DSN、密码、token 或云凭证。
