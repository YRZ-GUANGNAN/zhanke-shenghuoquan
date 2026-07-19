# WeChat Capsule Dedicated Row Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让全部 26 个自定义导航页面为微信官方胶囊保留独立系统行，并把所有业务导航与可点击控件统一移动到下一行。

**Architecture:** 将胶囊几何计算拆成可由 Node 直接验证的纯函数，由 Taro 运行时适配器输出统一 CSS 变量。新增 `CapsuleSystemRow` 与 `CapsulePageHeader` 两个共享边界：标准页面组合使用完整头部，品牌渐变页面只复用空系统行；静态契约检查保证后续新增的自定义导航页面不能回退到固定顶部间距。

**Tech Stack:** Taro 4.2、React 18、TypeScript 5.4、SCSS、Node.js 24、微信小程序构建工具

---

## 文件结构与职责

**新增：**

- `apps/miniapp/src/utils/capsule-layout.js`：只做窗口、状态栏与胶囊矩形的纯几何计算，不导入 Taro 或 React。
- `apps/miniapp/src/utils/capsule-layout.d.ts`：为纯 JavaScript 几何模块提供 TypeScript 类型。
- `apps/miniapp/src/components/capsule-page-header.tsx`：渲染独立系统行、业务导航行和三个内容槽。
- `apps/miniapp/src/components/capsule-page-header.scss`：定义统一高度、点击区域、标题居中和三种导航变体。
- `scripts/check-miniapp-capsule-layout.cjs`：执行几何断言、页面覆盖检查和旧布局禁用检查。

**修改：**

- `apps/miniapp/src/utils/capsule-safe.ts`：从旧的“右侧挤压”变量切换为系统行、业务行和总高度变量。
- `package.json`：加入 `check:miniapp-capsule-layout` 并纳入 `verify`。
- `scripts/check-home-real-tools-contract.cjs`：删除已过时的首页右侧挤压断言，改为共享头部断言。
- `apps/miniapp/src/pages/**/index.tsx`、`index.scss`：迁移全部声明 `navigationStyle: 'custom'` 的页面。
- `docs/REAL-WECHAT-ACCOUNT-ACCEPTANCE-REPORT.md`：只记录实际完成的平台结果，未验证平台保留 `PENDING`。

**删除：**

- `apps/miniapp/src/pages/home/header-layout.ts`：首页不再拥有独立的胶囊布局算法。

---

### Task 1: 建立可测试的胶囊几何模型

**Files:**
- Create: `apps/miniapp/src/utils/capsule-layout.js`
- Create: `apps/miniapp/src/utils/capsule-layout.d.ts`
- Create: `scripts/check-miniapp-capsule-layout.cjs`
- Modify: `apps/miniapp/src/utils/capsule-safe.ts`
- Modify: `package.json`

- [ ] **Step 1: 先写失败的几何检查脚本**

创建 `scripts/check-miniapp-capsule-layout.cjs`，先只写几何断言：

```js
const assert = require('node:assert/strict')
const path = require('node:path')

const layoutModule = path.join(
  __dirname,
  '..',
  'apps',
  'miniapp',
  'src',
  'utils',
  'capsule-layout.js',
)
const { calculateCapsuleLayout } = require(layoutModule)

const cases = [
  {
    name: '320px Android',
    windowInfo: { windowWidth: 320, statusBarHeight: 24 },
    menu: { top: 28, right: 312, bottom: 60, left: 225, width: 87, height: 32 },
    expectedSystemRowHeight: 76,
  },
  {
    name: '375px iPhone',
    windowInfo: { windowWidth: 375, statusBarHeight: 47 },
    menu: { top: 51, right: 367, bottom: 83, left: 280, width: 87, height: 32 },
    expectedSystemRowHeight: 99,
  },
  {
    name: '430px iPhone Max',
    windowInfo: { windowWidth: 430, statusBarHeight: 54 },
    menu: { top: 58, right: 422, bottom: 90, left: 335, width: 87, height: 32 },
    expectedSystemRowHeight: 106,
  },
]

for (const value of cases) {
  const layout = calculateCapsuleLayout(value.windowInfo, value.menu)
  assert.equal(layout.systemRowHeight, value.expectedSystemRowHeight, value.name)
  assert.equal(layout.businessRowTop, layout.systemRowHeight, value.name)
  assert.equal(layout.businessRowHeight, 48, value.name)
  assert.equal(layout.totalHeaderHeight, layout.systemRowHeight + 48, value.name)
  assert.ok(layout.businessRowTop > value.menu.bottom, value.name)
}

const fallback = calculateCapsuleLayout({ windowWidth: 375, statusBarHeight: 24 })
assert.deepEqual(fallback, {
  systemRowHeight: 76,
  businessRowTop: 76,
  businessRowHeight: 48,
  totalHeaderHeight: 124,
})

const totalFallback = calculateCapsuleLayout({ windowWidth: 375, statusBarHeight: 0 })
assert.equal(totalFallback.systemRowHeight, 72)

console.log('miniapp capsule geometry: ok')
```

- [ ] **Step 2: 运行检查并确认失败**

Run:

```powershell
node scripts/check-miniapp-capsule-layout.cjs
```

Expected: FAIL，错误包含 `Cannot find module`，因为 `capsule-layout.js` 尚未创建。

- [ ] **Step 3: 实现纯几何模块与类型声明**

创建 `apps/miniapp/src/utils/capsule-layout.js`：

