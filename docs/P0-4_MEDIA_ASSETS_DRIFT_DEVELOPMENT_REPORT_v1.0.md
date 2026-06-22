# P0-4 Media Assets Drift Development Report v1.0

## Scope

This change is limited to backend schema drift reconciliation on branch
`fix/p0-4-media-drift`, based on `c9528b2f`. It does not run or modify the
MiniApp E2E flow, media business services, state machines, repositories, COS,
STS, review, consumer, GC, takeout domain G, or P0-5.

## Implementation

### Migration 015

`backend/migrations/015_media_assets_drift_reconcile.sql`:

- restores all 13 `media_assets` definitions to the fresh 001-014 baseline;
- normalizes nullable legacy reference fields before applying NOT NULL;
- fails closed if duplicate non-null `review_task_id` values would block the
  unique constraint;
- adds `uq_media_review_task_v11` only when absent;
- is idempotent for repeated forward execution.

### Rollback

`backend/migrations/rollback/015_media_assets_drift_reconcile.sql`:

- backfills legacy-required fields before restoring the exact pre-015 real
  definitions;
- removes only the index introduced by 015;
- does not remove v1.1 columns or any 009-014 tables.

### Runtime drift prevention

`backend/cmd/server/main.go` no longer passes `legacymedia.Asset` to
`AutoMigrate`. This is a schema ownership correction only. The legacy service,
routes, Repository and media behavior remain unchanged.

## `public_id` disposition

The column was retained as nullable compatibility state because current
production code still reads it. Its unique index was retained. This is the
minimum safe DDL; direct DROP was rejected by RCA evidence.

## Files changed

- `backend/cmd/server/main.go`
- `backend/migrations/015_media_assets_drift_reconcile.sql`
- `backend/migrations/rollback/015_media_assets_drift_reconcile.sql`
- RCA, Development, Acceptance and Residue reports
- schema and acceptance evidence under `docs/evidence/`

Migrations 001-014 were not modified. Migration number 016 remains unused and
reserved for G-①.

## Verification summary

- disposable MySQL 8.4 forward/rollback/forward syntax preflight: PASS;
- real MySQL 8.4 forward/rollback/forward: PASS;
- real backend insert after final forward: PASS;
- final `media_assets` diff vs fresh 001-014: 0;
- `go test ./... -count=1`: PASS;
- `npm run verify`: PASS.
