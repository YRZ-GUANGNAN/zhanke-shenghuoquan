# UI-20 Static Shell Closeout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将分散的新版静态页面整合到单一小程序壳，修复坏路由、统一发布和底部导航，并阻止旧 UI 重新进入可达路由。

**Architecture:** 以 `master@9c0ca376` 为唯一基线，选择性 cherry-pick 已验收页面提交，再通过共享 `StudentBottomNav`、角色参数和稳定 Mock ID 收口导航。`app.config.ts` 作为新版页面白名单，旧源码保留但不注册；新增只读路由检查脚本防止以后再次出现缺页或旧路由回流。

**Tech Stack:** Taro 4、React 18、TypeScript、SCSS、Node.js 路由检查脚本、微信小程序构建。

---

### Task 1: 安全整合已完成页面

**Files:**
- Import: `apps/miniapp/src/pages/legal/**`
- Import: `apps/miniapp/src/pages/search/**`
- Import: `apps/miniapp/src/pages/user-profile/**`
- Modify: `apps/miniapp/src/app.config.ts`
- Modify: `apps/miniapp/src/pages/profile/index.tsx`
- Modify: `apps/miniapp/src/pages/home/index.tsx`
- Modify: `apps/miniapp/src/pages/student-secondhand/index.tsx`
- Modify: `apps/miniapp/src/pages/post-detail/index.tsx`
- Modify: `apps/miniapp/src/pages/secondhand-detail/index.tsx`

- [ ] **Step 1: 引入法律条款提交**

Run:

```powershell
git cherry-pick f61a80d4
```

Expected: 新增 `pages/legal`，不包含 `project.config.json`。

- [ ] **Step 2: 引入搜索页单提交**

Run:

```powershell
git cherry-pick db13e36f
```

Expected: 新增 `pages/search`；`pages/blacklist` 保留；不出现 `apps/miniap/`。

- [ ] **Step 3: 按原顺序引入通用用户主页提交集**

Run:

```powershell
git cherry-pick 81d06744^..97877fe1
```

Expected: 新增 `pages/user-profile`、数据文件及三处入口；发生冲突时只在当前新版 `profile` 结构上保留法律入口与编辑资料入口。

- [ ] **Step 4: 检查负范围**

Run:

```powershell
git status --short
git diff --name-only master...HEAD | Select-String 'project.config.json|apps/miniap/'
```

Expected: 第二条无输出。

### Task 2: 建立共享新版学生底部导航

**Files:**
- Create: `apps/miniapp/src/components/student-bottom-nav.tsx`
- Modify: `apps/miniapp/src/pages/home/index.tsx`
- Modify: `apps/miniapp/src/pages/student-takeout/index.tsx`
- Modify: `apps/miniapp/src/pages/student-secondhand/index.tsx`
- Modify: `apps/miniapp/src/pages/profile/index.tsx`

- [ ] **Step 1: 创建共享导航组件**

组件保持现有 `student-bottom-nav*` class 名，不引入新视觉：

```tsx
import React from 'react'
import { Text, View } from '@tarojs/components'
import Taro from '../services/taro'

export type StudentNavKey = 'home' | 'takeout' | 'market' | 'profile'

const items = [
  { key: 'home', label: '首页', icon: 'home', url: '/pages/home/index' },
  { key: 'takeout', label: '外卖', icon: 'takeaway', url: '/pages/student-takeout/index' },
  { key: 'market', label: '二手', icon: 'market', url: '/pages/student-secondhand/index' },
  { key: 'profile', label: '我的', icon: 'profile', url: '/pages/profile/index' }
] as const

export default function StudentBottomNav({ active, className = '' }: { active: StudentNavKey; className?: string }) {
  const open = (url: string) => void Taro.redirectTo({ url })
  return (
    <View className={`student-bottom-nav ${className}`.trim()}>
      {items.slice(0, 2).map((item) => (
        <View key={item.key} className={`student-bottom-nav__item ${active === item.key ? 'student-bottom-nav__item--active' : ''}`} onClick={() => open(item.url)}>
          <View className={`student-bottom-nav__icon student-bottom-nav__icon--${item.icon}`} />
          <Text>{item.label}</Text>
        </View>
      ))}
      <View className='student-bottom-nav__fab' onClick={() => void Taro.navigateTo({ url: '/pages/publish-entry/index' })}><Text>+</Text></View>
      {items.slice(2).map((item) => (
        <View key={item.key} className={`student-bottom-nav__item ${active === item.key ? 'student-bottom-nav__item--active' : ''}`} onClick={() => open(item.url)}>
          <View className={`student-bottom-nav__icon student-bottom-nav__icon--${item.icon}`} />
          <Text>{item.label}</Text>
        </View>
      ))}
    </View>
  )
}
```

