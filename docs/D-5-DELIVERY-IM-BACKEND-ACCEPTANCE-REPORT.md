# D-5 Delivery IM Backend Acceptance Report

Date: 2026-07-02

Initial implementation commit: `ce451ee5`

Read-receipt degradable patch commit: `65d115c9`

Result: **CONDITIONAL PASS / ready for conditional owner merge**

## 1. Acceptance matrix

| Gate | Result | Evidence |
|---|---|---|
| Baseline and scope | PASS | Started from `6c39d477`; backend only; no takeout/frontend change |
| Migration 027 apply | PASS | Real MySQL 8.4.0 from initial D-5 acceptance |
| Rollback | PASS | Real MySQL 8.4.0 from initial D-5 acceptance |
| Rollback with facts | PASS, fail-closed | MySQL error 1644 / SQLSTATE 45000 from initial D-5 acceptance |
| Re-apply | PASS | Table and media CHECK restored from initial D-5 acceptance |
| No cross-domain FK | PASS | `delivery_im_conversations` FK count 0 |
| Conversation state machine | PASS | Positive/negative transition matrix |
| CAS and lease recovery | PASS | Real MySQL assignment/lease/retry tests |
| Participant fence | PASS | Merchant/current rider 200; outsider/old rider 404 |
| Assignment generation | PASS | New rider receives a new group and generation |
| Release revocation | PASS | Old rider removed before local revocation |
| Terminal read-only | PASS | Remote group mute precedes local CAS; remote failure leaves local status unchanged |
| Tencent account import/group create | PASS | Real Tencent IM |
| Tencent idempotent create | PASS | Real Tencent IM |
| Tencent member remove/re-add | PASS | Real Tencent IM |
| Tencent remote read-only | PASS | `MuteAllMember=On` verified by `get_group_info` |
| Tencent group destroy | PASS | Real Tencent IM |
| Tencent group residue | PASS | D-5 group prefix count 0 |
| Tencent group read receipt default | PASS, degraded | `ENABLE_DELIVERY_IM_READ_RECEIPT` default off; test skips before calling Tencent receipt APIs |
| Tencent group read receipt upgrade path | RESERVED | Non-empty send/confirm/query implementation retained; rerun after Tencent capability upgrade |
| Unread count behavior | PASS, unchanged | Not touched; existing Tencent-client-native unread/setMessageRead path remains separate |
| Read receipt DTO contract | PASS | `readReceiptEnabled` retained and tested false/true |
| Private image STS/COS upload | PASS | Real Tencent STS/COS |
| Completion/fixed final version | PASS | Real COS versioned object |
| Review and private binding | PASS (local review mock) | `purpose=delivery_im`, active/visible private reference |
| Signed image GET | PASS | Fixed version, TTL 60 seconds |
| Cross-owner image fence | PASS | Rejected |
| Generic public resolve | PASS | `delivery_im` returns 404 |
| Real WeChat image review | CONDITIONAL | Adapter remains an acknowledged production blocker; images disabled in production |
| Delivery core import-free | PASS | Boundary check |
| Existing product C2C unchanged | PASS | Diff and full regression |
| `go test ./... -count=1` | PASS | All backend packages |
| `npm run check:boundaries` | PASS | A-G, Delivery, Media and deliveryim rules |
| `npm run verify` | PASS | Typecheck plus backend tests |
| `npm run build` | PASS | Miniapp and Admin; existing miniapp bundle-size warning only |
| AutoMigrate | PASS | Zero matches |

## 2. File-level precheck

The D-5 read receipt and unread-count paths are separate:

- Group read receipt capability is represented by real Tencent receipt operations in `backend/internal/module/im/tencent.go` and gated by `TestTencentWorkGroupReadReceiptAgainstRealIM`.
- Delivery conversation HTTP responses expose only the capability flag `readReceiptEnabled`.
- Unread count and `setMessageRead` remain Tencent-native client capabilities; this patch does not change delivery conversation state, repository, message storage or user credentials.

The new config flag follows the existing `envBool` pattern:

- `ENABLE_DELIVERY_IM_READ_RECEIPT=false` by default.
- No production hard block is needed because this is optional enhancement functionality.

## 3. Real MySQL evidence

Commands were executed against the configured real MySQL database without logging credentials.

Observed after the patch:

- `real_mysql_version=8.4.0`
- `d5_assignment_generation=PASS participant_fence=PASS old_rider_404=PASS lease=PASS revocation=PASS`
- `real_http=PASS real_mysql=PASS merchant=200 current_rider=200 outsider=404 old_rider=404 new_assignment=PASS remote_read_only_fail_closed=PASS`

The D-5 migration file was not changed in this patch; the prior migration apply/rollback/fail-closed/reapply evidence remains valid.

## 4. Real Tencent IM evidence

The external lifecycle test after the patch reported:

`real_tencent_im=PASS account_import=PASS ownerless_work_group=PASS idempotent_create=PASS remote_read_only=PASS member_remove=PASS member_readd=PASS destroy=PASS`

The real read-receipt test with the destructive flags set and the new config flag unset reported:

`SKIP: set ENABLE_DELIVERY_IM_READ_RECEIPT=true after Tencent IM read receipts are enabled`

This confirms the default path no longer calls Tencent `send_group_msg` with receipt, `group_msg_receipt` or `get_group_msg_receipt`, and therefore no longer hits Code `10062`.

Group residue after the real Tencent run:

`real_tencent_group_residue=0 cleaned=0`

The preserved upgrade test remains:

```powershell
$env:DELIVERY_D5_RUN_REAL_TENCENT_IM='1'
$env:DELIVERY_D5_RUN_REAL_READ_RECEIPT='1'
$env:ENABLE_DELIVERY_IM_READ_RECEIPT='true'
go test ./internal/module/im -run '^TestTencentWorkGroupReadReceiptAgainstRealIM$' -count=1 -v
```

## 5. Real COS/private-image evidence

The external test after the patch reported:

`real_sts=PASS cos_put=PASS completion=PASS purpose=delivery_im local_mock_review=PASS fixed_final_version=<redacted> signed_get_ttl=60 private_reference=PASS owner_fence=PASS`

The signed URL was fetched successfully and contained a fixed version. Neither credentials nor raw COS identity is present in this report.

## 6. Regression evidence

- `go test ./internal/config -count=1`: PASS
- `go test ./internal/module/admin/deliveryim ./internal/module/im -count=1`: PASS
- `DELIVERY_D5_RUN_REAL_TENCENT_IM=1 DELIVERY_D5_RUN_REAL_READ_RECEIPT=1` with read receipt switch unset: PASS with receipt test skipped before Tencent receipt calls
- `go test ./... -count=1`: PASS
- `npm run check:boundaries`: PASS
- `npm run verify`: PASS
- `npm run build`: PASS
- Miniapp warning: existing `vendors.js` size warning; no build failure
- `rg "db\.AutoMigrate\(" backend/cmd/server`: zero matches

The temporary worktree dependency junctions used only to reuse the already-installed workspace packages were removed after verification.

## 7. Final decision

D-5 backend is now acceptable for **CONDITIONAL PASS** landing:

1. Group read receipts are gracefully degraded behind `ENABLE_DELIVERY_IM_READ_RECEIPT=false`, with a complete upgrade path preserved.
2. Production image sending remains conditional until the real WeChat image-review adapter is connected.

No D-6 work is authorized.
