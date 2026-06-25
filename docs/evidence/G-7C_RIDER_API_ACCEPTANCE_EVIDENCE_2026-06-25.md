# G-⑦c Rider API Acceptance Evidence

Date: 2026-06-25

## Real MySQL rider HTTP acceptance

Command:

```powershell
$env:MYSQL_DSN = '<redacted real MySQL DSN from backend/.env>'
go test ./internal/module/admin/takeoutrider `
  -run '^(TestTakeoutRiderAPIRealHTTPMySQLConcurrencyOwnerWalletAndResidue|TestZZTakeoutRiderHTTPResidueAgainstMySQL84)$' `
  -count=1 -v
```

Key output:

```text
real_mysql_version=8.4.0
real_http=PASS
rider_register=PASS
concurrent_claim_200_409=PASS
in_flight_limit=PASS
fulfillment_owner_scope=PASS
delivery_completion_earning_idempotent=PASS
wallet_balance_guard=PASS
withdrawals_owner_scope=PASS
PASS TestTakeoutRiderAPIRealHTTPMySQLConcurrencyOwnerWalletAndResidue
PASS TestZZTakeoutRiderHTTPResidueAgainstMySQL84
```

## Boundary check

Command:

```powershell
npm run check:boundaries
```

Output:

```text
ok A-G boundaries plus isolated P0-4 media domain
```

## Backend full regression

Command:

```powershell
go test ./... -count=1
```

Result:

```text
PASS for all backend packages, including:
ok campus-circle/backend/cmd/server
ok campus-circle/backend/internal/module/admin/takeoutrider
ok campus-circle/backend/internal/module/takeout
```

## Full workspace verification

Command:

```powershell
npm run verify
```

Result:

```text
check:boundaries PASS
admin typecheck PASS
miniapp typecheck PASS
backend tests PASS
```

## Build

Command:

```powershell
npm run build
```

Result:

```text
miniapp build PASS
admin build PASS
```

Known warning:

```text
vendors.js exceeds the recommended size limit
```

This is the existing Taro/Webpack bundle-size warning and is not introduced by G-⑦c backend.
