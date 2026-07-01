# D-4 Delivery Proof Photos Frontend Development Report

Date: 2026-07-01  
Branch: `feat/d4-delivery-proof-frontend`  
Base: `master@1b565c6a` (`d4-delivery-proof-backend-baseline`)  
Implementation commit: `87d3d88`

## 1. Scope and boundary

This change is a MiniApp-only integration against the frozen D-4 backend contract. It changes no backend file, migration, Delivery state machine, Repository, Media state machine, COS adapter, or private authorization rule.

Changed files:

- `apps/miniapp/src/services/media.ts`
- `apps/miniapp/src/services/delivery-proof.ts`
- `apps/miniapp/src/services/delivery-rider.ts`
- `apps/miniapp/src/pages/delivery-rider/index.tsx`
- `apps/miniapp/src/pages/delivery-merchant/index.tsx`
- `apps/miniapp/src/pages/delivery-shared.scss`

`git diff --name-only -- backend` returned no files.

## 2. Pre-development contract verification

The frozen backend exposes:

- `PATCH /api/v1/delivery/rider/orders/:id/picked-up`
- `PATCH /api/v1/delivery/rider/orders/:id/delivered`
- body: `{ expectedRowVersion, proofAssetPublicId }`
- `GET /api/v1/delivery/orders/:orderId/proofs`
- `GET /api/v1/delivery/orders/:orderId/proofs/:proofType/:slot/resolve`
- proof types: `pickup` and `delivery`
- MVP slot: `0`
- resolve result: a 60-second signed URL without asset ID, COS key, or version ID

The existing upload client already implemented one-time ticket creation, constrained COS PUT, completion, and status polling. D-4 extends that client rather than using the legacy B/C empty-binding path.

## 3. Implementation

### 3.1 Delivery-proof upload

`MediaPurpose` now accepts `delivery_proof`.

Client constraints:

- longest edge: 1920 px;
- compressed file maximum: 5 MiB;
- existing safe byte-copy, MD5, content-type and ticket workflow remain unchanged;
- status is polled until the backend returns both `state=approved` and `canUse=true`;
- rejected, review-failed, expired, upload-failed and deleted assets fail closed;
- a 90-second client wait timeout does not advance the Delivery state.

### 3.2 Mandatory rider proof

The rider cannot invoke `picked-up` or `delivered` directly from the UI.

The sequence is:

1. choose/capture one image;
2. compress and upload it through the new Media pipeline;
3. wait for authoritative backend approval;
4. pass the returned asset public ID to the frozen D-4 PATCH endpoint;
5. refresh claimable, assigned, detail and private proof views.

The buttons are now:

- `拍取货照并确认` for `to_pickup -> picked_up`;
- `拍送达照并确认` for `delivering -> delivered`.

CAS/stale errors refresh the order instead of locally advancing it. Review or binding failures display an honest message and leave the order state unchanged.

### 3.3 Private display

`delivery-proof.ts` uses only the Delivery participant endpoints. Active proofs are resolved immediately before display and the signed URL is held only in component state.

- no generic Media resolve call;
- no storage persistence;
- no raw asset/key/version exposure;
- detail views refresh signed URLs every 50 seconds, before the 60-second expiry;
- the claim pool does not import or call the proof service.

Both rider and merchant detail views render pickup and delivery proof images plus binding/dead-letter/revoked states.

## 4. Known limitations

- MVP remains one photo per proof stage (`slot=0`), matching the frozen backend contract.
- Replacement/revocation UI is deliberately out of scope.
- Local acceptance used the backend’s existing non-production mock content approval; production WeChat content-safety behavior remains a backend concern.

## 5. Development conclusion

The frontend implementation is complete within the authorized scope. Backend diff is zero and no D-5 work was introduced.

