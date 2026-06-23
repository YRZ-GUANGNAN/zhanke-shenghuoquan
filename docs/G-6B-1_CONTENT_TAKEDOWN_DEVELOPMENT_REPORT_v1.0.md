# G-⑥b-1 Content Takedown Development Report v1.0

Date: 2026-06-23
Branch: `feat/g6b1-content-takedown`
Base: `1ecf6c0f docs(G-6a-ui): record takeout admin acceptance`

## Scope

Implemented backend-only administrator takedown for frozen B/C content domains:

- B Community posts: admin can force `posts.review_status` to `taken_down`.
- C Trade products: admin can force `products.review_status` to `taken_down`.
- Admin HTTP endpoints added behind existing `AuthRequired + AdminRequired`.

No frontend, no P0-5, no G-⑥b-ui, no F review workflow reuse, no migration.

## Preflight conclusions

### Gate status

- `master` includes G-⑥a-ui baseline commit.
- Tags present: `g6a-ui-takeout-admin-baseline`, `g6a2-takeout-review-baseline`.
- `backend/cmd/server` has no `db.AutoMigrate(`.
- Worktree was clean before implementation.

### Real DB schema

Real MySQL version: `8.4.0`.

`posts.review_status` and `products.review_status` are both:

- `varchar(24)`
- `NOT NULL`
- default `submitted`
- no DB-level CHECK / ENUM constraints

Conclusion: `taken_down` is an application-layer status extension; no migration is required.

### Visibility audit

Community visibility is positive-only:

```go
query = query.Where("review_status = ?", review.StatusApproved)
```

This appears in public post detail/list paths and following feed:

- `FindPost(..., publicOnly=true)`
- `ListPosts(..., publicOnly=true)`
- `ListFollowingPosts(...)`

Trade visibility is positive-only:

```go
query = query.Where("review_status = ?", review.StatusApproved)
```

This appears in public product detail/list and search truth-check paths:

- `Find(..., publicOnly=true)`
- `List(..., publicOnly=true)`
- `ListByIDs(...)`

Trade service also defaults public list status to `selling` when no status is provided. Therefore `taken_down` is naturally invisible without changing existing visibility predicates.

## Implementation

### Community

- Added `community.ReviewStatusTakenDown`.
- Added `Repository.AdminTakedownPost(ctx, id)`.
- Added `Service.AdminTakedown(ctx, postID, operatorID, reason)`.

Behavior:

- Locks target post row.
- Idempotent if already `taken_down`.
- Does not check author ownership.
- Only updates `review_status`.

### Trade

- Added `trade.ReviewStatusTakenDown`.
- Added `Repository.AdminTakedown(ctx, id)`.
- Added `Service.AdminTakedown(ctx, productID, operatorID, reason)`.

Behavior:

- Locks target product row.
- Idempotent if already `taken_down`.
- Does not check seller ownership.
- Only updates `review_status`; preserves `products.status`.
- Enqueues one `SearchDelete` event when transitioning into `taken_down`.

### Admin transport

Added routes under the existing admin handler:

- `PATCH /api/v1/admin/community/posts/:id/takedown`
- `PATCH /api/v1/admin/trade/products/:id/takedown`

Both routes require:

- `AuthRequired`
- `AdminRequired`
- request body with `reason`

Errors:

- no token: 401
- non-admin: 403
- not found: 404
- invalid request/reason: 400

### Reason handling

B/C schemas do not currently contain a dedicated admin takedown audit/reason field, and this task forbids reusing F review semantics or adding new audit infrastructure. The backend validates that `reason` is present and non-empty, but does not persist it. This is explicitly recorded as a remaining limitation rather than silently pretending full audit persistence exists.

## Files changed

- `backend/internal/module/admin/handler.go`
- `backend/internal/module/admin/content_takedown_http_mysql_test.go`
- `backend/internal/module/community/model.go`
- `backend/internal/module/community/repository.go`
- `backend/internal/module/community/service.go`
- `backend/internal/module/community/service_test.go`
- `backend/internal/module/trade/model.go`
- `backend/internal/module/trade/repository.go`
- `backend/internal/module/trade/service.go`
- `backend/internal/module/trade/service_test.go`

## Migration

No migration generated. Real MySQL confirmed no CHECK / ENUM constraint on B/C `review_status`.

## Boundary statement

- F `admin/review` package was not modified.
- Takeout, media, A/D/E domains were not modified.
- Existing B/C public visibility predicates were not changed.
- Existing B/C method signatures were not changed.
- `main.go` was not modified.

Final commit hash: recorded in final handoff after commit creation.
