# Rider Workbench Deep Refinement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在保留真实骑手身份、真实订单状态流转和旧版业务骨架的前提下，深度精修骑手工作台的头部层级、状态导航、图标、订单卡片和加载/空白/错误状态。

**Architecture:** 继续由 `delivery-rider/index.tsx` 负责骑手数据加载和订单操作，只调整表现层结构；新增页面内聚的 `RiderWorkbenchIcon` 映射组件和四个本地 SVG 资源。新增静态契约锁定胶囊安全行、真实数据调用、图标资源和禁止字符图标等边界，避免后续视觉回退。

**Tech Stack:** React 18、Taro 4、TypeScript、SCSS/WXSS、Node.js 静态契约脚本、微信小程序构建。

---

## 文件结构

### 新建文件

- `apps/miniapp/src/pages/delivery-rider/rider-workbench-icon.tsx`
  - 页面专用图标映射组件，只负责把语义名称映射为本地 SVG。
- `apps/miniapp/src/pages/delivery-rider/icons/action-refresh.svg`
  - 刷新操作图标。
- `apps/miniapp/src/pages/delivery-rider/icons/action-message.svg`
  - 订单消息操作图标。
- `apps/miniapp/src/pages/delivery-rider/icons/action-phone.svg`
  - 订单电话操作图标。
- `apps/miniapp/src/pages/delivery-rider/icons/empty-route.svg`
  - 加载、权限提示和空订单状态使用的配送路线图标。
- `scripts/check-rider-workbench-clarity.cjs`
  - 骑手页视觉与真实数据边界静态契约。

### 修改文件

- `apps/miniapp/src/pages/delivery-rider/index.tsx`
  - 保留现有数据和命令函数，重排头部、刷新、状态导航、空状态和订单次要操作。
- `apps/miniapp/src/pages/delivery-rider/index.scss`
  - 实现深度精修后的尺寸、色彩、间距、卡片、图标与响应式规则。
- `scripts/check-miniapp-business-closure.cjs`
  - 将新的骑手页结构和图标组件纳入真实业务闭环检查。
- `package.json`
  - 注册 `check:rider-workbench-clarity`，并加入根级 `verify`。

### 明确不修改

- `apps/miniapp/src/services/delivery-rider.ts`
- `apps/miniapp/src/hooks/use-identity-guard.ts`
- 后端、数据库、支付开关、身份审核和订单状态机文件。

---

### Task 1: 建立会失败的骑手页清晰度契约

**Files:**
- Create: `scripts/check-rider-workbench-clarity.cjs`
- Modify: `package.json`
- Test: `scripts/check-rider-workbench-clarity.cjs`

- [ ] **Step 1: 新增静态契约脚本**

创建 `scripts/check-rider-workbench-clarity.cjs`，完整内容如下：

```js
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const pageRoot = path.join(root, 'apps', 'miniapp', 'src', 'pages', 'delivery-rider')
const source = fs.readFileSync(path.join(pageRoot, 'index.tsx'), 'utf8')
const style = fs.readFileSync(path.join(pageRoot, 'index.scss'), 'utf8')
const iconComponentPath = path.join(pageRoot, 'rider-workbench-icon.tsx')
const icons = [
  ['action-refresh', 'refreshIcon'],
  ['action-message', 'messageIcon'],
  ['action-phone', 'phoneIcon'],
  ['empty-route', 'emptyIcon'],
]

assert.ok(fs.existsSync(iconComponentPath), 'rider workbench icon component is required')
const iconComponent = fs.readFileSync(iconComponentPath, 'utf8')

for (const [icon, binding] of icons) {
  const file = path.join(pageRoot, 'icons', `${icon}.svg`)
  assert.ok(fs.existsSync(file), `${icon}.svg is required`)
  const svg = fs.readFileSync(file, 'utf8')
  assert.match(svg, /viewBox="0 0 48 48"/, `${icon}.svg must use the shared viewBox`)
  assert.match(iconComponent, new RegExp(`\\b${binding}\\b`), `${icon}.svg must be registered`)
}

assert.match(source, /RiderWorkbenchIcon/)
assert.match(source, /name='refresh'/)
assert.match(source, /name='message'/)
assert.match(source, /name='phone'/)
assert.match(source, /name='empty'/)
assert.match(source, /className='rider-grab-task-summary'/)
assert.match(source, /rider-grab-refresh/)
assert.match(source, /className='rider-empty-state'/)
assert.match(source, /CapsuleSystemRow/)
assert.match(source, /getDeliveryRiderMe/)
assert.match(source, /listClaimableDeliveryOrders/)
assert.match(source, /listAssignedDeliveryOrders/)
assert.doesNotMatch(source, /<Text>骑<\/Text>/)
assert.doesNotMatch(source, /rider-grab-hero__orb/)
assert.doesNotMatch(source, /rider-grab-list-head/)

assert.match(style, /\.rider-grab-task-summary\s*\{/)
assert.match(style, /\.rider-grab-refresh\s*\{/)
assert.match(style, /\.rider-grab-tabs\s*\{[\s\S]*grid-template-columns:\s*repeat\(3,\s*minmax\(0,\s*1fr\)\)/)
assert.match(style, /\.rider-empty-state\s*\{/)
assert.match(style, /@keyframes\s+rider-refresh-spin/)
assert.doesNotMatch(
  style.match(/\.rider-grab-content\s*\{[^}]*\}/)?.[0] || '',
  /\bpadding\s*:/,
  'ScrollView must not use unsupported padding',
)

console.log('rider workbench clarity contract: ok')
```