```js
const BUSINESS_ROW_HEIGHT = 48

function positiveNumber(value, fallback) {
  return Number.isFinite(value) && value > 0 ? value : fallback
}

function validMenuRect(menu) {
  return Boolean(
    menu &&
    Number.isFinite(menu.top) &&
    Number.isFinite(menu.bottom) &&
    Number.isFinite(menu.left) &&
    Number.isFinite(menu.right) &&
    menu.width > 0 &&
    menu.height > 0 &&
    menu.bottom > menu.top &&
    menu.right > menu.left
  )
}

function calculateCapsuleLayout(windowInfo, menu) {
  const statusBarHeight = Math.max(20, positiveNumber(windowInfo?.statusBarHeight, 20))
  const fallbackSystemRowHeight = statusBarHeight + 52
  const systemRowHeight = validMenuRect(menu)
    ? Math.max(menu.bottom + 8, fallbackSystemRowHeight)
    : fallbackSystemRowHeight

  return {
    systemRowHeight,
    businessRowTop: systemRowHeight,
    businessRowHeight: BUSINESS_ROW_HEIGHT,
    totalHeaderHeight: systemRowHeight + BUSINESS_ROW_HEIGHT,
  }
}

module.exports = { BUSINESS_ROW_HEIGHT, calculateCapsuleLayout }
```

创建 `apps/miniapp/src/utils/capsule-layout.d.ts`：

```ts
import type { MenuButtonRect, WindowInfo } from '../services/taro'

export interface CapsuleLayout {
  systemRowHeight: number
  businessRowTop: number
  businessRowHeight: number
  totalHeaderHeight: number
}

export const BUSINESS_ROW_HEIGHT: 48
export function calculateCapsuleLayout(
  windowInfo: WindowInfo,
  menu?: MenuButtonRect,
): CapsuleLayout
```

- [ ] **Step 4: 将 Taro 适配器改成统一 CSS 变量**

把 `apps/miniapp/src/utils/capsule-safe.ts` 改为：

```ts
import type { CSSProperties } from 'react'
import Taro from '../services/taro'
import { calculateCapsuleLayout, type CapsuleLayout } from './capsule-layout'

export type CapsuleSafeStyle = CSSProperties & {
  '--capsule-system-row-height': string
  '--capsule-business-row-top': string
  '--capsule-business-row-height': string
  '--capsule-total-header-height': string
}

export function getCapsuleLayout(): CapsuleLayout {
  try {
    return calculateCapsuleLayout(
      Taro.getWindowInfo(),
      Taro.getMenuButtonBoundingClientRect(),
    )
  } catch {
    return calculateCapsuleLayout({
      windowWidth: 375,
      statusBarHeight: 20,
      pixelRatio: 1,
    })
  }
}

export function getCapsuleSafeStyle(): CapsuleSafeStyle {
  const layout = getCapsuleLayout()
  return {
    '--capsule-system-row-height': `${layout.systemRowHeight}px`,
    '--capsule-business-row-top': `${layout.businessRowTop}px`,
    '--capsule-business-row-height': `${layout.businessRowHeight}px`,
    '--capsule-total-header-height': `${layout.totalHeaderHeight}px`,
  }
}
```

- [ ] **Step 5: 把检查加入根脚本并运行**

在 `package.json` 的 `scripts` 中加入：

```json
"check:miniapp-capsule-layout": "node scripts/check-miniapp-capsule-layout.cjs"
```

在 `verify` 中把它放在 `check:home-real-tools-contract` 前面。

Run:

```powershell
npm.cmd run check:miniapp-capsule-layout
npm.cmd --workspace apps/miniapp run typecheck
```

Expected: 两条命令均退出码 `0`，输出包含 `miniapp capsule geometry: ok`。

- [ ] **Step 6: 提交几何模块**

```powershell
git add -- package.json scripts/check-miniapp-capsule-layout.cjs apps/miniapp/src/utils/capsule-layout.js apps/miniapp/src/utils/capsule-layout.d.ts apps/miniapp/src/utils/capsule-safe.ts
git commit -m "测试：建立微信胶囊独占行几何模型"
```

---

### Task 2: 建立共享胶囊系统行和业务导航组件

**Files:**
- Create: `apps/miniapp/src/components/capsule-page-header.tsx`
- Create: `apps/miniapp/src/components/capsule-page-header.scss`
- Modify: `scripts/check-miniapp-capsule-layout.cjs`

- [ ] **Step 1: 扩展契约检查并确认组件尚不存在**

在 `scripts/check-miniapp-capsule-layout.cjs` 的几何断言后加入：

```js
const fs = require('node:fs')

const componentPath = path.join(
  __dirname,
  '..',
  'apps',
  'miniapp',
  'src',
  'components',
  'capsule-page-header.tsx',
)
const stylePath = componentPath.replace(/\.tsx$/, '.scss')
assert.ok(fs.existsSync(componentPath), 'CapsulePageHeader component is required')
assert.ok(fs.existsSync(stylePath), 'CapsulePageHeader styles are required')

const componentSource = fs.readFileSync(componentPath, 'utf8')
const styleSource = fs.readFileSync(stylePath, 'utf8')
assert.match(componentSource, /export function CapsuleSystemRow/)
assert.match(componentSource, /export default function CapsulePageHeader/)
assert.match(styleSource, /var\(--capsule-system-row-height\)/)
assert.match(styleSource, /var\(--capsule-business-row-height\)/)
assert.match(styleSource, /min-width:\s*44px/)
assert.match(styleSource, /min-height:\s*44px/)
assert.match(styleSource, /pointer-events:\s*none/)
```

Run:

```powershell
npm.cmd run check:miniapp-capsule-layout
```

Expected: FAIL，错误为 `CapsulePageHeader component is required`。

- [ ] **Step 2: 创建共享组件**

创建 `apps/miniapp/src/components/capsule-page-header.tsx`：

