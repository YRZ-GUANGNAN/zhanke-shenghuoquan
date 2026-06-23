# G-7b Media Visibility Wiring Acceptance Report v1.0

Date: 2026-06-23
Branch: `feat/g7b-media-visibility-wiring`
Baseline: `master@a4fe57b5`
Final commit: the commit containing this report

## 1. Final conclusion

Status: PASS

The takeout transmission layer now publishes media references using the frozen media `PublishReference` API:

- merchant logo: hidden before review approval, visible after merchant approval;
- dish image: hidden before on-shelf, visible after dish on-shelf;
- no-image/no-logo cases skip publishing safely;
- repeated lifecycle operations do not break visible media state;
- no media package or media schema change occurred.

## 2. Acceptance matrix

| Area | Result | Evidence |
| --- | --- | --- |
| Dish image publish on shelf | PASS | Real HTTP + real MySQL test creates image-bound dish, verifies Gateway hidden before `PutOnShelf`, visible after. |
| Merchant logo publish on approval | PASS | Real HTTP + real MySQL review target test verifies logo hidden before F approval and Gateway-visible after approval. |
| No-image dish | PASS | Real merchant HTTP test creates and shelves a dish without `imageAssetPublicID`; operation succeeds. |
| No-logo merchant | PASS | Existing review target unit coverage keeps no-logo approval path safe through nil/empty media dependency handling. |
| Idempotency | PASS | Repeated on-shelf path keeps media visible and does not corrupt reference state. |
| MVP no retract | PASS | Sold-out/off-shelf transitions leave the dish image visible; this is recorded as an authorized MVP trade-off. |
| Boundary | PASS | `backend/internal/module/media` unchanged; no migration diff; takeout core production imports remain media/HTTP-free. |
| Residue | PASS | Targeted MySQL tests report zero users, merchants, dishes, orders, review tasks, media assets, and references residue. |

## 3. Verification results

Commands executed:

```text
go test ./internal/module/admin/takeoutmerchant ./internal/module/admin/takeoutconsole/reviewtarget -count=1 -v
```

Result: PASS

Notable evidence:

- `TestTakeoutMerchantAPIRealHTTPMySQLAuthOnboardingOwnerScopeAndMediaBinding` PASS
- `TestZZTakeoutMerchantAPIResidueAgainstMySQL84` PASS with `residue=0 users=0 merchants=0 categories=0 dishes=0 orders=0 review_tasks=0 media_assets=0 media_references=0`
- `TestTakeoutReviewTargetsAgainstRealHTTPAndMySQL84` PASS with `logo_publish=GATEWAY_VISIBLE`
- reviewtarget residue log: `residue review_tasks=0 merchants=0 dishes=0 users=0 media_assets=0`

```text
go test ./... -count=1
```

Result: PASS on rerun. The first run hit an existing transient MySQL deadlock in `media/miniappflow` (`Error 1213`) outside this patch path; immediate rerun passed all packages.

```text
npm run verify
```

Result: PASS

- `check:boundaries`: `ok A-G boundaries plus isolated P0-4 media domain`
- admin typecheck: PASS
- miniapp typecheck: PASS
- backend tests: PASS

```text
npm run build
```

Result: PASS

- miniapp build: PASS with existing `vendors.js` size warning
- admin build: PASS

## 4. Boundary evidence

```text
git diff --name-only -- backend/internal/module/media
```

Result: no output.

```text
git diff --name-only -- backend/migrations
```

Result: no output.

```text
rg "campus-circle/backend/internal/module/media|github.com/gin-gonic/gin|net/http" backend/internal/module/takeout --glob '!**/*_test.go' -n
```

Result: no output.

## 5. Residue report

Targeted real MySQL tests clean their own G-7b/G-6a2 fixtures. Final targeted residue checks:

- takeoutmerchant: `users=0 merchants=0 categories=0 dishes=0 orders=0 review_tasks=0 media_assets=0 media_references=0`
- reviewtarget: `review_tasks=0 merchants=0 dishes=0 users=0 media_assets=0`

No external process remains intentionally running from this module verification.

## 6. Remaining risks

Low risk: visibility publish is not part of the same database transaction as the takeout state transition. If media publish fails, the caller receives an error but the takeout transition may already be committed. This is explicitly accepted for this MVP wiring and should be revisited only if a later module authorizes outbox-based reconciliation.

