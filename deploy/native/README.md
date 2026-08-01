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

Release builds must inject non-secret build metadata so `/version` identifies
the running artifact. From the repository root, use the fixed build entrypoint;
it cross-compiles a Linux amd64 binary and writes a SHA-256 sidecar:

```powershell
.\scripts\build-native-backend.ps1 -Version 2026.08.01
```

The equivalent underlying command is:

```sh
go build -trimpath \
  -ldflags "-s -w -X main.buildVersion=${BUILD_VERSION} -X main.buildCommit=${BUILD_COMMIT} -X main.buildTime=${BUILD_TIME}" \
  -o campus-api ./cmd/server
```

Use `/healthz` for process liveness and `/readyz` for MySQL and Meilisearch
dependency readiness. The native Nginx route for `/readyz` is loopback-only;
run readiness probes on the host instead of exposing database probes publicly.
Neither endpoint replaces a real miniapp business-flow acceptance test.
