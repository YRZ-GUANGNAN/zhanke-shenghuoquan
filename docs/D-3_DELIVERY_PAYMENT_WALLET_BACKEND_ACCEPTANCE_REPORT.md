# D-3 Delivery Payment + Merchant Wallet Backend Acceptance Report

Date: 2026-06-30

Implementation commit: `e15bfa4aefe324611fb10280707fef55b62b6256`

Result: **PASS for the authorized backend scaffold**

## 1. Acceptance matrix

| Gate | Result | Evidence |
|---|---|---|
| MySQL version | PASS | Real server reported `8.4.0` |
| Migration 025 apply/rollback/apply | PASS | Six tables removed and recreated successfully |
| Schema constraints/indexes | PASS | Settlement/order, ledger idempotency, payment request, notification and refund uniqueness checked |
| Cross-domain foreign keys | PASS | 0 across all six D-3 tables |
| Mock recharge | PASS | Wallet +500 and exactly one credit ledger; repeat request did not credit twice |
| Wallet order payment | PASS | -150 balance, debit ledger, settlement, ready state and outbox committed together |
| Insufficient balance | PASS | Error returned; order remained `pending_fee_payment`; no settlement |
| Merchant owner fence | PASS | Cross-merchant pay returned not-found/forbidden semantics |
| Single order payment | PASS | Concurrent wallet and mock paths produced one successful settlement only |
| Amount authority | PASS | 150 cents read from the delivery-order snapshot; callback 151 was rejected |
| WeChat callback idempotency | PASS | Repeated signed-domain confirmation produced one notification/settlement |
| Merchant refund | PASS | Only unclaimed `ready_for_claim` order accepted |
| Admin refund | PASS | Claimed order rejected merchant self-refund and accepted admin refund |
| Original-source refund | PASS | Wallet credited wallet ledger; mock/WeChat used their own source paths |
| Refund retry/idempotency | PASS | Failed cloud result remained pending; success callback converged once |
| Mock production isolation | PASS | Production/mock, mock/real, partial-real and dual-mock combinations rejected |
| Payment feature gate | PASS | Disabled funds/callback routes returned 404; enabled routes mounted |
| Authentication | PASS | Wallet route returned 401 without token |
| D-2 compatibility | PASS | Existing transition/outbox/CAS tests remained green |
| Boundary check | PASS | Independent Delivery and isolated P0-4 media boundary check green |
| AutoMigrate | PASS | 0 startup `db.AutoMigrate(` calls |
| Residue | PASS | Scoped D-3 database rows and temporary dependency links = 0 |

## 2. Real MySQL evidence

Executed against the configured real MySQL 8.4 database:

```text
real_mysql_version=8.4.0
migration_025_apply_rollback_apply=PASS tables=6 foreign_keys=0 money_checks=PASS
wallet_funds=PASS recharge+atomic_debit+insufficient_rollback+owner_fence+idempotent_refund
single_settlement=PASS wallet_vs_mock; wechat_callback=PASS amount+dedup+ready_outbox
refund_rules=PASS merchant_ready/admin_claimed/original_source/retry/idempotent_callback
D-3 HTTP=PASS auth+owner_fence+mock_recharge+wallet_pay+gate_404+callback_gate
D-3 residue tables=0 orders=0 scoped_outbox=0
```

The test identities use isolated merchant IDs/openids and were deleted after
the suite. Existing database rows were not modified.

## 3. Regression evidence

Commands and results:

```text
go test ./... -count=1                 PASS (with real MySQL D-1/D-2/D-3 tests enabled)
npm run check:boundaries               PASS
npm run verify                         PASS
npm run build                          PASS
admin TypeScript/build                 PASS
miniapp TypeScript/build               PASS
```

The miniapp build emitted only the pre-existing `vendors.js` size warning.

## 4. Security evidence

- Authentication protects merchant wallet/payment/refund routes.
- Merchant ID is derived from the JWT context, never accepted from request JSON.
- Admin refund uses the existing `AdminRequired` middleware.
- WeChat callbacks are the only unauthenticated routes; they require timestamp,
  RSA signature, and APIv3 AES-GCM verification before funds code runs.
- Callback body size is capped.
- Payment amount, merchant, order purpose, out-trade number and transaction
  identity are checked against server-side facts.
- No secret, private key, raw callback body, wallet internal ID, or COS/IM value
  is exposed in DTOs or reports.

## 5. Remaining risks

1. **External dependency, not a backend-scaffold failure:** real WeChat JSAPI
   prepay, payment callback, refund API and refund callback cannot be cloud-tested
   until the merchant account, certificates and callback domains are available.
   Configuration remains fail-closed.
2. Admin refund after `picked_up`/`delivering` refunds the service fee but keeps
   the frozen D-1 fulfillment state because those states cannot transition to
   `cancelled`. Operations must resolve the still-active delivery separately.
3. A pending WeChat refund is retried by repeating the idempotent admin/merchant
   operation; no new scheduled refund-retry worker was authorized in D-3.
4. D-3 frontend wallet/payment/refund screens are explicitly out of scope.

## 6. Conclusion

D-3 backend satisfies the authorized funds invariants: one order is paid once,
wallet mutations have matching immutable ledger facts, insufficient funds fail
closed, payment success and rider-pool admission are atomic, refunds are
original-source and idempotent, and mock payment cannot activate in production.

The branch must remain unmerged until owner review. D-4 and D-3 frontend work
remain unauthorized.
