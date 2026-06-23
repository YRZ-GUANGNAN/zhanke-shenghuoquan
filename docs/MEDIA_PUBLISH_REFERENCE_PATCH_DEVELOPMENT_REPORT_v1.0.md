# Media Publish Reference Patch Development Report v1.0

## Scope

Authorized scope: add a complete Media Domain capability for publishing an existing media reference to visible state by business coordinates.

This patch does not connect takeout, admin, miniapp, or any A-F/G business workflow. It only extends `backend/internal/module/media`.

## Preflight

- `git log --oneline -1 master`: `baf9cb60 fix(media): add CAS parent visibility promotion capability`
- Required tags present:
  - `media-visibility-patch-baseline`
  - `g7b-merchant-api-baseline`
- `rg "db\.AutoMigrate\(" backend/cmd/server`: 0 matches
- Main worktree status before development: clean
- Development branch/worktree: `feat/media-publish-reference`

## File-Level Findings

### Existing lower-level Media methods

- `Service.TransitionReference(ctx, id, version, from, to)` delegates status CAS to repository.
- `Service.TransitionReferenceVisibility(ctx, id, version, hidden, visible)` only allows `hidden -> visible`, is idempotent when already visible, and returns existing CAS errors such as `ErrConcurrentUpdate`.
- `Repository.FindReference(ctx, id)` only reads by internal reference id.
- `Repository.FindAsset(ctx, assetPublicID)` resolves public asset id to the internal `media_assets.id`.

### Reference coordinate contract

`media_asset_references` has:

- `UNIQUE (reference_type, reference_id, slot)`
- `UNIQUE (asset_id, reference_type, reference_id, slot)`

Therefore the safe publish coordinate is:

`asset_public_id + reference_type + reference_id + slot`

The service resolves `asset_public_id -> asset_id`, then uses the asset-bound coordinate to prevent publishing the wrong asset.

### CAS sequence

Reference creation starts with `row_version = 1`.

The publish sequence is:

1. `pending -> active` using row version 1.
2. Re-read reference.
3. `hidden -> visible` using row version 2.
4. Final expected state: `active + visible`, `row_version = 3`.

## Implementation

Changed files:

- `backend/internal/module/media/repository.go`
  - Added `FindReferenceByCoordinates(ctx, assetID, referenceType, referenceID, slot)`.
  - Uses the existing unique coordinate `(asset_id, reference_type, reference_id, slot)`.
  - Read-only and normalized through existing `normalizeNotFound`.
- `backend/internal/module/media/service.go`
  - Added `PublishReference(ctx, assetPublicID, referenceType, referenceID, slot)`.
  - Resolves asset public id to internal asset id.
  - Loads the matching reference by asset-bound coordinate.
  - Delegates status change to existing `TransitionReference`.
  - Re-reads row version after status promotion.
  - Delegates visibility change to existing `TransitionReferenceVisibility`.
  - Keeps already active/visible references idempotent.
- `backend/internal/module/media/service_test.go`
  - Updated test stub to satisfy the expanded repository interface.
- `backend/internal/module/media/publish_reference_mysql_test.go`
  - Added real MySQL acceptance coverage for publish, idempotency, active/hidden completion, asset-bound coordinate validation, missing asset/reference, and residue.

## Explicit Non-Changes

- No migration added.
- No media state machine change.
- No Gateway behavior change.
- No takeout/admin/miniapp wiring.
- No `main.go` change.
- No old `internal/pkg/media` change.
- No P0-5 work.

## Known Notes

`npm ci` was run in the worktree only to install ignored `node_modules` for local build verification. No package files were changed.
