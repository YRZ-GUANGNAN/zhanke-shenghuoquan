# G-7c UI Rider Development Report

Date: 2026-06-25  
Branch: `feat/g7c-ui-rider`  
Base: `27044b07 feat(G-7c): add rider role API transport`  
Final commit: see `git log --oneline -1` and the owner handoff message

## Scope

This module implements the WeChat MiniApp rider role UI only.

Allowed scope:

- `apps/miniapp/src/**`
- `docs/**`

Explicitly untouched:

- `backend/**`
- database migrations
- `backend/cmd/server/main.go`
- package manifests and dependency graph
- BottomNav route set
- G-7a buyer pages and G-7b merchant behavior

## Pre-gate verification

- `git log --oneline -1 master` returned `27044b07 feat(G-7c): add rider role API transport`.
- Required tags exist:
  - `g7a-ui-buyer-baseline`
  - `g7b-ui-merchant-baseline`
  - `g7c-rider-api-baseline`
- Main worktree was clean before development.

## File-level contract verification

### Backend rider transport contract

Verified from `backend/internal/module/admin/takeoutrider/handler.go` and `types.go`.

Base route is registered by the server at:

- `/api/v1/takeout/rider`

Endpoints consumed by the MiniApp page:

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

CAS request body:

```json
{ "expectedRowVersion": 1 }
```

Withdrawal request body:

```json
{ "amountCents": 100 }
```

Key response DTOs verified:

- rider identity: `registered`, `rider`
- rider profile: `riderUserId`, `status`, `realName`, `phone`, `rowVersion`
- claimable orders: `order`, `items`, order `rowVersion`
- fulfillments: `fulfillment`, `order`, `items`
- wallet: `availableWithdrawCents`, `pendingSettlementCents`, `pendingWithdrawCents`, `settledCents`, `recentEarnings`
- withdrawals: `withdrawPublicId`, `amountCents`, `status`

### Existing frontend patterns

The implementation follows the existing merchant and buyer MiniApp patterns:

- API client style from `apps/miniapp/src/services/g-takeout-merchant.ts`
- token/signature envelope through `apps/miniapp/src/services/api.ts`
- profile-page role entry pattern from `apps/miniapp/src/pages/profile/index.tsx`
- single-page tab style from `apps/miniapp/src/pages/takeout-merchant/index.tsx`
- no new state library; React local state is sufficient

## Implementation

### New MiniApp API client

Added:

- `apps/miniapp/src/services/g-takeout-rider.ts`

Capabilities:

- rider registration and identity loading
- claimable order listing
- claim with `expectedRowVersion`
- fulfillment listing and four lifecycle transitions
- wallet loading
- withdrawal listing and submission
- money formatting helper `centsToYuan`

### New rider page

Added:

- `apps/miniapp/src/pages/takeout-rider/index.tsx`
- `apps/miniapp/src/pages/takeout-rider/index.scss`

Page functions:

- unregistered state: real name and phone registration form
- registered state: rider status card
- tabs:
  - claim pool
  - my deliveries
  - wallet
- claim pool:
  - lists ready-for-pickup orders
  - sends claim request with order `rowVersion`
  - maps 409/concurrency failures to a friendly "already claimed or state changed" message
- delivery:
  - filters by fulfillment status
  - advances `ASSIGNED -> TO_STORE -> PICKED -> TO_USER -> DELIVERED`
  - each transition sends the current fulfillment `rowVersion`
- wallet:
  - shows available, pending settlement, pending withdrawal, settled amounts
  - submits withdrawal in cents
  - maps insufficient-balance errors to a friendly message

### Profile entry and route registration

Updated:

- `apps/miniapp/src/pages/profile/index.tsx`
  - added `Rider Center` row under the existing `Merchant Center` row
- `apps/miniapp/src/app.config.ts`
  - registered `pages/takeout-rider/index`

BottomNav was not changed.

## Boundary result

- No backend files changed.
- No migration files changed.
- No package manifest changed.
- No new dependencies added.
- No BottomNav rider tab added.
- Rider center is reachable only through Profile page.
