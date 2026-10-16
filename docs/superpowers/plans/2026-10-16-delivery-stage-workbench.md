# 配送阶段化工作台 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改变配送接口、身份守卫和业务状态机的前提下，优化骑手工作台、商家工作台和配送详情页的阶段信息、按订单操作反馈与真实数据展示。

**Architecture:** 保留现有页面与 service 层边界。骑手页只增加展示层的订单级忙碌状态和安全刷新包装；商家页只修正加载期数量与错误展示；详情页复用已有图标、状态时间戳和私有凭证服务。所有操作仍调用现有方法和 rowVersion/CAS。

**Tech Stack:** Taro + React + TypeScript + SCSS；Node.js 静态契约检查；npm workspace 构建。

---

## 文件变更总览

- Modify: `apps/miniapp/src/pages/delivery-rider/index.tsx` — 订单级操作状态、动作结果与刷新结果分层、骑手身份重新同步。
- Modify: `apps/miniapp/src/pages/delivery-rider/index.scss` — 忙碌卡片、凭证阶段和失败重试的视觉反馈。
- Modify: `apps/miniapp/src/pages/merchant-home/index.tsx` — 去除初始虚假数量，加载期间显示真实加载语义。
- Modify: `apps/miniapp/src/pages/merchant-home/index.scss` — 数量加载态与订单局部失败样式。
- Modify: `apps/miniapp/src/pages/merchant-order-detail/index.tsx` — 详情页联系骑手入口复用既有线性图标，保持私聊路由不变。
- Modify: `apps/miniapp/src/pages/merchant-order-detail/index.scss` — 图标按钮触控区与状态层级。
- Modify: `scripts/check-rider-workbench-clarity.cjs` — 增加订单级锁定、凭证反馈和安全刷新契约断言。
- Modify: `scripts/check-merchant-home-clarity.cjs` — 增加真实数量/加载态断言。
- Create: `scripts/check-delivery-stage-workbench.cjs` — 组合检查配送阶段化行为的关键字符串和接口边界。

## Task 1: 骑手页改为订单级操作状态

**Files:**
- Modify: `apps/miniapp/src/pages/delivery-rider/index.tsx:85-305,475-656`

- [ ] **Step 1: 先固定当前契约基线**

运行：

```powershell
node scripts/check-rider-workbench-clarity.cjs
node scripts/check-role-workbench-ui.cjs
```

预期：两个脚本均输出 `PASS` 或 `ok`，确认修改前的页面契约成立。

- [ ] **Step 2: 用订单 ID 记录忙碌状态**

将单一状态：

```ts
const [busyOrderId, setBusyOrderId] = useState('')
```

替换为：

```ts
type BusyAction = 'claim' | 'command' | 'proof' | 'release'
type BusyByOrder = Record<string, BusyAction | undefined>

const [busyByOrder, setBusyByOrder] = useState<BusyByOrder>({})

const markBusy = (orderId: string, action?: BusyAction) => {
  setBusyByOrder((current) => {
    const next = { ...current }
    if (action) next[orderId] = action
    else delete next[orderId]
    return next
  })
}

const isOrderBusy = (orderId: string): boolean => Boolean(busyByOrder[orderId])
```

所有 `claim`、`runCommand`、`runProofCommand`、`release` 入口只检查 `isOrderBusy(order.deliveryOrderPublicId)`，并在 `finally` 中调用 `markBusy(orderId)` 清理。

- [ ] **Step 3: 让卡片只锁定当前订单**

抢单卡、已分配卡和释放按钮使用当前订单的 `busy` 值：

```tsx
const busy = isOrderBusy(order.deliveryOrderPublicId)
<View className={`rider-order-card ${busy ? 'rider-order-card--busy' : ''}`}>
  <View className={`rider-order-card__grab ${busy ? 'rider-order-card__grab--disabled' : ''}`}>
    <Text>{busy ? '处理中…' : '抢单'}</Text>
  </View>
</View>
```

删除所有基于全局 `busyOrderId` 的 `disabled={!!busyOrderId}` 和 `if (!busyOrderId)` 条件，避免操作一单时锁死其他订单。

- [ ] **Step 4: 保持释放确认弹窗的当前订单语义**

`AppDialog.loading` 改为：

```tsx
loading={releaseCandidate ? isOrderBusy(releaseCandidate.deliveryOrderPublicId) : false}
```

释放确认只针对 `releaseCandidate`，不影响列表其他订单。

- [ ] **Step 5: 运行类型检查与骑手契约检查**

运行：

```powershell
node scripts/check-rider-workbench-clarity.cjs
npm --workspace apps/miniapp run typecheck
```

预期：检查通过，且 `delivery-rider.ts` 的 URL、HTTP 方法、字段名没有 diff。