```tsx
import type { ReactNode } from 'react'
import { Text, View } from '@tarojs/components'
import { getCapsuleSafeStyle } from '../utils/capsule-safe'
import './capsule-page-header.scss'

export type CapsuleHeaderVariant = 'standard' | 'search' | 'tabs'

interface CapsuleSystemRowProps {
  className?: string
}

interface CapsulePageHeaderProps {
  className?: string
  variant?: CapsuleHeaderVariant
  title?: string
  left?: ReactNode
  center?: ReactNode
  right?: ReactNode
  divider?: boolean
}

export function CapsuleSystemRow({ className = '' }: CapsuleSystemRowProps) {
  return (
    <View
      className={`capsule-system-row ${className}`.trim()}
      style={getCapsuleSafeStyle()}
    />
  )
}

export default function CapsulePageHeader({
  className = '',
  variant = 'standard',
  title,
  left,
  center,
  right,
  divider = true,
}: CapsulePageHeaderProps) {
  return (
    <View
      className={`capsule-page-header ${divider ? 'capsule-page-header--divider' : ''} ${className}`.trim()}
      style={getCapsuleSafeStyle()}
    >
      <View className='capsule-page-header__system-row' />
      <View className={`capsule-page-header__business-row capsule-page-header__business-row--${variant}`}>
        {left ? <View className='capsule-page-header__slot capsule-page-header__slot--left'>{left}</View> : null}
        {title ? <Text className='capsule-page-header__title'>{title}</Text> : null}
        {center ? <View className='capsule-page-header__center'>{center}</View> : null}
        {right ? <View className='capsule-page-header__slot capsule-page-header__slot--right'>{right}</View> : null}
      </View>
    </View>
  )
}
```

- [ ] **Step 3: 创建组件样式**

创建 `apps/miniapp/src/components/capsule-page-header.scss`：

```scss
@use '../styles/design-tokens.scss' as *;

.capsule-system-row,
.capsule-page-header__system-row {
  display: block;
  height: var(--capsule-system-row-height);
  min-height: var(--capsule-system-row-height);
  pointer-events: none;
}

.capsule-page-header {
  position: relative;
  z-index: 10;
  flex: 0 0 auto;
  background: $bg-surface;
}

.capsule-page-header--divider {
  border-bottom: 1rpx solid $divider-nav;
}

.capsule-page-header__business-row {
  position: relative;
  display: flex;
  align-items: center;
  height: var(--capsule-business-row-height);
  min-height: var(--capsule-business-row-height);
  padding: 0 16px;
  box-sizing: border-box;
}

.capsule-page-header__slot {
  position: relative;
  z-index: 2;
  display: flex;
  align-items: center;
  min-width: 44px;
  min-height: 44px;
}

.capsule-page-header__slot--right {
  justify-content: flex-end;
  margin-left: auto;
}

.capsule-page-header__center {
  display: flex;
  flex: 1 1 auto;
  align-items: center;
  min-width: 0;
}

.capsule-page-header__title {
  position: absolute;
  top: 50%;
  left: 50%;
  color: $text-strong;
  font-size: 32rpx;
  font-weight: 800;
  line-height: 1;
  pointer-events: none;
  transform: translate(-50%, -50%);
}

.capsule-page-header__business-row--search,
.capsule-page-header__business-row--tabs {
  gap: 9px;
}
```

- [ ] **Step 4: 运行检查与类型检查**

```powershell
npm.cmd run check:miniapp-capsule-layout
npm.cmd --workspace apps/miniapp run typecheck
```

Expected: 两条命令均 PASS。

- [ ] **Step 5: 提交共享组件**

```powershell
git add -- scripts/check-miniapp-capsule-layout.cjs apps/miniapp/src/components/capsule-page-header.tsx apps/miniapp/src/components/capsule-page-header.scss
git commit -m "功能：新增微信胶囊独占行头部组件"
```

---

### Task 3: 迁移资料与设置类标准页面

**Files:**
- Modify: `apps/miniapp/src/pages/about/index.tsx`
- Modify: `apps/miniapp/src/pages/about/index.scss`
- Modify: `apps/miniapp/src/pages/blacklist/index.tsx`
- Modify: `apps/miniapp/src/pages/blacklist/index.scss`
- Modify: `apps/miniapp/src/pages/feedback/index.tsx`
- Modify: `apps/miniapp/src/pages/feedback/index.scss`
- Modify: `apps/miniapp/src/pages/edit-profile/index.tsx`
- Modify: `apps/miniapp/src/pages/edit-profile/index.scss`
- Modify: `apps/miniapp/src/pages/legal/index.tsx`
- Modify: `apps/miniapp/src/pages/legal/index.scss`
- Modify: `apps/miniapp/src/pages/settings/index.tsx`
- Modify: `apps/miniapp/src/pages/settings/index.scss`
- Modify: `apps/miniapp/src/pages/my-content/index.tsx`
- Modify: `apps/miniapp/src/pages/my-content/index.scss`
- Modify: `apps/miniapp/src/pages/identity-application/index.tsx`
- Modify: `apps/miniapp/src/pages/identity-application/index.scss`
- Modify: `scripts/check-miniapp-capsule-layout.cjs`

- [ ] **Step 1: 为本组页面加入失败的覆盖断言**

在检查脚本中加入：

```js
const miniappPagesRoot = path.join(__dirname, '..', 'apps', 'miniapp', 'src', 'pages')
const standardPages = [
  'about',
  'blacklist',
  'feedback',
  'edit-profile',
  'legal',
  'settings',
  'my-content',
  'identity-application',
]

for (const pageName of standardPages) {
  const source = fs.readFileSync(path.join(miniappPagesRoot, pageName, 'index.tsx'), 'utf8')
  assert.match(source, /CapsulePageHeader/, `${pageName} must use CapsulePageHeader`)
}
```

Run: `npm.cmd run check:miniapp-capsule-layout`

Expected: FAIL，首个未迁移页面报告 `must use CapsulePageHeader`。

- [ ] **Step 2: 用标准头部替换八个页面的旧导航容器**

每个页面添加：

