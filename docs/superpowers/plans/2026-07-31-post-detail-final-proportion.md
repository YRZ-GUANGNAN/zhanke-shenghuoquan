# 帖子详情最终比例收口 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 收敛帖子详情与“我的内容”二级页比例，使 Android/iPhone 真机在不触碰微信胶囊的前提下呈现紧凑、清晰、可操作的最终界面。

**Architecture:** 保留 `CapsulePageHeader` 的全局几何契约，仅在 `post-detail` 页面定义局部业务行、内容、评论空态与操作栏比例。通过静态回归契约锁定关键类名、尺寸上限和真实数据边界，再运行 Taro 类型检查及微信小程序构建验证。

**Tech Stack:** Taro 3、React、TypeScript、SCSS、Node.js 静态契约、微信小程序。

---

### Task 1: 锁定比例回归契约

**Files:**
- Modify: `scripts/check-miniapp-regression-contract.cjs`
- Test: `scripts/check-miniapp-regression-contract.cjs`

- [ ] **Step 1: 添加失败断言**

为作者业务行、紧凑关注按钮、无评论空态和底栏动作结构添加类名及样式断言；禁止关注按钮恢复到 72×44px，禁止无评论只渲染裸文本。

- [ ] **Step 2: 验证断言先失败**

Run: `node scripts/check-miniapp-regression-contract.cjs`

Expected: FAIL，提示缺少紧凑关注按钮或评论空态。

- [ ] **Step 3: 保留失败输出作为实施边界**

只修改帖子详情与“我的内容”相关文件，不借测试失败扩张到后端、首页或聊天业务。

### Task 2: 重构帖子详情视觉层级

**Files:**
- Modify: `apps/miniapp/src/pages/post-detail/index.tsx`
- Modify: `apps/miniapp/src/pages/post-detail/index.scss`
- Test: `scripts/check-miniapp-regression-contract.cjs`

- [ ] **Step 1: 增加无评论语义空态**

当 `comments.length === 0` 时渲染 `post-detail-comments__empty`，包含“还没有评论”和“说说你的看法，成为第一个评论的人”；接口错误仍保留独立重试态。

- [ ] **Step 2: 收敛作者与关注比例**

将关注按钮拆为 88rpx 点击容器和 116×64rpx 视觉胶囊；作者头像、昵称、学校信息按 64rpx/28rpx/22rpx 设置，并保留文本截断。这里显式使用 `rpx`，规避 Taro 把源码 `px` 自动转换后造成的真机折半。

- [ ] **Step 3: 收敛正文与评论比例**

标题调整为 36rpx/50rpx，正文调整为 29rpx/46rpx，无图顶部调整为 24rpx；评论空态采用最大 176rpx 的紧凑卡片。

- [ ] **Step 4: 收敛底部操作栏**

输入框高度调整为 64rpx；点赞、收藏、评论均使用 72rpx 宽的纵向动作项和 88rpx 触控目标，避免数字与图标挤压。

- [ ] **Step 5: 运行回归契约**

Run: `node scripts/check-miniapp-regression-contract.cjs`

Expected: `miniapp regression contract: PASS`

### Task 3: 收敛“我的内容”列表密度

**Files:**
- Modify: `apps/miniapp/src/pages/my-content/index.scss`
- Test: `scripts/check-miniapp-regression-contract.cjs`

- [ ] **Step 1: 调整卡片与缩略图**

行最小高度调整为 144rpx，缩略图调整为 104rpx，间距调整为 16rpx，标题保持两行截断。

- [ ] **Step 2: 调整状态与空态**

状态胶囊使用 20rpx 字号并限制宽度；空态高度调整为 260rpx，避免只有一条内容时页面显得空旷。

- [ ] **Step 3: 再次运行回归契约**

Run: `node scripts/check-miniapp-regression-contract.cjs`

Expected: `miniapp regression contract: PASS`

### Task 4: 完整验证与交付

**Files:**
- Verify: `apps/miniapp`
- Verify: `scripts`

- [ ] **Step 1: 运行类型检查**

Run: `npm.cmd --workspace apps/miniapp run typecheck`

Expected: exit code 0。

- [ ] **Step 2: 构建微信小程序**

Run: `npm.cmd --workspace apps/miniapp run build:weapp`

Expected: 编译成功并生成 `apps/miniapp/dist`。

- [ ] **Step 3: 运行专项检查**

Run: `npm.cmd run check:miniapp-capsule-layout`

Run: `npm.cmd run check:miniapp-static-routes`

Run: `npm.cmd run check:miniapp-business-closure`

Run: `npm.cmd run check:miniapp-regression-fixes`

Expected: 所有命令 exit code 0。

- [ ] **Step 4: 检查补丁格式**

Run: `git diff --check -- apps/miniapp/src/pages/post-detail apps/miniapp/src/pages/my-content scripts/check-miniapp-regression-contract.cjs`

Expected: 无输出。

- [ ] **Step 5: 微信开发者工具复核**

编译后检查帖子详情无图、有图、无评论、长标题和“我的内容”四标签；确认胶囊无重叠、内容无溢出、动作可点击。

- [ ] **Step 6: 交付用户上传**

不调用微信上传 CLI。提供建议版本号、版本描述、验证结果和用户只需执行的上传步骤。
