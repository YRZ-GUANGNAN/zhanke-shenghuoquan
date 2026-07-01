# D-5 Delivery IM Backend Residue Report

Date: 2026-07-01

Implementation commit: `ce451ee`

## Required scoped residue

| Resource | Result | Verification |
|---|---|---|
| MySQL delivery orders/conversations used by D-5 tests | 0 | Scoped cleanup runs before and after each real test |
| MySQL media assets/tickets/projections/references used by D-5 COS test | 0 | Owner-scoped transactional cleanup and post-count |
| COS staging object versions | 0 | Fixed-key version listing after deletion |
| COS final object versions | 0 | Fixed-key version listing after deletion |
| COS DeleteMarkers | 0 | Fixed-key marker listing after deletion |
| Tencent D-5 test Work Groups | 0 | Real `get_appid_group_list` prefix scan after group destruction |
| Local test processes | 0 | No server or worker was left running |
| Temporary dependency junctions | 0 | Root, Admin and Miniapp junctions removed |
| Untracked build output | 0 | Build outputs are ignored; no source/worktree residue introduced |

Real Tencent group check:

`real_tencent_group_residue=0 cleaned=0`

## Tencent test identity note

Tencent IM account import is idempotent and production SDKAppIDs do not support deleting imported accounts through the tested `account_delete` call (`71000 Err_TLS_PT_Open_Login_Not_Demo_SDKAppID`). The finalized external tests therefore reuse four deterministic acceptance identities rather than creating new identities per run.

During early development runs, timestamped acceptance identities were imported before this correction. Tencent exposes no usable delete/list path for those identities on this SDKAppID, so strict “all historical imported IM identities = 0” cannot be independently attested. They have no active D-5 groups; the real group-prefix scan is zero. This limitation is recorded rather than hidden.

## Secrets and evidence hygiene

- `.env` was read only at runtime and was not staged.
- No Tencent secret, COS secret, JWT secret, database password, signed URL or full version ID is included.
- No temporary composition root, binary, `node_modules` or build output is staged.

## Residue conclusion

All residue categories explicitly required by D-5 acceptance—database test facts, Tencent test groups, COS versions/DeleteMarkers and local processes—are zero. The only recorded external-account limitation is the non-deletable historical Tencent test identity metadata described above.
