# G-⑥b-1 Content Takedown Acceptance Evidence

Date: 2026-06-23
Branch: `feat/g6b1-content-takedown`

## Preflight

```text
master head: 1ecf6c0f docs(G-6a-ui): record takeout admin acceptance
tags:
  g6a-ui-takeout-admin-baseline
  g6a2-takeout-review-baseline
AutoMigrate scan: 0 matches under backend/cmd/server
worktree before development: clean
```

## Real DB schema evidence

```text
real_mysql_version=8.4.0
schema_check=PASS posts.review_status/products.review_status varchar(24) no CHECK/ENUM; migration_required=false
```

## Visibility evidence

Community public visibility:

```go
query = query.Where("review_status = ?", review.StatusApproved)
```

Trade public visibility:

```go
query = query.Where("review_status = ?", review.StatusApproved)
```

Trade search truth-check:

```go
query := r.db.WithContext(ctx).Where("id IN ? AND review_status = ?", ids, review.StatusApproved)
```

Conclusion: `taken_down` is not publicly visible without changing visibility predicates.

## Real HTTP + MySQL acceptance

```text
=== RUN   TestContentTakedownRealHTTPAndMySQL84
real_mysql_version=8.4.0
schema_check=PASS posts.review_status/products.review_status varchar(24) no CHECK/ENUM; migration_required=false
g6b1_real_http=PASS auth=401/403/200 post_taken_down_hidden=PASS product_taken_down_hidden=PASS product_status_preserved=PASS idempotent=PASS f_review_unchanged=PASS search_delete_event_idempotent=PASS
--- PASS: TestContentTakedownRealHTTPAndMySQL84
=== RUN   TestZZContentTakedownResidueAgainstMySQL84
g6b1_residue posts=0 products=0 users=0 search_outbox=0
--- PASS: TestZZContentTakedownResidueAgainstMySQL84
PASS
```

## Full backend regression

```text
go test ./... -count=1
PASS
```

## Boundary

```text
npm run check:boundaries
ok A-G boundaries plus isolated P0-4 media domain
```

## Verify

```text
npm run verify
PASS
```

Includes:

- check:boundaries
- admin typecheck
- miniapp typecheck
- backend tests

## Build

```text
npm run build
PASS
```

Notes:

- Miniapp build emitted existing `vendors.js (731 KiB)` warning.
- Admin Vite build completed successfully.

```text
go build ./...
PASS
```