```tsx
import CapsulePageHeader from '../../components/capsule-page-header'
```

按以下准确映射替换原导航 JSX：

```tsx
// about
<CapsulePageHeader title='关于' left={<View className='about-nav__back' onClick={() => void Taro.navigateBack()}><Text>‹</Text></View>} />

// blacklist
<CapsulePageHeader title='黑名单' left={<View className='student-blacklist-nav__back' onClick={() => void Taro.navigateBack()}><Text>‹</Text></View>} />

// feedback
<CapsulePageHeader title='意见反馈' left={<View className='feedback-nav__back' onClick={() => void Taro.navigateBack()}><Text>‹</Text></View>} />

// edit-profile
<CapsulePageHeader
  title='编辑资料'
  left={<Text className='edit-profile-nav__action' onClick={() => void Taro.navigateBack()}>取消</Text>}
  right={<Text className='edit-profile-nav__save' onClick={() => void save()}>{saving ? '保存中' : '保存'}</Text>}
/>

// legal
<CapsulePageHeader title='法律条款' left={<View className='student-legal-nav__back' onClick={() => void Taro.navigateBack()}><Text>‹</Text></View>} />

// settings
<CapsulePageHeader title='设置' left={<View className='student-settings-nav__back' onClick={() => void Taro.navigateBack()}><Text>‹</Text></View>} />

// my-content
<CapsulePageHeader title='我的内容' left={<View className='my-content-header__back' onClick={() => void Taro.navigateBack()}><Text>‹</Text></View>} />

// identity-application
<CapsulePageHeader title={copy.title} left={<View className='identity-application-nav__back' onClick={() => void cancelAndBack()}><Text>‹</Text></View>} />
```

- [ ] **Step 3: 删除八个旧导航容器的固定高度和顶部内边距**

从对应 `index.scss` 删除旧 `.about-nav`、`.student-blacklist-nav`、`.feedback-nav`、`.edit-profile-nav`、`.student-legal-nav`、`.student-settings-nav`、`.my-content-header`、`.identity-application-nav` 容器规则。保留按钮自身的颜色和图形规则，并统一把按钮尺寸改为：

```scss
width: 44px;
height: 44px;
display: flex;
align-items: center;
justify-content: center;
```

不得保留 `padding-top: 78rpx`、`84rpx`、`86rpx`、`90rpx`、`96rpx` 或 `112rpx` 作为导航避让。

- [ ] **Step 4: 运行本组检查**

```powershell
npm.cmd run check:miniapp-capsule-layout
npm.cmd --workspace apps/miniapp run typecheck
```

Expected: PASS，无 TypeScript 错误。

- [ ] **Step 5: 提交标准页迁移**

```powershell
git add -- scripts/check-miniapp-capsule-layout.cjs apps/miniapp/src/pages/about apps/miniapp/src/pages/blacklist apps/miniapp/src/pages/feedback apps/miniapp/src/pages/edit-profile apps/miniapp/src/pages/legal apps/miniapp/src/pages/settings apps/miniapp/src/pages/my-content apps/miniapp/src/pages/identity-application
git commit -m "界面：迁移资料与设置页胶囊独占行"
```

---

### Task 4: 迁移发布、详情与聊天页面

**Files:**
- Modify: `apps/miniapp/src/pages/publish-entry/index.tsx`
- Modify: `apps/miniapp/src/pages/publish-entry/index.scss`
- Modify: `apps/miniapp/src/pages/publish-post/index.tsx`
- Modify: `apps/miniapp/src/pages/publish-post/index.scss`
- Modify: `apps/miniapp/src/pages/publish-secondhand/index.tsx`
- Modify: `apps/miniapp/src/pages/publish-secondhand/index.scss`
- Modify: `apps/miniapp/src/pages/post-detail/index.tsx`
- Modify: `apps/miniapp/src/pages/post-detail/index.scss`
- Modify: `apps/miniapp/src/pages/secondhand-detail/index.tsx`
- Modify: `apps/miniapp/src/pages/secondhand-detail/index.scss`
- Modify: `apps/miniapp/src/pages/secondhand-chat/index.tsx`
- Modify: `apps/miniapp/src/pages/secondhand-chat/index.scss`
- Modify: `scripts/check-miniapp-capsule-layout.cjs`

- [ ] **Step 1: 加入本组覆盖断言并确认失败**

```js
const publishingAndDetailPages = [
  'publish-entry',
  'publish-post',
  'publish-secondhand',
  'post-detail',
  'secondhand-detail',
  'secondhand-chat',
]
for (const pageName of publishingAndDetailPages) {
  const source = fs.readFileSync(path.join(miniappPagesRoot, pageName, 'index.tsx'), 'utf8')
  assert.match(source, /CapsulePageHeader/, `${pageName} must use CapsulePageHeader`)
}
```

Run: `npm.cmd run check:miniapp-capsule-layout`

Expected: FAIL，首个未迁移页面报告缺少共享头部。

- [ ] **Step 2: 迁移三个发布页面**

添加共享组件 import，并替换为：

```tsx
// publish-entry
<CapsulePageHeader title='发布' left={<View className='publish-entry-topbar__back' onClick={() => void Taro.navigateBack()}><Text>‹</Text></View>} />

// publish-post
<CapsulePageHeader
  title='发布帖子'
  left={<Text className='publish-post-nav__side' onClick={() => void Taro.navigateBack()}>取消</Text>}
  right={
    <Text className={`publish-post-nav__publish${canPublish ? ' publish-post-nav__publish--active' : ''}`} onClick={() => void handlePublish()}>
      {submitting ? '发布中' : '发布'}
    </Text>
  }
/>

// publish-secondhand
<CapsulePageHeader title='发布二手' left={<View className='publish-secondhand-nav__back' onClick={() => void Taro.navigateBack()}><Text>‹</Text></View>} />
```