- [ ] **Step 2: 替换四个页面的重复导航 JSX**

分别使用：

```tsx
<StudentBottomNav active='home' />
<StudentBottomNav active='takeout' />
<StudentBottomNav active='market' className='student-bottom-nav--secondhand' />
<StudentBottomNav active='profile' />
```

删除各页面重复的 `navItems` 和导航点击函数，但保留原 SCSS，保证像素结构不变。

- [ ] **Step 3: 运行类型检查**

Run:

```powershell
npm --workspace apps/miniapp run typecheck
```

Expected: PASS。

- [ ] **Step 4: 提交导航收口**

```powershell
git add apps/miniapp/src/components/student-bottom-nav.tsx apps/miniapp/src/pages/home/index.tsx apps/miniapp/src/pages/student-takeout/index.tsx apps/miniapp/src/pages/student-secondhand/index.tsx apps/miniapp/src/pages/profile/index.tsx
git commit -m "refactor(ui): 统一学生端底部导航"
```

### Task 3: 收口发布选择

**Files:**
- Modify: `apps/miniapp/src/pages/publish-entry/index.tsx`

- [ ] **Step 1: 更新入口模型**

将 `url` 改为可选，并使帖子入口指向新版页面：

```ts
type PublishEntry = {
  key: 'post' | 'secondhand' | 'delivery'
  icon: string
  title: string
  subtitle: string
  tag: string
  tone: 'blue' | 'orange' | 'green'
  url?: string
}
```

```ts
const entries = [
  { key: 'post', url: '/pages/publish-post/index' },
  { key: 'secondhand' },
  { key: 'delivery', url: '/pages/merchant-publish/index' }
]
```

实际对象保留原图标、标题、说明、标签和颜色字段。

- [ ] **Step 2: 禁止回流旧二手发布 UI**

```ts
const openEntry = (entry: PublishEntry) => {
  if (!entry.url) {
    void Taro.showToast({ title: '二手发布静态页待设计', icon: 'none', duration: 1400 })
    return
  }
  void Taro.navigateTo({ url: entry.url })
}
```

- [ ] **Step 3: 提交发布入口**

```powershell
git add apps/miniapp/src/pages/publish-entry/index.tsx
git commit -m "fix(ui): 收口新版发布入口"
```

### Task 4: 修复搜索和用户主页入口

**Files:**
- Modify: `apps/miniapp/src/pages/post-detail/index.tsx`
- Modify: `apps/miniapp/src/pages/secondhand-chat/index.tsx`

- [ ] **Step 1: 帖子标签进入搜索页**

