# D-2 Delivery Dispatch Frontend Acceptance Report

Date: 2026-06-29  
Branch: `feat/d2-delivery-frontend`  
Implementation commit: `5e958a3`  
Conclusion: **PASS**

## 1. Acceptance matrix

| Gate | Result | Evidence |
|---|---|---|
| Frontend-only scope | PASS | Backend diff 0; no migration or package manifest changes |
| Independent delivery pages | PASS | `/pages/delivery-merchant/index` and `/pages/delivery-rider/index` |
| Profile role entries | PASS | Both entries are controlled by `TARO_APP_ENABLE_DELIVERY`; BottomNav unchanged |
| Delivery flag default | PASS | Build config defaults to enabled unless explicitly `false` |
| Mock fee flag default | PASS | Build config enables only for exact `true`; default is hidden |
| Merchant idempotency | PASS | One 26-character request id is reused until create succeeds and is sent in header/body |
| Server-owned fee | PASS | UI has no fee input and submits no fee field; returned fee was ¥1.50 |
| Merchant owner isolation | PASS | Merchant list/detail use authenticated merchant endpoints only |
| Rider active gate | PASS | Real `/me` returned active rider before pool/actions were rendered |
| Claim-pool privacy | PASS | Before claim, the real page showed route summary, fee, id/time only; no contact, phone, or detailed address |
| Claim conflict | PASS | Real Rider B won; Rider A received HTTP 409, friendly message, and refreshed empty pool |
| Assigned PII visibility | PASS | Full pickup and recipient information appeared only in Rider A's assigned view in the main E2E run |
| CAS commands | PASS | Claim and every fulfillment command used backend row version |
| No local status mutation | PASS | Every command refetched backend list/detail |
| Stage timeline | PASS | Real database and page showed all successful timestamps; failed/cancelled stages stayed unoccurred |
| Boundary check | PASS | `ok A-G boundaries plus independent delivery and isolated P0-4 media domains` |
| Full regression | PASS | Go, verify, miniapp build, admin build all passed |
| Residue | PASS | Users/orders/riders/outbox all 0; ports 8080/7700 stopped; temporary files removed |

## 2. Real local E2E evidence

Environment:

- WeChat DevTools Stable `2.01.2510290`;
- actual miniapp build from this branch;
- actual backend `cmd/server`;
- actual JWT middleware and `domainA.token`;
- actual MySQL configured by `backend/.env`;
- local Meilisearch process required by the full server;
- `ENABLE_DELIVERY=true`;
- backend local fee confirmation enabled for this acceptance run;
- frontend mock fee button enabled only in the dedicated local build.

Main flow:

1. Merchant token was injected into the existing miniapp storage key.
2. Merchant page created order `01KWA17RPSZARPWBETZVTGZMZQ`.
3. Page displayed `pending_fee_payment`, server fee ¥1.50, creation time, and unoccurred later stages.
4. Local fee confirmation changed the order to `ready_for_claim`; `readyForClaimAt` appeared.
5. Active rider loaded the real claimable pool.
6. The claimable card contained no pickup/recipient phone or detailed address.
7. Rider claimed the order and the assigned page then displayed complete pickup/recipient information.
8. Rider progressed:
   - `claimed`;
   - `to_pickup`;
   - `picked_up`;
   - `delivering`;
   - `delivered`.
9. The final page displayed creation, ready-for-claim, claimed, to-pickup, picked-up, delivering, and delivered times.
10. Direct MySQL evidence before cleanup:
    - status `delivered`;
    - row version `7`;
    - six post-creation success timestamps present.

## 3. Real concurrent claim evidence

A separate fenced fixture used two active test riders against order `01KWA2ED30BC2MBGAX7WKRJKG1`.

1. Rider A opened the claimable pool at row version 2.
2. Rider B claimed the same order first and received `claimed`, row version 3.
3. Rider A clicked the still-visible stale claim button.
4. Backend returned `409 Conflict`.
5. The page displayed: the delivery order has already been claimed or its status changed, and the latest data has been refreshed.
6. The claimable pool refreshed immediately and the stale card disappeared.

This proves both backend CAS behavior and the miniapp's user-facing stale/conflict handling without mocks.

## 4. Automated verification

### Go

Command:

`go test ./... -count=1`

Result: PASS across server, config, admin delivery transport, delivery domain, and all existing modules.

### Monorepo verification

Command:

`npm run verify`

Result:

- boundary check PASS;
- admin typecheck PASS;
- miniapp typecheck PASS;
- backend test suite PASS.

### Build

Command:

`npm run build`

Result:

- miniapp production build PASS;
- admin production build PASS;
- only the existing `vendors.js` size and async-chunk warnings remained.

## 5. Boundary evidence

- `git diff -- backend`: 0 lines.
- `git diff -- package.json apps/admin/package.json apps/miniapp/package.json`: 0 lines.
- Legacy `/pages/takeout-*` and takeout service/store diff: 0 lines.
- No new dependency, router, state library, HTTP backend, migration, worker, or state machine.
- Existing legacy takeout gate was not changed.
- D-3, D-4, D-5, and P0-5 were not entered.

## 6. Residue report

Both real acceptance fixtures were removed after evidence collection.

First E2E fence:

- users: 0
- delivery orders: 0
- delivery outbox: 0
- takeout riders: 0

Concurrent-claim fence:

- users: 0
- delivery orders: 0
- delivery outbox: 0
- takeout riders: 0

Runtime cleanup:

- backend listener 8080: stopped
- Meilisearch listener 7700: stopped
- temporary Meilisearch databases: removed
- temporary JWT fixture files: removed
- temporary fixture source/log/PID files: removed
- copied ignored `project.private.config.json`: removed
- dependency junctions used for worktree verification: removed

## 7. Final decision

**D-2 Delivery Dispatch Frontend: PASS**

The merchant creation and local fee-confirmation flow, rider eligibility/claim/privacy flow, complete delivery progression, stage timestamps, CAS conflict UX, boundaries, regression suite, build, and cleanup all passed.

Stop point: branch is ready for owner review and later fast-forward merge. No merge, tag, or D-3 work was performed.