- [ ] **Step 6: 提交任务 1**

```powershell
git add apps/miniapp/src/pages/delivery-rider/index.tsx
git commit -m "优化骑手配送订单级操作状态"
```

## Task 2: 骑手动作结果与凭证状态反馈

**Files:**
- Modify: `apps/miniapp/src/pages/delivery-rider/index.tsx:100-305,620-656`
- Modify: `apps/miniapp/src/pages/delivery-rider/index.scss:343-640,725-750`

- [ ] **Step 1: 为列表刷新增加安全包装**

在 `refreshOrders` 之后新增：

```ts
const refreshAfterAction = async (): Promise<boolean> => {
  try {
    await refreshOrders()
    return true
  } catch (value) {
    setError(errorMessage(value, '操作已提交，列表刷新失败，点击重试'))
    return false
  }
}
```

`claim`、`runCommand`、`runProofCommand`、`release` 的成功路径先显示动作成功提示，再调用 `refreshAfterAction`；刷新失败不改写动作成功结果，只保留可点击重试状态。

- [ ] **Step 2: 让动作成功和刷新失败分层**

以 `runCommand` 为例，保留现有 command 参数并改成：

```ts
await command(order)
await Taro.showToast({ title: success, icon: 'success' })
await refreshAfterAction()
```

catch 仍使用后端错误文案并刷新一次；finally 只清理当前订单忙碌状态。

- [ ] **Step 3: 明确凭证上传阶段**

在 `runProofCommand` 中按顺序设置当前订单 action，并将 loading 文案固定为：

```ts
markBusy(order.deliveryOrderPublicId, 'proof')
// 选择照片：沿用 chooseAndUploadImages
// 上传/审核：沿用 waitForMediaApproval
// 提交：沿用 markPickedUp 或 markDelivered
```

失败文案区分为“凭证提交失败，请重试”，不新增审核接口，不把 `uploaded.assetId` 当作履约成功。

- [ ] **Step 4: 增加忙碌卡片和重试样式**

在 `index.scss` 增加：

```scss
.rider-order-card--busy {
  .rider-order-card__secondary,
  .rider-order-card__grab {
    opacity: .58;
  }
}

.rider-grab-error {
  min-height: 72rpx;
  display: flex;
  align-items: center;
  padding: 0 24rpx;
  border: 1rpx solid $rider-warning-soft;
  border-radius: 16rpx;
  background: $rider-warning-soft;
  color: $rider-warning;
}
```

继续保留现有刷新旋转、空态和加载更多样式，不改变页面高度或 ScrollView padding 契约。

- [ ] **Step 5: 同步骑手身份状态**

`refreshOrders` 成功后补充一次已有 `getDeliveryRiderMe()` 结果更新 `me`；不增加新请求路径。若身份请求失败，保留列表结果并显示已有错误条。

- [ ] **Step 6: 运行配送检查**

运行：

```powershell
node scripts/check-rider-workbench-clarity.cjs
node scripts/check-delivery-order-clipboard.cjs
npm --workspace apps/miniapp run typecheck
```

预期：所有检查通过，且凭证服务调用仍为 `chooseAndUploadImages('delivery_proof', ...)` 与 `waitForMediaApproval`。

- [ ] **Step 7: 提交任务 2**

```powershell
git add apps/miniapp/src/pages/delivery-rider/index.tsx apps/miniapp/src/pages/delivery-rider/index.scss
git commit -m "完善骑手配送动作与凭证反馈"
```

## Task 3: 商家工作台只展示真实数量和状态

**Files:**
- Modify: `apps/miniapp/src/pages/merchant-home/index.tsx:34-55,180-450`
- Modify: `apps/miniapp/src/pages/merchant-home/index.scss:206-447` — 统计数字加载态、订单标签和列表错误提示样式。

- [ ] **Step 1: 删除加载期虚假数量**

将：

```ts
function initialNumbersByTab(): MerchantNumbersByTab {
  return { pending: 1, delivering: 1, completed: 1 }
}
```

改为：

```ts
function initialNumbersByTab(): MerchantNumbersByTab {
  return { pending: 0, delivering: 0, completed: 0 }
}
```

数量渲染时使用已有 `loading`：初次加载且该列表为空时显示 `—`，请求完成后显示真实 total。

- [ ] **Step 2: 保留已有列表数据并分离局部失败**

刷新失败时不清空 `ordersByTab`，继续显示已加载订单；错误区域使用“配送单刷新失败，点击重试”，点击后调用现有 `loadWorkbench`。不创建新的错误接口或模拟订单。

- [ ] **Step 3: 保持真实状态与金额来源**

