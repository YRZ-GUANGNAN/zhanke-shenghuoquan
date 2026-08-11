# 首页“发现”统一卡片 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将首页发现页的帖子与二手商品统一为同宽双列卡片，并为真实图片、无图和破图提供一致的媒体区。

**Architecture:** 保留现有两路分页请求、交错合并与详情路由，只收敛渲染层。`cards` 直接拆为双列，单一 `renderDiscoverCard` 根据真实图片可用性切换图片或诚实无图状态。

**Tech Stack:** React 18、Taro 4.2、TypeScript、SCSS、Node.js 契约检查

---

### Task 1: 更新首页卡片契约

**Files:**
- Modify: `scripts/check-home-feed-card-contract.cjs`

- [x] **Step 1: 把断言改为统一网格契约**

要求源码不再包含 `student-text-stream` 与 `student-text-card`，要求 `splitCards(cards)`、`student-media-card--no-image`、4:3 比例和底部安全间距存在。

- [x] **Step 2: 运行契约并确认旧实现失败**

Run: `npm run check:home-feed-card-contract`

Expected: FAIL，指出旧的全宽文字卡或 4:5 比例仍存在。

### Task 2: 合并发现页渲染分支

**Files:**
- Modify: `apps/miniapp/src/pages/home/index.tsx`

- [x] **Step 1: 删除文字卡与媒体卡分组**

删除 `textCards`、`mediaCards` 与 `renderTextCard`，使用：

```ts
const cardColumns = useMemo(() => splitCards(cards), [cards])
```

- [x] **Step 2: 实现单一卡片渲染**

`renderDiscoverCard` 保留 `failedImageKeys` 回退；无图时增加 `student-media-card--no-image`，显示帖子/二手类型和真实 `mediaLabel`。标题、作者、头像与互动数始终位于同一结构中；二手卡额外显示真实价格、在售状态和星形收藏数。

- [x] **Step 3: 统一双列输出**

两列分别渲染 `cardColumns.left` 和 `cardColumns.right`，不再先输出全宽文字流。

### Task 3: 收敛视觉比例与底部安全区

**Files:**
- Modify: `apps/miniapp/src/pages/home/index.scss`

- [x] **Step 1: 删除旧文字流样式**

移除 `.student-text-stream` 与 `.student-text-card*`，避免两套卡片外观继续共存。

- [x] **Step 2: 调整统一卡片**

将媒体区设为 `aspect-ratio: 4 / 3`；无图区使用低饱和背景、类型标签与分类文字，标题仍在卡片正文中。统一圆角、边框、标题行高和按压反馈。

- [x] **Step 3: 增加安全间距**

为包含信息流与加载反馈的 `.student-discover-content` 增加覆盖固定底部导航与安全区的下内边距，移除无效的小型末尾占位。

### Task 4: 验证与提交

**Files:**
- Test: `scripts/check-home-feed-card-contract.cjs`
- Test: `apps/miniapp/src/pages/home/index.tsx`
- Test: `apps/miniapp/src/pages/home/index.scss`

- [x] **Step 1: 运行聚焦契约**

Run: `npm run check:home-feed-card-contract && npm run check:miniapp-regression-fixes && npm run check:miniapp-no-fake-pages`

Expected: 三项检查全部退出码 0。

- [x] **Step 2: 运行类型检查与构建**

Run: `npm --workspace apps/miniapp run typecheck && npm run build:miniapp`

Expected: TypeScript 与微信小程序构建成功；仅允许既有包体积提示。

- [x] **Step 3: 审核差异并提交**

Run: `git diff --check && git status --short`

只暂存本计划列出的文件，保留用户原有未跟踪文档；提交信息使用 `优化：首页发现卡片统一呈现`。

### Final audit refinements

- [x] 相邻行反向分配左右列，避免帖子与二手商品长期固化成两种类型列。
- [x] 帖子话题与商品价格共用等高辅助信息槽，减少卡片高度漂移。
- [x] 图片失败键包含当前 URL，图片地址更新后允许重新加载。
- [x] 将底部安全区放到信息流与加载反馈的共同容器，避免状态提示被固定导航遮挡。
