# G-7a Buyer API Acceptance Evidence

## Commands

```powershell
go test ./internal/module/admin/takeoutbuyer ./internal/module/takeout -count=1
npm run check:boundaries
go test ./... -count=1
$env:MYSQL_DSN=<redacted from owner backend/.env>; go test ./internal/module/admin/takeoutbuyer -run 'TestTakeoutBuyerAPIRealHTTPMySQLPublicVisibilityOrderAndOwnership|TestZZTakeoutBuyerHTTPResidueAgainstMySQL84' -count=1 -v
npm run verify
npm run build
```

## Real HTTP + real MySQL evidence

```text
=== RUN   TestTakeoutBuyerAPIRealHTTPMySQLPublicVisibilityOrderAndOwnership
    http_mysql_test.go:157: real_http=PASS auth=401 buyer_visibility=active/on_shelf only image_display_path=PASS server_amount_recompute=PASS buyer_order_scope=PASS no_payment_routes=PASS
--- PASS: TestTakeoutBuyerAPIRealHTTPMySQLPublicVisibilityOrderAndOwnership (0.14s)
=== RUN   TestZZTakeoutBuyerHTTPResidueAgainstMySQL84
--- PASS: TestZZTakeoutBuyerHTTPResidueAgainstMySQL84 (0.00s)
PASS
ok  	campus-circle/backend/internal/module/admin/takeoutbuyer	1.649s
```

The GORM `record not found` lines during this test are expected fail-closed probes for unauthenticated/non-active visibility cases.

## Full backend regression

```text
go test ./... -count=1
PASS for all backend packages, including:
ok  	campus-circle/backend/internal/module/admin/takeoutbuyer
ok  	campus-circle/backend/internal/module/takeout
ok  	campus-circle/backend/internal/module/media
ok  	campus-circle/backend/internal/module/user
```

## Verify

```text
npm run verify
ok A-G boundaries plus isolated P0-4 media domain
admin typecheck PASS
miniapp typecheck PASS
backend tests PASS
```

## Build

```text
npm run build
miniapp build PASS with existing vendors.js size warning
admin build PASS
```

## Boundary evidence

`npm run check:boundaries`:

```text
ok A-G boundaries plus isolated P0-4 media domain
```

No new `db.AutoMigrate(...)` call was introduced.
