# G-7a Buyer API Residue Report

## Database residue

The real MySQL test registers cleanup before and after execution. It removes:

- `mock_openid_g7a-buyer-a`
- `mock_openid_g7a-buyer-b`
- merchants with `name LIKE 'G7A%'`
- categories with `name LIKE 'G7A%'`
- dishes with `name LIKE 'G7A%'`
- orders and order items belonging to G-7a merchants

Residue verification:

```text
=== RUN   TestZZTakeoutBuyerHTTPResidueAgainstMySQL84
--- PASS: TestZZTakeoutBuyerHTTPResidueAgainstMySQL84 (0.00s)
```

Result: database residue = 0 for G-7a acceptance data.

## Process residue

No long-running backend, frontend, worker, or browser process was left running. Verification used one-shot `httptest.NewServer`, `go test`, `npm run verify`, and `npm run build`; the HTTP test server is closed with `defer server.Close()`.

## Dependency/build residue

The independent worktree did not have `node_modules`; `npm ci` was run from the existing lockfile so typecheck/build scripts could execute. `node_modules` and build outputs are git-ignored and were not staged.

## Git residue

Only G-7a buyer API source files and docs are staged for commit. No migration, frontend, media, A-F, P0-5, or payment code was modified.
