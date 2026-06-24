# G-7a Buyer API Acceptance Report

Branch: `feat/g7a-buyer-api`

Final commit hash: recorded in final handoff after commit.

## Acceptance matrix

| Gate | Result | Evidence |
| --- | --- | --- |
| No token rejected | PASS | Real HTTP test returns `401` for `/api/v1/takeout/buyer/merchants` without bearer token. |
| Logged-in buyer can browse | PASS | Real HTTP test uses normal WeChat-login JWT user, no admin role required. |
| Active merchants only | PASS | Active merchant appears; resting merchant is absent and detail returns `404`. |
| On-shelf dishes only | PASS | On-shelf dishes appear; off-shelf dish absent. |
| Dishes require active merchant | PASS | Dishes under resting merchant return `404`. |
| Dish image display path | PASS | DTO contains `/api/v1/miniapp/media/display/dish/:dishPublicId/0/content` for image-backed dish. |
| No-image dish path empty | PASS | DTO helper only emits image URL when `image_asset_public_id` is present. |
| Server-side amount recompute | PASS | Test sends bogus amount fields; returned order total equals current dish price times quantity plus fee config. |
| Order starts pending payment | PASS | Created order status is `pending_payment`. |
| Off-shelf dish order rejected | PASS | Real HTTP returns `409`. |
| Non-active merchant order rejected | PASS | Real HTTP returns `404` before order creation. |
| Buyer order list/detail | PASS | Current buyer can list and read own order with items. |
| Cross-buyer order read blocked | PASS | Buyer B reading Buyer A order returns `404`. |
| No buyer payment endpoint | PASS | `/orders/:id/pay` and `/orders/:id/mark-paid` return `404`. |
| Boundary check | PASS | `npm run check:boundaries`. |
| Full Go regression | PASS | `go test ./... -count=1`. |
| Full verify | PASS | `npm run verify`. |
| Build | PASS | `npm run build`; only existing Taro vendors size warning. |
| Residue | PASS | Real MySQL residue test returns zero for users, merchants, categories, dishes, and orders created by G-7a test. |

## Verdict

G-7a buyer backend API: PASS.

Remaining work is intentionally outside this module:

- G-7a frontend buyer pages
- G-4 real payment/callback
- G-7c rider frontend or other role work
