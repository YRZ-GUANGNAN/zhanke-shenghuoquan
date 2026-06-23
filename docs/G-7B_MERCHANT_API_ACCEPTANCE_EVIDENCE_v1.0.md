# G-7B Merchant API Acceptance Evidence v1.0

Date: 2026-06-23  
Branch: `feat/g7b-merchant-api`  
Final commit: recorded in owner handoff / `git log` output.

## Preflight evidence

```text
git log --oneline -1 master
9d7ff3e6 feat(G-6b-ui): add admin content takedown actions
```

```text
git tag --list g6b-ui-content-takedown-baseline g6a2-takeout-review-baseline
g6a2-takeout-review-baseline
g6b-ui-content-takedown-baseline
```

```text
rg "db\.AutoMigrate\(" backend/cmd/server
# no matches
```

```text
git status --short
# clean before worktree branch changes
```

## File-level verification evidence

### Auth

`backend/internal/module/user/middleware.go`:

- `AuthRequired()` authenticates Bearer JWT and sets `domainA.user`.
- `CurrentUser(c)` reads `domainA.user`.
- `CurrentUserID(c)` returns current authenticated user ID.
- `AdminRequired()` is separate and checks admin/superadmin role.

G-7B uses `AuthRequired()` only.

### Takeout review target

`backend/internal/module/admin/takeoutconsole/reviewtarget/target.go`:

```text
MerchantObjectType = "takeout_merchant"
DishObjectType     = "takeout_dish"
```

Merchant review approved calls existing `MerchantService.Approve`.

### Media reference API

`backend/internal/module/media/service.go`:

```text
CreateReferenceRecord(ctx, media.AssetReference)
TransitionReference(ctx, id, version, from, to)
```

`backend/internal/module/media/model.go` confirms:

```text
PurposeDish         = "dish"
PurposeMerchantLogo = "merchant_logo"
```

## Real HTTP/MySQL evidence

Executed with real MySQL DSN loaded from the local untracked `backend/.env` in the main project checkout. The DSN and credentials were not printed, not written to reports, and not committed.

Command:

```text
go test ./internal/module/admin/takeoutmerchant -run 'TestTakeoutMerchantAPIRealHTTPMySQLAuthOnboardingOwnerScopeAndMediaBinding|TestZZTakeoutMerchantAPIResidueAgainstMySQL84' -count=1 -v
```

Relevant result:

```text
=== RUN   TestTakeoutMerchantAPIRealHTTPMySQLAuthOnboardingOwnerScopeAndMediaBinding
real_http=PASS auth=401 owner_scope=PASS onboarding_review=PASS duplicate=409 media_reference=PASS cross_merchant=PASS order_flow=PASS stale_CAS=409
--- PASS: TestTakeoutMerchantAPIRealHTTPMySQLAuthOnboardingOwnerScopeAndMediaBinding

=== RUN   TestZZTakeoutMerchantAPIResidueAgainstMySQL84
residue=0 users=0 merchants=0 categories=0 dishes=0 orders=0 review_tasks=0 media_assets=0 media_references=0
--- PASS: TestZZTakeoutMerchantAPIResidueAgainstMySQL84

PASS
```

Notes:

- GORM `record not found` diagnostic lines appeared while creating fresh test fixtures and while asserting cross-merchant denial paths. They are expected in this test and not acceptance failures.
- Test data cleanup ran at the start and end of the test.

## Boundary evidence

```text
npm run check:boundaries
ok A-G boundaries plus isolated P0-4 media domain
```

This confirms:

- takeout core still has zero Gin/HTTP import.
- transport code lives outside takeout core, under admin transport adapter space.

## Full regression evidence

```text
npm run verify
PASS
```

Included:

- `npm run check:boundaries`
- admin typecheck
- miniapp typecheck
- backend `go test ./...`

## Build evidence

```text
npm run build
PASS
```

Included:

- `npm run build:miniapp`
- `npm run build:admin`

Observed warning:

- miniapp `vendors.js (731 KiB)` exceeds recommended bundle size.

This warning existed before and is not introduced by G-7B backend work.

## Residue report

Tracked test residue:

| Area | Result |
| --- | --- |
| users | 0 |
| merchants | 0 |
| categories | 0 |
| dishes | 0 |
| orders | 0 |
| review tasks | 0 |
| media assets | 0 |
| media references | 0 |
| dev/server process | none left running |

`node_modules` was installed in the worktree to run frontend typecheck/build. It is ignored and not part of the commit.