- [ ] **Step 2: 注册检查命令**

在根 `package.json` 的 `scripts` 中加入：

```json
"check:rider-workbench-clarity": "node scripts/check-rider-workbench-clarity.cjs"
```

并把根级 `verify` 更新为包含该命令：

```json
"verify": "npm run check:external-takeout-directory && npm run check:miniapp-static-routes && npm run check:miniapp-business-closure && npm run check:boundaries && npm run check:miniapp-publish-contract && npm run check:miniapp-domain-search && npm run check:miniapp-settings-contract && npm run check:identity-application-contract && npm run check:miniapp-capsule-layout && npm run check:home-real-tools-contract && npm run check:real-wechat-secret-hygiene && npm run check:merchant-home-clarity && npm run check:rider-workbench-clarity && npm run typecheck && npm run test:backend"
```

- [ ] **Step 3: 运行契约并确认它先失败**

Run:

```powershell
npm run check:rider-workbench-clarity
```

Expected: FAIL，首个明确错误为：

```text
AssertionError [ERR_ASSERTION]: rider workbench icon component is required
```

- [ ] **Step 4: 只提交契约**

```powershell
git add -- package.json scripts/check-rider-workbench-clarity.cjs
git commit -m "测试：锁定骑手工作台清晰度边界"
```

---

### Task 2: 添加统一的骑手页本地图标

**Files:**
- Create: `apps/miniapp/src/pages/delivery-rider/rider-workbench-icon.tsx`
- Create: `apps/miniapp/src/pages/delivery-rider/icons/action-refresh.svg`
- Create: `apps/miniapp/src/pages/delivery-rider/icons/action-message.svg`
- Create: `apps/miniapp/src/pages/delivery-rider/icons/action-phone.svg`
- Create: `apps/miniapp/src/pages/delivery-rider/icons/empty-route.svg`
- Test: `scripts/check-rider-workbench-clarity.cjs`

- [ ] **Step 1: 创建图标映射组件**

创建 `apps/miniapp/src/pages/delivery-rider/rider-workbench-icon.tsx`：

```tsx
import React from 'react'
import { Image } from '@tarojs/components'
import refreshIcon from './icons/action-refresh.svg'
import messageIcon from './icons/action-message.svg'
import phoneIcon from './icons/action-phone.svg'
import emptyIcon from './icons/empty-route.svg'

export type RiderWorkbenchIconName = 'refresh' | 'message' | 'phone' | 'empty'

const sources: Record<RiderWorkbenchIconName, string> = {
  refresh: refreshIcon,
  message: messageIcon,
  phone: phoneIcon,
  empty: emptyIcon,
}

export default function RiderWorkbenchIcon({
  name,
  className = '',
}: {
  name: RiderWorkbenchIconName
  className?: string
}) {
  return (
    <Image
      className={`rider-workbench-icon ${className}`.trim()}
      src={sources[name]}
      mode='aspectFit'
    />
  )
}
```

- [ ] **Step 2: 创建刷新图标**

创建 `apps/miniapp/src/pages/delivery-rider/icons/action-refresh.svg`：

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" fill="none">
  <path d="M38 17a16 16 0 1 0 1.2 15.1" stroke="#FFFFFF" stroke-width="3" stroke-linecap="round"/>
  <path d="M38 8v9h-9" stroke="#FFFFFF" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"/>
</svg>
```

- [ ] **Step 3: 创建消息图标**

创建 `apps/miniapp/src/pages/delivery-rider/icons/action-message.svg`：

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" fill="none">
  <path d="M9 11h30v22H22l-9 7v-7H9V11Z" stroke="#2476D6" stroke-width="3" stroke-linejoin="round"/>
  <path d="M16 19h16M16 25h11" stroke="#2476D6" stroke-width="3" stroke-linecap="round"/>
</svg>
```

