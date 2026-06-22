# G-⑥a-ui Takeout Admin Acceptance Evidence

日期：2026-06-23

## 契约补丁真实 MySQL + HTTP

```text
real_mysql_version=8.4.0
legacy_dish_query_without_category=PASS total=2
category_filter=PASS total=1
order_stage_timestamps=PASS paidAt returned
contract_patch_residue merchants=0 categories=0 dishes=0 orders=0 users=0
```

## 真实浏览器链路

```text
merchant_suspend_unsuspend=PASS
category_id_filter=PASS total=1
dish_category_details=PASS
order_items_and_stage_timestamps=PASS
rider_active_resting_active=PASS
fee_config_success_refetch=PASS rowVersion=v2
external_cas_update=PASS new_row_version=3
fee_config_stale_cas_409=PASS refresh_action=PASS
rider_config_success_refetch=PASS rowVersion=v2
settlement_summary=PASS
withdrawal_mark_paid=PASS rowVersion_advanced=PASS
browser_console_errors_and_warnings=0
```

其中 stale CAS 的页面原值为 v2，真实 HTTP 并发写推进至 v3，页面随后用 v2 提交，服务端返回 409；UI 明示“数据已被修改，请刷新后重试”并成功刷新至 v3。

## 全量回归

```text
go test ./... -count=1: PASS
npm run check:boundaries: ok A-G boundaries plus isolated P0-4 media domain
Admin typecheck: PASS
MiniApp typecheck: PASS
npm run verify: PASS
npm run build: PASS
go build ./...: PASS
go vet ./...: PASS
```

## 清理

```text
browser_fixture_residue=0 configs=restored
test HTTP listeners=0
temporary credential files=0
temporary source files=0
```
