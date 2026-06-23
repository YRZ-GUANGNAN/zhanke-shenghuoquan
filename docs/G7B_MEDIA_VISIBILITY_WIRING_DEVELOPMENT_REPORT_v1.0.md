# G-7b Media Visibility Wiring Development Report v1.0

Date: 2026-06-23
Branch: `feat/g7b-media-visibility-wiring`
Baseline: `master@a4fe57b5` (`media-publish-reference-baseline`)
Final commit: the commit containing this report

## 1. Scope

This patch wires the already-frozen media `PublishReference` capability into the takeout transmission layer only:

- dish image visibility is published after merchant `PutOnShelf` succeeds;
- merchant logo visibility is published after admin review target approves a merchant;
- no migration, no new route, no media package change, no takeout core state-machine change.

The MVP limitation is intentional and unchanged: this module publishes visibility only. It does not retract media visibility on dish off-shelf/sold-out, merchant suspension, or other later lifecycle changes.

## 2. Preflight and file-level findings

Preflight checks passed before development:

- `git log --oneline -1 master`: `a4fe57b5 fix(media): publish reference by coordinates with CAS visibility gates`
- `git tag --list media-publish-reference-baseline g7b-merchant-api-baseline`: both tags present
- `rg "db\.AutoMigrate\(" backend/cmd/server`: 0 matches
- main repository worktree clean before creating `feat/g7b-media-visibility-wiring`

File-level findings:

- `media.Service.PublishReference(ctx, assetPublicID, referenceType, referenceID, slot)` is the public media capability for resolving an asset/reference coordinate, moving `media_asset_references.status` from `pending` to `active` and `parent_visibility` from `hidden` to `visible` with CAS and idempotent behavior.
- `takeoutmerchant.Handler` already had a media service dependency for media reference binding during dish/onboarding creation, so dish publishing reuses that dependency.
- `takeoutmerchant.putDishOnShelf` has the required values after successful `catalog.PutOnShelf`: `dish.ImageAssetPublicID`, `dish.DishPublicID`, fixed `PurposeDish`, and slot `0`.
- `takeoutconsole/reviewtarget.MerchantTarget` had merchant status approval logic but did not receive media service injection. Minimal composition-root wiring now passes the existing `coreMediaService`.
- After merchant approval, `MerchantTarget` has `merchant.LogoAssetPublicID`, `merchant.MerchantPublicID`, fixed `PurposeMerchantLogo`, and slot `0`.

## 3. Implementation

Changed production files:

- `backend/internal/module/admin/takeoutmerchant/handler.go`
  - calls `publishDishImage` after successful dish on-shelf transition;
  - skips publishing when the dish has no image;
  - maps media publish errors to existing response envelope categories instead of swallowing them.
- `backend/internal/module/admin/takeoutconsole/reviewtarget/target.go`
  - injects `*media.Service` into `MerchantTarget`;
  - calls `publishMerchantLogo` after merchant approval succeeds;
  - skips publishing when the merchant has no logo.
- `backend/cmd/server/main.go`
  - minimally passes `coreMediaService` to `takeoutreview.NewTargets`.

Changed tests:

- `backend/internal/module/admin/takeoutmerchant/http_mysql_test.go`
  - proves dish image starts hidden and becomes Gateway-visible after on-shelf;
  - proves no-image dish can be put on shelf without media error;
  - proves off-shelf/sold-out do not retract visibility for MVP;
  - extends cleanup/residue checks to media assets/references.
- `backend/internal/module/admin/takeoutconsole/reviewtarget/http_mysql_test.go`
  - proves merchant logo starts hidden and becomes Gateway-visible after review approval;
  - keeps F non-atomicity observable;
  - extends cleanup/residue checks to media assets/references.

## 4. Boundary confirmation

- No files under `backend/internal/module/media` were modified.
- No migration files were added or modified.
- Production `backend/internal/module/takeout` still has zero media, Gin, or `net/http` imports.
- No new HTTP route or API contract was introduced.
- No G-7b frontend, G-7c, G-4, G-6c, P0-5, or drift-alignment work was started.

## 5. Known trade-off

If `PublishReference` fails after the business transition has succeeded, this patch does not roll back the already-completed takeout transition. The error is returned through the existing handler/target error path so the inconsistency is visible to callers and tests. This matches the authorized MVP scope: publish direction only, no new transaction/outbox design.

