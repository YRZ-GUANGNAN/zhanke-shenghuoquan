# Native production deployment

This directory contains the native Ubuntu deployment used for the low-memory
production host. Secrets are intentionally excluded.

Domain responsibilities:

- `https://zhandui.top/` is the management website. It serves the Admin static
  build and proxies its `/api/` requests to the local Go API.
- `https://zhanduiwang.top/` remains the mini-program API and existing public
  entry point. The mini-program build must continue to use this API domain;
  changing the management host does not change the API contract.

During the DNS and certificate migration, keep the existing
`nginx-zhanduiwang.top.conf` server blocks enabled. Enable
`nginx-zhandui.top.acme.conf` first, and switch to `nginx-zhandui.top.conf`
only after `zhandui.top` resolves to this host and the certificate files under
`/etc/letsencrypt/live/zhandui.top/` exist. The two files must not be enabled
at the same time because both own the port-80 `zhandui.top` server block.

Runtime layout:

- `/opt/campus-circle/current/bin/campus-api`
- `/opt/campus-circle/bin/meilisearch`
- `/etc/campus-circle/campus-api.env`
- `/etc/campus-circle/meilisearch.env`
- `/var/lib/campus-meilisearch`
- `/var/www/campus-admin/current` (existing `zhanduiwang.top` site; kept unchanged)
- `/var/www/campus-admin-zhandui/current` (independent `zhandui.top` Admin site)

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
`https://zhandui.top/` without embedding a second API hostname.

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

## Management-domain migration

Before changing Nginx, verify the DNS record:

```powershell
Resolve-DnsName zhandui.top -Type A
```

The answer must be the production host address `8.138.227.107`. If it still
points elsewhere, change the Cloudflare A record first and wait for the low
TTL to expire; do not disable the existing `zhanduiwang.top` site while
waiting.

On the host, publish the HTTP ACME challenge server, request the apex
certificate using the already registered Certbot account, and then replace it
with the HTTPS server:

```sh
sudo certbot certonly --webroot -w /var/www/letsencrypt \
  -d zhandui.top --non-interactive --keep-until-expiring
sudo test -s /etc/letsencrypt/live/zhandui.top/fullchain.pem
sudo test -s /etc/letsencrypt/live/zhandui.top/privkey.pem
# disable nginx-zhandui.top.acme.conf and enable nginx-zhandui.top.conf
sudo nginx -t
sudo systemctl reload nginx
```

Use `migrate-admin-domain.ps1` from the repository root for the guarded
sequence. It requires exactly one A record for `8.138.227.107`, rejects an
unexpected AAAA record, verifies the local and remote SHA-256 of the release
archive, creates the ACME webroot, switches both the Admin `current` symlink
and the Nginx site link, and restores both links if `nginx -t` or a later
step fails:

```powershell
.\deploy\native\migrate-admin-domain.ps1 -BuildAdmin -Deploy -RequestCertificate -EnableHttps
```

If a later step fails, the script restores the previous Nginx link and Admin
`current` symlink independently. It leaves the failed release directory for
inspection and never removes the existing `zhanduiwang.top` release.
