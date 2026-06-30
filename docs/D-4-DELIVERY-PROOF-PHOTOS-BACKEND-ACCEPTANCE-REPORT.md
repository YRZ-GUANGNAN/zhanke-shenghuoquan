# D-4 Delivery Proof Photos Backend Acceptance Report

Date: 2026-06-30  
Implementation commit: `a40fe44`  
Environment: real MySQL 8.4.0 and real Tencent Cloud STS/COS

## 1. Migration acceptance

Migration 026 was executed against the real MySQL 8.4 database:

| Check | Result |
|---|---|
| apply 026 | PASS |
| rollback 026 | PASS |
| re-apply 026 | PASS |
| `delivery_proofs` columns | PASS, 18 |
| order/type/slot unique index | PASS |
| asset and operation idempotency indexes | PASS |
| cross-domain foreign keys | PASS, 0 |
| Media reference CHECK accepts `delivery_proof` | PASS |
| rollback with an existing proof fact | PASS, refused with SQLSTATE 45000 |
| rollback silently deletes facts | PASS, impossible |
| AutoMigrate | PASS, 0 |

The final database state is migration 026 applied.

## 2. Domain/CAS acceptance

Real MySQL test:

```text
TestD4ProofTransactionCASIdempotencyLeaseAndParticipantAgainstMySQL84
PASS
```

Verified:

- proof insert and order status CAS occur in one transaction;
- stale row versions are rejected;
- repeated identical proof submission returns one immutable proof;
- a different asset at the same coordinate is rejected;
- merchant and current rider participant reads pass;
- nonparticipant reads fail closed;
- an unexpired lease cannot be reclaimed;
- an expired lease is reclaimed with a new token;
- stale worker completion is rejected;
- recovered worker completion activates the proof.

## 3. HTTP privacy acceptance

Real HTTP + real MySQL test:

```text
TestD4PrivateProofParticipantFenceAgainstRealHTTPMySQL84
PASS
```

| Scenario | Result |
|---|---|
| merchant lists/resolves own order proof | 200 PASS |
| currently assigned rider lists/resolves | 200 PASS |
| unrelated authenticated user | 404 PASS |
| former rider after assignment changes | 404 PASS |
| generic `/miniapp/media/resolve/delivery_proof/...` | 404 PASS |
| private signed URL TTL | 60 seconds PASS |
| `Cache-Control` | `no-store` PASS |
| separate raw COS key/version fields | absent PASS |
| proof list leaks asset ID or URL | no PASS |
| claim pool leaks proof/asset/URL | no PASS |

The rider handler also has explicit tests proving that pickup/delivery transitions reject a missing proof and fail closed when the proof service is unavailable.

## 4. Real STS/COS acceptance

External test:

```text
TestD4DeliveryProofUploadReviewBindAndFixedVersionResolveAgainstRealCOS
PASS
```

Verified against Tencent Cloud:

- real STS authorization;
- exact client PUT to COS;
- Completion HEAD/GET validation and immutable final promotion;
- purpose `delivery_proof`;
- fixed final version ID;
- local non-production Review Projection approval;
- Media reference publication;
- owner isolation;
- 60-second signed GET;
- signed URL contains COS signature and fixed `versionId`;
- signed GET returned the exact uploaded bytes;
- all created COS versions and delete markers were removed.

No Mock COS, fake HEAD or fake Copy was used.

## 5. D-2/D-3 regression

`TestD2DeliveryRealHTTPMySQLAuthConcurrencyOutboxAndGates` passed against real MySQL after the endpoint extension. Existing claim concurrency, owner fences, outbox behavior and fulfillment progression remain intact. The only intentional contract change is that `picked-up` and `delivered` now require `proofAssetPublicId`.

No D-3 funds, wallet, payment or refund production code was modified.

## 6. Full regression

| Command | Result |
|---|---|
| `go test ./... -count=1` | PASS |
| `npm run check:boundaries` | PASS |
| `npm run verify` | PASS |
| admin typecheck/build | PASS |
| mini-app typecheck/build | PASS |
| `npm run build` | PASS |

The mini-app build retained only the existing `vendors.js` size warning. No frontend source was changed.

## 7. Boundary proof

- Delivery core imports no Media, Takeout, Gin, HTTP or COS package.
- The only Delivery-to-Media bridge is `admin/deliveryproof`.
- Media storage and visibility writes use existing Media Service/Gateway boundaries.
- No cross-domain foreign key was introduced.
- Legacy takeout, community, trade, IM and D-5 were not changed.
- `db.AutoMigrate(` remains absent from the server.

## 8. Remaining risk

The new Media domain currently lacks the real WeChat image-review producer in production. Local mock approval proves orchestration but cannot replace real content-safety evidence.

Final result: **CONDITIONAL PASS**

The D-4 backend is complete and frozen-candidate quality. Production closure requires the separately authorized real WeChat image-review adapter and its cloud acceptance evidence.
