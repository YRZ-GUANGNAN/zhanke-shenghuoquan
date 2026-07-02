# D-6b Delivery Admin Console API Acceptance Report

Date: 2026-07-02

Branch: `feat/d6b-delivery-admin-console`

Baseline: `631cb128 feat(D-6a): add delivery fee config and rider wallet settlement core`

Final commit: recorded in final handoff after commit creation.

## Acceptance result

PASS.

D-6b adds the administrator Delivery Console backend API as an exposure layer only. The implementation preserves delivery core fund logic, row-version CAS semantics, existing D-6a/D-3 rules, and existing delivery behavior.

## Real HTTP + MySQL evidence

Command:

```powershell
$env:MYSQL_DSN = <loaded from backend/.env without printing secrets>
go test ./internal/module/admin/deliveryconsole -run 'TestDeliveryConsoleAdminHTTPAgainstRealMySQL84|TestZZDeliveryConsoleResidueAgainstRealMySQL84' -count=1 -v
```

Result:

```text
--- PASS: TestDeliveryConsoleAdminHTTPAgainstRealMySQL84
d6b_real_http_mysql=PASS auth_401_403=PASS fee_config_CAS=PASS override_snapshot=PASS rider_wallet_earning_pay_offline_idempotent=PASS
--- PASS: TestZZDeliveryConsoleResidueAgainstRealMySQL84
d6b_residue=PASS scoped_rows=0
PASS
ok campus-circle/backend/internal/module/admin/deliveryconsole
```

Covered:

- No token => 401
- Ordinary logged-in user => 403
- Admin user => allowed
- Global fee config GET/PUT
- CAS stale row version => 409
- Invalid amount rule `delivery_fee > 0 && platform_cut < delivery_fee` => 400
- Merchant fee override upsert and list
- New order after global config update uses new fee snapshot
- Merchant override order uses override snapshot
- Delivered order generates rider earning
- Admin paid-offline operation is idempotent and does not represent real platform payout
- Admin order/funds/wallet/payout/summary reads return successfully
- Scoped residue = 0

## Backend regression evidence

Commands:

```powershell
go test ./internal/module/admin/deliveryconsole ./cmd/server
go test ./... -count=1
```

Results:

```text
ok campus-circle/backend/internal/module/admin/deliveryconsole
ok campus-circle/backend/cmd/server
```

```text
go test ./... -count=1 PASS
```

All backend packages passed.

## Boundary and full verify evidence

Commands:

```powershell
npm run check:boundaries
npm run verify
```

Results:

```text
ok A-G boundaries plus independent delivery and isolated P0-4 media domains
npm run verify PASS
```

`npm run verify` covered:

- `check:boundaries`
- admin typecheck
- miniapp typecheck
- backend tests

The D-6b worktree did not contain dependency folders, so a temporary junction to the main repository `node_modules` was used for npm verification and removed immediately afterward. The junction was not committed.

## Build evidence

Command:

```powershell
npm run build
```

Result:

```text
build:miniapp PASS
build:admin PASS
```

The miniapp build emitted the existing `vendors.js` size warning. This is a pre-existing bundle-size warning and not a D-6b functional failure.

## AutoMigrate evidence

Command:

```powershell
rg "db\.AutoMigrate\(" backend/cmd/server
```

Result:

```text
0 matches
```

Startup schema mutation remains retired.

## Core fund logic preservation

Confirmed by diff and implementation:

- No delivery core state machine changed.
- No delivery core financial method rewritten.
- No migration added.
- Admin fee config writes call D-6a repository methods.
- Admin rider settlement calls D-6a rider wallet method.
- Admin layer does not calculate or mutate wallet/earning/payout core rules directly.

## API contract for D-6c frontend

D-6c can consume these stable admin endpoints under `/api/v1/admin/delivery`:

- dashboard: `GET /summary`
- orders: `GET /orders`, `GET /orders/:id`, `GET /orders/:id/funds`
- global fee config: `GET /fee-config`, `PUT /fee-config`
- merchant overrides: `GET /merchant-fee-overrides`, `GET /merchant-fee-overrides/:merchantUserId`, `PUT /merchant-fee-overrides/:merchantUserId`, `PATCH /merchant-fee-overrides/:merchantUserId/disable`
- merchant wallets: `GET /merchant-wallets`, `GET /merchant-wallets/:merchantUserId`, `GET /merchant-wallets/:merchantUserId/ledger`
- rider wallets: `GET /rider-wallets`, `GET /rider-wallets/:riderUserId`, `GET /rider-wallets/:riderUserId/ledger`
- rider earnings/payouts: `GET /rider-earnings`, `GET /rider-earnings/:id`, `POST /rider-earnings/:id/pay-offline`, `GET /rider-payouts`
- refunds: `GET /refunds`, `GET /refunds/:id`

All are admin-only.

## Final status

D-6b backend API is ready for owner review and later fast-forward merge. D-6c frontend remains out of scope.

