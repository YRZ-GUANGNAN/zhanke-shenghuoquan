# G-7a-0b Media Resolve URL Acceptance Evidence

## Preflight evidence

```text
git log --oneline -1 master
813eb765 feat(G-7a): add buyer takeout backend API

git tag --list g7a-buyer-api-baseline media-display-endpoint-baseline
g7a-buyer-api-baseline
media-display-endpoint-baseline

rg "db\.AutoMigrate\(" backend/cmd/server
<no matches>

git status --short --branch
## master
```

## Implementation evidence

Changed production files:

```text
backend/internal/module/media/miniappflow/handler.go
backend/internal/module/media/miniappflow/types.go
```

Changed/added tests:

```text
backend/internal/module/media/miniappflow/display_handler_test.go
backend/internal/module/media/miniappflow/resolve_handler_external_test.go
```

No backend route group, takeout, frontend, migration, Gateway, display service, or COS signer contract was changed.

## Real E2E evidence

Command used with credentials loaded from local `backend/.env` into process environment only; credentials were not printed and are not committed.

```powershell
$env:MEDIA_RESOLVE_RUN_REAL_E2E='1'
go test ./internal/module/media/miniappflow -run '^TestResolveContentURLAgainstRealHTTPMySQLAndCOS$' -count=1 -v
```

Result:

```text
=== RUN   TestResolveContentURLAgainstRealHTTPMySQLAndCOS
    resolve_handler_external_test.go:150: real_http=PASS auth=401 visible_resolve=200 hidden=404 invalid=400 signed_cos_get=PASS display_302_unchanged=PASS db_residue=0
--- PASS: TestResolveContentURLAgainstRealHTTPMySQLAndCOS (1.07s)
PASS
ok  	campus-circle/backend/internal/module/media/miniappflow	2.668s
```

The test performs:

- real MySQL fixture creation for visible and hidden media references;
- real Tencent COS object PUT with version ID;
- authenticated real Gin route request to resolve endpoint;
- no-token request returning `401`;
- hidden reference returning `404`;
- invalid reference type returning `400`;
- signed COS URL fetch returning the expected payload;
- existing display endpoint returning `302`;
- database cleanup verification with residue `0`;
- COS version/delete-marker cleanup through fixed key cleanup helper.

## Regression evidence

```text
go test ./... -count=1
PASS for all backend packages
```

```text
npm run check:boundaries
ok A-G boundaries plus isolated P0-4 media domain
```

```text
npm run verify
check:boundaries PASS
admin typecheck PASS
miniapp typecheck PASS
backend tests PASS
```

```text
npm run build
miniapp build PASS with existing vendors size warning
admin build PASS
```

## Security evidence

- JWT remains only in `Authorization` header for the resolve request.
- JWT is never placed into URL.
- Response envelope contains only:
  - `url`
  - `expiresInSeconds`
- The JSON contract does not expose separate bucket, key, final version, upload ticket, or storage identity fields.
- Display endpoint continues to use the same signed URL flow via `302`.