继续使用 `statusCopy`、`ordersTotalByTab`、`deliveryFeeCents` 和 `updatedAt`；不新增“营业中”、距离、预计时间或收入文案。只调整信息层级和加载态样式。

- [ ] **Step 4: 增加商家工作台契约检查**

在 `scripts/check-merchant-home-clarity.cjs` 增加断言：源码不再匹配 `return { pending: 1, delivering: 1, completed: 1 }`，并匹配 `loading` 与真实 totals 的组合展示。

- [ ] **Step 5: 运行检查并提交任务 3**

```powershell
node scripts/check-merchant-home-clarity.cjs
node scripts/check-role-workbench-ui.cjs
npm --workspace apps/miniapp run typecheck
git add apps/miniapp/src/pages/merchant-home/index.tsx apps/miniapp/src/pages/merchant-home/index.scss scripts/check-merchant-home-clarity.cjs
git commit -m "修正商家配送工作台真实数量展示"
```

## Task 4: 配送详情页图标和状态层级

**Files:**
- Modify: `apps/miniapp/src/pages/merchant-order-detail/index.tsx:1-6,177-192`
- Modify: `apps/miniapp/src/pages/merchant-order-detail/index.scss:154-218`

- [ ] **Step 1: 复用已有消息图标**

从 `pages/merchant-home/merchant-home-icon.tsx` 复用 `message` 图标组件，保留 `openConversation` 和现有私聊路由：

```tsx
<View className='merchant-detail-rider-card__circle' onClick={openConversation}>
  <MerchantHomeIcon name='message' className='merchant-detail-rider-card__message-icon' />
</View>
```

删除按钮内的纯文本“聊”，不修改 `delivery-conversation` 页面。

- [ ] **Step 2: 保持真实时间线与凭证状态**

只使用当前 `order` 的阶段时间戳和 `loadPrivateDeliveryProofs` 返回值；继续区分 `binding`、`active`、`deadletter`、`revoked`，不增加静态异常或地图卡片。

- [ ] **Step 3: 统一触控区和状态样式**

为消息图标增加不小于现有 44PX 触控区、按压态和无骑手时的 disabled 样式；不改变导航返回和刷新按钮。

- [ ] **Step 4: 运行详情相关检查并提交任务 4**

```powershell
node scripts/check-merchant-home-clarity.cjs
npm --workspace apps/miniapp run typecheck
git add apps/miniapp/src/pages/merchant-order-detail/index.tsx apps/miniapp/src/pages/merchant-order-detail/index.scss
git commit -m "统一配送详情联系骑手入口"
```

## Task 5: 聚合检查、构建与最终审计

**Files:**
- Create: `scripts/check-delivery-stage-workbench.cjs`
- Modify: `package.json` — 增加 `check:delivery-stage-workbench`，并纳入 `verify`。

- [ ] **Step 1: 编写聚合静态检查**

检查以下内容：

```js
assert.match(riderSource, /busyByOrder/)
assert.match(riderSource, /refreshAfterAction/)
assert.doesNotMatch(riderSource, /disabled=\{!!busyOrderId\}/)
assert.doesNotMatch(merchantHomeSource, /pending:\s*1,\s*delivering:\s*1,\s*completed:\s*1/)
assert.match(detailSource, /MerchantHomeIcon.*message|name='message'/)
assert.match(riderServiceSource, /\/delivery\/rider\/orders/)
assert.match(merchantServiceSource, /\/delivery\/merchant\/orders/)
```

脚本只检查源码契约，不连接后端、不伪造登录、不写数据库。

- [ ] **Step 2: 将检查接入 package scripts**

在 `package.json` 增加：

```json
"check:delivery-stage-workbench": "node scripts/check-delivery-stage-workbench.cjs"
```

并将该命令加入现有 `verify` 链，不移除任何已有检查。

- [ ] **Step 3: 执行完整验证**

```powershell
node scripts/check-delivery-stage-workbench.cjs
node scripts/check-rider-workbench-clarity.cjs
node scripts/check-role-workbench-ui.cjs
node scripts/check-merchant-home-clarity.cjs
node scripts/check-miniapp-business-closure.cjs
node scripts/check-feature-policy.cjs
npm run typecheck
npm run build:miniapp
git diff --check
```

预期：全部通过；构建产物生成成功；`git diff --check` 无空白错误；`git diff -- apps/miniapp/src/services apps/miniapp/src/hooks` 为空，证明接口和守卫未被改动。

- [ ] **Step 4: 检查最终工作区并提交**

```powershell
git status --short --branch
git diff --stat HEAD~5..HEAD
git log -5 --oneline
```

只提交源码、检查脚本、设计说明和实施计划；不加入已有 `artifacts/` 历史产物，也不自动上传 GitHub 或微信。

