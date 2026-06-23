# G-7b MiniApp Merchant Pages Development Report v1.0

Date: 2026-06-23
Branch: `feat/g7b-ui-merchant`
Baseline: `master@6b304f4e` (`g7b-media-visibility-wiring-baseline`)
Final commit: the commit containing this report

## 1. Scope

This module adds the MiniApp merchant-side pages for domain G:

- merchant center entry from Profile;
- onboarding form with merchant logo upload;
- category creation/editing/reordering;
- dish creation/editing/status operations with dish image upload;
- merchant order board with accept/start preparing/mark ready actions;
- frontend media binding path: upload image -> receive `assetId` -> send it to frozen merchant API fields.

No backend file, migration, route, admin app, package manifest, or dependency was changed.

## 2. Preflight

Required checks passed before development:

- `git log --oneline -1 master`: `6b304f4e fix(G-7b): wire takeout media PublishReference visibility`
- `git tag --list g7b-media-visibility-wiring-baseline g7b-merchant-api-baseline`: both present
- `git status`: clean in main repository before creating this worktree

Worktree created:

```text
git worktree add ../g7b-ui-merchant -b feat/g7b-ui-merchant master
```

## 3. File-level findings

### Media upload

`apps/miniapp/src/services/media.ts` exposes:

- `chooseAndUploadImages(purpose, count, onProgress?)`
- returns `UploadedMedia[]`
- the usable asset public identifier is `UploadedMedia.assetId`

Existing purpose mapping before this module:

- `post_image -> post`
- `product_image -> product`
- `avatar -> avatar`

This module appends:

- `dish -> dish`
- `merchant_logo -> merchant_logo`

### Existing publish page break point

`apps/miniapp/src/pages/publish/index.tsx` already uploads images but then blocks business submission when `images.length > 0`; if allowed through, it still submits `imageAssetIds: []`. The break is therefore on the frontend business-binding side, not on upload completion. The merchant page avoids this by sending `logoAssetPublicId` and `imageAssetPublicId` directly.

### API and session pattern

`apps/miniapp/src/services/api.ts`:

- base URL defaults to `http://127.0.0.1:8080/api/v1`
- token is read from `domainA.token`
- response envelope is `{ code, message, data }`
- errors throw `message`

`apps/miniapp/src/stores/session.ts` stores the same token key.

### Backend merchant API contract

Actual backend routes under `/api/v1/takeout/merchant`:

- `GET /me`
- `POST /onboarding`
- `GET/POST /categories`
- `GET/PATCH /categories/:id`
- `GET/POST /dishes`
- `GET/PATCH /dishes/:id`
- `PATCH /dishes/:id/on-shelf`
- `PATCH /dishes/:id/sold-out`
- `PATCH /dishes/:id/restock`
- `PATCH /dishes/:id/off-shelf`
- `PATCH /dishes/:id/remove`
- `GET /orders`
- `GET /orders/:id`
- `PATCH /orders/:id/accept`
- `PATCH /orders/:id/start-preparing`
- `PATCH /orders/:id/mark-ready`

Important request fields:

- onboarding: `logoAssetPublicId`
- dish create/update: `imageAssetPublicId`, `expectedStatus`, `expectedRowVersion`
- status/order transitions: `expectedRowVersion`, and for on/off shelf also `expectedStatus`

### Media visibility

Frontend does not call a media visibility endpoint. Backend `g7b-media-visibility-wiring-baseline` automatically publishes:

- merchant logo when merchant review is approved;
- dish image when dish is put on shelf.

## 4. Implementation

Changed MiniApp files:

- `apps/miniapp/src/services/media.ts`
  - appended `dish` and `merchant_logo` purpose support.
- `apps/miniapp/src/services/g-takeout-merchant.ts`
  - new typed merchant API client.
- `apps/miniapp/src/pages/takeout-merchant/index.tsx`
  - new merchant center page.
- `apps/miniapp/src/pages/takeout-merchant/index.scss`
  - page styles using existing global card/button conventions.
- `apps/miniapp/src/pages/takeout-merchant/index.config.ts`
  - page title.
- `apps/miniapp/src/app.config.ts`
  - registered the merchant page.
- `apps/miniapp/src/pages/profile/index.tsx`
  - appended a Profile settings entry to navigate to merchant center.

## 5. Boundary confirmation

- No `backend/**` file changed.
- No migration changed.
- No `apps/admin/**` file changed.
- No `package.json` or `package-lock.json` diff.
- No new dependency, router library, tabBar, or state library.
- Existing buyer pages are not rewritten; Profile only gains one merchant entry.

