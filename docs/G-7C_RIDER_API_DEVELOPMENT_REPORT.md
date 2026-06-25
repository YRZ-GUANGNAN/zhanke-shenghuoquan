# G-⑦c Rider API Development Report

Date: 2026-06-25

## Scope

Implemented only the rider role transport adapter for domain G:

- New package: `backend/internal/module/admin/takeoutrider`
- New route group: `/api/v1/takeout/rider/*`
- Minimal composition-root wiring in `backend/cmd/server/main.go`

No takeout core domain code, migrations, buyer/merchant/admin transports, frontend, A-F domains, or media code were changed.

## Pre-gate result

- `master`: `d9cab2f9 chore: ignore local Meilisearch runtime artifacts (data.ms/, meilisearch.exe)`
- Parent chain includes `c7248b83 fix(dev): enable local mock WeChat login without media safety credentials`
- Required tags present:
  - `dev-mock-login-baseline`
  - `g7a-buyer-api-baseline`
  - `g7b-merchant-api-baseline`
- `backend/cmd/server` AutoMigrate matches: `0`
- Main workspace was clean before branch creation.

## File-level contract verification

- Rider registration uses `RiderService.RegisterRider(ctx, RegisterRiderInput)` with `RiderUserID`, `RealName`, `Phone`; created rider is `active` with `row_version=1`.
- Rider identity is derived from `takeout_riders.rider_user_id == current token user id`.
- Claiming uses `RiderService.ClaimOrder(ctx, riderUserID, orderPublicID, expectedOrderRowVersion)`.
- Claim concurrency is handled by the domain layer: rider row lock + active check + in-flight limit + fulfillment existence check + `StartDelivering` CAS + `takeout_fulfillments.order_public_id` unique key.
- Claimable pool is `ListClaimableOrders(ctx)` → `ListOrdersByStatus(ready_for_pickup)`.
- Fulfillment transitions are `ASSIGNED -> TO_STORE -> PICKED -> TO_USER -> DELIVERED`.
- Fulfillment transition methods do not receive caller rider id, so transport owner validation was added before every fulfillment mutation.
- Withdrawal domain service only validates nonzero amount; transport balance validation was added before `CreateWithdrawRequest`.
- Withdrawal list in the core service is admin/global; rider transport uses rider-scoped repository queries.

## Implemented endpoints

Mounted under `/api/v1/takeout/rider` with `AuthRequired`.

- `GET /me`
- `POST /register`
- `GET /claimable-orders`
- `POST /orders/:orderPublicId/claim`
- `GET /fulfillments`
- `GET /fulfillments/:fulfillmentPublicId`
- `PATCH /fulfillments/:fulfillmentPublicId/to-store`
- `PATCH /fulfillments/:fulfillmentPublicId/picked`
- `PATCH /fulfillments/:fulfillmentPublicId/to-user`
- `PATCH /fulfillments/:fulfillmentPublicId/complete`
- `GET /wallet`
- `GET /withdrawals`
- `POST /withdrawals`

## Security controls added in transport

- Cross-rider fulfillment detail and mutation are rejected as not found.
- Every fulfillment transition checks `currentUserID == fulfillment.rider_user_id` before calling the domain transition.
- Wallet and withdrawal list are scoped to current rider only.
- Withdrawal creation checks `amount_cents <= pending_settlement_earnings - pending_withdrawals`.
- Claim CAS/concurrency failures map to HTTP 409.

## Files changed

- `backend/cmd/server/main.go`
- `backend/internal/module/admin/takeoutrider/handler.go`
- `backend/internal/module/admin/takeoutrider/http_mysql_test.go`
- `backend/internal/module/admin/takeoutrider/repository.go`
- `backend/internal/module/admin/takeoutrider/service.go`
- `backend/internal/module/admin/takeoutrider/types.go`

## Notes

`npm ci` was run in the isolated worktree to install ignored `node_modules` for frontend typecheck/build. No dependency upgrade or package contract change was made.
