# G-④ Payment Acceptance Evidence

## 1. Preflight evidence

```text
git log --oneline -1 master
e80fe1bb fix(dev): improve local mock login and media visibility

git tag --list dev-local-experience-baseline g7a-buyer-api-baseline g7c-rider-api-baseline
dev-local-experience-baseline
g7a-buyer-api-baseline
g7c-rider-api-baseline

rg "db\.AutoMigrate\(" backend/cmd/server
0 matches

latest migration before G-4:
021_takeout_withdraw.sql
```

## 2. Real MySQL 8.4 migration evidence

```text
=== RUN   Test00PaymentMigrationRollbackApplyAgainstRealMySQL84
real_mysql_version=8.4.0
migration_022_apply_rollback_apply=PASS tables=2 foreign_keys=0
--- PASS: Test00PaymentMigrationRollbackApplyAgainstRealMySQL84
```

## 3. Real HTTP + MySQL mock payment evidence

```text
=== RUN   TestTakeoutPaymentMockRealHTTPMySQLPaidAndDownstream
real_http_mysql=PASS mock_payment_pending_to_paid=PASS server_amount_used=PASS downstream_merchant_rider_chain=PASS
--- PASS: TestTakeoutPaymentMockRealHTTPMySQLPaidAndDownstream
```

Verified path:

```text
buyer create order
-> pending_payment
-> POST /api/v1/takeout/buyer/orders/:orderPublicId/pay
-> mock provider
-> payment_attempt(provider=mock,status=paid)
-> order paid
-> merchant accept
-> merchant start-preparing
-> merchant mark-ready
-> rider register
-> rider claim
-> rider to-store/picked/to-user/complete
```

## 4. Regression evidence

```text
go test ./... -count=1
PASS

npm run verify
PASS

npm run build
PASS
```

MiniApp build warning observed:

```text
vendors.js exceeds recommended size limit
```

This is an existing bundle-size warning and is not introduced by G-④ payment logic.

## 5. Boundary evidence

```text
npm run check:boundaries
ok A-G boundaries plus isolated P0-4 media domain
```

## 6. Dependency note

`npm ci` was run in the isolated G-4 worktree because the worktree had no `node_modules`. It did not modify `package.json` or `package-lock.json`. npm audit reported existing dependency vulnerabilities; dependency upgrades are outside this authorization.

