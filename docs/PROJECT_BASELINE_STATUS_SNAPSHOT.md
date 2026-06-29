# Project Baseline Status Snapshot

Date: 2026-06-26  
Repository state: `master` at `1fc26659 feat(G-4): add payment scaffold with isolated mock payment`  
Purpose: lightweight status snapshot for handoff while waiting for the next formal module authorization.

This document is not a new acceptance report and does not change any module status. It records the current frozen baselines visible from git tags, migrations, and existing reports.

## 1. Repository baseline

Current confirmed state:

- Active branch: `master`
- Current HEAD: `1fc26659`
- Latest committed module: `G-④ Payment Scaffold + Mock Payment`
- Current highest migration: `022_takeout_payment.sql`
- `db.AutoMigrate(` under `backend/cmd/server`: 0 matches
- Working tree before this document: clean

Recent history:

```text
1fc26659 feat(G-4): add payment scaffold with isolated mock payment
e80fe1bb fix(dev): improve local mock login and media visibility
3d603864 feat(G-7c): add rider miniapp center
27044b07 feat(G-7c): add rider role API transport
d9cab2f9 chore: ignore local Meilisearch runtime artifacts (data.ms/, meilisearch.exe)
c7248b83 fix(dev): enable local mock WeChat login without media safety credentials
```

## 2. Frozen baseline tags

Core infrastructure and media:

| Area | Tag | Status |
| --- | --- | --- |
| Startup schema authority | `infra-automigrate-retired` | Frozen |
| P0-3 Transaction / IM | `p0-3-pass` | Frozen |
| P0-4 Media baseline | `p0-4-media-baseline` | Frozen |
| Media purpose for takeout | `media-purpose-takeout` | Frozen |
| Media display endpoint | `media-display-endpoint-baseline` | Frozen |
| Media resolve URL endpoint | `media-resolve-url-baseline` | Frozen |
| Media visibility patch | `media-visibility-patch-baseline` | Frozen |
| Media publish reference patch | `media-publish-reference-baseline` | Frozen |
| Media visibility wiring | `g7b-media-visibility-wiring-baseline` | Frozen |

Takeout / G-domain:

| Module | Tag | Status |
| --- | --- | --- |
| G-① Merchant Domain | `g1-merchant-baseline` | Frozen |
| G-② Catalog Domain | `g2-catalog-baseline` | Frozen |
| G-③ Order Domain | `g3-order-baseline` | Frozen |
| G-④ Payment Scaffold + Mock Payment | `g4-payment-baseline` | Frozen |
| G-⑤ Rider Domain | `g5-rider-baseline` | Frozen |
| G-⑥a-1 Takeout Admin Backend | `g6a1-takeout-admin-baseline` | Frozen |
| G-⑥a-2 Takeout Review Wiring | `g6a2-takeout-review-baseline` | Frozen |
| G-⑥a UI Takeout Admin | `g6a-ui-takeout-admin-baseline` | Frozen |
| G-⑥b-1 Content Takedown Backend | `g6b1-content-takedown-baseline` | Frozen |
| G-⑥b UI Content Takedown | `g6b-ui-content-takedown-baseline` | Frozen |
| G-⑦a Buyer API | `g7a-buyer-api-baseline` | Frozen |
| G-⑦a Buyer UI | `g7a-ui-buyer-baseline` | Frozen |
| G-⑦b Merchant API | `g7b-merchant-api-baseline` | Frozen |
| G-⑦b Merchant UI | `g7b-ui-merchant-baseline` | Frozen |
| G-⑦c Rider API | `g7c-rider-api-baseline` | Frozen |
| G-⑦c Rider UI | `g7c-ui-rider-baseline` | Frozen |

Developer/local experience:

| Area | Tag | Status |
| --- | --- | --- |
| Local mock login | `dev-mock-login-baseline` | Frozen |
| Local developer experience | `dev-local-experience-baseline` | Frozen |

## 3. Migration sequence

Current migration chain is contiguous through `022`:

```text
001_init.sql
002_seed_dev.sql
003_domain_convergence.sql
004_auth_hardening.sql
005_community_loop.sql
006_transaction_closure.sql
007_media_pipeline.sql
008_media_dimensions_and_audit.sql
009_media_domain_v11.sql
010_media_upload_ticket_service.sql
011_media_upload_completion.sql
012_media_review_projection.sql
013_media_consumer.sql
014_media_gc.sql
015_media_assets_drift_reconcile.sql
016_takeout_merchant.sql
017_takeout_catalog.sql
018_media_reference_type_takeout.sql
019_takeout_order.sql
020_takeout_rider.sql
021_takeout_withdraw.sql
022_takeout_payment.sql
```

Important constraint remains active: schema authority is migrations only. Startup `AutoMigrate` remains retired.

## 4. Current functional posture

### P0 foundation

- P0-1 Authorization: PASS / frozen by report.
- P0-2 Community Loop: PASS / frozen by report.
- P0-3 Transaction Closure: PASS / frozen by `p0-3-pass`.
- P0-4 Media Pipeline: PASS / frozen by media baseline and follow-up media tags.

### G takeout stack

Current takeout stack now has:

- Merchant onboarding/domain core.
- Catalog/category/dish domain core.
- Order domain core.
- Rider domain core.
- Payment scaffold with isolated local mock payment.
- Admin backend and admin UI for takeout.
- Content takedown backend and admin UI.
- Buyer API and miniapp buyer pages.
- Merchant API and miniapp merchant pages.
- Rider API and miniapp rider pages.
- Media display/resolve and publish-reference support for takeout images.

## 5. Known open items

These are not defects introduced by this snapshot; they are the next known authorization candidates or external dependencies.

1. Real WeChat Pay production integration remains blocked on merchant account credentials.
   - Current G-④ includes scaffold and local mock payment.
   - Production mock payment is fail-closed by config validation.

2. Enterprise WeCom webhook order notifications are not implemented yet.
   - Likely trigger candidates:
     - merchant group: order reaches `paid`;
     - rider group: order reaches `ready_for_pickup`.
   - Needs separate authorization before implementation.

3. Production operations/P0-5 remains outside current scope.
   - No monitoring/backup/alerting rollout should be started without explicit authorization.

4. Historical schema drift outside already reconciled media drift remains a known deferred item.
   - Do not assume a migration-only rebuild equals the current production shape until the drift-alignment专项 is authorized and completed.

## 6. Suggested next formal step

If continuing by the established discipline, the next safe module is:

`Enterprise WeCom webhook notification` read/design/development authorization.

Recommended boundary:

- No order state-machine changes unless explicitly authorized.
- No blocking webhook calls inside critical order transactions.
- Webhook URLs must be `.env` configuration only and never committed.
- Sending failure must not convert successful order transitions into user-facing failure.

