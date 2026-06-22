# P0-4 AutoMigrate Retirement Acceptance Report v1.0

## Result

**PASS** — application startup no longer executes schema DDL, while both the
current real database shape Y and a clean migration shape X remain bootable.

## A. Code proof

```text
rg "db\.AutoMigrate\(" backend/cmd/server
matches = 0
```

No production `AutoMigrate`, GORM migrator, `CREATE TABLE` or `ALTER TABLE`
call remains outside migration files.

## B. Real database functional proof — shape Y

The changed application binary was started against the existing real MySQL
8.4 database. The unrelated Meilisearch bootstrap dependency was served by a
temporary local protocol harness; database access and the application binary
were real. No mock or replacement database was used.

```text
real_db_boot=PASS
GET /healthz=200
```

The normalized real schema snapshot committed with 015 contained 511 rows.
The new post-startup snapshot also contains 511 rows and is byte-equivalent as
normalized JSON:

```text
prior_015_snapshot_rows=511
current_Y_snapshot_rows=511
prior_EQ_current=true
```

Regression:

```text
go test ./... -count=1              PASS
npm run verify                      PASS
  check:boundaries                  PASS
  admin typecheck                   PASS
  miniapp typecheck                 PASS
  backend tests                     PASS
```

## C. No-schema-rewrite controlled experiment — shape X

An isolated MySQL 8.4 instance was created and migrations 001 through 015 were
applied in order. Every migration reported PASS. A schema-only snapshot was
taken before application startup (SX1), then the changed application was
started and stopped successfully against that database, and a second
schema-only snapshot was taken (SX2).

Canonical schema snapshots exclude the volatile InnoDB AUTO_INCREMENT counter,
which is data state rather than DDL state.

```text
migrated_shape_app_boot=PASS
GET /healthz=200

SX1_NORMALIZED_SHA256=97AE942F041DD1607A06D01624117D1D89177F08A0FE1B0C42CE09A8104D5A19
SX2_NORMALIZED_SHA256=97AE942F041DD1607A06D01624117D1D89177F08A0FE1B0C42CE09A8104D5A19
SX1_EQ_SX2=true
```

The raw dumps differed only in
`trade_search_outbox AUTO_INCREMENT=4 -> 7`, caused by the existing background
search worker touching seeded outbox data. No column, constraint, index, table,
engine or collation changed.

The pre-change counterexample is retained in the 015 RCA evidence:
`P0-4_MEDIA_LEGACY_AUTOMIGRATE_REPRO.json`, where legacy AutoMigrate reproduced
the migration drift.

## D. Current production baseline — shape Y

The current real database was exported with `mysqldump --no-data` and as a
normalized information-schema snapshot. Comparing current shape Y with fresh
001-015 shape X gives exactly 68 differences:

| Table | Differences |
|---|---:|
| `products` | 11 |
| `users` | 9 |
| `posts` | 8 |
| `notifications` | 7 |
| `navigation_items` | 6 |
| `review_tasks` | 5 |
| `trade_search_outbox` | 4 |
| `im_user_mappings` | 4 |
| `comments` | 3 |
| `post_images` | 3 |
| `product_images` | 3 |
| `im_conversations` | 3 |
| `navigation_categories` | 2 |
| **Total** | **68** |

These differences were observed and recorded but not modified. Until a later
alignment task is explicitly accepted, production must not be rebuilt solely
from migrations 001-015 on the assumption that shape X already equals shape Y.

Evidence:

- `docs/evidence/P0-4_AUTOMIGRATE_RETIREMENT_PRODUCTION_BASELINE_Y.sql`;
- `docs/evidence/P0-4_AUTOMIGRATE_RETIREMENT_PRODUCTION_BASELINE_Y.json`;
- `docs/evidence/P0-4_AUTOMIGRATE_RETIREMENT_X_Y_DIFF.json`.

## Boundary verification

- migrations 001-015 unchanged;
- no migration 016;
- no model, tag, Service, Repository or Route changes;
- no 68-item reconciliation;
- no G/takeout changes;
- no P0-5 work;
- no master merge.
