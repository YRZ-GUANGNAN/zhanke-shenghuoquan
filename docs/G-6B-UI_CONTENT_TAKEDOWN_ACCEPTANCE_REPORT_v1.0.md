# G-⑥b-ui Content Takedown Acceptance Report v1.0

Date: 2026-06-23  
Branch: `feat/g6b-ui-content-takedown`  
Base: `835d4c5d`  
Final commit: recorded in the owner handoff / `git log` output. A commit cannot stably embed its own final hash because Git hashes are content-addressed.  
Status: `CONDITIONAL PASS`

## Summary

The frontend implementation, static validation, real Gin handler validation, real JWT authorization checks and real MySQL state checks passed.

The only conditional item is authenticated browser-click evidence: the real admin page loaded in Vite, but the in-app browser security policy prevented the agent from writing the seeded admin token into `localStorage.domainA.token`. Since this admin frontend has no login page, the agent could not complete the final "logged-in browser click" step without owner-side token injection. No PASS is claimed for that specific browser-click item.

## Static validation

| Check | Result |
|---|---:|
| `npm --workspace apps/admin run typecheck` | PASS |
| `npm --workspace apps/admin run build` | PASS |
| `npm run verify` | PASS |
| `package.json` / lockfile diff | PASS, no diff |
| Backend tracked file diff | PASS, none |
| Boundary check | PASS |

## Real runtime validation environment

The full `cmd/server` was attempted first and failed before route startup due missing local COS credentials:

```text
configure Tencent COS: COS credentials, bucket, region and app id are required
```

Authorized fallback was used:

- temporary composition root;
- real MySQL from the main project `backend/.env`;
- real GORM repositories;
- real services;
- real Gin handlers;
- real `AuthRequired` + `AdminRequired` JWT middleware;
- no mock backend handler;
- no bypass of HTTP route contracts.

The temporary server exposed the frozen production endpoints at `127.0.0.1:18080` for the validation run only. It was stopped and deleted after the run.

## Real HTTP + MySQL acceptance matrix

| Case | Result | Evidence |
|---|---:|---|
| Seeded post visible in admin list before takedown | PASS | `adminPost=true` |
| Seeded product visible in admin list before takedown | PASS | `adminProduct=true` |
| Seeded post visible in public feed before takedown | PASS | `publicPost=true` |
| Seeded product visible in public list before takedown | PASS | `publicProduct=true` |
| Post takedown without token | PASS | HTTP 401 `authentication required` |
| Post takedown with ordinary user | PASS | HTTP 403 `admin required` |
| Product takedown without token | PASS | HTTP 401 `authentication required` |
| Product takedown with ordinary user | PASS | HTTP 403 `admin required` |
| Empty post reason rejected by backend | PASS | HTTP 400 `post id, operator and takedown reason are required` |
| Admin post takedown | PASS | HTTP 200, `updated=true` |
| Repeated post takedown idempotent | PASS | HTTP 200 |
| Admin product takedown | PASS | HTTP 200, `updated=true` |
| Repeated product takedown idempotent | PASS | HTTP 200 |
| Post DB status | PASS | `posts.review_status=taken_down` |
| Product DB review status | PASS | `products.review_status=taken_down` |
| Product business status preserved | PASS | `products.status=selling` |
| Post hidden from public list after takedown | PASS | public count / API visibility = 0 |
| Product hidden from public list after takedown | PASS | public count / API visibility = 0 |

Direct DB check after takedown:

```json
{
  "post_review_status": "taken_down",
  "product_review_status": "taken_down",
  "product_status": "selling",
  "public_post_count": 0,
  "public_product_count": 0
}
```

## Frontend behavior verification

| Requirement | Result | Evidence |
|---|---:|---|
| Existing admin page loads from real Vite dev server | PASS | `http://127.0.0.1:5173` returned the real app |
| Existing API client token/envelope path reused | PASS | `f-admin.ts` uses existing `/api/v1`, `domainA.token`, envelope parsing |
| Post row takedown action added | PASS | source/build verified |
| Product row takedown action added | PASS | source/build verified |
| Button shown only for approved rows | PASS | `reviewStatus === 'approved'` guard |
| Second confirmation exists | PASS | `window.confirm(...)` in `takedownReason` |
| Reason prompt exists | PASS | `window.prompt(...)` in `takedownReason` |
| Reason copy is honest | PASS | copy says reason is only for this operation and backend has no archive field |
| Loading/disabled state exists | PASS | `busy` row id disables button and renders `下架中…` |
| Empty reason frontend does not misjudge success | PASS | frontend submits and displays backend 400 error |
| Authenticated browser-click run | CONDITIONAL | blocked by browser localStorage security policy; owner can re-run after manually setting `domainA.token` |

## Security and permission checks

- No token: 401.
- Ordinary user token: 403.
- Admin token: 200.
- Frontend did not introduce a new authentication path.
- Frontend still uses the existing `domainA.token` and `Authorization: Bearer` mechanism.

## Boundary checks

- `npm run verify`: PASS.
- `check:boundaries`: PASS.
- No backend production file modified.
- No migration modified.
- No `package.json` or dependency changes.
- No router/state library added.

## Conclusion

`CONDITIONAL PASS`

The code is ready for owner validation and branch review. The only remaining condition is manual authenticated browser-click evidence, because this agent environment could not set `domainA.token` in the in-app browser and the admin app has no login screen. The backend contract and persistence behavior have been verified against real MySQL and real Gin/JWT middleware.