删除三个旧导航容器的高度、网格和顶部 padding，保留发布按钮视觉状态。

- [ ] **Step 3: 迁移详情和聊天的内容型导航**

```tsx
// post-detail
<CapsulePageHeader
  variant='standard'
  left={<Text className='post-detail-nav__back' onClick={() => void Taro.navigateBack()}>‹</Text>}
  center={
    <View className='post-detail-author'>
      <View className='post-detail-author__avatar'>
        {post.avatar ? <Image src={post.avatar} mode='aspectFill' /> : post.author.slice(0, 1)}
      </View>
      <View className='post-detail-author__main'>
        <Text className='post-detail-author__name'>{post.author}</Text>
        <Text className='post-detail-author__meta'>{post.school}</Text>
      </View>
    </View>
  }
  right={<Text className={`post-detail-follow${following ? ' post-detail-follow--active' : ''}`} onClick={handleFollow}>{following ? '已关注' : '关注'}</Text>}
/>

// secondhand-detail
<CapsulePageHeader title='宝贝详情' left={<View className='secondhand-detail-header__back' onClick={goBack} />} />

// secondhand-chat
<CapsulePageHeader
  variant='standard'
  left={<View className='secondhand-chat-header__back' onClick={goBack} />}
  center={
    <View className='secondhand-chat-seller' onClick={canOpenSellerProfile ? openSellerProfile : undefined}>
      <View className='secondhand-chat-seller__avatar'>
        {peerAvatarImage ? (
          <Image className='secondhand-chat-seller__avatar-image' src={peerAvatarImage} mode='aspectFill' />
        ) : (
          <Text>{peerAvatar}</Text>
        )}
      </View>
      <View className='secondhand-chat-seller__meta'>
        <Text className='secondhand-chat-seller__name'>{status === 'loading' ? '正在连接私信' : peerName}</Text>
        <Text className='secondhand-chat-seller__state'>
          {status === 'ready' ? peerState : status === 'error' ? '连接失败' : '安全初始化中'}
        </Text>
      </View>
    </View>
  }
  right={<View className={`secondhand-chat-header__home ${canOpenSellerProfile ? '' : 'secondhand-chat-header__home--disabled'}`} onClick={canOpenSellerProfile ? openSellerProfile : undefined}><Text>{canOpenSellerProfile ? '主页' : '交易中'}</Text></View>}
/>
```

- [ ] **Step 4: 修正固定内容起点**

`post-detail` 当前使用错误的 `184px` 固定导航高度。改为让共享头部固定，并让正文使用统一总高度：

```scss
.post-detail-page {
  padding-top: var(--capsule-total-header-height);
}

.capsule-page-header.post-detail-nav {
  position: fixed;
  top: 0;
  right: 0;
  left: 0;
}
```

页面根节点必须设置 `style={getCapsuleSafeStyle()}`，正文不得再写 `padding-top: 184px`。

- [ ] **Step 5: 运行检查并提交**

```powershell
npm.cmd run check:miniapp-capsule-layout
npm.cmd --workspace apps/miniapp run typecheck
git add -- scripts/check-miniapp-capsule-layout.cjs apps/miniapp/src/pages/publish-entry apps/miniapp/src/pages/publish-post apps/miniapp/src/pages/publish-secondhand apps/miniapp/src/pages/post-detail apps/miniapp/src/pages/secondhand-detail apps/miniapp/src/pages/secondhand-chat
git commit -m "界面：迁移发布详情与私聊页胶囊独占行"
```

Expected: 检查与类型检查 PASS，提交成功。

---

### Task 5: 迁移首页、搜索、二手与外卖头部

**Files:**
- Modify: `apps/miniapp/src/pages/home/index.tsx`
- Modify: `apps/miniapp/src/pages/home/index.scss`
- Delete: `apps/miniapp/src/pages/home/header-layout.ts`
- Modify: `apps/miniapp/src/pages/search/index.tsx`
- Modify: `apps/miniapp/src/pages/search/index.scss`
- Modify: `apps/miniapp/src/pages/student-secondhand/index.tsx`
- Modify: `apps/miniapp/src/pages/student-secondhand/index.scss`
- Modify: `apps/miniapp/src/pages/student-takeout/index.tsx`
- Modify: `apps/miniapp/src/pages/student-takeout/index.scss`
- Modify: `scripts/check-home-real-tools-contract.cjs`
- Modify: `scripts/check-miniapp-capsule-layout.cjs`

- [ ] **Step 1: 加入主页面覆盖和首页新契约断言**

在胶囊检查脚本加入 `home`、`search`、`student-secondhand`、`student-takeout` 的 `CapsulePageHeader` 断言。

在 `scripts/check-home-real-tools-contract.cjs` 删除以下旧断言：

```js
assert.match(home, /--home-header-right/)
assert.match(headerLayout, /calculateHomeHeaderLayout/)
assert.match(homeStyle, /var\(--home-header-top/)
assert.match(homeStyle, /var\(--home-header-height/)
assert.match(homeStyle, /var\(--home-header-right/)
```

改为：

```js
assert.match(home, /CapsulePageHeader/)
assert.match(home, /variant='tabs'/)
assert.doesNotMatch(home, /calculateHomeHeaderLayout/)
assert.doesNotMatch(homeStyle, /--home-header-right/)
```

Run: `npm.cmd run check:miniapp-capsule-layout`

Expected: FAIL，主页面尚未迁移。

- [ ] **Step 2: 迁移首页标签与搜索入口**

删除 `headerLayout`、`headerStyle` 和 `calculateHomeHeaderLayout` import。把首页头部替换为：

