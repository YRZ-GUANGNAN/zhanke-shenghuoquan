# G-7B Merchant API Development Report v1.0

Date: 2026-06-23  
Branch: `feat/g7b-merchant-api`  
Base: `9d7ff3e6 feat(G-6b-ui): add admin content takedown actions`  
Final commit: recorded in owner handoff / `git log` output. A commit cannot stably embed its own final hash because Git hashes are content-addressed.

## Scope

Implemented the backend merchant-role transport layer for the takeout domain G:

- `GET /api/v1/takeout/merchant/me`
- `POST /api/v1/takeout/merchant/onboarding`
- owner-scoped merchant category management
- owner-scoped merchant dish management
- owner-scoped merchant order list/detail and merchant-side order transitions

This is the first takeout transport layer for ordinary authenticated users. It uses A-domain JWT authentication and derives merchant identity from `takeout_merchants.owner_user_id`; it does not add role fields to the A-domain user table.

Out of scope and not implemented:

- miniapp frontend pages
- buyer endpoints
- rider endpoints
- payment callbacks
- admin console changes
- new migrations
- P0-5 / operations work

## Preflight verification

### Git and baseline

- `git log --oneline -1 master` returned `9d7ff3e6 feat(G-6b-ui): add admin content takedown actions`.
- `git tag --list g6b-ui-content-takedown-baseline g6a2-takeout-review-baseline` returned both required tags.
- `rg "db\.AutoMigrate\(" backend/cmd/server` returned no matches.
- `git status --short` was clean before branch work.

### A-domain auth middleware

File: `backend/internal/module/user/middleware.go`

- `func (s *Service) AuthRequired() gin.HandlerFunc`
- `func CurrentUser(c *gin.Context) User`
- `func CurrentUserID(c *gin.Context) uint`
- `func AdminRequired() gin.HandlerFunc`

Conclusion: `AuthRequired()` is the reusable "authenticated user" middleware. `AdminRequired()` is not used for this merchant API.

### Takeout services

Merchant service:

- `CreateMerchant(ctx, input)`
- `Approve(ctx, publicID, expectedRowVersion)`
- `Reject(ctx, publicID, expectedRowVersion)`
- `OpenForBusiness(ctx, publicID, ownerUserID, expectedRowVersion)`
- `Rest(ctx, publicID, ownerUserID, expectedRowVersion)`
- `Suspend(ctx, publicID, expectedStatus, expectedRowVersion)`
- `Unsuspend(ctx, publicID, expectedRowVersion)`
- `FindMerchant(ctx, publicID)`

Catalog service:

- `CreateCategory`
- `RenameCategory`
- `ReorderCategory`
- `FindCategory`
- `CreateDish`
- `UpdateDish`
- `PutOnShelf`
- `MarkSoldOut`
- `Restock`
- `TakeOffShelf`
- `RemoveDish`
- `FindDish`

Order service:

- `CreateOrder`
- `MarkPaid`
- `Accept`
- `StartPreparing`
- `MarkReady`
- `StartDelivering`
- `CompleteDelivery`
- `Cancel`
- `Refund`
- `FindOrder`
- `ListOrderItems`

### F review integration

File: `backend/internal/module/admin/review/service.go`

- `type Submitter interface { Submit(context.Context, Submission) (Task, error) }`
- `Submit(ctx, review.Submission)` creates a `review_tasks` row with `submitted` status.

Takeout review target:

- Path: `backend/internal/module/admin/takeoutconsole/reviewtarget`
- Merchant object type: `takeout_merchant`
- Dish object type: `takeout_dish`

The merchant onboarding endpoint reuses `takeout_merchant` for F-domain submission.

### Existing admin transport pattern

The existing admin transport adapter is:

- `backend/internal/module/admin/takeoutconsole`

It is outside `backend/internal/module/takeout`, keeping takeout core free from Gin/HTTP imports. G-7B follows the same transport-adapter pattern by adding:

- `backend/internal/module/admin/takeoutmerchant`

### Media binding status

Existing fields:

- `takeout_merchants.logo_asset_public_id`
- `takeout_dishes.image_asset_public_id`

Existing domain behavior:

- `MerchantService.CreateMerchant` accepts and stores `LogoAssetPublicID`.
- `CatalogService.CreateDish` and `UpdateDish` accept and store `ImageAssetPublicID`.

Media service public binding capability:

- `media.Service.CreateReferenceRecord(ctx, media.AssetReference)`
- `media.Service.TransitionReference(ctx, id, version, from, to)`

Backend already supports:

- `PurposeDish = "dish"`
- `PurposeMerchantLogo = "merchant_logo"`

## Implementation

### New transport package

Added:

- `backend/internal/module/admin/takeoutmerchant/handler.go`
- `backend/internal/module/admin/takeoutmerchant/types.go`
- `backend/internal/module/admin/takeoutmerchant/http_mysql_test.go`

Key behavior:

- Mount root: `/api/v1/takeout/merchant`
- Auth: `AuthRequired()` only; no `AdminRequired()`
- Merchant identity lookup: `takeout_merchants.owner_user_id = current user ID`
- Active merchant is required for category/dish/order operations.
- Not-onboarded users can only call onboarding and `me`.

### Takeout core minimal additions

Added:

- `backend/internal/module/takeout/merchant_identity.go`
- `backend/internal/module/takeout/merchant_identity_repository.go`

This adds a minimal owner lookup service:

- `FindByOwnerUserID(ctx, ownerUserID)`

No Gin/HTTP imports were added to takeout core.

Extended query capability:

- `AdminQueryService.OrdersByMerchant(ctx, merchantPublicID, status, page, size)`
- repository method `ListOrdersByMerchant`

This is used to scope merchant order lists to the current merchant.

### Composition root

Updated:

- `backend/cmd/server/main.go`

Registered:

```text
/api/v1/takeout/merchant/*
```

with:

```go
api.Group("/takeout/merchant", userService.AuthRequired())
```

No `db.AutoMigrate` call was added.

### Media reference binding

The merchant API validates media ownership and purpose before creating or mutating business objects:

- merchant logo must be owned by current user and have `purpose = merchant_logo`
- dish image must be owned by current user and have `purpose = dish`

Only after validation does it create/update the merchant or dish and create a media reference fact:

- `reference_type = merchant_logo` for merchant logo
- `reference_type = dish` for dish image
- initial reference status: `pending`
- parent visibility: `hidden`

The implementation intentionally does not make images visible, does not call MediaGateway, and does not implement frontend binding. That remains in later G-7 work.

## Boundary result

- No new migration.
- No change to A-domain user schema or roles.
- No change to G-1 to G-6 state machines.
- No change to admin takeout console behavior.
- No frontend changes.
- No P0-5 work.
- `npm run check:boundaries`: PASS.

