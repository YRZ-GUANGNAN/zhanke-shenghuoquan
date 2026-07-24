# 商家首页清晰化实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改变商家首页真实数据、审核门禁、身份切换和页面路由的前提下，用清晰 SVG 图标、`2 × 2` 快捷入口和更明确的信息层级替换当前模糊拥挤的首页表现。

**Architecture:** 保留 `MerchantHomeContent` 的数据加载和交互函数，只替换展示层。新增一个商家首页私有图标组件统一管理五个本地 SVG，并用静态合同脚本锁定图标、布局、胶囊避让和真实数据边界。

**Tech Stack:** Taro 4、React 18、TypeScript、SCSS、微信小程序本地 SVG、Node.js 静态合同检查。

---

## 文件结构

- Create: `apps/miniapp/src/pages/merchant-home/merchant-home-icon.tsx`  
  只负责把商家首页图标名称映射到本地 SVG，不包含导航和业务状态。
- Create: `apps/miniapp/src/pages/merchant-home/icons/action-publish.svg`
- Create: `apps/miniapp/src/pages/merchant-home/icons/action-wallet.svg`
- Create: `apps/miniapp/src/pages/merchant-home/icons/action-bill.svg`
- Create: `apps/miniapp/src/pages/merchant-home/icons/action-message.svg`
- Create: `apps/miniapp/src/pages/merchant-home/icons/empty-orders.svg`  
  五个图标使用相同 `48 × 48` 视口与统一描边规范。
- Modify: `apps/miniapp/src/pages/merchant-home/index.tsx:1-380`  
  保留数据流，重组顶部数据、快捷入口、状态标签和空状态 JSX。
- Modify: `apps/miniapp/src/pages/merchant-home/index.scss:65-520`  
  移除模糊装饰，建立高对比顶部、`2 × 2` 快捷卡片、等宽标签和空状态样式。
- Create: `scripts/check-merchant-home-clarity.cjs`  
  锁定本地 SVG、私有图标组件、`2 × 2` 布局、无文字符号图标和真实业务函数。
- Modify: `package.json:8-29`  
  注册 `check:merchant-home-clarity` 并加入总体验证。

### Task 1: 建立商家首页清晰度合同

**Files:**
- Create: `scripts/check-merchant-home-clarity.cjs`
- Modify: `package.json:8-29`

- [ ] **Step 1: 写入会失败的静态合同**

创建脚本，明确检查五个 SVG、私有图标组件、`2 × 2` 布局和业务边界：

```js
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const pageRoot = path.join(root, 'apps', 'miniapp', 'src', 'pages', 'merchant-home')
const source = fs.readFileSync(path.join(pageRoot, 'index.tsx'), 'utf8')
const style = fs.readFileSync(path.join(pageRoot, 'index.scss'), 'utf8')
const iconComponentPath = path.join(pageRoot, 'merchant-home-icon.tsx')
const icons = [
  ['action-publish', 'publishIcon'],
  ['action-wallet', 'walletIcon'],
  ['action-bill', 'billIcon'],
  ['action-message', 'messageIcon'],
  ['empty-orders', 'emptyIcon'],
]

assert.ok(fs.existsSync(iconComponentPath), 'merchant home icon component is required')
const iconComponent = fs.readFileSync(iconComponentPath, 'utf8')
for (const [icon, binding] of icons) {
  const file = path.join(pageRoot, 'icons', `${icon}.svg`)
  assert.ok(fs.existsSync(file), `${icon}.svg is required`)
  const svg = fs.readFileSync(file, 'utf8')
  assert.match(svg, /viewBox="0 0 48 48"/, `${icon}.svg must use the shared viewBox`)
  assert.match(iconComponent, new RegExp(`\\b${binding}\\b`), `${icon}.svg must be registered`)
}

assert.match(source, /MerchantHomeIcon/)
assert.match(source, /icon:\s*'publish'/)
assert.match(source, /icon:\s*'wallet'/)
assert.match(source, /icon:\s*'bill'/)
assert.match(source, /icon:\s*'message'/)
assert.match(source, /name='empty'/)
assert.doesNotMatch(source, /icon:\s*['"](?:\+|¥|楼|账|聊)['"]/)
assert.match(style, /\.merchant-home-shortcuts[\s\S]*grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\)/)
assert.match(style, /\.merchant-home-tabs[\s\S]*grid-template-columns:\s*repeat\(3,\s*minmax\(0,\s*1fr\)\)/)
assert.match(source, /CapsuleSystemRow/)
assert.match(source, /listMerchantDeliveryOrders/)
assert.match(source, /getDeliveryWallet/)
assert.match(source, /getIdentityApplication\('merchant'\)/)

console.log('merchant home clarity contract: ok')
```