```ts
const openTagSearch = (tag: string) => {
  void Taro.navigateTo({ url: `/pages/search/index?keyword=${encodeURIComponent(tag.replace(/^#/, ''))}` })
}
```

- [ ] **Step 2: 二手聊天顶部进入卖家主页**

```ts
const openSellerProfile = () => {
  void Taro.navigateTo({ url: '/pages/user-profile/index?userId=xiangbei&tab=secondhand' })
}
```

将聊天顶部头像/姓名容器绑定 `openSellerProfile`，不修改聊天气泡和输入区视觉。

- [ ] **Step 3: 验证三类用户主页入口**

Run:

```powershell
rg -n "pages/user-profile/index" apps/miniapp/src/pages/post-detail apps/miniapp/src/pages/secondhand-detail apps/miniapp/src/pages/secondhand-chat
```

Expected: 三个目录均有命中。

### Task 5: 修复配送会话角色路由

**Files:**
- Modify: `apps/miniapp/src/pages/delivery-conversation/index.tsx`
- Modify: `apps/miniapp/src/pages/merchant-order-detail/index.tsx`

- [ ] **Step 1: 两种角色都使用现有新版详情页并携带角色**

```ts
merchant: {
  detailUrl: '/pages/merchant-order-detail/index?orderId=B1079&role=merchant'
},
rider: {
  detailUrl: '/pages/merchant-order-detail/index?orderId=B1079&role=rider'
}
```

- [ ] **Step 2: 详情页按角色展示联系对象**

```ts
const params = Taro.getCurrentInstance().router?.params || {}
const role = params.role === 'rider' ? 'rider' : 'merchant'
const contact = role === 'rider'
  ? { avatar: '茶', name: '茶知味 · 商家', phone: '188 5530 2217' }
  : { avatar: '李', name: '李伟 · 骑手 · 12:35 接单', phone: '137 5560 8814' }

const openConversation = () => {
  void Taro.navigateTo({ url: `/pages/delivery-conversation/index?orderId=B1079&role=${role}` })
}
```

保留商家角色现有截图；骑手角色只替换联系人信息和会话参数。

- [ ] **Step 3: 提交坏路由修复**

```powershell
git add apps/miniapp/src/pages/delivery-conversation/index.tsx apps/miniapp/src/pages/merchant-order-detail/index.tsx
git commit -m "fix(ui): 修复骑手配送详情跳转"
```

### Task 6: 接通内容 ID 与“我的内容”详情

**Files:**
- Modify: `apps/miniapp/src/pages/home/index.tsx`
- Modify: `apps/miniapp/src/pages/student-secondhand/index.tsx`
- Modify: `apps/miniapp/src/pages/my-content/index.tsx`
- Modify: `apps/miniapp/src/pages/post-detail/index.tsx`
- Modify: `apps/miniapp/src/pages/secondhand-detail/index.tsx`

- [ ] **Step 1: 为列表项增加稳定 ID 与详情类型**

```ts
type DetailTarget = 'post' | 'secondhand'

type DiscoveryCard = {
  id: string
  target: DetailTarget
  // 保留现有视觉字段
}
```

- [ ] **Step 2: 使用 ID 打开详情**

```ts
const openCard = (card: DiscoveryCard) => {
  const path = card.target === 'secondhand' ? 'secondhand-detail' : 'post-detail'
  void Taro.navigateTo({ url: `/pages/${path}/index?id=${encodeURIComponent(card.id)}` })
}
```

二手列表使用同样方式传 `item.id`。

- [ ] **Step 3: 接通“我的内容”四类条目**

```ts
const openPost = (postId: string, anchor?: string) => {
  const suffix = anchor ? `&anchor=${encodeURIComponent(anchor)}` : ''
  void Taro.navigateTo({ url: `/pages/post-detail/index?id=${encodeURIComponent(postId)}${suffix}` })
}

const openProduct = (productId: string) => {
  void Taro.navigateTo({ url: `/pages/secondhand-detail/index?id=${encodeURIComponent(productId)}` })
}
```

收藏、帖子、评论使用 `openPost`；宝贝使用 `openProduct`。

- [ ] **Step 4: 未知 ID 保持默认 Mock 回退**

详情页读取 `router.params.id`，只在存在对应 Mock 时替换内容；未知值继续展示当前默认详情，不报错、不白屏。

- [ ] **Step 5: 提交内容路由**

```powershell
git add apps/miniapp/src/pages/home/index.tsx apps/miniapp/src/pages/student-secondhand/index.tsx apps/miniapp/src/pages/my-content/index.tsx apps/miniapp/src/pages/post-detail/index.tsx apps/miniapp/src/pages/secondhand-detail/index.tsx
git commit -m "feat(ui): 接通静态内容详情路由"
```

### Task 7: 建立新版页面白名单与自动检查

**Files:**
- Modify: `apps/miniapp/src/app.config.ts`
- Create: `scripts/check-miniapp-static-routes.cjs`
- Modify: `package.json`

- [ ] **Step 1: 将 app.config.ts 收口到新版页面**

保留以下页面：

```ts
[
  'pages/home/index',
  'pages/search/index',
  'pages/publish-entry/index',
  'pages/publish-post/index',
  'pages/student-takeout/index',
  'pages/student-secondhand/index',
  'pages/secondhand-detail/index',
  'pages/secondhand-chat/index',
  'pages/profile/index',
  'pages/user-profile/index',
  'pages/my-content/index',
  'pages/edit-profile/index',
  'pages/settings/index',
  'pages/blacklist/index',
  'pages/legal/index',
  'pages/merchant-home/index',
  'pages/merchant-publish/index',
  'pages/merchant-wallet/index',
  'pages/merchant-messages/index',
  'pages/merchant-order-detail/index',
  'pages/delivery-rider/index',
  'pages/rider-wallet/index',
  'pages/delivery-conversation/index',
  'pages/post-detail/index'
]
```

将 `navigationBarTitleText` 改为 `湛科生活圈`。

- [ ] **Step 2: 编写路由检查脚本**

脚本读取 `apps/miniapp/src/app.config.ts`，验证：

```js
const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const src = path.join(root, 'apps', 'miniapp', 'src')
const config = fs.readFileSync(path.join(src, 'app.config.ts'), 'utf8')
const registered = new Set([...config.matchAll(/'pages\/[^']+\/index'/g)].map((match) => match[0].slice(1, -1)))

for (const route of registered) {
  const file = path.join(src, `${route}.tsx`)
  if (!fs.existsSync(file)) throw new Error(`registered page missing: ${route}`)
}

const forbidden = ['pages/market/index', 'pages/takeout/index', 'pages/tools/index', 'pages/publish/index']
for (const route of forbidden) {
  if (registered.has(route)) throw new Error(`legacy route registered: ${route}`)
}
```

继续递归扫描 `src/pages/**/*.tsx` 中字面量 `/pages/.../index`，只对新版白名单文件验证目标存在且已注册；对保留但未注册的旧源码不作失败判断。

- [ ] **Step 3: 增加 npm script**

在根 `package.json` scripts 中加入：

```json
"check:miniapp-static-routes": "node scripts/check-miniapp-static-routes.cjs"
```

- [ ] **Step 4: 运行检查并提交**

```powershell
npm run check:miniapp-static-routes
git add apps/miniapp/src/app.config.ts scripts/check-miniapp-static-routes.cjs package.json
git commit -m "chore(ui): 冻结新版静态壳路由白名单"
```

### Task 8: 完整验收与报告

**Files:**
- Create: `docs/UI-20-STATIC-SHELL-CLOSEOUT-DEVELOPMENT.md`
- Create: `docs/UI-20-STATIC-SHELL-CLOSEOUT-ACCEPTANCE.md`
- Create: `docs/UI-20-STATIC-SHELL-CLOSEOUT-RESIDUE.md`

- [ ] **Step 1: 安装隔离 worktree 依赖**

```powershell
npm install --package-lock=false
```

Expected: 不修改 `package-lock.json`。

- [ ] **Step 2: 执行自动验收**

```powershell
npm --workspace apps/miniapp run typecheck
npm run check:miniapp-static-routes
npm run build:miniapp
```

Expected: 全部 PASS；只允许既有 Sass `@import` 与 vendors 体积告警。

- [ ] **Step 3: 验证构建产物**

```powershell
Test-Path apps/miniapp/dist/app.json
Get-Content apps/miniapp/dist/app.json -Encoding utf8
```

Expected: `True`；页面清单与新版白名单一致。

- [ ] **Step 4: 验证负范围和残留**

```powershell
git diff --name-only master...HEAD -- backend apps/admin
git status --short
git ls-files | Select-String 'apps/miniap/'
```

Expected: 第一、三条无输出；提交报告前状态只含预期文档。

- [ ] **Step 5: 写入三份报告并提交**

报告必须记录基线、整合来源、路由矩阵、构建结果、旧 UI 白名单、未实现的真实 API 功能和残留状态。

```powershell
git add docs/UI-20-STATIC-SHELL-CLOSEOUT-*.md
git commit -m "docs(ui): 记录静态壳路由收口验收"
git status --short
```

Expected: 最终工作树 clean。

## Plan self-review

- 覆盖了设计中的三个既有分支整合、搜索、用户主页、配送坏路由、统一发布、内容 ID、新版白名单和共享导航。
- 没有把二手发布视觉设计或真实 API 接线偷偷纳入范围。
- 类型、路由名称和提交顺序前后一致。
- 没有 `TODO`、`TBD` 或“后续补实现”式占位步骤；明确保留的产品缺口以禁用提示处理。