- [ ] **Step 4: 创建电话图标**

创建 `apps/miniapp/src/pages/delivery-rider/icons/action-phone.svg`：

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" fill="none">
  <path d="M16 8 10 13c-1.5 1.3-1.7 3.5-.7 5.2 5.2 9.2 11.4 15.4 20.6 20.6 1.8 1 4 .8 5.3-.7l4.8-5.8-9-5-3 4c-4.7-2.6-8.5-6.4-11-11l4-3-5-9.3Z" stroke="#2476D6" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"/>
</svg>
```

- [ ] **Step 5: 创建配送路线空状态图标**

创建 `apps/miniapp/src/pages/delivery-rider/icons/empty-route.svg`：

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" fill="none">
  <path d="M13 38c8 0 6-12 15-12h7" stroke="#2476D6" stroke-width="3" stroke-linecap="round" stroke-dasharray="3 5"/>
  <path d="M13 24c4.4 0 8-3.6 8-8s-3.6-8-8-8-8 3.6-8 8 8 16 8 16 8-11.6 8-16-3.6-8-8-8Z" fill="#EAF4FF" stroke="#2476D6" stroke-width="3" stroke-linejoin="round"/>
  <circle cx="13" cy="16" r="2.5" fill="#2476D6"/>
  <rect x="32" y="20" width="11" height="11" rx="3" fill="#FFF1EB" stroke="#EF7D4F" stroke-width="3"/>
</svg>
```

- [ ] **Step 6: 运行类型检查**

Run:

```powershell
npm --workspace apps/miniapp run typecheck
```

Expected:

```text
> @campus-circle/miniapp@0.1.0 typecheck
> tsc --noEmit
```

进程退出码为 `0`。

- [ ] **Step 7: 再次运行清晰度契约**

Run:

```powershell
npm run check:rider-workbench-clarity
```

Expected: FAIL，错误转为 `index.tsx` 尚未使用 `RiderWorkbenchIcon` 或仍存在字符图标，证明资源契约已经生效。

具体失败断言应为：

```text
The input did not match the regular expression /RiderWorkbenchIcon/
```

- [ ] **Step 8: 提交图标资源**

```powershell
git add -- apps/miniapp/src/pages/delivery-rider/rider-workbench-icon.tsx apps/miniapp/src/pages/delivery-rider/icons
git commit -m "美化：添加骑手工作台图标资源"
```

---

### Task 3: 重排骑手页结构并保持真实业务逻辑

**Files:**
- Modify: `apps/miniapp/src/pages/delivery-rider/index.tsx`
- Modify: `scripts/check-miniapp-business-closure.cjs`
- Test: `scripts/check-rider-workbench-clarity.cjs`
- Test: `scripts/check-miniapp-business-closure.cjs`

- [ ] **Step 1: 引入图标组件**

在 `apps/miniapp/src/pages/delivery-rider/index.tsx` 的本地导入中加入：

```tsx
import RiderWorkbenchIcon from './rider-workbench-icon'
```

- [ ] **Step 2: 替换加载和未授权状态的字符图标**

将两处：

```tsx
<View className='rider-grab-placeholder__icon'><Text>骑</Text></View>
```

替换为：

```tsx
<RiderWorkbenchIcon name='empty' className='rider-grab-placeholder__icon' />
```

加载状态外层增加修饰类：

```tsx
<View className='rider-grab-placeholder rider-grab-placeholder--loading'>
```

未授权状态继续保留原有 `me.message` 和“返回学生端”操作，不改变身份门禁。

- [ ] **Step 3: 删除装饰圆并重排当前任务区**

删除：

```tsx
<View className='rider-grab-hero__orb rider-grab-hero__orb--one' />
<View className='rider-grab-hero__orb rider-grab-hero__orb--two' />
```

保留 `CapsuleSystemRow` 和 `rider-grab-user`，将原 `rider-grab-wallet-summary` 整块替换为：

```tsx
<View className='rider-grab-task-summary'>
  <View className='rider-grab-task-summary__primary'>
    <Text className='rider-grab-task-summary__label'>当前配送任务</Text>
    <View className='rider-grab-task-summary__value-row'>
      <Text className='rider-grab-task-summary__value'>{activeTaskCount}</Text>
      <Text className='rider-grab-task-summary__unit'>单</Text>
    </View>
  </View>
  <View className='rider-grab-task-summary__stats'>
    <View className='rider-grab-task-summary__stat'>
      <Text className='rider-grab-task-summary__stat-value'>{claimable.length}</Text>
      <Text className='rider-grab-task-summary__stat-label'>可抢</Text>
    </View>
    <View className='rider-grab-task-summary__divider' />
    <View className='rider-grab-task-summary__stat'>
      <Text className='rider-grab-task-summary__stat-value'>{completedCount}</Text>
      <Text className='rider-grab-task-summary__stat-label'>已完成</Text>
    </View>
  </View>
  <View
    className={`rider-grab-refresh ${loading ? 'rider-grab-refresh--loading' : ''}`}
    onClick={() => {
      if (!loading) void load()
    }}
  >
    <RiderWorkbenchIcon
      name='refresh'
      className={`rider-grab-refresh__icon ${loading ? 'rider-grab-refresh__icon--spin' : ''}`}
    />
    <Text>{loading ? '刷新中' : '刷新'}</Text>
  </View>
</View>
```

