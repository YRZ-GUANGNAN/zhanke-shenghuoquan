# G-7a Buyer API Development Report

Branch: `feat/g7a-buyer-api`

Baseline: `3c1e8f35 feat(G-7a): add media display redirect endpoint`

Final commit hash: recorded in final handoff after commit.

## Scope

Implemented only the G-7a backend buyer transport layer:

- New transport package: `backend/internal/module/admin/takeoutbuyer`
- New public read query methods appended to domain G takeout query/repository layer
- Main composition-root wiring for `/api/v1/takeout/buyer/*`

No frontend, no migration, no payment endpoint, no media package changes, no takeout state-machine/CAS changes, no G-7a frontend, no G-7c, no G-4 payment, no P0-5.

## Preflight and file-level verification

- `master` baseline was `3c1e8f35`.
- Tags `media-display-endpoint-baseline` and `g7b-merchant-api-baseline` exist.
- `backend/cmd/server` contains no `db.AutoMigrate(...)`.
- Main worktree was clean before development.

Verified from source:

- `OrderService.CreateOrder(ctx, CreateOrderInput)` accepts `BuyerUserID`, `MerchantPublicID`, `Items`, `Address`, and `Remark`.
- `CreateOrder` validates dish ULID/quantity, rejects duplicate dishes, loads dishes through `CatalogReader.FindDish`, requires `DishOnShelf`, requires dish merchant match, recalculates line/item/delivery/total amounts server-side, and creates `pending_payment` order records.
- Existing `AdminQueryService.Merchants` / `Dishes` accept caller-supplied status filters and do not provide a dedicated public-visible query contract.
- There was no existing `ListOrdersByBuyer`.
- The media display endpoint is registered as `GET /api/v1/miniapp/media/display/:referenceType/:referenceId/:slot/content`; buyer dish DTOs point to `/api/v1/miniapp/media/display/dish/:dishPublicId/0/content`.
- `takeoutmerchant` was used as the transport style reference; buyer transport is simpler and has no owner/admin gate.
- `AuthRequired()` and `UnmutedRequired()` are existing Gin middleware; `CurrentUserID(c)` supplies the authenticated buyer ID.

## Implementation

### Domain G query additions

Appended to existing takeout query/repository files:

- `BuyerQueryRepository`
- `BuyerQueryService`
- `ListActiveMerchants`
- `FindActiveMerchant`
- `ListOnShelfDishesByMerchant`
- `ListOrdersByBuyer`

Public-visible filtering is enforced at query level:

- merchants: `status = active`
- dishes: `takeout_dishes.status = on_shelf` and parent merchant `status = active`
- buyer orders: `buyer_user_id = CurrentUserID`, optional status filter

### Buyer transport endpoints

Mounted under:

```text
/api/v1/takeout/buyer
```

with:

```go
AuthRequired(), UnmutedRequired()
```

Endpoints:

- `GET /merchants`
- `GET /merchants/:merchantPublicId`
- `GET /merchants/:merchantPublicId/dishes`
- `POST /orders`
- `GET /orders`
- `GET /orders/:orderPublicId`

No payment or mark-paid endpoint was added.

### Image display path

Dish DTOs include `imageContentUrl` only when `image_asset_public_id` is present:

```text
/api/v1/miniapp/media/display/dish/:dishPublicId/0/content
```

Merchant logo DTOs similarly expose `logoContentUrl` when a logo asset exists, using the existing display endpoint path.

## Boundary

- takeout core remains free of Gin, `net/http`, and A-F imports.
- Buyer transport lives under `internal/module/admin/takeoutbuyer`, outside takeout core.
- No migration was added.
