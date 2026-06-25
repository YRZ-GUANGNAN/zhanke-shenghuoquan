# G-7c UI Rider Acceptance Evidence

Date: 2026-06-25  
Branch: `feat/g7c-ui-rider`  
Final commit: see `git log --oneline -1` and the owner handoff message

## Automated verification

### MiniApp typecheck

Command:

```powershell
npm --workspace apps/miniapp run typecheck
```

Result:

```text
PASS
tsc --noEmit completed successfully.
```

### MiniApp build

Command:

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

These are pre-existing bundle-size warnings and are not introduced by this module.

### Repository verify

Command:

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

### Backend full regression

Command:

```powershell
cd backend
go test ./... -count=1
```

Result:

```text
PASS
```

### Package manifest diff

Command:

```powershell
git diff -- package.json package-lock.json apps/miniapp/package.json
```

Result:

```text
PASS
No diff.
```

## Runtime verification status

No mock backend was introduced or used by this module.

The code is ready for owner-run real WeChat Developer Tools verification against:

- real backend
- real MySQL
- existing JWT/token flow
- frozen G-7c rider API

Manual runtime evidence still to be collected by owner:

1. unregistered user enters Profile -> Rider Center -> registration form
2. rider registers successfully and becomes active
3. claimable ready-for-pickup order appears
4. claim succeeds; competing/stale claim maps to a friendly conflict message
5. fulfillment advances through to-store, picked, to-user, complete
6. wallet updates after completed delivery
7. withdrawal success, invalid amount, and over-balance error messages display correctly

## Residue

Generated local development artifacts are ignored:

- `node_modules/`
- `apps/miniapp/node_modules/`
- `apps/miniapp/dist/`
- `apps/miniapp/.swc/`

Tracked residue: none beyond the intended frontend source and docs files.
