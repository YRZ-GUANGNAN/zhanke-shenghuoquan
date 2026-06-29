# D-2 Delivery Dispatch Frontend Development Report

Date: 2026-06-29  
Branch: `feat/d2-delivery-frontend`  
Base: `master@2ada5cc1` (`d2-dto-timestamps-baseline`)  
Implementation commit: `5e958a3`

## 1. Scope result

D-2 miniapp frontend integration is complete. This change only adds the delivery merchant and delivery rider clients, pages, feature flags, routes, and Profile entries.

No backend file, migration, delivery domain state machine, repository, old takeout page/service, Community, Trade, Media, IM, Admin, or package dependency was changed.

## 2. Preflight verification

- `master` contained the frozen D-2 backend and DTO timestamp patch.
- Tags `d2-delivery-dispatch-backend-baseline` and `d2-dto-timestamps-baseline` existed.
- Main working tree was clean before the worktree was created.
- `backend/cmd/server` contained zero `db.AutoMigrate(` calls.
- Merchant HTTP contract was verified from the frozen handler/types:
  - create, list, detail, cancel, and local mock fee confirmation;
  - `Idempotency-Key` plus `clientRequestId`;
  - server-owned fee snapshot;
  - nullable eight-stage timestamps.
- Rider HTTP contract was verified:
  - `/me`, claimable pool, claim, assigned list/detail;
  - `to-pickup`, `picked-up`, `delivering`, `delivered`, and release;
  - all commands use the backend `rowVersion`;
  - claimable DTO is the minimal non-PII view.
- Existing miniapp infrastructure was reused:
  - `domainA.token`;
  - common request envelope/error handling;
  - Profile role entry pattern;
  - Taro page registration and existing feature-flag build injection.

## 3. Implementation

### 3.1 API clients

Added:

- `apps/miniapp/src/services/delivery-merchant.ts`
- `apps/miniapp/src/services/delivery-rider.ts`

The clients:

- reuse the existing authenticated `request()` wrapper;
- do not introduce a second token or authentication path;
- submit `expectedRowVersion` for all CAS-sensitive commands;
- generate a 26-character ULID-compatible client request id and reuse it until creation succeeds;
- send the id in both `Idempotency-Key` and `clientRequestId`;
- never submit a client-controlled delivery fee;
- model all eight nullable stage timestamps from the frozen DTO.

### 3.2 Delivery merchant page

Added independent route `/pages/delivery-merchant/index`.

Capabilities:

- create a delivery order from pickup and recipient information;
- display server-calculated delivery fee;
- list and inspect only the current merchant user's orders;
- display actual backend timestamps and show unoccurred stages as unoccurred;
- cancel only when the frozen backend state allows it;
- expose local mock fee confirmation only when the dedicated frontend flag is enabled;
- refresh list and detail after every command instead of assigning a status locally.

### 3.3 Delivery rider page

Added independent route `/pages/delivery-rider/index`.

Capabilities:

- show rider registration/active state from `/me`;
- show the claimable pool using only `routeSummary`, fee, id, status/version, and creation time;
- reveal contact, phone, and full addresses only after successful assignment;
- claim and progress the assigned order through pickup and delivery;
- release only from backend-permitted states;
- display the real backend stage timeline;
- refresh backend data after every success or error.

### 3.4 Entries and feature flags

Added Profile entries only; BottomNav was not changed.

- `TARO_APP_ENABLE_DELIVERY`
  - default: `true`;
  - controls both delivery role entries.
- `TARO_APP_ENABLE_DELIVERY_MOCK_FEE`
  - default: `false`;
  - controls only the explicitly labelled local fee-confirmation button.

Legacy takeout gating remains unchanged.

## 4. CAS and error behavior

- Backend state and row version are the only sources of truth.
- No page locally advances an order status.
- Merchant and rider commands always refetch list/detail after completion.
- Claim conflicts, stale row versions, invalid transitions, and rider mismatch errors produce an explicit refresh message.
- A real two-rider conflict was executed during acceptance:
  - Rider B claimed the order with row version 2;
  - Rider A then submitted the stale row version 2 from the visible page;
  - backend returned HTTP 409;
  - the page displayed the friendly conflict message and refreshed the pool to empty.

## 5. Verification summary

- Miniapp TypeScript: PASS
- Miniapp build: PASS
- Admin TypeScript/build: PASS
- `go test ./... -count=1`: PASS
- `npm run check:boundaries`: PASS
- `npm run verify`: PASS
- `npm run build`: PASS
- Real WeChat DevTools + real Gin handlers + real MySQL: PASS
- Backend diff: 0 lines
- Package manifest diff: 0 lines
- Legacy takeout page/service diff: 0 lines
- AutoMigrate matches: 0
- Test residue: 0

The only build warnings were the pre-existing `vendors.js` size and async-chunk recommendations.

## 6. Known limitations

- Real delivery-fee payment remains D-3 scope.
- Pickup/delivery photo evidence remains D-4 scope.
- Merchant-rider IM remains D-5 scope.
- The mock fee control remains local-only and hidden by default.

No later module was started.
