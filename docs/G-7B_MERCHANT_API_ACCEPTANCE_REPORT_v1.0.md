# G-7B Merchant API Acceptance Report v1.0

Date: 2026-06-23  
Branch: `feat/g7b-merchant-api`  
Base: `9d7ff3e6 feat(G-6b-ui): add admin content takedown actions`  
Final commit: recorded in owner handoff / `git log` output.

## Acceptance conclusion

Status: PASS

G-7B backend merchant-role transport layer is implemented and verified against:

- real Gin routing
- real A-domain JWT middleware
- real MySQL 8.4 database
- real domain services and repositories

No mock backend route or synthetic handler bypass was used for acceptance.

## API matrix

| Area | Endpoint | Result |
| --- | --- | --- |
| Identity | `GET /api/v1/takeout/merchant/me` | PASS |
| Onboarding | `POST /api/v1/takeout/merchant/onboarding` | PASS |
| Categories | `GET /categories`, `POST /categories`, `GET /categories/:id`, `PATCH /categories/:id` | PASS |
| Dishes | `GET /dishes`, `POST /dishes`, `GET /dishes/:id`, `PATCH /dishes/:id` | PASS |
| Dish state | `PATCH /dishes/:id/on-shelf`, `/sold-out`, `/restock`, `/off-shelf`, `/remove` | PASS |
| Orders | `GET /orders`, `GET /orders/:id` | PASS |
| Order state | `PATCH /orders/:id/accept`, `/start-preparing`, `/mark-ready` | PASS |

All endpoints are under:

```text
/api/v1/takeout/merchant
```

## Auth and owner-scope verification

| Check | Result |
| --- | --- |
| Missing token returns 401 | PASS |
| Authenticated non-merchant calling merchant endpoint returns explicit onboarding-required rejection | PASS |
| Merchant identity is derived from `takeout_merchants.owner_user_id` | PASS |
| `AdminRequired()` is not used | PASS |
| Merchant A cannot mutate Merchant B dish | PASS |
| Merchant A cannot mutate Merchant B order | PASS |
| Cross-merchant attempts do not modify target data | PASS |

## Onboarding and F review verification

| Check | Result |
| --- | --- |
| Onboarding creates `takeout_merchants` row in `pending_review` | PASS |
| Onboarding submits F review task | PASS |
| Review object type is `takeout_merchant` | PASS |
| Duplicate onboarding for active/pending merchant returns 409 | PASS |
| F review `Start` + `Decide(approved)` activates merchant through existing target adapter | PASS |

## Dish/category verification

| Check | Result |
| --- | --- |
| Active merchant creates own category | PASS |
| Active merchant creates own dish | PASS |
| Dish image asset public ID is stored | PASS |
| Merchant A cannot operate Merchant B dish | PASS |
| Stale/invalid CAS maps to conflict | PASS |
| Dish state transitions through service layer | PASS |

## Order verification

| Check | Result |
| --- | --- |
| Merchant order list is filtered by current merchant | PASS |
| Merchant order detail includes items | PASS |
| Merchant A can accept own paid order | PASS |
| Merchant A can start preparing own accepted order | PASS |
| Merchant A can mark own preparing order ready | PASS |
| Merchant B cannot operate Merchant A order | PASS |
| Stale order row version returns 409 | PASS |

## Media binding verification

| Check | Result |
| --- | --- |
| Merchant logo asset must be current user's `merchant_logo` asset | PASS |
| Dish image asset must be current user's `dish` asset | PASS |
| Foreign user's media asset is rejected with 403 | PASS |
| Rejected foreign media request does not create dish residue | PASS |
| Media reference is created for merchant logo | PASS |
| Media reference is created for dish image | PASS |

## Verification commands

Real HTTP/MySQL acceptance:

```text
go test ./internal/module/admin/takeoutmerchant -run 'TestTakeoutMerchantAPIRealHTTPMySQLAuthOnboardingOwnerScopeAndMediaBinding|TestZZTakeoutMerchantAPIResidueAgainstMySQL84' -count=1 -v
PASS
```

Full verification:

```text
npm run verify
PASS
```

Build:

```text
npm run build
PASS
```

Build note: miniapp build still emits the existing `vendors.js` size warning. This is unchanged and non-blocking for this backend task.

## Residue

Test cleanup verified:

- users: 0
- merchants: 0
- categories: 0
- dishes: 0
- orders: 0
- review tasks: 0
- media assets: 0
- media references: 0

No dev/server process was left running.

## Remaining risks

Low:

- Media reference is created as `pending` and `hidden`; frontend-stage binding and visibility progression are still future work.
- Onboarding plus F review submission is not a single cross-domain DB transaction, consistent with existing F review architecture. The route validates media before creating merchant records to reduce fail-open risk.

No high or medium risks block G-7B backend acceptance.

