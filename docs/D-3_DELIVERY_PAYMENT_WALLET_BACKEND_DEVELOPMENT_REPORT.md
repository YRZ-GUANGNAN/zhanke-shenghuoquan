# D-3 Delivery Payment + Merchant Wallet Backend Development Report

Date: 2026-06-30

Branch: `feat/d3-delivery-payment`

Base: `master@a6d02005`

Implementation commit: `e15bfa4aefe324611fb10280707fef55b62b6256`

## 1. Scope

D-3 implements the backend funds loop for the independent Delivery domain:

- merchant wallet balance and append-only ledger facts;
- wallet recharge and wallet balance payment;
- per-order WeChat JSAPI payment scaffold and local mock payment;
- payment notification verification/decryption and idempotent processing;
- one-settlement-per-delivery-order enforcement;
- merchant and administrator refund rules;
- WeChat refund scaffold and refund callback;
- fail-closed configuration and runtime route gates.

No frontend, D-4 capability, legacy takeout payment reuse, or modification to
community/trade/media/IM was introduced.

## 2. Preflight findings

- Baseline and tag gates passed; migration 024 was the previous maximum.
- `db.AutoMigrate(` remained absent from `backend/cmd/server`.
- D-2 already provided `TransitionToReadyAndEnqueue`, which atomically changes
  `pending_fee_payment -> ready_for_claim` and creates the rider-pool outbox event.
- D-3 extracted only an unexported transaction primitive
  `transitionToReadyAndEnqueueTx`; the D-2 public method and behavior remain unchanged.
- The old takeout payment implementation was used only as a structural reference.
  D-3 neither imports it nor reads/writes its tables.

## 3. Database changes

Migration `025_delivery_payment_wallet.sql` creates six Delivery-owned tables:

1. `delivery_merchant_wallets`
2. `delivery_wallet_ledger`
3. `delivery_payment_attempts`
4. `delivery_payment_notifications`
5. `delivery_fee_settlements`
6. `delivery_refunds`

Key invariants:

- integer cents only;
- no foreign keys to other domains;
- unique wallet per merchant;
- unique ledger idempotency key;
- unique payment request `(merchant_user_id, purpose, client_request_id)`;
- unique payment notification transaction/body digest;
- unique settlement per delivery order;
- unique refund per settlement;
- CHECK constraints freeze purpose/provider/status/reference and ledger balance math.

Migration SHA-256:

- apply: `76D573D900111C81012D0948A2D65267D4BD728B9A980F83173205407BFC1DC6`
- rollback: `E25675E251BD77B5A691B4945117A297063A517B1D7E53391A40B61B4DC08B3D`

## 4. Funds transaction design

Wallet payment uses one MySQL transaction and a fixed lock order:

`delivery order -> wallet -> ledger -> settlement -> order CAS -> outbox`

The amount is always read from `delivery_orders.delivery_fee_cents`; clients
cannot submit or override it. Insufficient balance rolls back every write.
The settlement unique key prevents wallet, WeChat, or mock sources from paying
the same delivery order twice.

Recharge writes the paid attempt, wallet CAS increase, and credit ledger in one
transaction. WeChat payment callbacks write the deduplicated notification,
paid attempt, settlement, ready state, and outbox in one transaction.

## 5. Refund rules

- `ready_for_claim` without a rider: merchant may request a full refund.
- `claimed`, `to_pickup`, `picked_up`, or `delivering`: merchant self-refund is
  rejected; an administrator endpoint is required.
- `pending_fee_payment` and `payment_failed`: no paid settlement exists to refund.
- `delivered` and `cancelled`: new refunds are rejected.
- wallet source returns funds to the same wallet and writes a credit ledger;
- mock source creates real refund/settlement facts but performs no cloud call;
- WeChat source moves to `refund_pending`, invokes the independent WeChat refund
  adapter, and settles through the verified callback.

An existing refund is returned before stale command checks, so retries remain
idempotent and never create a second credit.

## 6. HTTP contract

Authenticated merchant routes:

- `GET /api/v1/delivery/merchant/wallet`
- `GET /api/v1/delivery/merchant/wallet/ledger`
- `POST /api/v1/delivery/merchant/wallet/recharges`
- `GET /api/v1/delivery/merchant/payments/:id`
- `POST /api/v1/delivery/merchant/orders/:id/pay-with-wallet`
- `POST /api/v1/delivery/merchant/orders/:id/pay-with-wechat`
- `POST /api/v1/delivery/merchant/orders/:id/refunds`

Administrator route:

- `POST /api/v1/admin/delivery/orders/:id/refunds`

Unauthenticated, signature-verified WeChat callback routes:

- `POST /api/v1/delivery/payments/wechat/notify`
- `POST /api/v1/delivery/payments/wechat/refund-notify`

DTOs expose camel-case public identifiers and state only; raw database IDs,
credentials, platform keys, and payment secrets are not returned.

## 7. Configuration and isolation

New settings:

- `ENABLE_DELIVERY_PAYMENT=false`
- `ALLOW_MOCK_DELIVERY_PAYMENT=false`
- `DELIVERY_WECHAT_PAY_NOTIFY_URL=`
- `DELIVERY_WECHAT_REFUND_NOTIFY_URL=`

WeChat merchant credentials reuse the platform's existing `WECHAT_PAY_*`
configuration because the receiving legal entity is the same, while Delivery
uses independent callback URLs and tables.

Validation rejects:

- mock Delivery payment in production;
- mock Delivery payment combined with any real payment credential;
- simultaneous D-2 fee-confirmation mock and D-3 payment mock;
- an enabled, partially configured real Delivery payment setup;
- incomplete shared WeChat Pay core credentials.

When the D-3 gate is closed, funds services, providers, merchant/admin payment
routes, and unauthenticated callbacks are not constructed or mounted.

## 8. Boundary result

- Delivery core remains free of Gin, HTTP, SDK, takeout, and A-F imports.
- `admin/deliverypayment` is the only external-payment adapter.
- No legacy takeout payment package/table is imported or accessed.
- AutoMigrate remains retired.
- No migration before 025 was modified.