```tsx
<CapsulePageHeader
  variant='tabs'
  className='student-home-header'
  center={
    <View className='student-home-header__tabs'>
      {tabs.map((tab) => (
        <View key={tab.key} className={`student-home-header__tab ${activeTab === tab.key ? 'student-home-header__tab--active' : ''}`} onClick={() => setActiveTab(tab.key)}>
          <Text>{tab.label}</Text>
        </View>
      ))}
    </View>
  }
  right={<View className='student-home-header__search' onClick={openSearch} />}
/>
```

删除 `apps/miniapp/src/pages/home/header-layout.ts`。首页 SCSS 不再声明 `--home-header-top`、`--home-header-height`、`--home-header-right`。

- [ ] **Step 3: 迁移搜索页**

正常分支使用：

```tsx
<CapsulePageHeader
  variant='search'
  className='student-search-header'
  left={<View className='student-search-back' onClick={() => void Taro.navigateBack()} />}
  center={
    <View className='student-search-box'>
      <View className='student-search-box__icon' />
      <Input
        className='student-search-box__input'
        value={keyword}
        placeholder={copy.placeholder}
        confirmType='search'
        onInput={(event) => setKeyword(event.detail.value)}
        onConfirm={() => void submitSearch()}
      />
    </View>
  }
  right={<Text className='student-search-submit' onClick={() => void submitSearch()}>{loading ? '搜索中' : '搜索'}</Text>}
/>
```

无效入口分支使用 `title='搜索入口无效'` 和同一个返回按钮。保留现有搜索数据、热词和历史逻辑。

- [ ] **Step 4: 迁移二手和外卖页**

```tsx
// student-secondhand
<CapsulePageHeader
  variant='search'
  className='student-secondhand-header__title-row'
  left={<Text className='student-secondhand-header__title'>二手集市</Text>}
  center={
    <View className='student-secondhand-search' onClick={openSearch}>
      <View className='student-secondhand-search__icon' />
      <Text>搜二手好物</Text>
    </View>
  }
  divider={false}
/>

// student-takeout
<CapsulePageHeader
  variant='search'
  className='student-takeout-header__row'
  left={<Text className='student-takeout-header__title'>外卖</Text>}
  center={
    <View className='student-takeout-search' onClick={() => void Taro.navigateTo({ url: '/pages/search/index?scope=takeout' })}>
      <View className='student-takeout-search__icon' />
      <Text>搜索商家</Text>
    </View>
  }
  divider={false}
/>
```

二手分类条和外卖副标题保留在共享业务导航行下方。固定外卖头部的页面根节点使用 `style={getCapsuleSafeStyle()}`，正文顶部改为基于 `var(--capsule-total-header-height)` 的 `calc()`，不得继续写 `204rpx` 固定避让。

- [ ] **Step 5: 运行主页面检查并提交**

```powershell
npm.cmd run check:miniapp-capsule-layout
npm.cmd run check:home-real-tools-contract
npm.cmd --workspace apps/miniapp run typecheck
git add -- scripts/check-home-real-tools-contract.cjs scripts/check-miniapp-capsule-layout.cjs apps/miniapp/src/pages/home apps/miniapp/src/pages/search apps/miniapp/src/pages/student-secondhand apps/miniapp/src/pages/student-takeout
git commit -m "界面：迁移学生主页面胶囊独占行"
```

Expected: 三条验证命令 PASS，首页工具真实数据过滤逻辑保持不变。

---

### Task 6: 迁移个人、公开主页及钱包品牌头部

**Files:**
- Modify: `apps/miniapp/src/pages/profile/index.tsx`
- Modify: `apps/miniapp/src/pages/profile/index.scss`
- Modify: `apps/miniapp/src/pages/user-profile/index.tsx`
- Modify: `apps/miniapp/src/pages/user-profile/index.scss`
- Modify: `apps/miniapp/src/pages/merchant-home/index.tsx`
- Modify: `apps/miniapp/src/pages/merchant-home/index.scss`
- Modify: `apps/miniapp/src/pages/merchant-wallet/index.tsx`
- Modify: `apps/miniapp/src/pages/merchant-wallet/index.scss`
- Modify: `apps/miniapp/src/pages/rider-wallet/index.tsx`
- Modify: `apps/miniapp/src/pages/rider-wallet/index.scss`
- Modify: `scripts/check-miniapp-capsule-layout.cjs`

- [ ] **Step 1: 加入品牌头部覆盖断言并确认失败**

```js
const heroPages = ['profile', 'user-profile', 'merchant-home', 'merchant-wallet', 'rider-wallet']
for (const pageName of heroPages) {
  const source = fs.readFileSync(path.join(miniappPagesRoot, pageName, 'index.tsx'), 'utf8')
  assert.match(source, /CapsuleSystemRow/, `${pageName} must use CapsuleSystemRow`)
}
```

Run: `npm.cmd run check:miniapp-capsule-layout`

Expected: FAIL，首个品牌头部页面缺少系统行。

- [ ] **Step 2: 在五个品牌头部最上方加入空系统行**

每个页面添加：

```tsx
import { CapsuleSystemRow } from '../../components/capsule-page-header'
```

按下面的逐页插入点放置 `<CapsuleSystemRow />`；装饰性 orb 仍可先写在 JSX 中，但必须保持 `pointer-events: none`，系统行必须位于第一个可点击业务节点之前：

| 页面 | hero 根节点 | 准确插入位置 |
|---|---|---|
| `profile` | `student-profile-hero` | 两个 `student-profile-hero__orb` 之后、`student-profile-user` 之前 |
| `user-profile` | `public-profile-hero` | `public-profile-nav` 之前 |
| `merchant-home` | `merchant-home-hero` | 两个 `merchant-home-hero__orb` 之后、`merchant-home-header` 之前 |
| `merchant-wallet` | `merchant-wallet-hero` | 两个 `merchant-wallet-hero__orb` 之后、`merchant-wallet-nav` 之前 |
| `rider-wallet` | `rider-wallet-hero` | 两个 `rider-wallet-hero__orb` 之后、`rider-wallet-nav` 之前 |

