# P0-4 Media Assets Drift Root Cause Analysis v1.0

## Conclusion

The real 500-class insert failure was caused by schema drift, not by the
MiniApp `readFileBytes` issue. The frozen v1.1 repository inserts
`asset_public_id` and does not populate legacy `public_id`, while the real
database had been changed back to `public_id VARCHAR(32) NOT NULL`.

The drift source is the startup `db.AutoMigrate(&legacymedia.Asset{})` call.
Applying that legacy model to a clean 001-014 schema reproduced all 13 real
`media_assets` differences exactly, including removal of
`uq_media_review_task_v11`.

## Real failure

```text
probe_result=FAIL error="Error 1364 (HY000): Field 'public_id' doesn't have a default value"
probe_residue=0
```

The probe bypassed the MiniApp and COS path. It was a direct backend database
transaction using only the frozen v1.1 asset fields.

## Migration history findings

1. Migration 007 created the legacy `public_id`-owned asset model.
2. Migration 009 added v1.1 fields and explicitly changed `public_id` to
   nullable `CHAR(32)`.
3. The real database contains the 009-014 tables, columns and indexes, proving
   that those migrations were applied at least in substance.
4. The real database has no migration tracking table. Therefore exact run time,
   operator and checksum history cannot be reconstructed from the database.
5. Server startup continued registering the legacy GORM `Asset` model. Its tags
   conflict with 009 and deterministically reproduce the real 13-item drift.

There is no evidence sufficient to attribute the change to a manual DDL
operator. The exact AutoMigrate reproduction is positive evidence for the
runtime schema mutator as the cause.

## Complete `media_assets` diff before 015

| Item | Real database | Fresh 001-014 contract |
|---|---|---|
| `public_id` | `VARCHAR(32) NOT NULL` | `CHAR(32) NULL` |
| `owner_user_id` | `NOT NULL` | nullable |
| `object_key` | `NOT NULL` | nullable |
| `declared_mime` | no default | default `''` |
| `declared_size` | no default | default `0` |
| `status` | no default | default `pending_upload` |
| `reference_type` | nullable | `NOT NULL DEFAULT ''` |
| `reference_id` | nullable | `NOT NULL DEFAULT 0` |
| `reference_sort` | nullable | `NOT NULL DEFAULT 0` |
| `reservation_token` | nullable `VARCHAR(32)` | `NOT NULL CHAR(32) DEFAULT ''` |
| `upload_expires_at` | `NOT NULL` | nullable |
| `review_available_at` | `NOT NULL` | nullable |
| `uq_media_review_task_v11` | missing | unique on `review_task_id` |

The complete machine-readable comparison is in
`docs/evidence/P0-4_MEDIA_SCHEMA_DIFF_BEFORE_015.json`.

## Full-database diff

The real schema differed from a fresh 001-014 schema in 81 normalized items:

- `media_assets`: 13, all handled by 015;
- other domain tables: 68, intentionally left unchanged because this task is
  limited to P0-4 media schema coordination.

The remaining differences are recorded by table in the diff evidence. They are
not prerequisites for fixing the media insert and must be handled only under
separate domain authorization.

## `public_id` safety decision

`public_id` could not be dropped because production code still references it:

- `backend/internal/pkg/media/model.go` maps `PublicID`;
- `backend/internal/pkg/media/repository.go` queries and updates by `public_id`;
- `backend/cmd/server/main.go` still composes the legacy media service used by
  existing routes and domain adapters.

Dropping the column or its unique index would break frozen legacy behavior and
violate the no-business-refactor boundary. Migration 015 therefore takes the
minimum safe option allowed by the task:

- retain `public_id` and `idx_media_public_id` for compatibility;
- restore `CHAR(32) NULL`, so v1.1 rows may use only `asset_public_id`;
- rely on MySQL's multiple-NULL unique-index semantics;
- stop registering the legacy `Asset` as a startup AutoMigrate target.

No media Service, Repository, state machine or HTTP behavior was changed.

## Prevention

`media_assets` is now migration-owned. The composition root continues to
AutoMigrate the separate legacy audit table, but no longer mutates
`media_assets`. Future schema changes must use numbered migrations and retain a
machine-verifiable schema snapshot.
