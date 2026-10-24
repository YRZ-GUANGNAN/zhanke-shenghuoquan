# 系统健康与管理后台状态加固 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复骑手/商家工作台的阶段手势边界、同步独立检查脚本与当前实现，并在管理后台清晰呈现支付与线下清账边界。

**Architecture:** 复用现有 `ChannelPager` 和 `useHorizontalNavigation`，新增只负责事件隔离与边缘转发的 `DeliveryStagePager`；脚本修复仅调整测试加载器/断言，不改变产品代码契约；管理端以展示层提示明确当前能力，不新增服务端 API 或支付动作。

**Tech Stack:** Taro React/TypeScript、SCSS、Node.js CommonJS 检查脚本、现有 Admin API 客户端、npm workspace。

---

### Task 1: 移植工作台阶段手势边界组件

**Files:**
- Create: `apps/miniapp/src/components/delivery-stage-pager.tsx`
- Modify: `apps/miniapp/src/pages/delivery-rider/index.tsx`
- Modify: `apps/miniapp/src/pages/merchant-home/index.tsx`
- Modify: `apps/miniapp/src/components/delivery-workspace-nav.tsx`（仅在当前实现需要 `step` 返回值时）
- Test: `scripts/test-delivery-stage-workbench.cjs`

- [ ] **Step 1: 对照主线现有 `ChannelPager` 与导航 hook，记录 props/返回类型。**

运行：

```powershell
rg -n "export type ChannelPagerProps|function useChannelInteraction|function useDeliveryWorkspaceNavigation|return \\{" apps/miniapp/src/components/channel-pager.tsx apps/miniapp/src/components/delivery-workspace-nav.tsx
```

预期：确认阶段 pager 仍接收 `keys/activeKey/onChange/renderPage/interaction`，且外层导航提供可复用的 `step` 或等价边缘切换函数。

- [ ] **Step 2: 写边界行为测试桩，覆盖内部阶段滑动、首尾边缘转发、隐藏取消和事件隔离。**

在 `scripts/test-delivery-stage-workbench.cjs` 的现有 harness 中加入断言：内部 `onChange` 不调用外层 `step`；首项向外和末项向外各调用一次；触摸取消/页面隐藏后不转发；所有触摸回调调用 `stopPropagation`。

- [ ] **Step 3: 实现 `DeliveryStagePager`。**

实现要点：

```tsx
export function createDeliveryStageBoundary() {
  const tracker = createHorizontalNavigation()
  let started: { signature: string; index: number } | null = null
  return {
    start(event, context) { /* reset + tracker.start */ },
    move(event) { tracker.move(event) },
    end(event, context) { /* 仅返回越过首尾的 -1/1，否则返回 0 */ },
    cancel() { started = null; tracker.cancel() },
  }
}
```

组件外层 `View` 的四个触摸事件必须停止冒泡；`ChannelPager` 仍负责阶段切换，只有 `end` 返回非零时调用 `onBoundarySwipe`。

- [ ] **Step 4: 接入骑手和商家工作台。**

将两处阶段 `ChannelPager` 替换为 `DeliveryStagePager`，保留原 `keys`、`activeKey`、`renderPage`、`interaction`、刷新/身份守卫和业务回调；`scopeKey` 包含角色与账号/能力状态，`onBoundarySwipe` 指向工作区导航的 `step`。

- [ ] **Step 5: 运行聚焦测试。**

运行：`node scripts/check-delivery-stage-workbench.cjs; npm run test:delivery-stage-workbench; npm run test:delivery-workspace`

预期：全部 exit 0，且原有订单、刷新和身份守卫断言仍通过。

### Task 2: 修复三个独立检查脚本

**Files:**
- Modify: `scripts/check-public-profile-gestures.cjs`
- Modify: `scripts/check-sheet-dismiss.cjs`
- Modify: `scripts/check-admin-identity-applications-contract.cjs`

- [ ] **Step 1: 修复公开主页测试桩。**

在脚本 Taro stub 中提供无副作用的 `useDidHide: () => {}`，不改变页面源码或手势断言。

- [ ] **Step 2: 修复 sheet 测试模块解析。**

在 `loaded.require` 中增加 `id === '@tarojs/taro'` 分支，返回测试所需的最小对象；保持对真实 `AppActionSheet`、拖拽阻尼、多指和 reduced-motion CSS 的断言。

- [ ] **Step 3: 将身份导航断言移到当前导航文件。**

读取 `apps/admin/src/admin-navigation.ts` 并在该文件上断言 `identities`；继续在 `App.tsx` 断言 `<IdentityApplications />`，不要求旧的 `key` 仍写在 `App.tsx`。

- [ ] **Step 4: 运行三个脚本。**

运行：

```powershell
node scripts/check-public-profile-gestures.cjs
node scripts/check-sheet-dismiss.cjs
node scripts/check-admin-identity-applications-contract.cjs
```

预期：三个命令均 exit 0。

### Task 3: 管理后台支付/钱包状态提示

**Files:**
- Modify: `apps/admin/src/DeliveryConsole.tsx`
- Modify: `apps/admin/src/styles.css`

- [ ] **Step 1: 增加纯展示的能力说明组件。**

在配送后台页签或相关面板顶部显示：`在线支付能力由服务端配置控制；当前骑手报酬操作仅登记线下清账，不会发起真实打款。历史订单、钱包余额和流水仍可查询。` 不根据 `payment === null` 猜测关闭状态，不新增请求。

- [ ] **Step 2: 强化清账操作的可见边界。**

保留 `payOffline`、Row Version、幂等键和现有确认弹窗；在 `EarningsPanel` 的说明区明确这是管理员登记线下支付，成功文案不使用“自动打款/已转账”。

- [ ] **Step 3: 添加窄屏可读样式。**

新增 `.delivery-capability-notice` 及 warning 变体，使用现有色彩 token/样式变量，允许长文案换行，不改变表格和分页结构。

- [ ] **Step 4: 运行 Admin typecheck。**

运行：`npm --workspace apps/admin run typecheck`

预期：exit 0，无 API 类型变化。

### Task 4: 全量验证与提交

**Files:**
- Verify only; no new product files beyond Tasks 1–3 and the approved design/plan docs.

- [ ] **Step 1: 运行完整验收。**

```powershell
npm run verify
npm run build
node scripts/check-public-profile-gestures.cjs
node scripts/check-sheet-dismiss.cjs
node scripts/check-admin-identity-applications-contract.cjs
git diff --check
```

允许既有构建体积/顺序警告；若出现 migration 047 或共享 MySQL 相关问题，不执行迁移，记录为 PENDING。

- [ ] **Step 2: 检查改动范围。**

运行 `git status --short` 与 `git diff --stat`，确认没有 `.env`、共享数据库、外部订单分支或无关页面改动；保留用户已有的三个未跟踪计划文件。

- [ ] **Step 3: 提交。**

```powershell
git add apps/miniapp/src/components/delivery-stage-pager.tsx apps/miniapp/src/pages/delivery-rider/index.tsx apps/miniapp/src/pages/merchant-home/index.tsx apps/admin/src/DeliveryConsole.tsx apps/admin/src/styles.css scripts/check-public-profile-gestures.cjs scripts/check-sheet-dismiss.cjs scripts/check-admin-identity-applications-contract.cjs scripts/test-delivery-stage-workbench.cjs docs/superpowers/specs/2026-10-24-system-health-hardening-design.md docs/superpowers/plans/2026-10-24-system-health-hardening.md
git commit -m "加固：修复工作台手势边界并明确后台支付状态"
```

预期：生成一个中文提交，且不 push、不部署。
