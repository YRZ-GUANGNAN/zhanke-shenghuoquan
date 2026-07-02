# D-6c Delivery Admin Console Frontend Acceptance Report

## Acceptance Summary

D-6c frontend implementation is compile-verified and boundary-verified. It adds a dedicated DeliveryConsole that connects to the frozen D-6b admin API contract without changing backend code.

Status: implementation complete; live browser E2E pending backend availability.

## Evidence

### Build and Typecheck

- `npm --workspace apps/admin run typecheck`: PASS
- `npm run build:admin`: PASS
- `npm run build`: PASS

The full build emitted only the existing miniapp vendor size / no async chunks warnings. No D-6c-specific build errors were observed.

### Boundary and Regression

- `npm run check:boundaries`: PASS
- `npm run verify`: PASS

### Backend Zero-change Evidence

No backend, migration, package, miniapp, or legacy takeout files were modified. The intended change area is limited to `apps/admin/src/**` plus D-6c reports under `docs/`.

### API Contract Coverage

The new `deliveryAdminAPI` client covers:

- `GET /api/v1/admin/delivery/summary`
- `GET /api/v1/admin/delivery/orders`
- `GET /api/v1/admin/delivery/orders/:id`
- `GET /api/v1/admin/delivery/orders/:id/funds`
- `GET /api/v1/admin/delivery/fee-config`
- `PUT /api/v1/admin/delivery/fee-config`
- Merchant fee override list/detail/upsert/disable
- Merchant wallet list/detail/ledger
- Rider wallet list/detail/ledger
- Rider earnings list/detail/pay-offline
- Rider payouts
- Refunds list/detail

### CAS and Idempotency

- Global fee config writes send `expectedRowVersion`.
- Merchant override writes optionally send `expectedRowVersion`; disable sends the selected row version.
- 409 status is preserved through `DeliveryAPIError.status` and displayed as a refresh-required conflict.
- Offline payout registration sends `expectedRowVersion` and `idempotencyKey`.

### Offline Payout Safety

The payout action requires a note and a confirmation prompt that states the operation only records offline payment and does not perform a real transfer. The frontend disables action buttons while the request is running and reloads backend state after completion.

### Funds State Integrity

D-6c does not locally mark orders, refunds, wallets, earnings, or payouts as completed. All displayed states come from D-6b API responses.

### Permission Behavior

The frontend keeps backend authorization as the source of truth. 401/403 responses are surfaced as API errors; D-6c does not fake admin permission client-side.

## Live E2E Status

During this turn, the local backend was not available at `127.0.0.1:8080`, so browser-level verification against live D-6b data was not completed. This is not replaced with mock data. The owner can validate visually by starting the D-6b backend with an admin token, opening apps/admin, selecting “配送运营”, and exercising the eight tabs.