该结构继续使用现有 `activeTaskCount`、`claimable.length` 和 `completedCount`，不得引入本地演示数字。

- [ ] **Step 4: 删除重复列表标题和第二个刷新入口**

将 `renderClaimableOrders()` 的开头改为：

```tsx
function renderClaimableOrders() {
  if (claimable.length === 0) {
    return <EmptyState title='当前暂无可抢配送单' description='新配送单出现后会显示在这里。' />
  }

  return (
    <View className='rider-grab-list'>
      {claimable.map((order) => {
        const route = parseRouteSummary(order.routeSummary)
        const busy = busyOrderId === order.deliveryOrderPublicId
        return (
          <View key={order.deliveryOrderPublicId} className='rider-order-card'>
            <View className='rider-order-card__top'>
              <View className='rider-order-card__serial'>
                <Text>{shortOrderId(order.deliveryOrderPublicId)}</Text>
                <Text className='rider-order-card__dot'>·</Text>
                <Text>{formatTime(order.createdAt)}</Text>
              </View>
              <View className='rider-order-card__fee'>
                <Text>配送费 </Text>
                <Text className='rider-order-card__fee-price'>¥{centsToYuan(order.deliveryFeeCents)}</Text>
              </View>
            </View>

            <View className='rider-order-route'>
              <View className='rider-order-route__line' />
              <View className='rider-order-route__row'>
                <View className='rider-order-route__marker rider-order-route__marker--pickup' />
                <View className='rider-order-route__info'>
                  <Text className='rider-order-route__label'>取货点</Text>
                  <Text className='rider-order-route__place'>{route.pickup}</Text>
                </View>
              </View>
              <View className='rider-order-route__row rider-order-route__row--drop'>
                <View className='rider-order-route__marker rider-order-route__marker--drop' />
                <View className='rider-order-route__info'>
                  <Text className='rider-order-route__label'>送达点</Text>
                  <View className='rider-order-route__drop-row'>
                    <Text className='rider-order-route__place'>{route.delivery}</Text>
                    <Text className='rider-order-route__lock'>抢单后显示门牌</Text>
                  </View>
                </View>
              </View>
            </View>

            <View className='rider-order-card__bottom'>
              <Text className='rider-order-card__privacy'>隐私保护 · 接单后可联系</Text>
              <View
                className={`rider-order-card__grab ${busy || rider.status !== 'active' ? 'rider-order-card__grab--disabled' : ''}`}
                onClick={() => {
                  if (!busy && rider.status === 'active') void claim(order)
                }}
              >
                <Text>{busy ? '接单中…' : rider.status === 'active' ? '抢单' : '当前不可接单'}</Text>
              </View>
            </View>
          </View>
        )
      })}
    </View>
  )
}
```

不得保留以下节点：

```tsx
<View className='rider-grab-list-head'>
```

- [ ] **Step 5: 区分无数据错误与有旧数据错误**

在 `tabs` 后加入当前状态的真实数量：

```tsx
const activeOrderCount = activeTab === 'grab'
  ? claimable.length
  : activeTab === 'pickup'
    ? pickupOrders.length
    : deliveryOrders.length
```

将 `ScrollView` 内容改为：

```tsx
<ScrollView scrollY enhanced showScrollbar={false} className='rider-grab-content'>
  <View className='rider-grab-content__inner'>
    {error && activeOrderCount === 0 ? (
      <View className='rider-load-error' onClick={() => void load()}>
        <Text className='rider-load-error__title'>订单加载失败</Text>
        <Text className='rider-load-error__desc'>{error}</Text>
        <Text className='rider-load-error__action'>点击重试</Text>
      </View>
    ) : (
      <>
        {error ? (
          <View className='rider-grab-error' onClick={() => void load()}>
            <Text>{error}，当前显示上次加载的数据，点击重试</Text>
          </View>
        ) : null}
        {activeTab === 'grab' ? renderClaimableOrders() : null}
        {activeTab === 'pickup' ? renderAssignedOrders(pickupOrders, 'pickup') : null}
        {activeTab === 'delivery' ? renderAssignedOrders(deliveryOrders, 'delivery') : null}
      </>
    )}
  </View>
</ScrollView>
```

