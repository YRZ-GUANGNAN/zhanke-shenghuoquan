# G-7a Buyer MiniApp Frontend Acceptance Evidence

## Preflight evidence

```text
git log --oneline -1 master
a2af8ba6 docs(G-7a): record media resolve URL acceptance evidence

git tag --list media-resolve-url-baseline g7a-buyer-api-baseline
g7a-buyer-api-baseline
media-resolve-url-baseline

git status --short --branch
## master
```

## File-level evidence

- Buyer routes verified in `backend/internal/module/admin/takeoutbuyer/handler.go`.
- Buyer DTO fields verified in `backend/internal/module/admin/takeoutbuyer/types.go`.
- Resolve endpoint verified in `backend/internal/module/media/miniappflow/handler.go`.
- Auth group verified in `backend/cmd/server/main.go`.
- Miniapp request/token pattern verified in `apps/miniapp/src/services/api.ts`.
- Existing BottomNav and page registration verified in `apps/miniapp/src/components.tsx` and `apps/miniapp/src/app.config.ts`.

## Automated evidence

```text
npm --workspace apps/miniapp run typecheck
> tsc --noEmit
PASS
```

```text
npm run build:miniapp
> taro build --type weapp
PASS
Warnings: existing vendors.js size warning and NoAsyncChunksWarning
```

```text
npm run verify
check:boundaries PASS
admin typecheck PASS
miniapp typecheck PASS
backend tests PASS
```

```text
git diff --check
PASS
```

## Media chain evidence from implementation

The component `ResolvedMediaImage`:

1. Receives `imageContentUrl` / `logoContentUrl`.
2. Converts `/api/v1/miniapp/media/display/:referenceType/:referenceId/:slot/content` into `/miniapp/media/resolve/:referenceType/:referenceId/:slot`.
3. Calls existing `request()` so token remains in `Authorization` header.
4. Stores returned `{ url, expiresInSeconds }` in an in-memory TTL cache.
5. Assigns only the returned COS signed URL to `<Image src>`.

No JWT is placed in `<Image src>`.

## Runtime evidence pending

Final "dish image visibly rendered" evidence requires WeChat Developer Tools / real miniapp runtime. This report does not claim that visual proof was executed by automation.
