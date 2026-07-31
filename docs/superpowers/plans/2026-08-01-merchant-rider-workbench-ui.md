# Merchant and Rider Workbench UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade the merchant and rider surfaces into a coherent, production-grade role workbench without changing business contracts or inventing data.

**Architecture:** Preserve the current Taro pages, hooks, services, route guards, and state machines. Add a static UI contract for high-risk structure, then reshape the two role home pages through existing JSX boundaries and Sass tokens; converge satellite pages with the already shipped shared UI primitives.

**Tech Stack:** Taro 4, React, TypeScript, Sass, Node.js contract checks, existing `AppButton`/`AppState`/`AppNotice`/`CapsuleSystemRow` components.

---

### Task 1: Lock the role-workbench structure

**Files:**
- Create: `scripts/check-role-workbench-ui.cjs`
- Modify: `package.json`

- [ ] **Step 1: Write the failing contract check**

```js
const fs = require('node:fs')
const read = (path) => fs.readFileSync(path, 'utf8')
const merchant = read('apps/miniapp/src/pages/merchant-home/index.tsx')
const rider = read('apps/miniapp/src/pages/delivery-rider/index.tsx')
for (const fragment of ['CapsuleSystemRow', 'merchant-home-metrics', 'merchant-home-tabs', 'merchant-order-card']) {
  if (!merchant.includes(fragment)) throw new Error(`merchant workbench missing: ${fragment}`)
}
for (const fragment of ['CapsuleSystemRow', 'rider-grab-task-summary', 'rider-grab-tabs', 'rider-order-card']) {
  if (!rider.includes(fragment)) throw new Error(`rider workbench missing: ${fragment}`)
}
```

- [ ] **Step 2: Run the new check before wiring it**

Run: `node scripts/check-role-workbench-ui.cjs`
Expected: PASS for existing structural anchors.

- [ ] **Step 3: Add visual safety assertions and wire the script**

Require both pages to retain identity guards and capsule safe rows, require role states and order cards, reject `window.alert`, placeholder records, and local mock arrays. Add `check:role-workbench-ui` to root scripts and invoke it from `verify`.

- [ ] **Step 4: Run the wired contract**

Run: `npm run check:role-workbench-ui`
Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add -- package.json scripts/check-role-workbench-ui.cjs
git commit -m "测试：锁定商家与骑手工作台边界"
```

### Task 2: Reshape the merchant home hierarchy

**Files:**
- Modify: `apps/miniapp/src/pages/merchant-home/index.tsx`
- Modify: `apps/miniapp/src/pages/merchant-home/index.scss`

- [ ] **Step 1: Add explicit workbench landmarks**

Keep the existing data expressions and handlers. Add section labels and accessibility copy around the shortcut and order regions; retain `IdentitySwitchModal`, `listMerchantDeliveryOrders`, `getDeliveryWallet`, and `getIdentityApplication` unchanged.

- [ ] **Step 2: Replace the heavy hero treatment**

Use a neutral page canvas, an independent capsule row, a compact white identity header, a three-column metric surface, a four-item shortcut grid, and sticky segmented tabs. The only filled brand action is “发布配送单”.

- [ ] **Step 3: Rebuild the merchant order-card rhythm**

Order layout must stay: state/time, rider if assigned, pickup/drop-off route, optional remark, fee and detail action. Use 24rpx outer radius, weak dividers, tabular amounts, and semantic status colors.

- [ ] **Step 4: Validate the merchant page**

Run: `npm run check:role-workbench-ui && npm --workspace apps/miniapp run typecheck`
Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add -- apps/miniapp/src/pages/merchant-home/index.tsx apps/miniapp/src/pages/merchant-home/index.scss
git commit -m "界面：重塑商家工作台信息层级"
```

### Task 3: Reshape the rider home hierarchy

**Files:**
- Modify: `apps/miniapp/src/pages/delivery-rider/index.tsx`
- Modify: `apps/miniapp/src/pages/delivery-rider/index.scss`

