# D-1 Delivery Domain Skeleton Acceptance Report

## 1. Verdict

**PASS**

Implementation commit:

```text
437926ba feat(D-1): add delivery domain skeleton with migration 023
```

D-1 satisfies the authorized pure-domain scope. It is not registered in
`main.go` and no D-2 capability was introduced.

## 2. Real MySQL 8.4 migration evidence

Database version:

```text
8.4.0
```

Migration sequence:

```text
023 apply -> rollback -> apply: PASS
tables: 1
delivery_orders columns: 26
foreign keys: 0
```

Verified indexes:

```text
uq_delivery_order_public_id
idx_delivery_order_merchant_created
idx_delivery_order_status_created
idx_delivery_order_rider_status
```

Verified CHECK constraints:

```text
chk_delivery_order_status
chk_delivery_order_fee_snapshot
```

The database rejected both an invalid status and a mismatched fee snapshot with
MySQL error 3819.

## 3. State-machine acceptance

All 15 legal edges were executed against real MySQL using Repository CAS:

| From | To | Result |
|---|---|---|
| pending_fee_payment | ready_for_claim | PASS |
| pending_fee_payment | payment_failed | PASS |
| pending_fee_payment | cancelled | PASS |
| payment_failed | pending_fee_payment | PASS |
| payment_failed | cancelled | PASS |
| ready_for_claim | claimed | PASS |
| ready_for_claim | cancelled | PASS |
| claimed | to_pickup | PASS |
| claimed | ready_for_claim | PASS |
| claimed | cancelled | PASS |
| to_pickup | picked_up | PASS |
| to_pickup | ready_for_claim | PASS |
| to_pickup | cancelled | PASS |
| picked_up | delivering | PASS |
| delivering | delivered | PASS |

Additional state checks:

- illegal edges fail closed;
- `payment_failed` retries to `pending_fee_payment`;
- cancellation after pickup is rejected;
- `delivered` and `cancelled` cannot exit;
- every relevant stage timestamp is persisted.

## 4. CAS and idempotency acceptance

| Scenario | Result |
|---|---|
| stale row version | `ErrConcurrentUpdate`, PASS |
| simultaneous transitions from one version | one success, one conflict, PASS |
| repeated fee-paid transition | same row/version returned, PASS |
| repeated claim by same rider | idempotent, PASS |
| repeated claim with different rider | `ErrRiderMismatch`, PASS |
| repeated delivered transition | idempotent, PASS |
| repeated cancelled transition | idempotent, PASS |
| requeue clears rider assignment | PASS |

Real concurrency evidence:

```text
delivery_cas_competition=PASS winners=1 conflicts=1
```

## 5. Money acceptance

- All money fields are `BIGINT UNSIGNED`.
- Go fields use `uint64`.
- No float, double or decimal type is used.
- Creation requires:

```text
delivery_fee_cents = rider_fee_cents + platform_cut_cents
```

- The three values are persisted as immutable order snapshots during D-1 state transitions.

## 6. Boundary acceptance

```text
npm run check:boundaries
ok A-G boundaries plus independent delivery and isolated P0-4 media domains
```

Verified:

- no HTTP/Gin dependency;
- no takeout import;
- no other business-domain import;
- no cross-domain FK;
- GORM access only in Repository;
- `db.AutoMigrate(` remains at zero under `backend/cmd/server`;
- no `main.go` change;
- legacy takeout gate behavior is unchanged.

## 7. Full regression

Final clean run:

```text
go test ./... -count=1   PASS
npm run verify           PASS
npm run build            PASS
```

The MiniApp build retained the existing `vendors.js` size warning. It did not
fail compilation and is unrelated to D-1.

## 8. Corrective test finding

An intermediate full run found 15 legal-transition fixtures whose test merchant
ID was outside the initial scoped cleanup list. Migration rollback correctly
refused to drop the non-empty table.

The cleanup scope was corrected to the two explicit D-1-only merchant IDs, the
15 rows were removed, and the full real-MySQL and repository regression was
rerun successfully. Final residue is zero.

## 9. Final scope decision

D-1 is ready for owner review and baseline freeze.

This PASS does not authorize:

- runtime wiring;
- HTTP transport;
- rider claim integration;
- payment;
- media or IM;
- D-2 development.
