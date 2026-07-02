# D-6a Delivery Fee Config + Rider Wallet Backend Development Report

Status: Development Complete  
Branch: `feat/d6a-delivery-fee-wallet`  
Final commit: see delivery handoff `git log` output.

## Scope

D-6a implements the backend funds core for the Delivery domain only:

- fixed-amount delivery fee configuration;
- global config plus merchant override by `merchant_user_id` logical reference;
- Delivery-owned rider wallet, rider earning records, rider ledger, and offline payout records;
- delivered-time rider earning settlement for both normal rider fulfillment and D-4 proof-photo delivery completion;
- repository/domain logic for offline payout registration;
- migration `028_delivery_fee_and_rider_wallet.sql` plus rollback.

No admin API or admin frontend was implemented. No legacy takeout rider wallet was reused.

## Preflight verification

- `git log --oneline -1 master` -> `c61ec0a8 feat(D-5): add delivery IM miniapp conversation UI`
- `git tag --list d5-delivery-im-frontend-baseline` -> present
- `rg "db\.AutoMigrate\(" backend/cmd/server` -> zero matches
- Current max Delivery migration before this module: `027_delivery_im.sql`
- Worktree was clean before development.

File-level findings:

- Existing delivery fee was hardcoded in `backend/cmd/server/main.go` as `FeePolicy{DeliveryFeeCents:150, RiderFeeCents:100, PlatformCutCents:50}`.
- `delivery_orders` already had immutable snapshot fields: `delivery_fee_cents`, `rider_fee_cents`, `platform_cut_cents`.
- Normal delivery completion path: `DispatchService.MarkDelivered -> Service.MarkDelivered -> Repository.Transition`.
- D-4 proof-photo completion path: `ProofService.AdvanceWithProof -> ProofRepository.CreateProofAndTransition`.
- D-3 merchant wallet tables are separate from rider wallet and were used only as implementation pattern.
- Delivery had no merchant profile table; merchant override uses `merchant_user_id` without cross-domain FK.
- Legacy takeout rider wallet tables exist but remain sealed/reference-only.

## Implementation summary

### Migration 028

Added:

- `delivery_fee_configs`
- `delivery_merchant_fee_overrides`
- `delivery_rider_wallets`
- `delivery_rider_earning_records`
- `delivery_rider_payout_records`
- `delivery_rider_ledger`

All references are logical references. No cross-domain foreign keys were added.

Rollback includes a fail-closed guard: if rider wallet, earning, payout, or ledger facts exist, rollback fails instead of dropping money facts.

### Fee policy

Added DB-backed fee policy lookup:

- merchant active override first;
- otherwise active global config;
- rider fee is computed as `delivery_fee_cents - platform_cut_cents`;
- order creation snapshots the computed values into `delivery_orders`.

Production `main.go` no longer hardcodes `150/100/50`; it wires `GormRepository` as the DB fee provider.

### Rider settlement

Added Delivery-owned rider settlement model and repository logic:

- delivered order writes one `delivery_rider_earning_records` row;
- `delivery_order_public_id` is unique and is the idempotency key;
- rider wallet is credited with the order snapshot `rider_fee_cents`;
- rider ledger receives append-only `income` entry;
- repeated delivered/idempotent retry does not double credit.

The settlement helper is invoked inside the same transaction for both:

- normal `Repository.Transition(..., StatusDelivered)`;
- proof-photo `ProofRepository.CreateProofAndTransition(..., StatusDelivered)`.

### Offline payout registration

Added repository logic for marking an earning as `paid_offline`:

- creates one payout record;
- appends rider ledger `payout`;
- debits rider wallet;
- idempotent for an already paid earning;
- does not perform real money transfer.

Admin HTTP trigger is intentionally deferred to D-6b.

## Boundary statement

Touched production code is limited to:

- `backend/cmd/server/main.go`
- `backend/internal/module/delivery/*`
- migration `028` and rollback

No legacy takeout, community, trade, media, IM, admin UI, miniapp UI, or migration `001-027` contract was changed.
