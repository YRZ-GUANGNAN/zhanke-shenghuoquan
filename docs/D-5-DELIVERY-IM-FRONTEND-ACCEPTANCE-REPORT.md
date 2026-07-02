# D-5 Delivery IM Frontend Acceptance Report

Date: 2026-07-02

Branch: `feat/d5-delivery-im-frontend`

Base: `89f87124 docs(D-5): record degradable read receipt acceptance`

Final commit hash: recorded in final handoff after commit creation.

## Preflight

- `git log --oneline -3` confirmed the branch starts from D-5 backend read-receipt baseline:
  - `89f87124 docs(D-5): record degradable read receipt acceptance`
  - `65d115c9 fix(D-5): make delivery IM read receipts degradable behind feature gate`
  - `3c7a2e8a docs(D-5): record delivery IM backend conditional acceptance`
- `git tag --list d5-delivery-im-backend-baseline d4-delivery-proof-frontend-baseline` returned both tags.
- `rg "db\.AutoMigrate\(" backend/cmd/server` returned zero matches.
- Main repository was restored clean before creating the feature worktree.

## Verification evidence

Commands executed in `C:\Users\33141\Documents\d5-delivery-im-frontend`:

- `npm --workspace apps/miniapp run typecheck` — PASS.
- `npm run build:miniapp` — PASS, with existing `vendors.js` size warning only.
- `npm run check:boundaries` — PASS:
  - `ok A-G boundaries plus independent delivery and isolated P0-4 media domains`
- `npm run verify` — PASS:
  - boundary check PASS
  - admin typecheck PASS
  - miniapp typecheck PASS
  - backend tests PASS
- `cd backend && go test ./... -count=1` — PASS.

## Acceptance matrix

| Requirement | Result | Evidence |
| --- | --- | --- |
| Merchant can enter conversation from delivery order detail | PASS | `delivery-merchant/index.tsx` adds `联系骑手` after `riderUserId` exists |
| Rider can enter conversation from assigned order detail | PASS | `delivery-rider/index.tsx` adds `联系商家` |
| Conversation is bound to backend order-scoped D-5 endpoint | PASS | `services/delivery-im.ts` calls `/delivery/orders/:orderId/conversation` |
| Text messages use Tencent Chat SDK group chat | PASS | `sendDeliveryTencentText` uses `CONV_GROUP` and backend `tencentConversationId` |
| Image messages use private Media chain, not Tencent native image upload | PASS | upload purpose `delivery_im`, backend `images/prepare`, custom message payload, participant-protected resolve |
| `imagesEnabled=false` hides/blocks image sending | PASS | UI condition plus send guard |
| Peer read receipt not displayed | PASS | UI text states no peer-read display; sends use `needReadReceipt:false` |
| Unread badge | PASS | Entry unread via `getDeliveryConversationUnreadByOrder`; conversation card displays unread count |
| Nonparticipant/old rider/ended conversation fail closed | PASS | Backend errors map to unavailable conversation toast; backend remains authorization source |
| Backend untouched | PASS | `git diff -- backend --stat` empty |
| AutoMigrate remains retired | PASS | zero matches in `backend/cmd/server` |

## Residue report

- Temporary worktree dependency junctions used for verification were removed:
  - `node_modules`
  - `apps/miniapp/node_modules`
- Ignored build outputs generated during verification were removed:
  - `apps/miniapp/.swc`
  - `apps/miniapp/dist`
- No backend server or dev server was left running by this task.
- No database migration, schema, or test data was created by this frontend task.

## Limitations

This acceptance is code/build/regression verification. A real Tencent Chat interactive device run should still be used by the owner for final manual UX confirmation because the task did not start a live backend, Tencent Chat session, or WeChat DevTools session.
