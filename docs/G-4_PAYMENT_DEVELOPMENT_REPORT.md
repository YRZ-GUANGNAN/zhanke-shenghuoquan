# G-④ Payment Scaffold + Mock Payment Development Report

Status: Development Complete  
Branch: `feat/g4-payment`  
Base: `e80fe1bb fix(dev): improve local mock login and media visibility`  
Final commit: recorded in handoff response after commit creation

## 1. Preflight

- `git log --oneline -1 master`: `e80fe1bb fix(dev): improve local mock login and media visibility`
- Required tags present: `dev-local-experience-baseline`, `g7a-buyer-api-baseline`, `g7c-rider-api-baseline`
- `rg "db\.AutoMigrate\(" backend/cmd/server`: 0 matches
- Existing max migration before this module: `021_takeout_withdraw.sql`
- Worktree was clean before implementation.

## 2. File-level reconfirmation

- `MarkPaid(ctx, orderPublicID, expectedRowVersion, payNo)` exists in `backend/internal/module/takeout/order_service.go`.
- `MarkPaid` uses `pending_payment -> paid`, service-side `payNo` validation, and repository CAS through `status + row_version`.
- `CreateOrder` recalculates item amount, delivery fee and total amount on the server.
- Existing buyer HTTP deliberately had no `/orders/:id/pay` route before G-④.
- Downstream merchant/rider chain already works once an order reaches `paid`.
- Existing mock login/media safety pattern uses a local-only gate; G-④ implemented an independent `ALLOW_MOCK_PAYMENT` gate.

## 3. Implemented scope

### Migration 022

Added:

- `backend/migrations/022_takeout_payment.sql`
- `backend/migrations/rollback/022_takeout_payment.sql`

New G-owned tables:

- `takeout_payment_attempts`
- `takeout_payment_notifications`

No cross-domain foreign keys were added.

### Takeout core domain

Added:

- `payment_model.go`
- `payment_repository.go`
- `payment_service.go`
- `payment_mysql_test.go`
- `payment_service_test.go`

Capabilities:

- Payment attempt model and status validation.
- Notification model and idempotency hash.
- Payment service for:
  - WeChat prepay attempt creation.
  - mock payment.
  - provider notification confirmation.
- Repository methods stay inside the existing `repository.go` GORM boundary.

### Payment transport

Added package:

- `backend/internal/module/admin/takeoutpayment`

Capabilities:

- `POST /api/v1/takeout/buyer/orders/:orderPublicId/pay`
- `POST /api/v1/takeout/payments/wechat/notify`
- mock provider
- unconfigured provider
- WeChat JSAPI provider skeleton
- RSA request signing
- JSAPI client parameter signing
- callback signature/decrypt skeleton

### Config

Added:

- `ALLOW_MOCK_PAYMENT`
- `WECHAT_PAY_MCH_ID`
- `WECHAT_PAY_API_V3_KEY`
- `WECHAT_PAY_MERCHANT_SERIAL_NO`
- `WECHAT_PAY_PRIVATE_KEY_PATH`
- `WECHAT_PAY_PLATFORM_PUBLIC_KEY_PATH`
- `WECHAT_PAY_NOTIFY_URL`
- `WECHAT_PAY_API_BASE`

Safety rules:

- `prod + ALLOW_MOCK_PAYMENT=true` is rejected.
- partial WeChat Pay credentials are rejected.
- mock payment plus any real WeChat Pay credential is rejected.

### MiniApp

Added:

- `payBuyerOrder()`
- typed payment response and JSAPI parameters
- `Taro.requestPayment` facade type
- pending-payment "去支付" button on buyer order detail page

The frontend refreshes order state after payment and never marks an order paid locally.

## 4. Explicitly not implemented

- No refund provider integration.
- No real WeChat Pay cloud/merchant-account live joint test, because merchant credentials are not available.
- No changes to existing order state machine.
- No changes to Community, Trade, Media, Admin Console unrelated behavior, P0-5, or 68 drift items.