`profile` 的完整 hero 结构为：

```tsx
<View className='student-profile-hero'>
  <CapsuleSystemRow />
  <View className='student-profile-user'>
    <View className='student-profile-user__avatar'>
      <Text>{profile.nickname.trim().slice(0, 1) || '同'}</Text>
    </View>
    <View className='student-profile-user__main'>
      <View className='student-profile-user__title-row'>
        <Text className='student-profile-user__name'>{profile.nickname || '校园用户'}</Text>
        <Text className='student-profile-user__badge'>{roleName}</Text>
      </View>
      <Text className='student-profile-user__meta'>
        用户号：{profile.id || '--'} · {profile.campus || profile.school || '校园用户'}
      </Text>
    </View>
    <View className='student-profile-user__edit'><Text>编辑资料</Text></View>
  </View>
</View>
```

其余四页不改动导航、余额、身份切换及返回事件，只在表中指定的位置插入系统行。

- [ ] **Step 3: 删除品牌头部的旧固定避让**

逐页执行下面的准确样式替换，删除旧顶部/右侧胶囊避让，并让固定高度页面改为内容流或 flex 剩余空间：

```scss
/* profile/index.scss */
.student-profile-hero {
  min-height: 0;
  padding: 0 36rpx 78rpx;
  box-sizing: border-box;
}

.student-profile-user {
  min-height: 48px;
}

/* user-profile/index.scss */
.public-profile-hero {
  padding: 0 30rpx 34rpx;
}

.public-profile-nav {
  min-height: 48px;
  height: auto;
}

.public-profile-nav__more {
  margin-right: 0;
}

/* merchant-home/index.scss */
.merchant-home-page {
  display: flex;
  flex-direction: column;
  height: 100vh;
}

.merchant-home-hero {
  flex: 0 0 auto;
  height: auto;
  padding: 0 34rpx 54rpx;
}

.merchant-home-header {
  min-height: 48px;
  padding-right: 0;
}

.merchant-home-shortcuts,
.merchant-home-tabs {
  flex: 0 0 auto;
}

.merchant-home-orders {
  flex: 1 1 auto;
  min-height: 0;
  height: auto;
}

/* merchant-wallet/index.scss */
.merchant-wallet-page {
  display: flex;
  flex-direction: column;
  height: 100vh;
}

.merchant-wallet-hero {
  flex: 0 0 auto;
  height: auto;
  padding: 0 36rpx 54rpx;
}

.merchant-wallet-nav {
  min-height: 48px;
  height: auto;
}

.merchant-wallet-scroll {
  flex: 1 1 auto;
  min-height: 0;
  height: auto;
}

/* rider-wallet/index.scss */
.rider-wallet-hero {
  min-height: 0;
  padding: 0 58rpx 54rpx;
  box-sizing: border-box;
}

.rider-wallet-nav {
  min-height: 48px;
  height: auto;
}
```

不得保留 `merchant-home-orders { height: calc(100vh - 578rpx); }` 或 `merchant-wallet-scroll { height: calc(100vh - 528rpx); }`，否则新增系统行会再次造成底部内容被截断。个人页整个 hero 继续可点击进入资料编辑；公开主页的返回与更多按钮都位于系统行下方。

- [ ] **Step 4: 运行检查并提交**

```powershell
npm.cmd run check:miniapp-capsule-layout
npm.cmd run check:home-real-tools-contract
npm.cmd --workspace apps/miniapp run typecheck
git add -- scripts/check-miniapp-capsule-layout.cjs apps/miniapp/src/pages/profile apps/miniapp/src/pages/user-profile apps/miniapp/src/pages/merchant-home apps/miniapp/src/pages/merchant-wallet apps/miniapp/src/pages/rider-wallet
git commit -m "界面：迁移身份与钱包页胶囊独占行"
```

Expected: PASS，资料编辑入口契约仍然通过。

---

### Task 7: 迁移商家操作页面并锁定全部自定义页面覆盖

**Files:**
- Modify: `apps/miniapp/src/pages/merchant-messages/index.tsx`
- Modify: `apps/miniapp/src/pages/merchant-messages/index.scss`
- Modify: `apps/miniapp/src/pages/merchant-order-detail/index.tsx`
- Modify: `apps/miniapp/src/pages/merchant-order-detail/index.scss`
- Modify: `apps/miniapp/src/pages/merchant-publish/index.tsx`
- Modify: `apps/miniapp/src/pages/merchant-publish/index.scss`
- Modify: `scripts/check-miniapp-capsule-layout.cjs`

- [ ] **Step 1: 加入剩余页面断言并确认失败**

```js
const merchantPages = ['merchant-messages', 'merchant-order-detail', 'merchant-publish']
for (const pageName of merchantPages) {
  const source = fs.readFileSync(path.join(miniappPagesRoot, pageName, 'index.tsx'), 'utf8')
  assert.match(source, /CapsulePageHeader/, `${pageName} must use CapsulePageHeader`)
}
```

Run: `npm.cmd run check:miniapp-capsule-layout`

Expected: FAIL，商家操作页面尚未迁移。

- [ ] **Step 2: 迁移三个商家操作头部**

```tsx
// merchant-messages
<CapsulePageHeader
  title='消息'
  left={<View className='merchant-messages-nav__back' onClick={handleBack}><Text>‹</Text></View>}
  right={<Text className={`merchant-messages-nav__read ${unreadIds.length === 0 ? 'merchant-messages-nav__read--muted' : ''}`} onClick={markAllRead}>全部已读</Text>}
/>

// merchant-order-detail
<CapsulePageHeader title='配送单详情' left={<View className='merchant-detail-nav__back' onClick={handleBack}><Text>‹</Text></View>} />

// merchant-publish
<CapsulePageHeader title='发布配送单' left={<View className='merchant-publish-nav__back' onClick={() => void Taro.navigateBack()}><Text>‹</Text></View>} />
```

