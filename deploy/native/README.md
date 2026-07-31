# Native production deployment

This directory contains the native Ubuntu deployment used for the low-memory
`zhanduiwang.top` host. Secrets are intentionally excluded.

Runtime layout:

- `/opt/campus-circle/current/bin/campus-api`
- `/opt/campus-circle/bin/meilisearch`
- `/etc/campus-circle/campus-api.env`
- `/etc/campus-circle/meilisearch.env`
- `/var/lib/campus-meilisearch`
- `/var/www/campus-admin/current`

The production database is initialized from `backend/migrations` in numeric
order, with `002_seed_dev.sql` explicitly skipped. The migration set is not
idempotent and has no migration ledger, so a failed initialization of a new,
unused database must be recovered by dropping and recreating only that new
database, then replaying the full filtered sequence from `001`.

The miniapp production build must set:

```text
TARO_APP_API_BASE=https://zhanduiwang.top/api/v1
```

The API and Meilisearch listen only on loopback. Only Nginx exposes ports 80
and 443.
