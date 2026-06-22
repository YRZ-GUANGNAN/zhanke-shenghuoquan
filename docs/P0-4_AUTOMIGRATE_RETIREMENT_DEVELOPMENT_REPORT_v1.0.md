# P0-4 AutoMigrate Retirement Development Report v1.0

## Scope

Branch: `retire/automigrate`

Base: `d030585ef1aaabbb5ff347433f6f9bcbe517a2ef`

This task retires startup schema mutation. It does not reconcile the 68 stored
cross-domain differences and does not change domain models, tags, services,
repositories, routes, migrations, G/takeout or P0-5.

## Safety precheck

The only production schema mutation entry was one `db.AutoMigrate(...)` call in
`backend/cmd/server/main.go`. It contained 18 A-F domain models plus
`legacymedia.AssetAudit`, for 19 models in total. `media_assets` had already
been removed by commit `d030585e`.

All 19 target tables are migration-owned:

- A: `users` — migrations 001, 004, 007;
- B: `posts`, `post_images`, `comments`, `post_likes`, `post_favorites`,
  `user_follows`, `notifications` — migrations 001, 003, 005, 007;
- C: `products`, `product_images`, `product_favorites`,
  `trade_search_outbox` — migrations 001, 003, 006, 007;
- D: `navigation_categories`, `navigation_items` — migration 001;
- E: `im_user_mappings`, `im_conversations` — migrations 001 and 006;
- F: `review_tasks`, `reports` — migration 001;
- Media audit: `media_asset_audits` — migration 008.

No AutoMigrate-exclusive table or column was found. Excluding migrations and
tests, no other production `AutoMigrate`, GORM migrator, `CREATE TABLE` or
`ALTER TABLE` operation exists. `database.Open` only opens MySQL and configures
the connection pool; CRUD does not depend on startup DDL.

## Change

The complete `db.AutoMigrate(...)` block was removed from
`backend/cmd/server/main.go`.

No replacement migration runner or migration tracking table was introduced.
Migration 016 remains unused and reserved for G-①.

## Production-code diff

- modified: `backend/cmd/server/main.go`;
- removed: one startup `db.AutoMigrate(...)` call;
- added production code: none;
- business behavior changes: none.
