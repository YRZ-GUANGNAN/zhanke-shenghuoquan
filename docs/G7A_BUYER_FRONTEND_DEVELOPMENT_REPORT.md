# G-7a Buyer MiniApp Frontend Development Report

## Summary

Status: Development Complete

Branch: `feat/g7a-ui-buyer`

Baseline: `a2af8ba6 docs(G-7a): record media resolve URL acceptance evidence`

Final commit: `TO_BE_FILLED`

This module adds the buyer-facing miniapp takeout pages and makes the new media resolve URL endpoint usable from `<Image>` rendering.

## Preflight

- `git log --oneline -1 master`: `a2af8ba6 docs(G-7a): record media resolve URL acceptance evidence`
- Tags present: `media-resolve-url-baseline`, `g7a-buyer-api-baseline`
- Worktree before development: clean

## File-level verification

### Buyer backend contract

Files:

- `backend/internal/module/admin/takeoutbuyer/handler.go`
- `backend/internal/module/admin/takeoutbuyer/types.go`

Confirmed routes:

- `GET /api/v1/takeout/buyer/merchants`
- `GET /api/v1/takeout/buyer/merchants/:merchantPublicId`
- `GET /api/v1/takeout/buyer/merchants/:merchantPublicId/dishes`
- `POST /api/v1/takeout/buyer/orders`
- `GET /api/v1/takeout/buyer/orders`
- `GET /api/v1/takeout/buyer/orders/:orderPublicId`

Confirmed DTO fields:

- Merchant includes `merchantPublicId`, `name`, `contactPhone`, `location`, `businessHours`, `logoAssetPublicId`, `logoContentUrl`, `status`.
- Dish includes `dishPublicId`, `merchantPublicId`, `categoryPublicId`, `name`, `description`, `priceCents`, `imageAssetPublicId`, `imageContentUrl`, `sortOrder`.
- Order uses backend status enum: `pending_payment`, `paid`, `accepted`, `preparing`, `ready_for_pickup`, `delivering`, `completed`, `cancelled`, `refunded`.
- Create order body uses `merchantPublicId`, `items[]`, `address`, `remark`.

### Media resolve contract

File:

- `backend/internal/module/media/miniappflow/handler.go`

Confirmed route:

```text
GET /api/v1/miniapp/media/resolve/:referenceType/:referenceId/:slot
```

Confirmed behavior:

- It is mounted under the existing `/miniapp/media` group guarded by `AuthRequired + UnmutedRequired`.
- It reuses display service `ContentURL(...)`.
- It returns JSON data `{ url, expiresInSeconds }`.

### DTO image coordinate

Buyer DTO currently returns display-coordinate URLs:

```text
/api/v1/miniapp/media/display/:referenceType/:referenceId/0/content
```

Frontend converts that coordinate into:

```text
/miniapp/media/resolve/:referenceType/:referenceId/0
```

The request goes through existing `request()` so JWT remains in the `Authorization` header and never enters `<Image src>`.

## Implementation

Changed miniapp files:

- `apps/miniapp/src/services/g-takeout-buyer.ts`
  - Buyer API client.
  - Media display-to-resolve URL conversion.
  - Money formatting helper.
- `apps/miniapp/src/media-image.tsx`
  - `ResolvedMediaImage` component.
  - Calls resolve endpoint through authenticated `Taro.request`.
  - Caches COS signed URLs in memory until TTL minus refresh skew.
- `apps/miniapp/src/stores/cart.ts`
  - Zustand cart store.
  - Single-merchant cart constraint.
  - Quantity and total helpers.
- `apps/miniapp/src/pages/takeout/**`
  - Buyer merchant list and dish list.
  - Dish images render through resolve endpoint.
  - Cart floating bar.
- `apps/miniapp/src/pages/takeout-cart/**`
  - Cart and checkout page.
  - Creates `pending_payment` order and clears cart.
- `apps/miniapp/src/pages/takeout-orders/**`
  - Buyer order list and status filter.
- `apps/miniapp/src/pages/takeout-order-detail/**`
  - Buyer order detail and `pending_payment` payment-pending notice.
- `apps/miniapp/src/app.config.ts`
  - Registers buyer takeout pages.
  - Keeps `pages/home/index` as first page.
- `apps/miniapp/src/components.tsx`
  - Adds BottomNav `takeout` tab without removing existing tabs.
- `apps/miniapp/src/types.ts`
  - Adds `takeout` to `TabKey`.
- `apps/miniapp/src/app.scss`
  - BottomNav grid expands from four to five columns.

## Boundary confirmation

- No backend files changed.
- No migrations changed.
- No package dependencies added.
- No route library or new state library added.
- Existing `api.ts` and `session.ts` exports are unchanged.
- JWT remains in request header only; image `src` receives only COS signed URL returned by resolve endpoint.

## Known manual verification item

Code/build validation is complete. The authorization's final "肉眼看到菜品图加载出来" criterion requires WeChat Developer Tools or a real miniapp runtime with real backend/MySQL/COS data. That manual visual evidence is not fabricated in this report and remains for owner-side final acceptance.
