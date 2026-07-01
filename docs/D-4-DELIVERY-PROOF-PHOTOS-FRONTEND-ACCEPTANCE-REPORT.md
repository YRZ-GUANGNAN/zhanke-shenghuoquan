# D-4 Delivery Proof Photos Frontend Acceptance Report

Date: 2026-07-01  
Implementation commit: `87d3d88`  
Result: **PASS**

## 1. Automated regression

| Check | Result |
|---|---|
| `npm run check:boundaries` | PASS |
| MiniApp TypeScript | PASS |
| Admin TypeScript | PASS |
| `npm run verify` | PASS |
| `go test ./... -count=1` | PASS |
| `npm run build:miniapp` | PASS |
| `npm run build:admin` | PASS |
| Full `npm run build` | PASS |
| AutoMigrate regression | unchanged |

The MiniApp build emitted only the pre-existing `vendors.js` size warning.

## 2. Real local end-to-end evidence

Environment:

- real MySQL 8.4;
- real backend handlers and authentication;
- real Tencent STS/COS upload and fixed object versions;
- WeChat Developer Tools Stable `2.01.2510290`;
- same checked-out MiniApp branch `feat/d4-delivery-proof-frontend`;
- local non-production Media auto-approval enabled by the existing mock-login gate.

Test order fingerprint: `01KW...XGQR`.

### 2.1 Rider pickup proof

Starting state: `to_pickup`, row version 4.

The rider selected a real local image in WeChat Developer Tools. The client completed compression, upload ticket creation, STS authorization, COS PUT, completion, final-ready processing and authoritative review polling before calling the proof-bearing PATCH endpoint.

Observed result:

- order: `to_pickup -> picked_up`;
- pickup proof: `active`;
- pickup asset: purpose `delivery_proof`, technical status `final_ready`;
- review projection: `approved`;
- fixed staging and final COS version IDs present.

### 2.2 Rider delivery proof

The rider advanced `picked_up -> delivering`, then repeated the mandatory photo flow.

Observed result:

- order: `delivering -> delivered`;
- final order row version: 7;
- delivery proof: `active`;
- delivery asset: purpose `delivery_proof`, technical status `final_ready`;
- review projection: `approved`;
- fixed staging and final COS version IDs present.

The recorded stage times were:

- picked up: `2026-07-01 00:48:23 +08:00`;
- delivered: `2026-07-01 00:49:19 +08:00`.

This proves the UI did not advance either proof-required transition before upload and review completed.

### 2.3 Rider and merchant visual display

The real MiniApp detail view was opened under the rider token and then under the owning merchant token.

Both participant views visibly rendered:

- `取货凭证`;
- `送达凭证`;
- both images;
- the completed delivery timeline.

No proof was loaded on the claim-pool tab.

## 3. Private resolve and leakage matrix

| Case | Result |
|---|---|
| owning merchant lists two proofs | PASS |
| assigned rider lists two proofs | PASS |
| list response leaks asset ID / URL / COS key / version | no |
| rider resolves pickup proof | 200 PASS |
| merchant resolves pickup proof | 200 PASS |
| unrelated logged-in user resolves proof | 404 PASS |
| generic Media resolve for `delivery_proof` | 404 PASS |
| claim-pool response contains proof / asset / URL | no |
| repeated resolve returns refreshed URL | PASS |
| signed GET lifetime | 60 seconds |

The signed host was recorded only in redacted form: `zkshq-***.cos.ap-guangzhou.myqcloud.com`. Two resolves separated by two seconds returned different signed URLs.

## 4. Existing behavior and boundary proof

- Delivery merchant creation/list/payment/refund UI remains intact.
- Rider registration, claim, release and non-proof transitions remain intact.
- Community, Trade, legacy Takeout, Admin, Media backend and IM files have zero diff.
- No dependency, router library, migration or backend endpoint was added.
- Delivery photos are never sent to generic Media display/resolve.

## 5. Final conclusion

All authorized D-4 frontend requirements, real DevTools photo flows, participant-only display checks, full regressions and residue checks passed.

**D-4 Delivery Proof Photos Frontend: PASS.**

Development stops here. D-5 is not started.

