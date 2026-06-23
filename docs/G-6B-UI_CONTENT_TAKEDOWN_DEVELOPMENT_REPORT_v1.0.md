# G-⑥b-ui Content Takedown Development Report v1.0

Date: 2026-06-23  
Branch: `feat/g6b-ui-content-takedown`  
Base: `835d4c5d feat(G-6b-1): add admin takedown for community and trade content`  
Final commit: recorded in the owner handoff / `git log` output. A commit cannot stably embed its own final hash because Git hashes are content-addressed.

## Scope

Implemented the admin frontend increment for G-⑥b content governance:

- Community post list: row-level administrator takedown action.
- Trade product list: row-level administrator takedown action.
- Frontend API calls to the frozen G-⑥b-1 backend takedown endpoints.

No backend, migration, domain model, route registration, or package dependency changes were made.

## Preflight file-level verification

### Admin frontend files

`git ls-files apps/admin/src` confirmed the existing admin frontend consists of:

- `apps/admin/src/App.tsx`
- `apps/admin/src/MediaConsole.tsx`
- `apps/admin/src/TakeoutConsole.tsx`
- `apps/admin/src/main.tsx`
- `apps/admin/src/services/f-admin.ts`
- `apps/admin/src/services/takeout-admin.ts`
- `apps/admin/src/store.ts`
- `apps/admin/src/styles.css`
- `apps/admin/src/types.ts`

### Existing list locations

The community and trade admin lists are both rendered in `apps/admin/src/App.tsx`:

- `Posts()` renders the post admin table with title, author, topic and review status.
- `Products()` renders the product admin table with title, seller, price, business status and review status.

The pre-existing lists had no detail page and no row-level operation buttons. The existing write-operation style in `Reviews()` uses local React state, direct `adminAPI` calls, inline error reporting and refresh-after-write; the new takedown UI follows this lightweight local-state pattern.

### API client pattern

`apps/admin/src/services/f-admin.ts` is the existing F/Admin client:

- Base path: `/api/v1`
- Token source: `localStorage.getItem('domainA.token')`
- Auth header: `Authorization: Bearer <token>`
- Response envelope: `{code,message,data}`
- Error handling: throws `Error(message || HTTP status)` when HTTP status is not OK or `code !== 0`.

No existing export signature was changed.

### Backend endpoint contract

`git show --stat 835d4c5d` confirmed G-⑥b-1 added the backend handler and tests. The actual registered endpoints in `backend/internal/module/admin/handler.go` are:

- `PATCH /api/v1/admin/community/posts/:id/takedown`
- `PATCH /api/v1/admin/trade/products/:id/takedown`

Request body:

```json
{"reason":"..."}
```

Success response follows the unified envelope and returns:

```json
{"updated":true}
```

Empty reason is rejected by the backend with HTTP 400. Missing token returns 401. Non-admin token returns 403.

## Implementation

### `apps/admin/src/services/f-admin.ts`

Added two API functions:

- `takedownPost(id, reason)`
- `takedownProduct(id, reason)`

Both reuse the existing token/envelope/error handling path.

### `apps/admin/src/App.tsx`

Added:

- shared `takedownReason(kind,title)` confirmation/prompt helper;
- `TakedownPosts()` list wrapper;
- `TakedownProducts()` list wrapper;
- section routing so `community` and `trade` use the new wrappers.

Behavior:

- Displays `下架` only for rows with `reviewStatus === 'approved'`.
- Shows a second confirmation before submitting.
- Prompts for reason with honest copy: the reason is used for this operation's validation only, and the backend currently has no takedown reason archive field.
- Disables the clicked row button and shows `下架中…` while the request is in flight.
- Refreshes the list after success.
- Shows backend error messages for failures, including empty reason 400.

### `apps/admin/src/styles.css`

Added minimal styles for:

- `.content-takedown-button`
- `.content-takedown-button:disabled`
- `.content-takedown-error`

## Boundary result

- Modified production frontend files only under `apps/admin/src/**`.
- No `backend/**` production file modified.
- No migration modified or added.
- No `main.go` change.
- No `package.json` / `package-lock.json` diff.
- No new router library or state management library introduced.

## Temporary validation infrastructure

The full `cmd/server` could not boot locally because this workstation process did not have complete COS credentials:

```text
configure Tencent COS: COS credentials, bucket, region and app id are required
```

Per owner authorization, a temporary composition root was used for acceptance. It assembled the same production Gin handlers, services, repositories and JWT middleware needed for A/F/B/C admin takedown validation against the real MySQL database. It did not mock backend handlers or bypass routes.

Temporary files were not committed and were removed before final commit:

- `backend/tmp-g6b-ui-e2e/server.go`
- `backend/tmp-g6b-ui-dbcheck/main.go`
- `tmp-g6b-ui-logs/**`

## Known limitation

The in-app browser loaded the real admin Vite page, but its security policy prevented writing the seeded admin token into `localStorage.domainA.token`. Because the admin UI has no login page, authenticated browser-click evidence could not be produced by the agent in this environment. The implementation therefore includes real HTTP/Gin/MySQL evidence plus source/build evidence for the confirm/prompt/loading UI, but the final authenticated browser click should be manually re-run by the owner if strict UI-click proof is required.