- [ ] **Step 2: 注册检查命令**

在根 `package.json` 的 `scripts` 中加入：

```json
"check:merchant-home-clarity": "node scripts/check-merchant-home-clarity.cjs"
```

并把 `npm run check:merchant-home-clarity` 放在 `verify` 的小程序合同检查段中、`npm run typecheck` 之前。

- [ ] **Step 3: 运行检查并确认按预期失败**

Run:

```powershell
npm.cmd run check:merchant-home-clarity
```

Expected: FAIL，首个错误为 `merchant home icon component is required`。

- [ ] **Step 4: 提交失败合同**

```powershell
git add -- package.json scripts/check-merchant-home-clarity.cjs
git commit -m "测试：锁定商家首页清晰度合同"
```

### Task 2: 新增统一 SVG 图标组件

**Files:**
- Create: `apps/miniapp/src/pages/merchant-home/merchant-home-icon.tsx`
- Create: `apps/miniapp/src/pages/merchant-home/icons/action-publish.svg`
- Create: `apps/miniapp/src/pages/merchant-home/icons/action-wallet.svg`
- Create: `apps/miniapp/src/pages/merchant-home/icons/action-bill.svg`
- Create: `apps/miniapp/src/pages/merchant-home/icons/action-message.svg`
- Create: `apps/miniapp/src/pages/merchant-home/icons/empty-orders.svg`

- [ ] **Step 1: 创建五个统一 SVG**

`action-publish.svg`：

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" fill="none">
  <rect x="8" y="6" width="27" height="36" rx="5" stroke="#2476D6" stroke-width="3"/>
  <path d="M15 16h13M15 23h10" stroke="#2476D6" stroke-width="3" stroke-linecap="round"/>
  <circle cx="35" cy="34" r="9" fill="#EFF6FF" stroke="#2476D6" stroke-width="3"/>
  <path d="M35 30v8M31 34h8" stroke="#2476D6" stroke-width="3" stroke-linecap="round"/>
</svg>
```

`action-wallet.svg`：

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" fill="none">
  <path d="M9 14V12a5 5 0 0 1 5-5h22" stroke="#2476D6" stroke-width="3" stroke-linecap="round"/>
  <rect x="7" y="12" width="34" height="29" rx="6" stroke="#2476D6" stroke-width="3"/>
  <path d="M31 21h11v11H31a5.5 5.5 0 0 1 0-11Z" fill="#EFF6FF" stroke="#2476D6" stroke-width="3"/>
  <circle cx="34" cy="26.5" r="1.75" fill="#2476D6"/>
</svg>
```

`action-bill.svg`：

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" fill="none">
  <path d="M10 7h28v35l-4-3-5 3-5-3-5 3-5-3-4 3V7Z" stroke="#2476D6" stroke-width="3" stroke-linejoin="round"/>
  <path d="M16 16h16M16 23h16M16 30h10" stroke="#2476D6" stroke-width="3" stroke-linecap="round"/>
</svg>
```

`action-message.svg`：

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" fill="none">
  <path d="M8 10h27a6 6 0 0 1 6 6v12a6 6 0 0 1-6 6H21l-9 7v-7H8a6 6 0 0 1-6-6V16a6 6 0 0 1 6-6Z" stroke="#2476D6" stroke-width="3" stroke-linejoin="round"/>
  <circle cx="15" cy="22" r="2" fill="#2476D6"/>
  <circle cx="23" cy="22" r="2" fill="#2476D6"/>
  <circle cx="31" cy="22" r="2" fill="#2476D6"/>
</svg>
```