这样首次失败不会伪装成空订单，后台刷新失败也不会清除旧数据。

- [ ] **Step 6: 给订单次要操作加入统一图标**

把消息按钮替换为：

```tsx
<View className='rider-order-card__secondary' onClick={() => openConversation(order.deliveryOrderPublicId)}>
  <RiderWorkbenchIcon name='message' className='rider-order-card__secondary-icon' />
  <Text>消息</Text>
</View>
```

把电话按钮替换为：

```tsx
<View
  className='rider-order-card__secondary'
  onClick={() => void Taro.makePhoneCall({ phoneNumber: contactPhone })}
>
  <RiderWorkbenchIcon name='phone' className='rider-order-card__secondary-icon' />
  <Text>电话</Text>
</View>
```

释放订单仍为文字危险操作，避免加入容易误解的装饰图标。

- [ ] **Step 7: 替换列表空状态**

将文件末尾的 `EmptyState` 替换为：

```tsx
function EmptyState({ title, description }: { title: string; description: string }) {
  return (
    <View className='rider-empty-state'>
      <RiderWorkbenchIcon name='empty' className='rider-empty-state__icon' />
      <View className='rider-empty-state__copy'>
        <Text className='rider-empty-state__title'>{title}</Text>
        <Text className='rider-empty-state__desc'>{description}</Text>
      </View>
    </View>
  )
}
```

- [ ] **Step 8: 加强现有业务闭环断言**

在 `scripts/check-miniapp-business-closure.cjs` 的骑手页 token 数组中加入：

```js
'RiderWorkbenchIcon',
"className='rider-grab-task-summary'",
"className='rider-empty-state'",
```

并在骑手页检查后增加：

```js
if (/<Text>骑<\/Text>|rider-grab-hero__orb|rider-grab-list-head/.test(riderPage)) {
  violations.push('pages/delivery-rider/index.tsx: legacy character icons or duplicate rider list controls must stay removed')
}
```

- [ ] **Step 9: 运行结构检查**

Run:

```powershell
npm run check:miniapp-business-closure
npm run check:rider-workbench-clarity
```

Expected:

```text
miniapp business closure check PASS
```

第二个检查应因 SCSS 尚未实现新选择器而 FAIL，明确失败断言为：

```text
The input did not match the regular expression /\.rider-grab-task-summary\s*\{/
```

此时 TSX、真实数据调用和字符图标相关断言必须已经通过。

- [ ] **Step 10: 提交结构改造**

```powershell
git add -- apps/miniapp/src/pages/delivery-rider/index.tsx scripts/check-miniapp-business-closure.cjs
git commit -m "重构：整理骑手工作台信息层级"
```

---

### Task 4: 实现深度精修样式

**Files:**
- Modify: `apps/miniapp/src/pages/delivery-rider/index.scss`
- Test: `scripts/check-rider-workbench-clarity.cjs`
- Test: `scripts/check-miniapp-capsule-layout.cjs`

- [ ] **Step 1: 收紧蓝色头部并删除旧装饰样式**

删除以下选择器及其完整规则：

```scss
.rider-grab-hero__orb
.rider-grab-hero__orb--one
.rider-grab-hero__orb--two
.rider-grab-wallet-summary
.rider-grab-wallet-summary__main
.rider-grab-wallet-summary__label
.rider-grab-wallet-summary__sub
.rider-grab-wallet-summary__amount-row
.rider-grab-wallet-summary__amount
.rider-grab-wallet-summary__unit
.rider-grab-wallet-summary__button
.rider-grab-wallet-summary__button--loading
.rider-grab-wallet-summary__refresh-icon
.rider-grab-wallet-summary__refresh-icon::after
.rider-grab-list-head
.rider-grab-list-head__refresh
```

将头部和身份区的关键规则改为：

