# Native production deployment

This directory contains the native Ubuntu deployment used for the low-memory
production host. Secrets are intentionally excluded.

Domain responsibilities:

- `https://zhanduiwang.top/` is the management website. It serves the Admin static
  build and proxies its `/api/` requests to the local Go API.
- `https://zhanduiwang.top/` remains the mini-program API and existing public
  entry point. The mini-program build continues to use `/api/v1` on this domain.

The owner corrected the originally requested `zhandui.top` to `zhanduiwang.top`
on 2026-10-12. The former domain is not managed by the verified Aliyun account.
Its old migration assets are retained as inactive historical files; do not run
`migrate-admin-domain.ps1` or enable `nginx-zhandui.top*.conf` for this deployment.
Keep the existing `nginx-zhanduiwang.top.conf` server blocks and certificate.

Runtime layout:

- `/opt/campus-circle/current/bin/campus-api`
- `/opt/campus-circle/bin/meilisearch`
- `/etc/campus-circle/campus-api.env`
- `/etc/campus-circle/meilisearch.env`
- `/var/lib/campus-meilisearch`
- `/var/www/campus-admin/current` (active `zhanduiwang.top` Admin release symlink)

The production database is initialized from `backend/migrations` in numeric
order, with `002_seed_dev.sql` explicitly skipped. The migration set is not
idempotent and has no migration ledger, so a failed initialization of a new,
unused database must be recovered by dropping and recreating only that new
database, then replaying the full filtered sequence from `001`.

The miniapp production build must set:

```text
TARO_APP_API_BASE=https://zhanduiwang.top/api/v1
```

The Admin build uses relative `/api/v1` requests, so it can be served from
`https://zhanduiwang.top/` without embedding a second API hostname.

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

## Admin release on the existing domain

Verify the DNS record before a release:

```powershell
Resolve-DnsName zhanduiwang.top -Type A
```

The answer must be the production host address `8.138.227.107`. The verified
Aliyun account already has the correct root and `www` records. No DNS migration,
new certificate, or Nginx reload is needed for an Admin static release.

Upload the built Admin archive and `release-admin.py` into an isolated staging
directory. Run this on the host with the actual archive hash and release ID:

```sh
sudo python3 /home/admin/ADMIN_STAGE/release-admin.py \
  --archive /home/admin/ADMIN_STAGE/admin.tar.gz \
  --sha256 SHA256 --release RELEASE
```

The default performs read-only preflight checks. Add `--deploy` after they pass.
The script validates the archive checksum and paths, creates a new release,
retains previous content-addressed assets for existing browser tabs, and
atomically switches `/var/www/campus-admin/current`. It verifies every newly
served asset over HTTPS, the backend health, the public feature policy, and
401 responses without an administrator session. A failed post-switch check
restores the previous symlink; old releases are retained. It does not change
Nginx, DNS, user roles, or business records.
