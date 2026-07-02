# D-6a Delivery Fee Config + Rider Wallet Backend Residue Report

Status: PASS  
Branch: `feat/d6a-delivery-fee-wallet`  
Final commit: see delivery handoff `git log` output.

## Residue policy

D-6a tests use scoped merchant/rider IDs:

- merchants: `9006001001`, `9006001002`, `9006001003`
- riders: `9006002001`, `9006002002`
- operator: `9006003001`

Cleanup removes scoped:

- delivery orders;
- delivery outbox entries;
- delivery proofs;
- merchant fee overrides;
- rider wallets;
- rider earnings;
- rider payouts;
- rider ledger rows.

Global fee config remains as intended schema seed/configuration, not test residue.

## Evidence

Real MySQL D-6a tests completed with cleanup hooks and the full delivery package real MySQL suite passed afterward:

```powershell
$env:DELIVERY_D1_MYSQL_DSN = <backend/.env MYSQL_DSN>
go test ./internal/module/delivery -count=1 -v
```

Relevant residue checks:

- existing D-1/D-2/D-3 residue tests PASS after D-6a settlement side effects;
- D-6a cleanup runs before and after D-6a tests;
- rollback guard test intentionally creates rider money facts, verifies rollback failure, then cleanup removes scoped facts.

## Local process/artifact residue

- No long-running backend/frontend processes were started for this module.
- `npm ci` was run in the worktree to provide ignored `node_modules` for typecheck/build.
- `node_modules` and build outputs are ignored and not part of git status.
- No credentials, `.env`, generated secrets, or local DSN values were added to git.

## Final status

`git status --short` before staging showed only D-6a source, migration, test, and report files.
