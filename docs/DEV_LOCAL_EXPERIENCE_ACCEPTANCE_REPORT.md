# DEV Local Experience Acceptance Report

Date: 2026-06-25  
Branch: `feat/dev-local-experience`  
Final commit: see owner handoff message / `git log --oneline -1`

## Conclusion

Status: **PASS**

The local-only development patch is complete and verified. It does not change production behavior.

## Acceptance matrix

| Item | Result | Evidence |
| --- | --- | --- |
| Local mock mode uses extended JWT TTL | PASS | `TestLocalMockWechatLoginUsesExtendedTokenTTLOnlyInDevNoCredentials` |
| Production/credentialed token TTL remains 15 minutes | PASS | same unit test covers prod, credentialed dev, non-mock dev |
| Config validation unchanged | PASS | no changes to `backend/internal/config/config.go`; `go test ./...` PASS |
| Existing local no-op WeChat media safety condition reused | PASS | `localMockWechatMode(cfg)` shared in `main.go` |
| New media local mock upload reaches approved | PASS | `TestCompleteUploadLocalMockAutoApprovesThroughProjectionEvents` |
| New media real MySQL projection becomes approved in local mock | PASS | `TestModule10LocalMockAutoApprovesProjectionAgainstMySQL84` |
| Production/credentialed review flow unchanged | PASS | `WithLocalMockAutoApproval` is only passed from main when local mock four-condition gate is true; existing non-option test still returns review pending |
| Media approval uses existing outbox/consumer path | PASS | test verifies submit + reviewing + decision events |
| No migration changes | PASS | no migration files changed |
| No frontend changes | PASS | no `apps/**` source changes |
| No package manifest changes | PASS | package diff empty |
| Boundary check | PASS | `npm run verify` |
| Backend full regression | PASS | `go test ./... -count=1` |
| MiniApp build | PASS | `npm run build:miniapp` |

## Commands executed

### Focused tests

```powershell
cd backend
go test ./internal/module/user ./internal/module/media/miniappflow ./cmd/server -count=1
```

Result:

```text
PASS
```

### Backend full regression

```powershell
cd backend
go test ./... -count=1
```

Result:

```text
PASS
```

### Full repository verify

```powershell
npm run verify
```

Result:

```text
PASS
check:boundaries: ok A-G boundaries plus isolated P0-4 media domain
admin typecheck: PASS
miniapp typecheck: PASS
backend go test ./...: PASS
```

### MiniApp build

```powershell
npm run build:miniapp
```

Result:

```text
PASS
Taro build --type weapp compiled successfully.
```

Observed warnings:

```text
AssetsOverSizeLimitWarning: vendors.js exceeds recommended size limit.
NoAsyncChunksWarning: webpack performance recommendations.
```

These are pre-existing bundle-size warnings.

### Package manifest check

```powershell
git diff -- package.json package-lock.json apps/admin/package.json apps/miniapp/package.json
```

Result:

```text
PASS
No diff.
```

### AutoMigrate check

```powershell
rg "db\.AutoMigrate\(" backend/cmd/server
```

Result:

```text
PASS
0 matches.
```

## Residue report

Ignored local build/dependency artifacts were created by verification:

- `node_modules/`
- `apps/miniapp/node_modules/`
- `apps/miniapp/dist/`
- `apps/miniapp/.swc/`

Tracked residue:

- none beyond intended source and documentation changes

Database residue:

- test suites completed successfully with their existing cleanup guards

## Final note

This patch is intentionally not a business feature. It only makes local manual verification less brittle by extending local mock login lifetime and allowing local mock image uploads to become visible without manual SQL.

