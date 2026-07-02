# D-5 Delivery IM Backend Development Report

Date: 2026-07-02

Branch: `feat/d5-delivery-im`

Baseline: `6c39d477` (`d4-delivery-proof-frontend-baseline`)

Initial implementation commit: `ce451ee5`

Initial report commit: `3c7a2e8a`

Read-receipt degradable patch commit: `65d115c9`

## 1. Scope delivered

D-5 adds a delivery-order-scoped merchant/rider communication backend without changing the sealed takeout system or the existing product C2C contract.

- One Tencent Work Group per delivery assignment generation.
- Delivery conversation model, state machine, Repository, lease/retry worker and CAS updates.
- Current-assignment participant fence based on the live `delivery_orders.rider_user_id`.
- Old-rider revocation before a delivery order can be released.
- Merchant access to archived assignment generations; old/new riders cannot cross generations.
- Generic Tencent IM account/group infrastructure in `internal/module/im`.
- Private `delivery_im` media purpose and participant-authorized 60-second signed image resolve.
- Delivery IM HTTP query and private-image preparation/resolve endpoints.
- `ENABLE_DELIVERY` composition and worker gate.
- Migration 027 and fail-closed rollback.

The frontend, message-body persistence, notification polling and D-6 work are not included.

## 2. Read-receipt degradable patch

Owner decided not to upgrade the Tencent IM paid plan for group read receipts at this stage. The only previously red D-5 gate was Tencent group read receipt support returning `Code=10062`.

This patch changes group read receipt support from a hard acceptance blocker into an explicitly gated optional enhancement:

- Added config flag `ENABLE_DELIVERY_IM_READ_RECEIPT`, default `false`.
- Added `Config.EnableDeliveryIMReadReceipt` using the existing `envBool` pattern.
- No `config.Validate` hard block was added; read receipts are optional and default off.
- Delivery conversation DTO now retains `readReceiptEnabled`, so frontend can later detect the capability without contract churn.
- `cmd/server` passes the config flag into the delivery IM handler.
- The preserved real read-receipt acceptance test now requires `ENABLE_DELIVERY_IM_READ_RECEIPT=true` in addition to the destructive real-test flags before calling Tencent receipt endpoints.
- The Tencent REST adapter now contains non-empty, reusable group read receipt methods:
  - `SendWorkGroupTextMessage(..., needReadReceipt=true)`
  - `ConfirmWorkGroupMessageRead(...)`
  - `GetWorkGroupMessageReadReceipts(...)`

The implementation is intentionally not a fake or empty stub. It remains ready for upgrade, but the default delivery state never calls Tencent receipt APIs and therefore avoids the unsupported `10062` path.

Upgrade steps when owner later buys/enables the required Tencent IM capability:

1. Upgrade Tencent IM to a plan that supports group message read receipts.
2. Enable group read receipt capability in Tencent console.
3. Set `ENABLE_DELIVERY_IM_READ_RECEIPT=true`.
4. Rerun:

```powershell
$env:DELIVERY_D5_RUN_REAL_TENCENT_IM='1'
$env:DELIVERY_D5_RUN_REAL_READ_RECEIPT='1'
$env:ENABLE_DELIVERY_IM_READ_RECEIPT='true'
go test ./internal/module/im -run '^TestTencentWorkGroupReadReceiptAgainstRealIM$' -count=1 -v
```

## 3. Architecture

### Delivery core

`internal/module/delivery` remains free of Gin, Tencent IM, COS and Media imports. It owns:

- `Conversation` and assignment generation facts;
- `provisioning / active / read_only / revoking / revoked / failed`;
- status plus `row_version` CAS;
- provisioning leases, bounded retries and crash recovery;
- participant queries that re-read the current delivery order;
- image-slot allocation;
- narrow `ConversationGroupPort` and `ConversationMediaPort` interfaces.

The read-receipt patch does not modify delivery core, conversation state machine, repository, CAS, assignment generation or participant authorization.

### Application composition