```scss
.rider-grab-hero {
  position: relative;
  flex: 0 0 auto;
  padding: 0 32rpx 30rpx;
  background: linear-gradient(152deg, #2f83d8 0%, #1f69ba 100%);
  color: $text-on-color;
  box-sizing: border-box;
}

.rider-grab-system-row {
  position: relative;
  z-index: 2;
}

.rider-grab-user {
  display: flex;
  align-items: center;
  min-height: 76rpx;
}

.rider-grab-user__avatar {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 68rpx;
  height: 68rpx;
  border: 2rpx solid rgba(255, 255, 255, .5);
  border-radius: 22rpx;
  background: rgba(255, 255, 255, .1);
  color: #fff;
  font-size: 30rpx;
  font-weight: 750;
}

.rider-grab-user__main {
  flex: 1 1 auto;
  min-width: 0;
  margin-left: 18rpx;
}

.rider-grab-user__name-row {
  display: flex;
  align-items: center;
  gap: 12rpx;
}

.rider-grab-user__name {
  max-width: 270rpx;
  overflow: hidden;
  color: #fff;
  font-size: 32rpx;
  font-weight: 800;
  line-height: 1.15;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rider-grab-user__badge {
  min-width: 56rpx;
  height: 32rpx;
  padding: 0 10rpx;
  border-radius: 999rpx;
  background: rgba(255, 255, 255, .18);
  color: #fff;
  font-size: 19rpx;
  font-weight: 650;
  line-height: 32rpx;
  text-align: center;
}

.rider-grab-user__meta {
  display: block;
  margin-top: 8rpx;
  color: rgba(255, 255, 255, .78);
  font-size: 22rpx;
}

.rider-grab-user__switch {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  justify-content: center;
  min-width: 132rpx;
  min-height: 58rpx;
  padding: 0 18rpx;
  border: 1rpx solid rgba(255, 255, 255, .34);
  border-radius: 999rpx;
  background: rgba(14, 72, 132, .18);
  color: #fff;
  font-size: 22rpx;
  font-weight: 650;
  box-sizing: border-box;
}
```

- [ ] **Step 2: 添加当前任务区和刷新样式**

```scss
.rider-grab-task-summary {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 28rpx;
  margin-top: 26rpx;
  padding: 24rpx 26rpx;
  border: 1rpx solid rgba(255, 255, 255, .18);
  border-radius: 24rpx;
  background: rgba(10, 64, 122, .2);
}

.rider-grab-task-summary__primary {
  min-width: 0;
}

.rider-grab-task-summary__label {
  display: block;
  color: rgba(255, 255, 255, .74);
  font-size: 21rpx;
}

.rider-grab-task-summary__value-row {
  display: flex;
  align-items: baseline;
  margin-top: 4rpx;
}

.rider-grab-task-summary__value {
  color: #fff;
  font-size: 52rpx;
  font-weight: 900;
  line-height: 1;
}

.rider-grab-task-summary__unit {
  margin-left: 8rpx;
  color: rgba(255, 255, 255, .86);
  font-size: 23rpx;
  font-weight: 700;
}

.rider-grab-task-summary__stats {
  display: flex;
  align-items: center;
  gap: 20rpx;
}

.rider-grab-task-summary__stat {
  display: flex;
  flex-direction: column;
  align-items: center;
  min-width: 62rpx;
}

.rider-grab-task-summary__stat-value {
  color: #fff;
  font-size: 30rpx;
  font-weight: 800;
  line-height: 1;
}

.rider-grab-task-summary__stat-label {
  margin-top: 8rpx;
  color: rgba(255, 255, 255, .7);
  font-size: 19rpx;
}

.rider-grab-task-summary__divider {
  width: 1rpx;
  height: 44rpx;
  background: rgba(255, 255, 255, .2);
}

.rider-grab-refresh {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8rpx;
  min-width: 116rpx;
  min-height: 60rpx;
  padding: 0 16rpx;
  border: 1rpx solid rgba(255, 255, 255, .32);
  border-radius: 18rpx;
  background: rgba(255, 255, 255, .12);
  color: #fff;
  font-size: 22rpx;
  font-weight: 700;
  box-sizing: border-box;
}

.rider-grab-refresh--loading {
  opacity: .72;
}

.rider-grab-refresh__icon {
  width: 30rpx;
  height: 30rpx;
}

.rider-grab-refresh__icon--spin {
  animation: rider-refresh-spin .8s linear infinite;
}

@keyframes rider-refresh-spin {
  to {
    transform: rotate(360deg);
  }
}
```

- [ ] **Step 3: 把三段导航改成独立分段卡**

```scss
.rider-grab-tabs {
  position: relative;
  z-index: 2;
  flex: 0 0 auto;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8rpx;
  min-height: 84rpx;
  margin: -18rpx 24rpx 0;
  padding: 8rpx;
  border: 1rpx solid rgba(216, 226, 236, .88);
  border-radius: 24rpx;
  background: #fff;
  box-shadow: 0 18rpx 44rpx -34rpx rgba(33, 63, 93, .42);
  box-sizing: border-box;
}

.rider-grab-tabs__item {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8rpx;
  min-width: 0;
  border-radius: 17rpx;
  color: $text-tertiary;
  font-size: 27rpx;
  font-weight: 600;
}

.rider-grab-tabs__item--active {
  background: #eaf4ff;
  color: #176bc1;
  font-weight: 750;
}

.rider-grab-tabs__item--active::after {
  display: none;
}

.rider-grab-tabs__count {
  min-width: 28rpx;
  height: 28rpx;
  padding: 0 7rpx;
  border-radius: 999rpx;
  background: #edf2f7;
  color: #6e8195;
  font-size: 18rpx;
  font-weight: 700;
  line-height: 28rpx;
  text-align: center;
}

.rider-grab-tabs__item--active .rider-grab-tabs__count {
  background: #fff;
  color: #176bc1;
}
```

