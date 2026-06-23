# G-7a-0 Media Display Endpoint Acceptance Evidence

## Git and scope

- Branch: `feat/media-display-endpoint`
- Baseline: `e4c40c5d feat(G-7b): add miniapp merchant center pages`
- Files changed are limited to:
  - `backend/cmd/server/main.go`
  - `backend/internal/module/media/**`
  - `backend/internal/module/media/miniappflow/**`
  - `docs/G7A0_MEDIA_DISPLAY_ENDPOINT_*`

## Route contract

Registered route:

```text
GET /api/v1/miniapp/media/display/:referenceType/:referenceId/:slot/content
```

The route is mounted under the existing miniapp media group:

```go
api.Group("/miniapp/media", userService.AuthRequired(), user.UnmutedRequired())
```

## Response behavior evidence

- `302` only when media is visible.
- `Location` contains a short-lived Tencent COS signed GET URL.
- Response body is empty.
- Invisible, hidden, revoked, unapproved, missing, or otherwise not displayable media returns `404`.
- Invalid `referenceType` or invalid `slot` returns `400`.

## Real MySQL evidence

Command:

```powershell
$env:MYSQL_DSN=<redacted from owner backend/.env>
go test ./internal/module/media -run 'TestMediaDisplayRepositoryReturnsOnlyVisibleFinalIdentityAgainstMySQL84|TestZZMediaDisplayDatabaseResidueAgainstMySQL84' -count=1 -v
```

Result:

```text
=== RUN   TestMediaDisplayRepositoryReturnsOnlyVisibleFinalIdentityAgainstMySQL84
--- PASS: TestMediaDisplayRepositoryReturnsOnlyVisibleFinalIdentityAgainstMySQL84 (0.05s)
=== RUN   TestZZMediaDisplayDatabaseResidueAgainstMySQL84
--- PASS: TestZZMediaDisplayDatabaseResidueAgainstMySQL84 (0.01s)
PASS
ok  	campus-circle/backend/internal/module/media	1.512s
```

## Regression evidence

```text
go test ./internal/module/media/... -count=1
ok  	campus-circle/backend/internal/module/media
ok  	campus-circle/backend/internal/module/media/miniappflow
```

```text
go test ./... -count=1
PASS for all backend packages
```

```text
npm run verify
ok A-G boundaries plus isolated P0-4 media domain
admin typecheck PASS
miniapp typecheck PASS
backend tests PASS
```

```text
npm run build
miniapp build PASS with existing vendors size warning
admin build PASS
```

## Real COS evidence

Current environment has no COS credentials in process env or this worktree `backend/.env`. The guarded external test was executed and skipped:

```text
=== RUN   TestMediaDisplaySignedGetAgainstRealCOS
    display_cos_external_test.go:30: real display COS credentials are not configured
--- SKIP: TestMediaDisplaySignedGetAgainstRealCOS (0.00s)
PASS
```

To run real COS validation later:

```powershell
$env:MEDIA_DISPLAY_RUN_REAL_COS='1'
# configure COS_SECRET_ID/COS_SECRET_KEY/COS_BUCKET/COS_REGION/COS_APP_ID
cd backend
go test ./internal/module/media -run '^TestMediaDisplaySignedGetAgainstRealCOS$' -count=1 -v
```

The test performs real PUT, fixed-version signed GET, body verification, and version/delete-marker cleanup.
