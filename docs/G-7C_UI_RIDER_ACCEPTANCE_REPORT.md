# G-7c UI Rider Acceptance Report

Date: 2026-06-25  
Branch: `feat/g7c-ui-rider`  
Final commit: see `git log --oneline -1` and the owner handoff message

## Conclusion

Status: **CONDITIONAL PASS**

Reason:

- frontend implementation is complete
- package manifests are unchanged
- boundary checks pass
- MiniApp typecheck passes
- MiniApp build passes
- backend full regression passes
- real WeChat Developer Tools runtime verification remains owner-side evidence to collect

This module is ready for owner manual verification and fast-forward review. It does not enter G-8/P0-5 or any new module.

## Acceptance matrix

| Item | Result | Evidence |
| --- | --- | --- |
| Rider service client added | PASS | `apps/miniapp/src/services/g-takeout-rider.ts` |
| Profile rider entry added | PASS | Profile page adds `Rider Center` row |
| BottomNav unchanged | PASS | No edit to `apps/miniapp/src/components.tsx` |
| Rider page registered | PASS | `pages/takeout-rider/index` added to `app.config.ts` |
| Rider registration UI | PASS | registration form for `realName` and `phone` |
| Claim pool UI | PASS | lists claimable orders and sends order `rowVersion` |
| Claim conflict handling | PASS | 409/concurrent errors map to friendly refresh prompt |
| Fulfillment lifecycle UI | PASS | supports `to-store`, `picked`, `to-user`, `complete` |
| Wallet UI | PASS | displays balances, earnings, withdrawal records |
| Withdrawal validation UX | PASS | invalid amount blocked; backend balance errors shown as friendly text |
| No backend changes | PASS | `backend/**` untouched |
| No migration changes | PASS | migration files untouched |
| No dependency changes | PASS | package diff empty |
| Boundary check | PASS | `npm run verify` |
| MiniApp typecheck | PASS | `npm --workspace apps/miniapp run typecheck` |
| MiniApp build | PASS | `npm run build:miniapp` |
| Backend regression | PASS | `go test ./... -count=1` |
| Real DevTools runtime | PENDING | requires owner manual run against live local backend |

## Security and boundary notes

- all role operations use the existing token-injecting `request()` helper
- claim and fulfillment writes include `expectedRowVersion`
- owner/isolation enforcement remains in the frozen G-7c backend
- no frontend-side trust is added for wallet ownership or fulfillment ownership
- no new storage key or auth path introduced

## Runtime checklist for owner

Use an existing valid local token and real backend.

1. Open MiniApp Profile page.
2. Tap `Rider Center`.
3. If not registered, submit real name and phone.
4. Prepare or locate an order in `ready_for_pickup`.
5. Open Claim Pool and claim the order.
6. Verify a competing stale claim is shown as "订单已被抢或状态已变化".
7. Open My Deliveries and advance:
   - 到店取餐
   - 已取餐
   - 开始送达
   - 送达完成
8. Open Wallet and verify income/balance changed.
9. Submit withdrawal:
   - valid amount succeeds
   - invalid amount shows local validation
   - over-balance amount is rejected by backend and shown as a friendly message

## Residue report

- No committed build output.
- No committed node_modules.
- No backend/test database writes performed by this frontend-only implementation.
- Runtime test data cleanup is not applicable because real DevTools runtime verification was not executed in this pass.
