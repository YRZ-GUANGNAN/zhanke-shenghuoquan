# D-4 Delivery Proof Photos Backend Development Report

Date: 2026-06-30  
Branch: `feat/d4-delivery-proof`  
Base: `master@52d07c8e` (`d3-delivery-frontend-baseline`)  
Implementation commit: `a40fe44`

## 1. Scope and preflight

The preflight gate passed before development:

- master contained the frozen D-3 frontend baseline;
- tag `d3-delivery-frontend-baseline` existed;
- migration 025 was the highest existing migration, so D-4 used 026;
- `backend/cmd/server` contained zero `db.AutoMigrate(` calls;
- the worktree was clean;
- no frontend, D-5, legacy takeout, community or trade implementation was changed.

The implementation is backend-only. The unresolved production WeChat image-review adapter was not expanded into this task.

## 2. Implemented domain facts

Migration `026_delivery_proofs.sql` creates `delivery_proofs` with:

- immutable public coordinates: `proof_public_id`, `delivery_order_public_id`, `proof_type`, `slot`;
- immutable logical media identity: `asset_public_id`;
- uploader identity: `uploader_user_id`;
- durable binding states: `binding`, `active`, `deadletter`, `revoked`;
- idempotency identity: `operation_id`;
- optimistic concurrency and crash recovery: `row_version`, `attempt_count`, `next_attempt_at`, `lease_token`, `lease_expires_at`;
- lifecycle timestamps;
- unique constraints on proof ID, order/type/slot, asset ID and operation ID;
- no database foreign keys to user, order or media domains.

The same migration expands `chk_media_reference_type` with `delivery_proof`. Its rollback refuses to proceed while either delivery proof facts or `delivery_proof` media references still exist.

The Delivery core now contains:

- `ProofType`: `pickup`, `delivery`;
- `ProofStatus`: `binding`, `active`, `deadletter`, `revoked`;
- an explicit proof status transition matrix;
- `ProofRepository` with order/proof atomic persistence, participant reads and lease/CAS binding primitives;
- `ProofService` with media prevalidation, idempotent order transition, binding recovery and private participant reads;
- `MediaProofPort`, which keeps Delivery core free of Media, COS, Gin and HTTP imports.

## 3. Atomic fulfillment behavior

The rider endpoints now require:

```json
{
  "expectedRowVersion": 4,
  "proofAssetPublicId": "01..."
}
```

The required transitions are:

- `to_pickup -> picked_up` with `pickup` proof;
- `delivering -> delivered` with `delivery` proof.

Before the Delivery transaction, the adapter verifies:

- the current user is the currently assigned rider;
- the media asset is owned by that rider;
- purpose is `delivery_proof`;
- storage is final and technical status is `final_ready`;
- final key and fixed version are present;
- Review Projection is `approved`.

The Delivery transaction then:

1. locks the order;
2. rechecks assigned-rider ownership;
3. enforces expected status and row version;
4. inserts a `binding` proof fact;
5. CAS-advances the order;
6. commits both or neither.

Repeated identical requests return the existing proof and order state. A different asset at the same order/type/slot fails with a conflict.

## 4. Durable Media binding

Cross-domain binding never writes Media tables directly from Delivery. The `admin/deliveryproof` adapter uses the existing Media Service and Gateway:

1. creates the Media reference as `pending + hidden`;
2. calls the existing idempotent `PublishReference`;
3. advances it to `active + visible`;
4. marks the Delivery proof `active` by lease-token and row-version CAS.

Transient binding failures leave the proof durably in `binding`, with exponential retry metadata. The background worker reclaims expired leases. Max-attempt failures become `deadletter`. No bare asynchronous goroutine is used for a one-shot binding.

## 5. Privacy boundary

Private endpoints:

- `GET /api/v1/delivery/orders/:orderId/proofs`
- `GET /api/v1/delivery/orders/:orderId/proofs/:proofType/:slot/resolve`

The endpoint sequence is:

1. JWT authentication;
2. order lookup;
3. participant fence: current user is the order merchant or current assigned rider;
4. proof must be active;
5. MediaGateway must confirm final + approved + active reference + parent visible;
6. return a 60-second fixed-version signed URL;
7. set `Cache-Control: no-store`.

Nonparticipants, former riders, missing proofs and inactive proofs return 404. The response does not contain asset ID, COS key or a separate version ID.

The generic mini-app Media display and resolve endpoints explicitly return 404 for `delivery_proof`, even though the purpose is valid for upload. Claim-pool DTOs continue to expose no proof, asset or URL fields.

## 6. Composition and boundaries

The composition root:

- creates a separate 60-second Media display signer;
- injects the Media adapter through `MediaProofPort`;
- constructs the proof service only when Delivery is enabled;
- mounts private proof routes under the Delivery authenticated group;
- runs the durable proof binding worker.

`scripts/check-boundaries.cjs` now recognizes `admin/deliveryproof` as the authorized Delivery-to-Media adapter and still forbids it from importing unrelated business domains.

## 7. Known limitation

The real WeChat image-review producer for the new P0-4 Media Projection remains unconnected. D-4 uses the existing local non-production mock approval only for end-to-end acceptance. This is not a D-4 code defect, but it prevents a production PASS.

Development status: **COMPLETE**  
Acceptance recommendation: **CONDITIONAL PASS**
