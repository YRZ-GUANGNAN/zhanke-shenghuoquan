# G-⑥b-1 Content Takedown Acceptance Report v1.0

Date: 2026-06-23
Branch: `feat/g6b1-content-takedown`

## Result

PASS.

G-⑥b-1 backend administrator takedown is implemented and verified against real MySQL 8.4 and real Gin/JWT middleware.

## Acceptance matrix

| Item | Result | Evidence |
| --- | --- | --- |
| Real MySQL 8.4 used | PASS | `real_mysql_version=8.4.0` |
| DB CHECK/ENUM preflight | PASS | posts/products `review_status` are `varchar(24)`, no CHECK/ENUM |
| No migration required | PASS | Schema preflight confirmed app-layer enum only |
| Community visibility positive-filtered | PASS | Public Feed uses `review_status = approved` |
| Trade visibility positive-filtered | PASS | Public List/ListByIDs use `review_status = approved`; service defaults status to selling |
| Admin auth no token | PASS | 401 |
| Ordinary user forbidden | PASS | 403 |
| Admin can takedown post | PASS | HTTP 200, `posts.review_status=taken_down` |
| Admin can takedown product | PASS | HTTP 200, `products.review_status=taken_down` |
| Post hidden from public feed | PASS | Real public HTTP list no longer contains post |
| Product hidden from public list | PASS | Real public HTTP list no longer contains product |
| Product trade status preserved | PASS | `products.status` remains `selling` |
| Idempotent repeated takedown | PASS | repeated post/product takedown returns 200 |
| Product search convergence | PASS | exactly one `SearchDelete` outbox event on transition |
| F review untouched | PASS | no review task count change |
| Boundary check | PASS | `ok A-G boundaries plus isolated P0-4 media domain` |
| Full Go regression | PASS | `go test ./... -count=1` |
| npm verify | PASS | boundaries, typecheck, backend tests |
| build | PASS | miniapp and admin builds pass; existing vendor size warning remains |
| residue | PASS | posts=0, products=0, users=0, search_outbox=0 |

## Commands executed

```powershell
go test ./internal/module/admin -run 'TestContentTakedownRealHTTPAndMySQL84|TestZZContentTakedownResidueAgainstMySQL84' -count=1 -v
go test ./... -count=1
npm run check:boundaries
npm run verify
npm run build
go build ./...
```

For real MySQL runs, `MYSQL_DSN` was injected into the process from the local environment and not printed.

## Known limitations

- `reason` is required and validated but not persisted because B/C currently have no takedown audit field, and this task explicitly forbids creating a new audit subsystem or reusing F review semantics.
- No restore path exists by design; takedown is one-way in this module.
- No frontend admin button is implemented; that belongs to G-⑥b-ui.

## Final decision

G-⑥b-1 backend: PASS.

Stop after commit and wait for owner merge/tag authorization.