删除旧容器固定高度、顶部 padding、绝对定位右操作；保留消息已读状态颜色和返回图形。

- [ ] **Step 3: 加入全量自动发现检查**

在 `scripts/check-miniapp-capsule-layout.cjs` 加入：

```js
const customNavigationPages = fs.readdirSync(miniappPagesRoot, { withFileTypes: true })
  .filter((entry) => entry.isDirectory())
  .map((entry) => entry.name)
  .filter((pageName) => {
    const configPath = path.join(miniappPagesRoot, pageName, 'index.config.ts')
    return fs.existsSync(configPath) && /navigationStyle:\s*'custom'/.test(fs.readFileSync(configPath, 'utf8'))
  })

for (const pageName of customNavigationPages) {
  const source = fs.readFileSync(path.join(miniappPagesRoot, pageName, 'index.tsx'), 'utf8')
  assert.match(source, /CapsulePageHeader|CapsuleSystemRow/, `${pageName} custom navigation is not capsule-safe`)
}

const allMiniappSource = customNavigationPages
  .flatMap((pageName) => ['index.tsx', 'index.scss'].map((fileName) => path.join(miniappPagesRoot, pageName, fileName)))
  .filter(fs.existsSync)
  .map((fileName) => fs.readFileSync(fileName, 'utf8'))
  .join('\n')

assert.doesNotMatch(allMiniappSource, /--capsule-safe-right|--home-header-right/)
```

- [ ] **Step 4: 运行全量覆盖检查并提交**

```powershell
npm.cmd run check:miniapp-capsule-layout
npm.cmd --workspace apps/miniapp run typecheck
git add -- scripts/check-miniapp-capsule-layout.cjs apps/miniapp/src/pages/merchant-messages apps/miniapp/src/pages/merchant-order-detail apps/miniapp/src/pages/merchant-publish
git commit -m "界面：完成全部自定义导航胶囊独占行迁移"
```

Expected: 检查自动发现 26 个自定义导航页面并全部 PASS。

---

### Task 8: 完整验证、公网真机包与验收记录

**Files:**
- Modify: `docs/REAL-WECHAT-ACCOUNT-ACCEPTANCE-REPORT.md`

- [ ] **Step 1: 运行静态契约、密钥与类型检查**

```powershell
npm.cmd run check:miniapp-capsule-layout
npm.cmd run check:home-real-tools-contract
npm.cmd run check:miniapp-static-routes
npm.cmd run check:miniapp-business-closure
npm.cmd run check:real-wechat-secret-hygiene
npm.cmd --workspace apps/miniapp run typecheck
git diff --check
```

Expected: 所有命令退出码 `0`；不输出 AppSecret、token 或 OpenID。

- [ ] **Step 2: 生成公网 API 真机包**

```powershell
$env:NODE_ENV='production'
$env:TARO_APP_API_BASE='https://zhanduiwang.top/api/v1'
npm.cmd --workspace apps/miniapp run build:weapp
```

Expected: `Compiled successfully`。`vendors.js` 体积警告可以记录，但不能把编译失败写成通过。

- [ ] **Step 3: 验证构建产物地址与页面配置**

```powershell
$publicHits = rg -l --fixed-strings 'https://zhanduiwang.top/api/v1' apps/miniapp/dist
$localHits = rg -l --fixed-strings 'http://127.0.0.1:8080/api/v1' apps/miniapp/dist
Write-Output ('PUBLIC_API_FILES=' + @($publicHits).Count)
Write-Output ('LOCAL_API_FILES=' + @($localHits).Count)
node -e "const fs=require('fs');const app=JSON.parse(fs.readFileSync('apps/miniapp/dist/app.json','utf8'));console.log('PAGES='+app.pages.length)"
```

Expected: `PUBLIC_API_FILES` 大于 `0`，`LOCAL_API_FILES=0`，`PAGES=24`。

- [ ] **Step 4: 当前可用真机回归**

在微信开发者工具点击“编译”与“真机调试”，按顺序验证：

1. 首页系统行只有微信胶囊，私信、发现、工具和搜索位于下一行。
2. “我的 → 编辑资料”的取消、标题、保存位于下一行，保存可点击。
3. 发布帖子页面的取消、标题、发布位于下一行。
4. 搜索页面输入框和搜索按钮位于下一行。
5. 二手私聊的返回、卖家信息和主页按钮位于下一行。
6. 商家首页身份切换位于系统行下方。
7. 页面整体下移后不存在正文遮挡、底栏覆盖或横向滚动。

Expected: 当前设备逐项 PASS；未持有的另一平台继续记录为 `PENDING`。

- [ ] **Step 5: 更新验收报告**

在 `docs/REAL-WECHAT-ACCOUNT-ACCEPTANCE-REPORT.md` 增加“微信胶囊独占行”条目，使用以下规则：

```markdown
| 微信胶囊独占行 | PASS 或 PENDING | 仅把已经完成真机点击验证的平台写为 PASS；未验证平台保持 PENDING。 |
```

不得把类型检查、构建或模拟器截图替代为另一平台真机 PASS。

- [ ] **Step 6: 提交验收记录**

```powershell
git add -- docs/REAL-WECHAT-ACCOUNT-ACCEPTANCE-REPORT.md
git commit -m "文档：记录微信胶囊独占行真机验收"
```

- [ ] **Step 7: 最终工作树审计**

```powershell
git status --short
git log -8 --oneline
```

Expected: 本功能文件全部已提交；用户原有 `.env.example`、部署文件、quickstart 文档和 `.codex-tmp` 仍保持原状态且没有进入本功能提交。
