# D-5 Delivery IM Backend Acceptance Report

Date: 2026-07-01

Implementation commit: `ce451ee`

Result: **CONDITIONAL PASS / baseline merge gate remains open**

## 1. Acceptance matrix

| Gate | Result | Evidence |
|---|---|---|
| Baseline and scope | PASS | Started from `6c39d477`; backend only; no takeout/frontend change |
| Migration 027 apply | PASS | Real MySQL 8.4.0 |
| Rollback | PASS | Real MySQL 8.4.0 |
| Rollback with facts | PASS, fail-closed | MySQL error 1644 / SQLSTATE 45000 |
| Re-apply | PASS | Table and media CHECK restored |
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
| Private image STS/COS upload | PASS | Real Tencent STS/COS |
| Completion/fixed final version | PASS | Real COS versioned object |
| Review and private binding | PASS (local review mock) | `purpose=delivery_im`, active/visible private reference |
| Signed image GET | PASS | Fixed version, TTL 60 seconds |
| Cross-owner image fence | PASS | Rejected |
| Generic public resolve | PASS | `delivery_im` returns 404 |
| Real WeChat image review | CONDITIONAL | Adapter remains an acknowledged production blocker; images disabled in production |
| Tencent group unread/read receipt | **FAIL / external gate** | Real receipt query returned item `Code=10062` |
| Delivery core import-free | PASS | Boundary check |
| Existing product C2C unchanged | PASS | Diff and full regression |
| `go test ./... -count=1` | PASS | All backend packages |
| `npm run check:boundaries` | PASS | A-G, Delivery, Media and deliveryim rules |
| `npm run verify` | PASS | Typecheck plus backend tests |
| `npm run build` | PASS | Miniapp and Admin; existing miniapp bundle-size warning only |
| AutoMigrate | PASS | Zero matches |

## 2. Real MySQL evidence

Commands were executed against the configured real MySQL database without logging credentials.

Observed:

- `real_mysql_version=8.4.0`
- `migration_027_apply=PASS rollback=PASS fail_closed=PASS reapply=PASS`
- `d5_assignment_generation=PASS participant_fence=PASS old_rider_404=PASS lease=PASS revocation=PASS`
- `real_http=PASS real_mysql=PASS merchant=200 current_rider=200 outsider=404 old_rider=404 new_assignment=PASS remote_read_only_fail_closed=PASS`

The rollback refusal was a deliberate `Error 1644 (45000)` while facts existed.

## 3. Real Tencent IM evidence

The external lifecycle test reported:

`real_tencent_im=PASS account_import=PASS ownerless_work_group=PASS idempotent_create=PASS remote_read_only=PASS member_remove=PASS member_readd=PASS destroy=PASS`

The ownerless group makes merchant and rider ordinary members, allowing Tencent all-member mute to enforce terminal read-only for both.

The real read-receipt test sent a group message with receipt requested, submitted the reader receipt and queried Tencent. Tencent returned:

`Code=10062, MsgSeq=1, ReadNum=0, UnreadNum=0`

This is not replaced by a local fake. D-5 cannot be promoted to formal PASS until the required group read-receipt capability is enabled in the Tencent console and:

```powershell
$env:DELIVERY_D5_RUN_REAL_TENCENT_IM='1'
$env:DELIVERY_D5_RUN_REAL_READ_RECEIPT='1'
go test ./internal/module/im -run '^TestTencentWorkGroupReadReceiptAgainstRealIM$' -count=1 -v
```

passes against the real account.

## 4. Real COS/private-image evidence

The external test reported:

`real_sts=PASS cos_put=PASS completion=PASS purpose=delivery_im local_mock_review=PASS fixed_final_version=<redacted> signed_get_ttl=60 private_reference=PASS owner_fence=PASS`

The signed URL was fetched successfully and contained a fixed `versionId`. Neither credentials nor raw COS identity is present in this report.

## 5. Regression evidence

- `go test ./... -count=1`: PASS
- `npm run check:boundaries`: PASS
- `npm run verify`: PASS
- `npm run build`: PASS
- Miniapp warning: existing `vendors.js` size warning; no build failure
- `rg "db\.AutoMigrate\(" backend/cmd/server`: zero matches

The temporary worktree dependency junctions used only to reuse the already-installed workspace packages were removed after verification.

## 6. Final decision

D-5 backend implementation and all locally controllable backend gates pass. The result remains **CONDITIONAL PASS** for two explicit production gates:

1. Tencent Work Group read receipts are not enabled (`10062`).
2. Real WeChat image review is not connected, so production image sending remains disabled.

Do not merge/tag `d5-delivery-im-backend-baseline` as formal PASS until the owner closes the Tencent receipt gate. No D-6 work is authorized.