`empty-orders.svg`：

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" fill="none">
  <path d="M8 17 24 7l16 10-16 10L8 17Z" fill="#EFF6FF" stroke="#2476D6" stroke-width="3" stroke-linejoin="round"/>
  <path d="M8 17v18l16 8 16-8V17M24 27v16" stroke="#2476D6" stroke-width="3" stroke-linejoin="round"/>
  <path d="m16 12 16 10" stroke="#2476D6" stroke-width="3" stroke-linecap="round"/>
</svg>
```

所有图形落在 `2..44` 的视觉安全区，不使用文字、Emoji、滤镜或外部引用。

- [ ] **Step 2: 创建私有图标组件**

```tsx
import React from 'react'
import { Image } from '@tarojs/components'
import publishIcon from './icons/action-publish.svg'
import walletIcon from './icons/action-wallet.svg'
import billIcon from './icons/action-bill.svg'
import messageIcon from './icons/action-message.svg'
import emptyIcon from './icons/empty-orders.svg'

export type MerchantHomeIconName = 'publish' | 'wallet' | 'bill' | 'message' | 'empty'

const sources: Record<MerchantHomeIconName, string> = {
  publish: publishIcon,
  wallet: walletIcon,
  bill: billIcon,
  message: messageIcon,
  empty: emptyIcon,
}

export default function MerchantHomeIcon({
  name,
  className = '',
}: {
  name: MerchantHomeIconName
  className?: string
}) {
  return (
    <Image
      className={`merchant-home-icon ${className}`.trim()}
      src={sources[name]}
      mode='aspectFit'
    />
  )
}
```

- [ ] **Step 3: 运行图标资产的局部检查**

Run:

```powershell
node -e "const fs=require('fs');for(const n of ['action-publish','action-wallet','action-bill','action-message','empty-orders']){const s=fs.readFileSync('apps/miniapp/src/pages/merchant-home/icons/'+n+'.svg','utf8');if(!/viewBox=\"0 0 48 48\"/.test(s))throw new Error(n)}console.log('merchant svg assets: ok')"
```

Expected: `merchant svg assets: ok`。

- [ ] **Step 4: 提交图标资源**

```powershell
git add -- apps/miniapp/src/pages/merchant-home/merchant-home-icon.tsx apps/miniapp/src/pages/merchant-home/icons
git commit -m "新增：商家首页清晰 SVG 图标"
```

### Task 3: 重组商家首页展示组件

**Files:**
- Modify: `apps/miniapp/src/pages/merchant-home/index.tsx:1-380`

- [ ] **Step 1: 引入图标组件并建立快捷入口配置**

在现有导入中增加：

```tsx
import MerchantHomeIcon, { type MerchantHomeIconName } from './merchant-home-icon'
```

在 `tabCopy` 后增加：

```tsx
const shortcutCopy: Array<{
  key: string
  label: string
  description: string
  icon: MerchantHomeIconName
  url: string
}> = [
  { key: 'publish', label: '发布配送单', description: '创建新的校内配送', icon: 'publish', url: '/pages/merchant-publish/index' },
  { key: 'wallet', label: '钱包', description: '查看余额与充值', icon: 'wallet', url: '/pages/merchant-wallet/index' },
  { key: 'bill', label: '账单', description: '核对资金流水', icon: 'bill', url: '/pages/merchant-wallet/index' },
  { key: 'message', label: '消息', description: '联系正在配送的骑手', icon: 'message', url: '/pages/merchant-messages/index' },
]
```

- [ ] **Step 2: 删除顶部模糊装饰节点**

从 `.merchant-home-hero` 内删除：

```tsx
<View className='merchant-home-hero__orb merchant-home-hero__orb--left' />
<View className='merchant-home-hero__orb merchant-home-hero__orb--right' />
```

保留 `CapsuleSystemRow`、商家资料、身份切换、余额和真实统计值。

- [ ] **Step 3: 把快捷入口替换为横向内容卡片**

```tsx
<View className='merchant-home-shortcuts'>
  {shortcutCopy.map((item) => (
    <View
      key={item.key}
      className={`merchant-home-shortcut merchant-home-shortcut--${item.key}`}
      onClick={() => void Taro.navigateTo({ url: item.url })}
    >
      <View className='merchant-home-shortcut__icon'>
        <MerchantHomeIcon name={item.icon} />
      </View>
      <View className='merchant-home-shortcut__copy'>
        <Text className='merchant-home-shortcut__label'>{item.label}</Text>
        <Text className='merchant-home-shortcut__description'>{item.description}</Text>
      </View>
    </View>
  ))}
