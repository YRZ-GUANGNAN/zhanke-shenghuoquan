# media-visibility-patch Development Report v1.0

Status: PASS

Branch: `feat/media-visibility-patch`

Commit: final branch commit; see handoff `git log --oneline -1`.

## Scope

This patch is a controlled P0-4 media subsystem extension. It only adds the missing parent visibility promotion capability inside `backend/internal/module/media`.

No takeout, admin, miniapp, A-F domain, migration, `main.go`, old `internal/pkg/media`, asynchronous consumer, or event pipeline code was changed.

## Preflight

- `git log --oneline -1 master`: `d00d5ddc feat(G-7b): add owner-scoped takeout merchant API`
- `git tag --list g7b-merchant-api-baseline media-purpose-takeout`: both tags present
- `rg "db\.AutoMigrate\(" backend/cmd/server`: 0 matches
- Initial worktree status: clean

## File-level findings

- `Service.TransitionReference(ctx, id, version, from, to)` validates `CanTransitionReference(from, to)` and delegates to repository `CASReferenceStatus`.
- `GormRepository.CASReferenceStatus` uses `id + row_version + status` as its CAS condition, updates `status`, increments `row_version`, and returns `ErrConcurrentUpdate` when affected rows are not exactly 1.
- `AssetReference` has a single shared `row_version`; parent visibility therefore uses the same CAS primitive as reference status.
- `ParentVisibility` values are `hidden` and `visible`.
- `MediaGateway` requires `ReferenceActive + ParentVisible` to return visible display metadata.

## Implemented changes

### Repository

Added:

```go
CASParentVisibility(ctx, id, expectedRowVersion, from, to ParentVisibility) error
```

Behavior:

- CAS predicate: `id = ? AND row_version = ? AND parent_visibility = ?`
- Updates only `parent_visibility`, `updated_at`, and `row_version`
- Reuses existing `casUpdate` error semantics
- Does not modify reference status, asset status, review projection, storage identity, or any other column

### Service

Added:

```go
TransitionReferenceVisibility(ctx, id, expectedRowVersion, from, to ParentVisibility) error
```

Behavior:

- Allows only `hidden -> visible`
- Rejects unsupported transitions with `ErrInvalidTransition`
- Reads the current reference first for idempotency
- If already `visible`, returns success without writing and without changing `row_version`
- Delegates state mutation only through repository CAS

### Tests

Added:

- `reference_visibility_mysql_test.go`
- service unit tests for allowed transition and idempotency

## Explicit non-goals

- No `visible -> hidden` reclaim path
- No `MediaParentVisibilityChanged` consumer/event implementation
- No takeout wiring
- No HTTP route
- No migration
- No Gateway behavior change
- No status machine change

## Known operational note

The worktree initially lacked `node_modules`; `npm ci` was used for verification only. No package or lockfile was changed.
