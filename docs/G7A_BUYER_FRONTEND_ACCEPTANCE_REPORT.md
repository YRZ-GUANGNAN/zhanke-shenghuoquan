# G-7a Buyer MiniApp Frontend Acceptance Report

## Conclusion

Status: CONDITIONAL PASS

Condition: owner must perform WeChat Developer Tools / real miniapp visual acceptance to confirm a real on-shelf dish image renders through resolve URL into `<Image src>`.

Branch: `feat/g7a-ui-buyer`

Baseline: `a2af8ba6 docs(G-7a): record media resolve URL acceptance evidence`

Implementation commit: `acc6d46`

Final branch HEAD: provided in the handoff output after the docs metadata commit.

## Acceptance matrix

| Item | Result | Evidence |
| --- | --- | --- |
| Preflight branch/tag/worktree | PASS | master at `a2af8ba6`, tags present, clean before development |
| Buyer service client | PASS | `apps/miniapp/src/services/g-takeout-buyer.ts` |
| Media resolve integration | PASS | `ResolvedMediaImage` converts display coordinates to resolve endpoint and caches signed URL |
| JWT not placed in image URL | PASS | resolve request uses existing `request()` header injection; `<Image>` only receives resolved COS signed URL |
| BottomNav adds takeout tab | PASS | `TabKey` and `BottomNav` updated; home remains first registered page |
| Merchant browsing page | PASS | `pages/takeout/index.tsx` |
| Dish list and add-to-cart | PASS | `pages/takeout/index.tsx` + `stores/cart.ts` |
| Single-merchant cart | PASS | cart store resets items when a dish from another merchant is added |
| Checkout creates order | PASS | `pages/takeout-cart/index.tsx` posts to `/takeout/buyer/orders` |
| Pending payment UX | PASS | order detail explicitly shows payment feature pending |
| Order list/detail | PASS | `pages/takeout-orders` and `pages/takeout-order-detail` |
| No package dependency change | PASS | no `package.json` diff |
| Miniapp typecheck | PASS | `npm --workspace apps/miniapp run typecheck` |
| Miniapp build | PASS | `npm run build:miniapp` with existing vendors size warning |
| Full repo verify | PASS | `npm run verify` |
| Real visual image rendering | PENDING | Requires owner-run WeChat Developer Tools / real miniapp runtime |

## Commands executed

```text
npm --workspace apps/miniapp run typecheck
PASS
```

```text
npm run build:miniapp
PASS with existing vendors.js size warning
```

```text
npm run verify
PASS
```

```text
git diff --check
PASS
```

## Manual acceptance path

Use real backend + real MySQL + real COS + WeChat Developer Tools:

1. Ensure a merchant is `active`.
2. Ensure at least one dish is `on_shelf` and has `imageContentUrl`.
3. Open miniapp `外卖` tab.
4. Select merchant.
5. Confirm dish card image is visibly rendered.
6. Confirm network panel shows one resolve call for the coordinate and that repeat render uses cache within TTL.
7. Add dish to cart.
8. Submit checkout.
9. Confirm created order shows `pending_payment` detail and cart is cleared.

## Final status rationale

The implementation is complete and automated verification passes. It is marked `CONDITIONAL PASS` only because the final visual proof must come from a real miniapp runtime, not from TypeScript/build output.
