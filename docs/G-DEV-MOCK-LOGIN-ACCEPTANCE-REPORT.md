# DEV-MOCK-LOGIN Acceptance Report

## Result

Status: PASS

The local mock WeChat login path is now usable through the full `cmd/server` process without requiring real WeChat AppID/Secret.

## Acceptance Environment

- Backend branch: `feat/dev-mock-login`
- Baseline: `master@b2d12d2c`
- Backend process: full `go run ./cmd/server`
- HTTP address used for acceptance: `:18080`
- Real database: existing local MySQL from `backend/.env`
- Meilisearch: real Meilisearch v1.46.0 binary started temporarily from system temp, with temporary DB path
- WeChat AppID/Secret during mock test: explicitly overridden to empty only in the temporary process environment
- `ALLOW_MOCK_WECHAT_LOGIN=true`

No `.env` file was copied, modified, or committed.

## Mock Mode Acceptance Evidence

Temporary process environment:

- `APP_ENV=dev`
- `WECHAT_APP_ID=`
- `WECHAT_APP_SECRET=`
- `ALLOW_MOCK_WECHAT_LOGIN=true`

Observed result:

```text
MEILI_START=PASS
HEALTHZ=PASS
LOGIN_CODE=0 TOKEN_PRESENT=True TOKEN_LENGTH=225
AUTH_ME_CODE=0 USER_ID=193 OPENID_EXPECTED_PREFIX=mock_openid_local-buyer-
```

Meaning:

- Full backend started successfully.
- Startup no longer failed at legacy `NewWechatSafety`.
- `GET /healthz` returned success.
- `POST /api/v1/auth/wechat-login` returned code `0` and a JWT token.
- `GET /api/v1/auth/me` with that token returned code `0`.
- The login path used the existing `mock_openid_<code>` behavior.

## Production Path Evidence

Production behavior remains fail-closed:

- `newWechatMediaSafety()` calls the real `legacymedia.NewWechatSafety(...)` unless all of these are true:
  - non-prod environment
  - `AllowMockWechat == true`
  - empty WeChat AppID
  - empty WeChat Secret
- Unit coverage:
  - `TestNewWechatMediaSafetyUsesNoopOnlyForLocalMockWithoutCredentials`
  - `TestNewWechatMediaSafetyKeepsRealPathWhenCredentialsProvided`
  - `TestNewWechatMediaSafetyRejectsNonMockEmptyCredentials`
  - `TestNewWechatMediaSafetyKeepsProductionPath`
- Existing config test coverage still confirms production rejects unsafe auth configuration.

## Regression Results

### Targeted Backend Tests

```text
go test ./cmd/server ./internal/pkg/media -count=1
PASS

go test ./internal/config ./internal/module/user -count=1
PASS
```

### Full Backend Tests

```text
go test ./... -count=1
PASS
```

### Boundary Check

```text
npm run check:boundaries
ok A-G boundaries plus isolated P0-4 media domain
PASS
```

### Verify

`npm run verify` was executed after temporarily linking this worktree to the already-installed dependency directories from the main checkout.

```text
npm run verify
PASS
```

The first attempt failed only because the new worktree had no `node_modules`; after linking dependency directories, the same command passed.

### Build

`npm run build` was executed after temporarily linking this worktree to the already-installed dependency directories from the main checkout.

```text
npm run build
PASS
```

Observed existing warning:

- Miniapp `vendors.js` exceeds the recommended 244 KiB size limit.

No new package dependency was added.

## Residue Report

Database:

```text
DB_CLEANUP_USERS_DELETED=1
DB_CLEANUP_USERS_REMAINING=0
```

Processes and ports:

```text
PROCESS_RESIDUE_CHECK=PASS ports 18080/7700 stopped if present
```

Temporary files:

- Temporary Meilisearch DB path removed.
- Temporary server logs removed.
- Temporary `node_modules` junctions removed.
- Temporary Meilisearch binary removed from system temp.

Git residue:

- No generated build artifacts are staged.
- Only authorized source files, tests, and reports remain in the worktree.

## Final Conclusion

DEV-MOCK-LOGIN is PASS.

The local development login blocker is closed without weakening production behavior.

