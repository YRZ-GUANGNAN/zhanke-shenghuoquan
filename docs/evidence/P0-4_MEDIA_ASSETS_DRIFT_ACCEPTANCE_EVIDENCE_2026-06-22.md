# P0-4 media_assets drift acceptance evidence

Date: 2026-06-22

Branch: `fix/p0-4-media-drift`

Base: `c9528b2f2270cf18b3ac000975072432bff67e62`

No credential, token, password, DSN, or row data is included in this evidence.

## Real database identity

- MySQL: `8.4.0`
- Database: `campus_circle`
- Endpoint: loopback MySQL acceptance environment
- `media_assets` rows before migration: `0`
- migration tracking tables: `NONE`

## Pre-015 backend insert

The probe used a database transaction and inserted the frozen v1.1 fields,
including `asset_public_id`, while deliberately omitting legacy `public_id`.
The transaction was rolled back and the idempotency key was checked afterward.

```text
probe_result=FAIL error="Error 1364 (HY000): Field 'public_id' doesn't have a default value"
probe_residue=0
```

## Migration and rollback execution

```text
migration_apply=PASS file=015_media_assets_drift_reconcile.sql
probe_result=PASS
probe_residue=0

rollback_apply=PASS file=rollback/015_media_assets_drift_reconcile.sql
probe_result=FAIL error="Error 1364 (HY000): Field 'public_id' doesn't have a default value"
probe_residue=0

migration_apply=PASS file=015_media_assets_drift_reconcile.sql
probe_result=PASS
probe_residue=0
```

The rollback replay reproducing the original error proves that the result is
caused by migration 015 rather than an unrelated environment change.

## Schema comparison

| Check | Before 015 | After 015 |
|---|---:|---:|
| Full schema differences vs fresh 001-014 | 81 | 68 |
| `media_assets` differences | 13 | 0 |
| Out-of-scope A/B/C/D/E/F differences | 68 | 68 |

The 68 out-of-scope differences were recorded but not changed.

## Regression

```text
go test ./... -count=1
PASS

npm run verify
PASS
  check:boundaries PASS
  admin typecheck PASS
  miniapp typecheck PASS
  backend tests PASS
```

## Residue

```text
media_assets probe residue = 0
media_assets total rows = 0
temporary database rows = 0
```

## Evidence files

- `P0-4_MEDIA_SCHEMA_REAL_BEFORE_015.sql`: real MySQL 8.4 no-data dump before reconciliation.
- `P0-4_MEDIA_SCHEMA_REAL_BEFORE_015.json`: normalized structure before reconciliation.
- `P0-4_MEDIA_SCHEMA_BASELINE_001_014.sql`: fresh 001-014 no-data baseline.
- `P0-4_MEDIA_SCHEMA_BASELINE_001_014.json`: normalized 001-014 baseline.
- `P0-4_MEDIA_SCHEMA_DIFF_BEFORE_015.json`: complete machine-readable 81-item diff.
- `P0-4_MEDIA_LEGACY_AUTOMIGRATE_REPRO.json`: isolated reproduction of the exact 13-item drift.
- `P0-4_MEDIA_SCHEMA_REAL_AFTER_015.sql`: real MySQL 8.4 no-data dump after reconciliation.
- `P0-4_MEDIA_SCHEMA_REAL_AFTER_015.json`: normalized structure after reconciliation.
- `P0-4_MEDIA_SCHEMA_DIFF_AFTER_015.json`: final machine-readable diff; media count is zero.

## SHA-256

```text
backend/migrations/015_media_assets_drift_reconcile.sql
E695B1D16BE340BD15AB1E8881CBFF1AF2E532A38775580CE9A38F7CFF65243F

backend/migrations/rollback/015_media_assets_drift_reconcile.sql
399E6CF31C9B0E689F73BF10C9DFBD53813F6A78C62E013D14BC495729372410

docs/evidence/P0-4_MEDIA_SCHEMA_REAL_BEFORE_015.sql
9528574DB3859EA1605B6D6D3BEEAE91853ED6A6599608BE70245D73039936A6

docs/evidence/P0-4_MEDIA_SCHEMA_REAL_AFTER_015.sql
B9A56F5400BF69F204337BF29FAF13B8A2908FF361046EEE759EAFA7DEFBB7BB
```
