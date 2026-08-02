# 发布入口细节优化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 精修发布入口的间距、比例、表面层级和交互反馈，不改变任何业务行为。

**Architecture:** 仅修改页面级 SCSS，并在既有 Node 契约中锁定双列布局和轻量卡片高度。React 结构、真实路由和点击处理保持原样。

**Tech Stack:** Taro、React、SCSS、Node.js 契约脚本

---

### Task 1: 锁定细节比例契约

**Files:**
- Modify: `scripts/check-publish-takeout-ui.cjs`

- [ ] **Step 1: 增加布局和尺寸断言**

```js
for (const required of [
  'grid-template-columns: repeat(2, minmax(0, 1fr))',
  'min-height: 304rpx',
  'min-height: 72rpx',
  'transform: scale(.985)',
]) assertIncludes(publishStyle, required)
```

- [ ] **Step 2: 运行契约并确认旧尺寸不能通过**

Run: `npm run check:publish-takeout-ui`

Expected: FAIL，提示缺少 `min-height: 304rpx`。

### Task 2: 精修发布入口样式

**Files:**
- Modify: `apps/miniapp/src/pages/publish-entry/index.scss`

- [ ] **Step 1: 收紧标题区与双卡外部节奏**

```scss
.publish-entry-intro {
  padding: 36rpx 36rpx 26rpx;
}

.publish-entry-list {
  gap: 16rpx;
  margin: 20rpx 30rpx 0;
}
```

- [ ] **Step 2: 统一卡片内部基线与触感**

```scss
.publish-entry-card {
  min-height: 304rpx;
  padding: 24rpx;
  border-radius: 28rpx;
}

.publish-entry-card__subtitle {
  min-height: 72rpx;
}

.publish-entry-card--pressed {
  transform: scale(.985);
}
```

- [ ] **Step 3: 降低辅卡表面噪声**

```scss
.publish-entry-card--secondhand {
  box-shadow: none;
}
```

- [ ] **Step 4: 运行聚焦验证**

Run: `npm run check:publish-takeout-ui && npm run check:miniapp-publish-contract && npm run check:miniapp-capsule-layout && npm --workspace apps/miniapp run typecheck`

Expected: 所有命令退出码为 0。

- [ ] **Step 5: 运行全量验证和构建**

Run: `npm run verify && npm run build:miniapp`

Expected: 命令退出码为 0；仅保留仓库既有的包体积和异步分包建议。

- [ ] **Step 6: 提交实现**

```powershell
git add -- apps/miniapp/src/pages/publish-entry/index.scss scripts/check-publish-takeout-ui.cjs
git commit -m "优化：精修发布入口细节"
```

