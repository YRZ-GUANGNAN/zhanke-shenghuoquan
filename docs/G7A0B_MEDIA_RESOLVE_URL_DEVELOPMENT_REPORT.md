# G-7a-0b Media Resolve URL Development Report

## Summary

Status: Development Complete

Branch: `feat/media-resolve-url`

Baseline: `813eb765 feat(G-7a): add buyer takeout backend API`

Final commit: `TO_BE_FILLED`

This controlled media patch adds one authenticated miniapp endpoint that resolves a visible media reference into a short-lived Tencent COS signed GET URL for use by miniapp `<Image>` components.

## Preflight

- `git log --oneline -1 master`: `813eb765 feat(G-7a): add buyer takeout backend API`
- Tags present: `g7a-buyer-api-baseline`, `media-display-endpoint-baseline`
- `rg "db\.AutoMigrate\(" backend/cmd/server`: no matches
- Worktree before development: clean

## File-level verification

- Existing display endpoint:
  - File: `backend/internal/module/media/miniappflow/handler.go`
  - Route: `GET /display/:referenceType/:referenceId/:slot/content`
  - Behavior: calls `display.ContentURL(...)`, then returns `302` with `Location`.
- Existing display service:
  - File: `backend/internal/module/media/display_service.go`
  - `MediaDisplayService.ContentURL(...)` validates `Purpose` and reference ID, calls `MediaGateway.BuildDisplayView(...)`, then rechecks fixed final identity via `FindVisibleDisplayObjectByReference(...)`, then signs via `SignedGetURL(...)`.
  - TTL: `MediaDisplayURLTTL = 5 * time.Minute`.
- Miniapp media route protection:
  - File: `backend/cmd/server/main.go`
  - `/api/v1/miniapp/media` is mounted under `AuthRequired()` and `UnmutedRequired()`.
- Buyer DTO coordinate shape:
  - File: `backend/internal/module/admin/takeoutbuyer/types.go`
  - `logoContentUrl` and `imageContentUrl` currently use `/api/v1/miniapp/media/display/:referenceType/:referenceId/0/content`.
  - Resolve uses the same `referenceType/referenceId/slot` coordinate.

## Implementation

Changed files:

- `backend/internal/module/media/miniappflow/handler.go`
  - Added route: `GET /resolve/:referenceType/:referenceId/:slot`.
  - Added `resolveContentURL` handler.
  - The handler reuses the existing display service `ContentURL(...)`; it does not duplicate visibility or signing logic.
- `backend/internal/module/media/miniappflow/types.go`
  - Added `ResolvedDisplayURL` DTO: `url`, `expiresInSeconds`.
- `backend/internal/module/media/miniappflow/display_handler_test.go`
  - Added unit coverage for resolve success, invisible fail-closed, and invalid route params.
- `backend/internal/module/media/miniappflow/resolve_handler_external_test.go`
  - Added guarded real acceptance test `TestResolveContentURLAgainstRealHTTPMySQLAndCOS`.
  - Default behavior is skip unless `MEDIA_RESOLVE_RUN_REAL_E2E=1`.

## Contract

Endpoint:

```text
GET /api/v1/miniapp/media/resolve/:referenceType/:referenceId/:slot
```

Success response:

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "url": "<short-lived COS signed GET URL>",
    "expiresInSeconds": 300
  }
}
```

Failure behavior:

- No token: `401`
- Invalid `referenceType` or invalid `slot`: `400`
- Missing, hidden, unapproved, revoked, or otherwise not visible media: `404`

## Boundary confirmation

- No JWT is placed in any URL.
- No new migration.
- No takeout change.
- No frontend change.
- No old media package dependency.
- Existing display `302` behavior unchanged.
- Resolve JSON does not expose separate `final_key`, `final_version_id`, upload ticket, bucket, or storage identity fields.
