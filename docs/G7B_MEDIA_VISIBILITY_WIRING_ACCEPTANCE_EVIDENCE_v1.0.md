# G-7b Media Visibility Wiring Acceptance Evidence v1.0

Date: 2026-06-23
Branch: `feat/g7b-media-visibility-wiring`
Final commit: the commit containing this report

## 1. Preflight

```text
master HEAD: a4fe57b5 fix(media): publish reference by coordinates with CAS visibility gates
tags: media-publish-reference-baseline, g7b-merchant-api-baseline present
AutoMigrate grep: 0 matches under backend/cmd/server
```

## 2. Real HTTP + real MySQL evidence

Targeted command:

```text
go test ./internal/module/admin/takeoutmerchant ./internal/module/admin/takeoutconsole/reviewtarget -count=1 -v
```

Observed result:

```text
PASS campus-circle/backend/internal/module/admin/takeoutmerchant
PASS campus-circle/backend/internal/module/admin/takeoutconsole/reviewtarget
```

Important logs:

```text
real_http=PASS auth=401 owner_scope=PASS onboarding_review=PASS duplicate=409 media_reference=PASS cross_merchant=PASS order_flow=PASS stale_CAS=409
residue=0 users=0 merchants=0 categories=0 dishes=0 orders=0 review_tasks=0 media_assets=0 media_references=0
F_non_atomicity=EXPOSED ... logo_publish=GATEWAY_VISIBLE
residue review_tasks=0 merchants=0 dishes=0 users=0 media_assets=0
```

## 3. Full regression evidence

Full backend regression:

```text
go test ./... -count=1
```

Result: PASS on rerun. The first attempt hit an unrelated transient MySQL deadlock in existing media miniapp-flow tests; the immediate rerun passed all packages.

Repository verification:

```text
npm run verify
```

Result:

```text
ok A-G boundaries plus isolated P0-4 media domain
admin typecheck PASS
miniapp typecheck PASS
backend tests PASS
```

Build:

```text
npm run build
```

Result:

```text
miniapp build PASS with existing vendors.js size warning
admin build PASS
```

## 4. Scope guard evidence

Media package unchanged:

```text
git diff --name-only -- backend/internal/module/media
```

Result: no output.

Migrations unchanged:

```text
git diff --name-only -- backend/migrations
```

Result: no output.

Takeout core remains free of media/HTTP production imports:

```text
rg "campus-circle/backend/internal/module/media|github.com/gin-gonic/gin|net/http" backend/internal/module/takeout --glob '!**/*_test.go' -n
```

Result: no output.

## 5. Honest MVP behavior

The tests intentionally verify that dish image visibility is not retracted when the dish moves through sold-out/off-shelf and then on-shelf again. This is not a bug in this module; it is the authorized MVP publish-only boundary.

