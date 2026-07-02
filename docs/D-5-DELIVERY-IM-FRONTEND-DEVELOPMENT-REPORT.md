# D-5 Delivery IM Frontend Development Report

Date: 2026-07-02

Branch: `feat/d5-delivery-im-frontend`

Base: `89f87124 docs(D-5): record degradable read receipt acceptance`

## Scope

Implemented the MiniApp frontend for delivery-order-bound merchant/rider conversation:

- Added `pages/delivery-conversation` for delivery conversation access from an order.
- Added `services/delivery-im.ts` as the frontend adapter for the frozen D-5 backend delivery IM endpoints and Tencent Chat SDK group messaging.
- Added merchant order-detail entry: `联系骑手`, shown only after a rider is assigned.
- Added rider assigned-order detail entry: `联系商家`.
- Added unread badge display on the entry buttons and conversation status card.
- Added `delivery_im` media purpose mapping for private IM image upload.

## Contract alignment

The frontend uses the frozen D-5 backend contract:

- `GET /api/v1/delivery/orders/:orderId/conversation`
- `GET /api/v1/delivery/conversations`
- `POST /api/v1/delivery/conversations/:conversationId/images/prepare`
- `GET /api/v1/delivery/conversations/:conversationId/images/:imageId/resolve`

Text messages are sent through Tencent Chat SDK WorkGroup conversation IDs returned by the backend.

Image messages do not use Tencent native image upload. The flow is:

1. Upload image via the existing Media upload chain with purpose `delivery_im`.
2. Wait for media approval.
3. Call delivery IM `images/prepare`.
4. Send a Tencent custom message containing only the backend-issued `{ conversationId, imageId }` payload.
5. Resolve each image through the participant-protected backend endpoint before rendering.

## Safety and boundary behavior

- Peer read receipt is not displayed. Sent messages explicitly set `needReadReceipt: false`.
- `readReceiptEnabled` is read from backend DTO, but the UI still only shows local unread behavior.
- `imagesEnabled=false` hides the image-send button and the send path is also guarded.
- Nonparticipant, old rider, revoked assignment, or wrong conversation access fails closed through backend 404/forbidden responses and is shown as a ended/unavailable conversation.
- Short private image URLs are used only in memory and are not persisted.
- Backend, migrations, delivery core, old takeout, media backend, IM backend, and admin surfaces were not modified.

## Files changed

- `apps/miniapp/src/services/delivery-im.ts`
- `apps/miniapp/src/pages/delivery-conversation/index.tsx`
- `apps/miniapp/src/pages/delivery-conversation/index.scss`
- `apps/miniapp/src/pages/delivery-conversation/index.config.ts`
- `apps/miniapp/src/pages/delivery-merchant/index.tsx`
- `apps/miniapp/src/pages/delivery-merchant/index.scss`
- `apps/miniapp/src/pages/delivery-rider/index.tsx`
- `apps/miniapp/src/pages/delivery-rider/index.scss`
- `apps/miniapp/src/services/media.ts`
- `apps/miniapp/src/app.config.ts`

## Notes

The implementation intentionally leaves Tencent Chat SDK text/history transport as the source of truth for message bodies. The backend remains the source of truth for conversation authorization, assignment generation, image prepare/resolve, and private media access.
