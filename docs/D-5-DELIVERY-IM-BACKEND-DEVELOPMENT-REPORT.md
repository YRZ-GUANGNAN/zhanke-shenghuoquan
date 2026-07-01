# D-5 Delivery IM Backend Development Report

Date: 2026-07-01

Branch: `feat/d5-delivery-im`

Baseline: `6c39d477` (`d4-delivery-proof-frontend-baseline`)

Implementation commit: `ce451ee`

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

## 2. Architecture

### Delivery core

`internal/module/delivery` remains free of Gin, Tencent IM, COS and Media imports. It owns:

- `Conversation` and assignment generation facts;
- `provisioning / active / read_only / revoking / revoked / failed`;
- status plus `row_version` CAS;
- provisioning leases, bounded retries and crash recovery;
- participant queries that re-read the current delivery order;
- image-slot allocation;
- narrow `ConversationGroupPort` and `ConversationMediaPort` interfaces.

### Application composition

`internal/module/admin/deliveryim` combines Delivery, generic IM and Media:

- imports/ensures Tencent identities;
- creates an ownerless Work Group whose merchant and rider are ordinary members;
- removes the old rider before completing local revocation;
- verifies asset owner, purpose, final fixed version and approved review projection;
- binds `delivery_im` references and returns only opaque image coordinates;
- verifies participant access before resolving a 60-second fixed-version URL.

The ownerless group is deliberate. When a delivery becomes `delivered` or `cancelled`, the service first sets Tencent `MuteAllMember=On`; only then does it CAS the local conversation to `read_only`. Since both participants are ordinary members, the remote barrier blocks both. A Tencent failure leaves the local conversation active and returns unavailable, rather than claiming read-only without enforcing it.

### Message storage and read state

Message bodies remain in Tencent IM. No backend message-body table was added. Existing `/api/v1/im/usersig` credentials remain the SDK login mechanism. Group unread counts, `setMessageRead` and peer read receipts remain Tencent-native client capabilities.

## 3. Migration 027

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

## 4. HTTP contract

Authenticated routes under `/api/v1/delivery`:

- `GET /orders/:orderId/conversation`
- `GET /conversations`
- `POST /conversations/:id/images/prepare`
- `GET /conversations/:id/images/:imageId/resolve`

Unauthorized participants, old riders and wrong assignment generations receive 404. The response exposes the Tencent conversation ID and opaque media coordinates, never COS key, version ID or permanent URL.

## 5. Privacy and production gates

- `delivery_im` is valid for upload but `PubliclyResolvable()` is false.
- Generic miniapp media resolve returns 404 for `delivery_im`.
- Image preparation is enabled only in the existing strict local mock-WeChat mode.
- Production image sending remains disabled until the real WeChat image-review adapter passes acceptance.
- Image resolve re-checks the current delivery participant fact every time.
- Delivered/cancelled conversations are remotely muted before local `read_only`.

## 6. Compatibility

- Existing product C2C methods and schema were not changed; only generic `EnsureUser` and group operations were added.
- No takeout code or migration was changed.
- No frontend file was changed.
- `db.AutoMigrate(` remains absent from `backend/cmd/server`.
- D-2/D-3/D-4 rider routes retain their contracts; claim and release gained the D-5 lifecycle side effect.

## 7. Development status

Development is complete at commit `ce451ee`. Formal acceptance remains conditional because the real Tencent environment returns group read-receipt item code `10062`; the control-panel capability must be enabled and the external receipt test rerun before the D-5 baseline can be declared PASS.