`internal/module/admin/deliveryim` combines Delivery, generic IM and Media:

- imports/ensures Tencent identities;
- creates an ownerless Work Group whose merchant and rider are ordinary members;
- removes the old rider before completing local revocation;
- verifies asset owner, purpose, final fixed version and approved review projection;
- binds `delivery_im` references and returns only opaque image coordinates;
- verifies participant access before resolving a 60-second fixed-version URL.

The ownerless group is deliberate. When a delivery becomes `delivered` or `cancelled`, the service first sets Tencent `MuteAllMember=On`; only then does it CAS the local conversation to `read_only`. Since both participants are ordinary members, the remote barrier blocks both. A Tencent failure leaves the local conversation active and returns unavailable, rather than claiming read-only without enforcing it.

### Message storage, unread count and read receipts

Message bodies remain in Tencent IM. No backend message-body table was added. Existing `/api/v1/im/usersig` credentials remain the SDK login mechanism.

Unread count and `setMessageRead` remain Tencent-native client capabilities and were not changed by this patch.

Group read receipt is now optional:

- default off: backend reports `readReceiptEnabled=false`; no Tencent receipt endpoint is called;
- future on: backend reports `readReceiptEnabled=true`; preserved adapter/test code can exercise Tencent receipt send/confirm/query.

## 4. Migration 027

`delivery_im_conversations` contains:

- public conversation and delivery-order IDs;
- assignment generation;
- merchant/rider logical IDs;
- deterministic Tencent group ID;
- lifecycle status and `row_version`;
- image slot counter;
- lease, retry, error and lifecycle timestamps.

Unique keys cover conversation public ID, `(delivery_order_public_id, assignment_version)` and Tencent group ID. No foreign keys were added.

The media reference CHECK is extended with `delivery_im`. Rollback first refuses to run if conversation facts or `delivery_im` references exist.

SHA-256:

- apply: `221438275900F50FD67A77C18D6F7925AD2341BD0217299BD47A6983AE695382`
- rollback: `35874A4547684AE9DF42B4BEC01E97A4EE527EB4AB07C236A0506635F0BE07BC`

The read-receipt patch adds no migration and changes no database contract.

## 5. HTTP contract

Authenticated routes under `/api/v1/delivery`:

- `GET /orders/:orderId/conversation`
- `GET /conversations`
- `POST /conversations/:id/images/prepare`
- `GET /conversations/:id/images/:imageId/resolve`

Unauthorized participants, old riders and wrong assignment generations receive 404. The response exposes the Tencent conversation ID, `imagesEnabled`, `readReceiptEnabled` and opaque media coordinates, never COS key, version ID or permanent URL.

## 6. Privacy and production gates

- `delivery_im` is valid for upload but `PubliclyResolvable()` is false.
- Generic miniapp media resolve returns 404 for `delivery_im`.
- Image preparation is enabled only in the existing strict local mock-WeChat mode.
- Production image sending remains disabled until the real WeChat image-review adapter passes acceptance.
- Image resolve re-checks the current delivery participant fact every time.
- Delivered/cancelled conversations are remotely muted before local `read_only`.
- Group read receipt is disabled unless `ENABLE_DELIVERY_IM_READ_RECEIPT=true`.

## 7. Compatibility

- Existing product C2C methods and schema were not changed; generic `EnsureUser` and group operations remain compatible.
- No takeout code or migration was changed.
- No frontend file was changed.
- `db.AutoMigrate(` remains absent from `backend/cmd/server`.
- D-2/D-3/D-4 rider routes retain their contracts.
- D-5 unread count behavior remains Tencent-client-native and unchanged.

## 8. Development status

D-5 backend development is complete with the read-receipt degradable patch at commit `65d115c9`.

The only remaining conditional production notes are explicit and non-blocking for this backend landing:

1. Group read receipts are disabled by default until Tencent plan/capability is upgraded.
2. Production image sending remains disabled until the real WeChat image-review adapter passes acceptance.

D-6 is not authorized.