- [ ] **Step 4: 收紧内容区、订单卡片和操作区**

```scss
.rider-grab-content {
  flex: 1 1 auto;
  width: 100%;
  min-height: 0;
  background: #eef3f8;
}

.rider-grab-content__inner {
  padding: 26rpx 24rpx calc(36rpx + env(safe-area-inset-bottom));
  box-sizing: border-box;
}

.rider-grab-list {
  display: flex;
  flex-direction: column;
  gap: 22rpx;
}

.rider-grab-list--assigned {
  padding-top: 0;
}

.rider-order-card {
  overflow: hidden;
  padding: 28rpx 28rpx 24rpx;
  border: 1rpx solid rgba(216, 226, 236, .92);
  border-radius: 24rpx;
  background: #fff;
  box-shadow: 0 18rpx 42rpx -36rpx rgba(39, 65, 92, .42);
  box-sizing: border-box;
}

.rider-order-card--assigned {
  padding: 28rpx;
  border-radius: 24rpx;
  box-shadow: 0 18rpx 42rpx -36rpx rgba(39, 65, 92, .42);
}

.rider-order-card__quick-actions {
  display: flex;
  align-items: center;
  gap: 10rpx;
  margin-top: 24rpx;
  padding-top: 22rpx;
  border-top: 1rpx solid rgba(221, 230, 239, .8);
}

.rider-order-card__secondary {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 7rpx;
  min-width: 88rpx;
  height: 60rpx;
  padding: 0 14rpx;
  border: 1rpx solid rgba(202, 216, 230, .96);
  border-radius: 16rpx;
  background: #fff;
  color: #2476d6;
  font-size: 21rpx;
  font-weight: 650;
  box-sizing: border-box;
}

.rider-order-card__secondary-icon {
  width: 27rpx;
  height: 27rpx;
}

.rider-order-card__grab {
  min-width: 150rpx;
  height: 66rpx;
  border-radius: 18rpx;
  background: #2476d6;
  box-shadow: none;
  font-size: 25rpx;
}
```

现有路线、备注、隐私提示、危险按钮和禁用态选择器保持原样；上述规则只覆盖订单容器和操作区，不删除真实地址与订单状态字段。

- [ ] **Step 5: 添加紧凑空状态、错误状态和图标基类**

```scss
.rider-workbench-icon {
  display: block;
  flex: 0 0 auto;
}

.rider-empty-state,
.rider-load-error {
  display: flex;
  align-items: center;
  margin-top: 6rpx;
  padding: 38rpx 32rpx;
  border: 1rpx solid rgba(216, 226, 236, .92);
  border-radius: 24rpx;
  background: rgba(255, 255, 255, .86);
  box-sizing: border-box;
}

.rider-empty-state__icon {
  width: 92rpx;
  height: 92rpx;
}

.rider-empty-state__copy {
  flex: 1 1 auto;
  min-width: 0;
  margin-left: 26rpx;
}

.rider-empty-state__title,
.rider-load-error__title {
  display: block;
  color: $text-strong;
  font-size: 28rpx;
  font-weight: 750;
}

.rider-empty-state__desc,
.rider-load-error__desc {
  display: block;
  margin-top: 10rpx;
  color: $text-muted;
  font-size: 23rpx;
  line-height: 1.5;
}

.rider-load-error {
  flex-direction: column;
  align-items: flex-start;
  border-color: #efcbc7;
  background: #fff4f2;
}

.rider-load-error__action {
  margin-top: 18rpx;
  color: #2476d6;
  font-size: 23rpx;
  font-weight: 700;
}

.rider-grab-placeholder__icon {
  width: 92rpx;
  height: 92rpx;
}

.rider-grab-placeholder--loading {
  box-shadow: none;
}
```

- [ ] **Step 6: 增加窄屏约束**

在文件末尾加入：

```scss
@media (max-width: 350px) {
  .rider-grab-hero {
    padding-right: 24rpx;
    padding-left: 24rpx;
  }

  .rider-grab-user__name {
    max-width: 210rpx;
  }

  .rider-grab-task-summary {
    gap: 18rpx;
    padding-right: 20rpx;
    padding-left: 20rpx;
  }

  .rider-grab-task-summary__stats {
    gap: 12rpx;
  }

  .rider-grab-refresh {
    min-width: 104rpx;
    padding: 0 12rpx;
  }

  .rider-order-card__secondary {
    min-width: 78rpx;
    padding: 0 10rpx;
  }
}
```

