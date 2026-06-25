# G-④ Payment Scaffold + Mock Payment Acceptance Report

Status: PASS  
Branch: `feat/g4-payment`  
Final commit: recorded in handoff response after commit creation

## 1. Acceptance matrix

| Item | Result | Evidence |
| --- | --- | --- |
| Migration 022 apply/rollback/apply | PASS | Real MySQL 8.4 test passed |
| No cross-domain FK | PASS | Schema assertion passed |
| AutoMigrate remains retired | PASS | `rg "db\.AutoMigrate\(" backend/cmd/server` returned 0 matches |
| Payment records domain model | PASS | `go test ./internal/module/takeout` |
| mock payment pending -> paid | PASS | Real HTTP + MySQL G-4 test |
| Downstream merchant/rider chain after mock paid | PASS | Real HTTP + MySQL G-4 test |
| Server-side amount source | PASS | G-4 test rejects frontend amount influence by using order total |
| Production mock payment hard block | PASS | `config.Validate()` tests |
| mock payment independent switch | PASS | `ALLOW_MOCK_PAYMENT` added; no reuse of mock login switch |
| WeChat Pay provider skeleton | PASS | Provider unit tests + compile |
| Callback parse/confirmation skeleton | PASS | Service/provider code paths compile; notification confirmation unit test covers idempotent core |
| MiniApp payment button | PASS | `npm run verify`, `npm run build` |
| Boundary check | PASS | `npm run check:boundaries` |
| Full backend regression | PASS | `go test ./... -count=1` |
| Residue | PASS | G-4 residue test passed |

## 2. Real MySQL evidence summary

Database source: local `backend/.env` `MYSQL_DSN`, value not printed.  
MySQL version observed: `8.4.0`.

Commands:

```powershell
go test ./internal/module/takeout -run '^Test00PaymentMigrationRollbackApplyAgainstRealMySQL84$' -count=1 -v
go test ./internal/module/admin/takeoutpayment -run 'TestTakeoutPaymentMockRealHTTPMySQLPaidAndDownstream|TestZZTakeoutPaymentResidueAgainstMySQL84' -count=1 -v
```

Results:

- `migration_022_apply_rollback_apply=PASS tables=2 foreign_keys=0`
- `real_http_mysql=PASS mock_payment_pending_to_paid=PASS server_amount_used=PASS downstream_merchant_rider_chain=PASS`
- `TestZZTakeoutPaymentResidueAgainstMySQL84`: PASS

## 3. Production safety conclusion

PASS. mock payment is isolated by a dedicated switch and production validation:

- `ALLOW_MOCK_PAYMENT` is independent from `ALLOW_MOCK_WECHAT_LOGIN`.
- production rejects mock payment.
- non-prod rejects mock payment when any real payment credential is present.
- frontend never sets `paid`; only the server-side payment provider path calls `MarkPaid`.

## 4. Remaining risks

| Risk | Level | Notes |
| --- | --- | --- |
| Real WeChat Pay merchant joint test not run | Medium | Merchant account and real credentials are not available yet. |
| Platform public key/certificate handling needs real environment validation | Medium | Skeleton supports configured platform public key path; live verification remains pending. |
| Refund provider integration not implemented | Low | Explicitly out of scope for this module. |

## 5. Final conclusion

G-④ payment scaffold + local mock payment reaches PASS for the authorized scope. It enables local end-to-end order flow without SQL shortcuts while keeping production mock payment fail-closed.

