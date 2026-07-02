# D-6b Delivery Admin Console API Development Report

Date: 2026-07-02

Branch: `feat/d6b-delivery-admin-console`

Baseline: `631cb128 feat(D-6a): add delivery fee config and rider wallet settlement core`

Final commit: recorded in final handoff after commit creation.

## Scope

D-6b implements the administrator HTTP API exposure layer for the already-frozen D-6a delivery fee/rider wallet core and D-3 merchant wallet/refund facts.

Implemented only the authorized backend API layer:

- New package: `backend/internal/module/admin/deliveryconsole`
- Main composition root registration in `backend/cmd/server/main.go`
- Boundary registration in `scripts/check-boundaries.cjs`
- Tests for handler behavior, boundary imports, and real MySQL HTTP acceptance

No frontend, no migration, no delivery domain state machine change, no core fund-rule rewrite.

## Pre-flight gate results

- `git log --oneline -1 master` => `631cb128 feat(D-6a): add delivery fee config and rider wallet settlement core`
- `git tag --list d6a-delivery-fee-wallet-baseline` => present
- `rg "db\.AutoMigrate\(" backend/cmd/server` => 0 matches
- Worktree branch => `feat/d6b-delivery-admin-console`
- Main repository worktree remained clean during D-6b worktree development

## File-level verification conclusions

- Admin authentication exists in `user.AuthRequired()` + `user.AdminRequired()`; admin role accepts `admin` and `super_admin`, ordinary users are rejected with 403.
- Existing `/api/v1/admin/delivery` group already hosts delivery admin routes in `main.go`; D-6b registers on the same group.
- Existing admin delivery packages use thin handler/composition packages under `backend/internal/module/admin/*`; D-6b follows that pattern with `admin/deliveryconsole`.
- D-6a fee repository already exposes `GlobalFeeConfig`, `UpdateGlobalFeeConfig`, `UpsertMerchantFeeOverride`, and fee policy methods with validation and CAS.
- D-6a rider wallet repository already exposes wallet/earning/payout behaviors, including `MarkRiderEarningPaidOffline`.
- D-3 delivery payment facts already provide merchant wallet, ledger, settlement, payment, and refund tables for read exposure.

## API contract added

All routes are mounted under `/api/v1/admin/delivery` and inherit `AuthRequired + AdminRequired`.

- `GET /summary`
- `GET /orders`
- `GET /orders/:id`
- `GET /orders/:id/funds`
- `GET /fee-config`
- `PUT /fee-config`
- `GET /merchant-fee-overrides`
- `GET /merchant-fee-overrides/:merchantUserId`
- `PUT /merchant-fee-overrides/:merchantUserId`
- `PATCH /merchant-fee-overrides/:merchantUserId/disable`
- `GET /merchant-wallets`
- `GET /merchant-wallets/:merchantUserId`
- `GET /merchant-wallets/:merchantUserId/ledger`
- `GET /rider-wallets`
- `GET /rider-wallets/:riderUserId`
- `GET /rider-wallets/:riderUserId/ledger`
- `GET /rider-earnings`
- `GET /rider-earnings/:id`
- `POST /rider-earnings/:id/pay-offline`
- `GET /rider-payouts`
- `GET /refunds`
- `GET /refunds/:id`

## Core design notes

- `deliveryconsole.Handler` receives interfaces/repositories from the composition root.
- Configuration writes call D-6a `FeeConfigRepository`; admin layer does not recalculate or bypass fee validation.
- Rider offline payout calls D-6a `RiderWalletRepository.MarkRiderEarningPaidOffline`; admin layer does not mutate wallet/earning/payout tables directly.
- Admin read queries use `GormQueryRepository` for read-only console projections and simple dashboard aggregation.
- Merchant fee override disable is the only admin-console repository write; it is a console management operation using row-version CAS and status transition to `disabled`.
- Delivery core package was not modified.

## Boundary notes

`scripts/check-boundaries.cjs` now includes a `deliveryConsoleSource` check to keep non-test `admin/deliveryconsole` code from importing unrelated domain packages.

`backend/internal/module/admin/deliveryconsole/boundary_test.go` asserts that non-test deliveryconsole files do not import forbidden domain packages.

## Verification summary

- Real MySQL 8.4 HTTP acceptance: PASS
- Admin auth 401/403: PASS
- Fee config CAS and invalid amount rejection: PASS
- Merchant override snapshot behavior: PASS
- Rider earning paid-offline idempotency: PASS
- Residue check: PASS
- `go test ./... -count=1`: PASS
- `npm run check:boundaries`: PASS
- `npm run verify`: PASS
- `npm run build`: PASS, with existing Taro vendor-size warning only

