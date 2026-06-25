# G-④ Payment Residue Report

Status: PASS

## 1. Test data residue

Command:

```powershell
go test ./internal/module/admin/takeoutpayment -run '^TestZZTakeoutPaymentResidueAgainstMySQL84$' -count=1 -v
```

Result:

```text
=== RUN   TestZZTakeoutPaymentResidueAgainstMySQL84
--- PASS: TestZZTakeoutPaymentResidueAgainstMySQL84
```

Checked residue categories:

- mock users with `mock_openid_g4-%`
- G4 merchants
- G4 orders
- G4 payment attempts

Result: zero G4 test residue.

## 2. Migration residue

The real MySQL database intentionally remains at schema version including G-④ migration 022 after successful apply/rollback/apply. This is expected for the module.

Tables now present:

- `takeout_payment_attempts`
- `takeout_payment_notifications`

## 3. Process residue

No long-running backend, dev server, or watcher process was started for final acceptance. No process cleanup was required.

## 4. Generated files

`npm run build` generated ignored build outputs only. `git status --short` shows only intended source, migration, and docs changes.

## 5. Secrets

No secrets were printed or committed. The real MySQL DSN was read from the owner project `.env` for local acceptance and was never written into repository files or reports.