- [ ] **Step 7: 运行样式和胶囊契约**

Run:

```powershell
npm run check:rider-workbench-clarity
npm run check:miniapp-capsule-layout
```

Expected:

```text
rider workbench clarity contract: ok
miniapp capsule geometry: ok
```

- [ ] **Step 8: 运行小程序类型检查**

Run:

```powershell
npm --workspace apps/miniapp run typecheck
```

Expected: 进程退出码为 `0`，无 TypeScript 错误。

- [ ] **Step 9: 提交视觉精修**

```powershell
git add -- apps/miniapp/src/pages/delivery-rider/index.scss
git commit -m "美化：深度精修骑手工作台界面"
```

---

### Task 5: 完成自动验证与微信开发者工具验收

**Files:**
- Verify only: `apps/miniapp/src/pages/delivery-rider/index.tsx`
- Verify only: `apps/miniapp/src/pages/delivery-rider/index.scss`
- Verify only: `apps/miniapp/src/pages/delivery-rider/icons/*.svg`

- [ ] **Step 1: 运行骑手业务和身份边界检查**

Run:

```powershell
npm run check:rider-workbench-clarity
npm run check:miniapp-business-closure
npm run check:miniapp-capsule-layout
npm run check:identity-application-contract
```

Expected: 四个命令全部退出码为 `0`，分别显示骑手清晰度、业务闭环、胶囊几何和身份申请契约通过。

- [ ] **Step 2: 运行小程序类型检查**

Run:

```powershell
npm --workspace apps/miniapp run typecheck
```

Expected: 退出码为 `0`。

- [ ] **Step 3: 运行支付关闭条件下的生产构建**

Run:

```powershell
$env:NODE_ENV='production'
$env:TARO_APP_MODE='production'
$env:TARO_APP_API_BASE_URL='https://zhanduiwang.top/api/v1'
$env:TARO_APP_DELIVERY_PAYMENT_ENABLED='false'
npm run build:miniapp
```

Expected:

```text
Compiled successfully
```

允许保留项目已有的 vendors 体积或样式顺序警告，但不得出现骑手页 TSX、SVG、SCSS/WXSS 编译错误。

- [ ] **Step 4: 检查提交边界**

Run:

```powershell
git status --short
git log -5 --oneline
```

Expected:

- 骑手页实施文件已经分别进入测试、图标、结构和样式提交。
- 用户原有 `.env.example`、`.gitignore`、小程序项目配置、部署和快速启动文档改动仍保持未提交，未混入本功能提交。

- [ ] **Step 5: 微信开发者工具手动验收**

在微信开发者工具顶部点击“编译”，使用真实审核通过的骑手身份依次检查：

1. 胶囊单独占行，切换身份和刷新按钮均可点击。
2. 蓝色头部保留，但身份、任务统计和刷新不再拥挤。
3. “待抢单”“待取货”“待送达”选中反馈明确，数量来自真实接口。
4. 空状态使用配送路线 SVG，不出现“骑”或“C”字符图标。
5. 后台刷新时旧数据保留，刷新图标旋转。
6. 网络失败时显示错误和重试，不伪装成空订单。
7. 有真实订单时验证抢单、前往取货、凭证上传、开始配送、确认送达、消息和电话入口。
8. iPhone 15 Pro 模拟器和一个较窄机型中检查文字截断、按钮溢出、底部安全区和卡片间距。

真机上的电话、会话和凭证上传只有实际测试后才能标记通过；自动检查和模拟器构建不得替代真机业务验收。

---

## 规格覆盖自查

- 胶囊独占安全行：Task 3 保留 `CapsuleSystemRow`，Task 4 与 Task 5 验证。
- 蓝色头部深度精修：Task 3 重排结构，Task 4 收紧视觉。
- 真实身份和订单：Task 3 不改数据函数，业务闭环脚本继续断言。
- 删除重复刷新与字符图标：Task 1 建立失败契约，Task 2 和 Task 3 修复。
- 统一本地图标：Task 2 创建四个 SVG 和映射组件。
- 加载、空列表、错误和旧数据失败降级：Task 3 与 Task 4 覆盖。
- 订单阶段操作不变：Task 3 只改表现层，Task 5 进行真实流程验收。
- 小屏、胶囊和底部安全区：Task 4 响应式规则，Task 5 模拟器检查。
- 不新增模拟数据、钱包、地图或支付：全部任务均未触及相应服务与后端。