</View>
```

不得修改四个 URL。

- [ ] **Step 4: 把空订单文本替换为完整空状态**

```tsx
{!loading && !loadError && groupedOrders[activeTab].length === 0 ? (
  <View className='merchant-home-empty'>
    <View className='merchant-home-empty__icon'>
      <MerchantHomeIcon name='empty' />
    </View>
    <Text className='merchant-home-empty__title'>
      暂无{tabCopy.find((item) => item.key === activeTab)?.label}配送单
    </Text>
    <Text className='merchant-home-empty__description'>
      新配送单和状态更新会显示在这里
    </Text>
  </View>
) : null}
```

- [ ] **Step 5: 运行类型检查，确认样式合同仍失败**

Run:

```powershell
npm.cmd --workspace apps/miniapp run typecheck
npm.cmd run check:merchant-home-clarity
```

Expected: TypeScript PASS；清晰度合同仍因 `2 × 2` SCSS 尚未实现而 FAIL。

- [ ] **Step 6: 提交 JSX 重组**

```powershell
git add -- apps/miniapp/src/pages/merchant-home/index.tsx
git commit -m "重构：商家首页信息与快捷入口"
```

### Task 4: 实现高对比清晰布局

**Files:**
- Modify: `apps/miniapp/src/pages/merchant-home/index.scss:65-520`

- [ ] **Step 1: 收敛顶部背景和数据层级**

删除 `.merchant-home-hero__orb*` 样式。调整顶部：

```scss
.merchant-home-hero {
  position: relative;
  flex: 0 0 auto;
  padding: 0 32rpx 42rpx;
  box-sizing: border-box;
  background: linear-gradient(160deg, $brand-primary-strong 0%, $brand-primary 100%);
}

.merchant-home-wallet__amount,
.merchant-home-counts__value {
  font-variant-numeric: tabular-nums;
}

.merchant-home-wallet__amount {
  margin-top: 12rpx;
  font-size: 58rpx;
  font-weight: 800;
  line-height: 1;
  letter-spacing: -1rpx;
}

.merchant-home-counts {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12rpx;
}
```

次级统计使用半透明度不低于 `.88` 的白色文字，不使用 `filter` 或 `backdrop-filter`。

- [ ] **Step 2: 实现 `2 × 2` 快捷卡片**

```scss
.merchant-home-shortcuts {
  position: relative;
  z-index: 3;
  flex: 0 0 auto;
  margin: -28rpx 28rpx 0;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16rpx;
}

.merchant-home-shortcut {
  min-width: 0;
  min-height: 132rpx;
  padding: 22rpx;
  border: 1rpx solid color.mix($brand-primary, $bg-surface, 14%);
  border-radius: 22rpx;
  background: $bg-surface;
  box-shadow: 0 12rpx 30rpx -24rpx rgba($brand-primary-strong, .58);
  display: flex;
  align-items: center;
  box-sizing: border-box;
  transition: transform .18s ease, background-color .18s ease;
}

.merchant-home-shortcut:active {
  transform: scale(.985);
  background: color.mix($brand-tint, $bg-surface, 36%);
}

.merchant-home-shortcut__icon {
  width: 76rpx;
  height: 76rpx;
  flex: 0 0 76rpx;
  padding: 14rpx;
  border-radius: 20rpx;
  background: color.mix($brand-primary, $bg-surface, 9%);
  box-sizing: border-box;
}

.merchant-home-icon {
  display: block;
  width: 100%;
  height: 100%;
}

.merchant-home-shortcut__copy {
  min-width: 0;
  margin-left: 18rpx;
  display: flex;
  flex-direction: column;
}

.merchant-home-shortcut__label {
  color: $text-strong;
  font-size: 26rpx;
  font-weight: 700;
  line-height: 1.2;
}

