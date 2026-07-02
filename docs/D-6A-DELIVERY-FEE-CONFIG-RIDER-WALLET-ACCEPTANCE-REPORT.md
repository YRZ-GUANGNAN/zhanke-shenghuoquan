# D-6a Delivery Fee Config + Rider Wallet Backend Acceptance Report

Status: PASS  
Branch: `feat/d6a-delivery-fee-wallet`  
Final commit: see delivery handoff `git log` output.

## Acceptance matrix

| Requirement | Result | Evidence |
| --- | --- | --- |
| Migration 028 apply/rollback/apply | PASS | `TestD6A00FeeWalletMigrationRollbackApplyAgainstRealMySQL84` |
| Rollback fail-closed with rider money facts | PASS | `TestD6ARollbackGuardRejectsMoneyFactsAgainstRealMySQL84` |
| No cross-domain FK | PASS | `assertD6ASchema` information_schema check |
| Global fee config used for new order | PASS | `TestD6AFeePolicyGlobalOverrideAndSnapshotAgainstRealMySQL84` |
| Merchant override wins over global config | PASS | same test |
| Old order snapshots do not drift after config change | PASS | same test |
| `main.go` no longer hardcodes delivery fee | PASS | production wiring uses `NewDispatchServiceWithFeeProvider(... repository ...)` |
| Normal delivered path credits rider | PASS | `TestD6ARiderEarningSettlementNormalProofIdempotencyAndPayoutAgainstRealMySQL84` |
| D-4 proof-photo delivered path credits rider | PASS | same test |
| Rider earning idempotency by delivery order | PASS | same test |
| Rider wallet CAS / balance accumulation | PASS | same test and delivery package real MySQL suite |
| Rider ledger append-only income/payout | PASS | same test |
| Offline payout registration only records status and ledger | PASS | same test |
| D-2/D-3/D-4/D-5 existing behavior preserved | PASS | full delivery package real MySQL suite |
| AutoMigrate still retired | PASS | `rg "db\.AutoMigrate\(" backend/cmd/server` zero matches |
| Boundary check | PASS | `npm run check:boundaries` |
| Full backend tests | PASS | `go test ./... -count=1` |
| Root verify | PASS | `npm run verify` |
| Root build | PASS | `npm run build` |

## Real MySQL evidence

The D-6a tests were run against the local real MySQL 8.4 DSN from `backend/.env` by setting `DELIVERY_D1_MYSQL_DSN` for the test process. Credentials were not printed or committed.

Commands:

```powershell
$env:DELIVERY_D1_MYSQL_DSN = <backend/.env MYSQL_DSN>
go test ./internal/module/delivery -run 'TestD6A' -count=1 -v
go test ./internal/module/delivery -count=1 -v
```

Observed PASS highlights:

- `real_mysql_version=8.4.0`
- `migration_028_apply_rollback_apply=PASS`
- `d6a_fee_policy=PASS global+merchant_override+snapshot_immutability`
- `d6a_rider_settlement=PASS normal_delivered+proof_delivered+idempotent+wallet_CAS+ledger+payout`
- `migration_028_rollback_guard=PASS money_facts_fail_closed`

## Full verification evidence

```powershell
go test ./... -count=1
npm run check:boundaries
npm run verify
npm run build
```

Results:

- backend `go test ./... -count=1`: PASS
- `npm run check:boundaries`: PASS
- `npm run verify`: PASS
- `npm run build`: PASS

Build emitted the existing Taro `vendors.js` size warning only; build completed successfully.

## Compatibility notes

D-6a adds a delivered-time side effect: rider earning settlement. This is intentional and backward-compatible. Existing D-2/D-3/D-4/D-5 flows still pass; delivered now also creates exactly one rider earning and ledger income entry.

The legacy takeout rider wallet remains untouched and sealed.
