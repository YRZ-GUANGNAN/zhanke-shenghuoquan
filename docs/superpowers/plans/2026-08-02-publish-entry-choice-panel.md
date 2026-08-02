# 发布类型选择面板 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将发布入口从编号流程卡重做为轻量、直观的双内容选择面板，同时保持真实路由和业务行为不变。

**Architecture:** 页面继续使用 `CapsulePageHeader` 和现有 Taro 路由服务，只在发布入口内部替换展示数据与结构。图标复用现有 `AppIcon` 资源，样式限定在页面 SCSS；脚本契约先锁定新文案、真实路由和被移除的旧结构。

**Tech Stack:** React、Taro、TypeScript、SCSS、Node.js 契约脚本

---

### Task 1: 锁定新的发布入口契约

**Files:**
- Modify: `scripts/check-publish-takeout-ui.cjs`

- [ ] **Step 1: 将发布入口必需值改为新文案与真实路由**

```js
for (const required of [
  '/pages/publish-post/index',
  '/pages/publish-secondhand/index',
  '想发布什么？',
  '发帖子',
  '卖闲置',
  "name: 'comment'",
  "name: 'market'",
]) assertIncludes(publishSource, required)
```

- [ ] **Step 2: 禁止旧流程式结构回归**

```js
for (const forbidden of [
  "order: '01'",
  "order: '02'",
  'publish-entry-card__index',
  'publish-entry-card__action',
  '去发布',
  '把内容发到正确的频道',
]) assertExcludes(`${publishSource}\n${publishStyle}`, forbidden)
```

- [ ] **Step 3: 运行契约并确认先失败**

Run: `npm run check:publish-takeout-ui`

Expected: FAIL，提示缺少“想发布什么？”或仍包含旧结构。

### Task 2: 实现双内容选择面板

**Files:**
- Modify: `apps/miniapp/src/pages/publish-entry/index.tsx`
- Modify: `apps/miniapp/src/pages/publish-entry/index.scss`

- [ ] **Step 1: 使用现有图标与新内容模型重构页面**

```tsx
type PublishEntry = {
  key: 'post' | 'secondhand'
  icon: 'comment' | 'market'
  title: string
  subtitle: string
  meta: string
  url: string
}

<Text className='publish-entry-intro__title'>想发布什么？</Text>
<Text className='publish-entry-intro__desc'>选择一种内容类型开始发布</Text>
```

- [ ] **Step 2: 将入口改为两列整卡点击布局**

```scss
.publish-entry-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18rpx;
}

.publish-entry-card {
  min-height: 320rpx;
  border-radius: 30rpx;
}
```

- [ ] **Step 3: 运行聚焦验证**

Run: `npm run check:publish-takeout-ui && npm run check:miniapp-publish-contract && npm run check:miniapp-capsule-layout && npm --workspace apps/miniapp run typecheck`

Expected: 所有命令退出码为 0。

- [ ] **Step 4: 运行全量验证与构建**

Run: `npm run verify && npm run build:miniapp`

Expected: 全量验证和构建退出码为 0；仅允许仓库既有的包体积或异步分包警告。

- [ ] **Step 5: 提交实现**

```powershell
git add -- apps/miniapp/src/pages/publish-entry/index.tsx apps/miniapp/src/pages/publish-entry/index.scss scripts/check-publish-takeout-ui.cjs
git commit -m "优化：重做发布类型选择面板"
```