- [ ] **Step 1: Preserve rider state and action semantics**

Keep `listClaimableDeliveryOrders`, assigned-order loading, claim/status/cancel handlers, contact routes, approval checks, and busy locks unchanged. Do not turn secondary contact actions into primary actions.

- [ ] **Step 2: Create the compact rider command header**

Move from a saturated full-width hero to a neutral identity header and one concise metric panel. Current tasks are the dominant number; claimable and completed counts are secondary; refresh is an icon action with an explicit label.

- [ ] **Step 3: Rebuild order states**

Use one card grammar for claimable and assigned orders. Keep privacy copy before claiming, expose full addresses only after assignment, and make the next valid state transition the sole filled button.

- [ ] **Step 4: Normalize loading, error, access, and empty states**

Align the existing state classes with `AppState` proportions: compact icon, direct title, one explanatory line, one retry or return action.

- [ ] **Step 5: Validate and commit**

Run: `npm run check:role-workbench-ui && npm --workspace apps/miniapp run typecheck`
Expected: PASS.

```powershell
git add -- apps/miniapp/src/pages/delivery-rider/index.tsx apps/miniapp/src/pages/delivery-rider/index.scss
git commit -m "界面：重塑骑手工作台与订单状态"
```

### Task 4: Converge merchant and rider satellite pages

**Files:**
- Modify: `apps/miniapp/src/pages/merchant-publish/index.scss`
- Modify: `apps/miniapp/src/pages/merchant-order-detail/index.scss`
- Modify: `apps/miniapp/src/pages/merchant-messages/index.scss`
- Modify: `apps/miniapp/src/pages/merchant-wallet/index.scss`
- Modify: `apps/miniapp/src/pages/rider-wallet/index.scss`
- Modify: `apps/miniapp/src/pages/delivery-conversation/index.scss`

- [ ] **Step 1: Normalize page shells and headers**

Apply the design-token canvas, capsule-safe header proportions, 24rpx section gaps, and 24–28rpx outer surfaces without changing JSX routes.

- [ ] **Step 2: Normalize forms and bottom actions**

Use 88rpx minimum action height, visible disabled states, `env(safe-area-inset-bottom)` padding, and input focus states. Preserve current payment feature flags and submission locks.

- [ ] **Step 3: Normalize message, ledger, timeline, and empty rows**

Use 96–112rpx row rhythm, tabular values, one-pixel dividers, consistent status colors, and honest empty/error copy already supplied by the pages.

- [ ] **Step 4: Validate and commit**

Run: `npm run check:miniapp-ui && npm run check:role-workbench-ui && npm --workspace apps/miniapp run typecheck`
Expected: PASS.

```powershell
git add -- apps/miniapp/src/pages/merchant-publish/index.scss apps/miniapp/src/pages/merchant-order-detail/index.scss apps/miniapp/src/pages/merchant-messages/index.scss apps/miniapp/src/pages/merchant-wallet/index.scss apps/miniapp/src/pages/rider-wallet/index.scss apps/miniapp/src/pages/delivery-conversation/index.scss
git commit -m "界面：统一角色端二级页面细节"
```

### Task 5: Full regression and production build

**Files:**
- Verify only; do not upload or modify production configuration.

- [ ] **Step 1: Check formatting and file scope**

Run: `git diff --check && git status --short`
Expected: no whitespace errors; only explicitly preserved user plan files remain untracked.

- [ ] **Step 2: Run the complete repository gate**

Run: `npm run verify`
Expected: all checks pass. Existing `vendors.js` size warning is non-blocking.

- [ ] **Step 3: Produce production bundles**

Run: `npm run build`
Expected: miniapp and admin builds succeed; no WeChat upload is performed.

- [ ] **Step 4: Record delivery state**

Run: `git log -8 --oneline && git status --short`
Expected: implementation commits are visible and no tracked change remains.
