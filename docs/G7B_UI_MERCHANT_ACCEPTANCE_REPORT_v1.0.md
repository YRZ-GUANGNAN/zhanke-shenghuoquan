# G-7b MiniApp Merchant Pages Acceptance Report v1.0

Date: 2026-06-23
Branch: `feat/g7b-ui-merchant`
Baseline: `master@6b304f4e`
Final commit: the commit containing this report

## 1. Conclusion

Status: CONDITIONAL PASS

Code-level acceptance is PASS:

- MiniApp typecheck PASS
- MiniApp build PASS
- root `npm run verify` PASS
- no backend/package/admin diff
- media binding fields are now sent as real asset public ids instead of empty arrays/empty values

Manual/real-device acceptance remains pending because this run did not operate WeChat DevTools or a real MiniApp session through the full form flow. No mock backend was used for automated checks; the limitation is only the interactive UI proof.

## 2. Acceptance matrix

| Area | Result | Evidence |
| --- | --- | --- |
| Merchant entry from Profile | PASS | Profile adds a single settings-row navigation to `/pages/takeout-merchant/index`; BottomNav unchanged. |
| Onboarding form | PASS | Page submits `POST /takeout/merchant/onboarding` with `logoAssetPublicId` from uploaded `UploadedMedia.assetId`. |
| Logo media binding | PASS (code) | `chooseAndUploadImages('merchant_logo', 1)` returns `assetId`; request uses backend field `logoAssetPublicId`. Backend publish is automatic after approval. |
| Category management | PASS | Uses `GET/POST/PATCH /takeout/merchant/categories` with row-version update. |
| Dish management | PASS | Uses `GET/POST/PATCH /takeout/merchant/dishes` with `imageAssetPublicId`, CAS fields, and status actions. |
| Dish image binding | PASS (code) | `chooseAndUploadImages('dish', 1)` returns `assetId`; create/update sends `imageAssetPublicId`. Backend publish is automatic on shelf. |
| Order board | PASS | Uses merchant order list/detail and accept/start-preparing/mark-ready endpoints with row version. |
| Role split | PASS | `GET /takeout/merchant/me` gates not-onboarded, pending, active, rejected, suspended states. |
| New dependencies | PASS | No package manifest diff. |
| Backend untouched | PASS | No `backend/**` diff. |

## 3. Verification commands

```text
npm --workspace apps/miniapp run typecheck
```

Result: PASS

```text
npm run build:miniapp
```

Result: PASS with existing `vendors.js` size warning.

```text
npm run verify
```

Result: PASS

Details:

- `check:boundaries`: `ok A-G boundaries plus isolated P0-4 media domain`
- admin typecheck: PASS
- miniapp typecheck: PASS
- backend tests: PASS

## 4. Runtime verification status

Not executed in this turn:

- WeChat DevTools/manual merchant onboarding flow;
- real logo upload through the page;
- admin approval followed by merchant active UI refresh;
- real dish image upload through the page;
- real order state operation from the page.

Reason: this step requires an interactive authenticated MiniApp session and real operator actions. The code is wired to the real API client and does not introduce mock services.

Recommended manual verification after owner opens DevTools:

1. Login with the same AppID/backend environment.
2. Profile -> Merchant Center.
3. Submit onboarding with logo.
4. Approve merchant from admin path.
5. Re-enter Merchant Center and verify active state.
6. Create category and dish with image.
7. Put dish on shelf and confirm backend Gateway visibility.
8. Operate paid order through accept -> preparing -> ready.

## 5. Residue report

No test data was created by this automated run. `npm ci` installed local ignored `node_modules` in the worktree; no package manifest changed.