.merchant-home-shortcut__description {
  margin-top: 8rpx;
  color: $text-secondary;
  font-size: 20rpx;
  line-height: 1.25;
}
```

- [ ] **Step 3: 把订单标签改为三等分**

```scss
.merchant-home-tabs {
  position: relative;
  z-index: 1;
  flex: 0 0 auto;
  margin: 18rpx 28rpx 0;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  border-bottom: 1rpx solid $divider-hairline;
}

.merchant-home-tab {
  position: relative;
  min-height: 82rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  color: $text-secondary;
}

.merchant-home-tab--active {
  color: $brand-primary-strong;
}

.merchant-home-tab--active::after {
  content: '';
  position: absolute;
  left: 25%;
  right: 25%;
  bottom: -1rpx;
  height: 5rpx;
  border-radius: 999rpx;
  background: $brand-primary;
}
```

- [ ] **Step 4: 增加清晰空状态**

```scss
.merchant-home-empty {
  padding: 58rpx 32rpx 72rpx;
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
}

.merchant-home-empty__icon {
  width: 104rpx;
  height: 104rpx;
  padding: 22rpx;
  border-radius: 28rpx;
  background: color.mix($brand-primary, $bg-surface, 8%);
  box-sizing: border-box;
}

.merchant-home-empty__title {
  margin-top: 24rpx;
  color: $text-primary;
  font-size: 26rpx;
  font-weight: 700;
}

.merchant-home-empty__description {
  margin-top: 10rpx;
  color: $text-secondary;
  font-size: 21rpx;
}
```

- [ ] **Step 5: 运行三个专项检查**

Run:

```powershell
npm.cmd run check:merchant-home-clarity
npm.cmd run check:miniapp-capsule-layout
npm.cmd run check:miniapp-business-closure
```

Expected:

```text
merchant home clarity contract: ok
miniapp capsule geometry: ok
miniapp business closure check PASS
```

- [ ] **Step 6: 提交样式**

```powershell
git add -- apps/miniapp/src/pages/merchant-home/index.scss
git commit -m "美化：提升商家首页图标与排版清晰度"
```

### Task 5: 全量构建验证与交付

**Files:**
- Modify: `docs/superpowers/plans/2026-07-24-merchant-home-clarity.md`

- [ ] **Step 1: 运行小程序类型检查**

Run:

```powershell
npm.cmd --workspace apps/miniapp run typecheck
```

Expected: exit code `0`，无 TypeScript 错误。

- [ ] **Step 2: 运行相关合同检查**

Run:

```powershell
npm.cmd run check:merchant-home-clarity
npm.cmd run check:miniapp-capsule-layout
npm.cmd run check:miniapp-business-closure
```

Expected: 三项均 PASS。

- [ ] **Step 3: 运行微信小程序生产构建**

Run:

```powershell
$env:NODE_ENV='production'
$env:TARO_APP_API_BASE='https://zhanduiwang.top/api/v1'
npm.cmd --workspace apps/miniapp run build:weapp
```

Expected: `Compiled successfully`，产物更新到 `apps/miniapp/dist`。

- [ ] **Step 4: 检查任务范围**

Run:

```powershell
git status --short
git diff --check
git diff --name-only 0c9f167..HEAD
```

Expected: 本任务提交只包含商家首页、五个 SVG、清晰度合同、根脚本注册和本计划；原有 `.env.example`、部署文件、微信项目私有配置等用户改动仍保持未暂存。

- [ ] **Step 5: 微信开发者工具视觉验收**

在 iPhone 15 Pro 模拟器中重新编译，确认：

1. 微信官方胶囊独占顶部安全行。
2. 四个快捷入口为 `2 × 2`，图标边缘清晰、尺寸一致。
3. 快捷卡片文字不截断、不重叠。
4. 钱包余额、今日单量、配送中层级明确。
5. 待接单、配送中、已完成三项等宽对齐。
6. 空订单状态包含图标、标题和说明。
7. 四个入口仍进入原页面。

- [ ] **Step 6: 记录计划完成状态**

把本计划中已执行步骤勾选为 `[x]`，然后提交：

```powershell
git add -- docs/superpowers/plans/2026-07-24-merchant-home-clarity.md
git commit -m "文档：记录商家首页清晰化验收"
```
