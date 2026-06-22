# P0-4 Media Assets Drift Acceptance Report v1.0

## Result

**PASS** for the authorized schema drift task.

This result does not change Module ⑩ or overall P0-4 E2E status. It closes only
the backend `media_assets` schema blocker.

## Acceptance matrix

| Requirement | Result | Evidence |
|---|---|---|
| Branch starts from `c9528b2f` | PASS | Git base verification |
| Real backend insert reproduces failure | PASS | MySQL 1364 on `public_id` |
| Full real vs fresh 001-014 schema diff | PASS | 81 before; 13 media + 68 out-of-scope |
| Root cause identified | PASS | legacy Asset AutoMigrate reproduced exact 13 diffs |
| Migration tracking investigated | PASS | no tracking table exists |
| 015 forward migration | PASS | real MySQL 8.4 |
| 015 rollback migration | PASS | real MySQL 8.4 |
| Forward after rollback | PASS | real MySQL 8.4 |
| Frozen insert succeeds after 015 | PASS | transaction probe |
| `media_assets` matches 001-014 | PASS | zero normalized differences |
| 001-014 unchanged | PASS | Git diff |
| No media business logic changes | PASS | only composition-root schema target changed |
| Probe residue | PASS | 0 |
| `go test ./... -count=1` | PASS | all backend packages |
| `npm run verify` | PASS | boundary, typecheck, backend tests |
| No P0-5/G/takeout changes | PASS | Git diff |

## Real error before fix

```text
Error 1364 (HY000): Field 'public_id' doesn't have a default value
```

## Real structure after fix

- `media_assets` normalized diff against fresh 001-014: `0`;
- out-of-scope normalized differences: `68`, unchanged;
- schema-only dump contains no data or credentials.

## Remaining observations

1. The repository has no database migration history table. This limits audit
   certainty and should be addressed only under a future authorized migration
   governance task.
2. The 68 cross-domain schema differences remain explicitly out of scope.
3. Legacy `public_id` remains as nullable compatibility state because it is
   still referenced by current production code. Its later removal requires a
   separately authorized legacy media retirement plan.

## Final gate

The schema fix is accepted and committed on the fix branch. It is not merged to
master, does not close P0-4, and does not authorize P0-5.
