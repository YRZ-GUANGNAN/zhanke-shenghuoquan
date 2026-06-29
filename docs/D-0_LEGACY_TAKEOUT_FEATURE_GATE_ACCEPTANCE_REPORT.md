# D-0 Legacy Takeout Feature Gate Acceptance Report

## 1. Verdict

**PASS**

Legacy platform takeout is archived by a fail-closed gate without deleting or modifying its business implementation. Explicitly enabling the gate restores runtime construction, F targets, routes and payment callback.

Implementation commit: `484975e`.

## 2. Gate acceptance matrix

| Check | Disabled/default | Enabled | Result |
|---|---:|---:|---|
| Legacy runtime constructed | No | Yes | PASS |
| Merchant/dish F targets registered | No | Yes | PASS |
| Admin takeout routes mounted | No | Yes | PASS |
| Merchant routes mounted | No | Yes | PASS |
| Buyer/payment routes mounted | No | Yes | PASS |
| Rider routes mounted | No | Yes | PASS |
| WeChat Pay callback mounted | No | Yes | PASS |
| Production requires legacy pay credentials | No | Yes | PASS |
| MiniApp takeout entries visible | No | Yes | PASS |
| Admin takeout navigation visible | No | Yes | PASS |
| Legacy pages/source/migrations retained | Yes | Yes | PASS |

## 3. Real backend HTTP evidence

The complete backend was started against the existing real MySQL configuration and an isolated temporary Meilisearch process.

### `ENABLE_LEGACY_TAKEOUT=false`

```text
GET  /healthz                                      200
GET  /api/v1/admin/takeout/merchants              404
GET  /api/v1/takeout/merchant/me                  404
GET  /api/v1/takeout/buyer/merchants              404
GET  /api/v1/takeout/rider/me                     404
POST /api/v1/takeout/payments/wechat/notify       404
GET  /api/v1/admin/reviews                        401
```

The shared F route returning 401 proves the rest of the server and F transport remained mounted while legacy takeout routes were absent.

### `ENABLE_LEGACY_TAKEOUT=true`

```text
GET  /healthz                                      200
GET  /api/v1/admin/takeout/merchants              401
GET  /api/v1/takeout/merchant/me                  401
GET  /api/v1/takeout/buyer/merchants              401
GET  /api/v1/takeout/rider/me                     401
POST /api/v1/takeout/payments/wechat/notify       400
GET  /api/v1/admin/reviews                        401
```

401/400 rather than 404 proves all representative legacy handlers and the public payment callback were restored.

## 4. Production payment gate evidence

A production-mode process was started with:

- `APP_ENV=prod`
- `ENABLE_LEGACY_TAKEOUT=false`
- all legacy WeChat Pay credentials empty
- all mock flags false
- the other mandatory production configuration supplied

Result:

```text
PROD_DISABLED_EMPTY_PAY_BOOT=200
```

The same executable was then started with `ENABLE_LEGACY_TAKEOUT=true` and the same empty pay credentials:

```text
PROD_ENABLED_EMPTY_PAY_REJECTED=True
error: production requires complete WeChat Pay credentials
```

This closes the critical production-startup gate while retaining fail-closed payment safety when legacy takeout is enabled.

## 5. Real MySQL 8.4 evidence

Shared database version:

```text
8.4.0
```

Core real-MySQL functional suites passed:

- merchant lifecycle and competing CAS;
- catalog lifecycle and competing CAS;
- order snapshots, fee calculation, settlement and competing CAS;
- rider lifecycle, claim limits, concurrent claim and rollback;
- withdrawal payment, CAS and settlement.

Real HTTP + MySQL suites passed:

- buyer API;
- admin console and UI contract;
- F review targets;
- merchant API and media binding;
- mock payment and downstream flow;
- rider concurrency, owner isolation and wallet.

The migration rollback tests were not run destructively against the shared non-empty business database. They correctly fail-closed when any real merchant/dish/order/fulfillment exists. D-0 contains no migration. Existing business rows were preserved, while the functional suites above used their scoped cleanup paths.

## 6. Frontend build-gate evidence

### False/default build

```text
MiniApp compiled flag: false
MiniApp app.config retains legacy takeout pages: true
Admin bundle contains 外卖运营 navigation: false
Admin bundle contains takeout API client: false
```

### True build

```text
MiniApp compiled flag: true
MiniApp bottom navigation contains /pages/takeout/index: true
Admin bundle contains 外卖运营 navigation: true
Admin bundle contains /api/v1/admin/takeout: true
```

Both build matrices completed successfully. The only MiniApp warning was the pre-existing `vendors.js` size recommendation.

## 7. Automated regression

```text
go test ./... -count=1          PASS
npm run check:boundaries        PASS
npm run typecheck               PASS
npm run verify                  PASS
npm run build                   PASS
MiniApp false build             PASS
MiniApp true build              PASS
Admin false build               PASS
Admin true build                PASS
```

Boundary output:

```text
ok A-G boundaries plus isolated P0-4 media domain
```

`db.AutoMigrate(` remains absent from `backend/cmd/server`.

## 8. Preservation evidence

- Legacy takeout core and all transport packages: zero diff.
- Migrations 016-022: zero diff.
- Existing real rows after testing:

```text
takeout_merchants       1
takeout_dishes          1
takeout_orders          1
takeout_fulfillments    1
```

Those counts match the preserved shared-data baseline observed before safe functional reruns.

