# 学生端外卖、资料头部与头像闭环 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 精修学生外卖页和“我的”资料头部，并验证头像通过真实媒体审核与用户资料接口跨页面、跨会话回显。

**Architecture:** 保留 `external_takeout_merchants` 目录服务和现有页面路由，只调整页面表达层。头像继续复用 `services/media.ts` 的受控上传与审核，`services/a-user.ts` 负责 `PATCH /auth/me` 和 `GET /auth/me`，编辑页在保存后以服务端资料为最终真相。

**Tech Stack:** Taro 4、React 18、TypeScript、SCSS、Zustand、Go/Gin、现有 COS 媒体管线。

---

## 文件结构

- `scripts/check-student-entry-avatar-contract.cjs`：新增静态合同检查，约束真实外卖目录与头像闭环。
- `package.json`：把新检查接入独立脚本与完整 `verify`。
- `apps/miniapp/src/pages/student-takeout/index.tsx`：调整外卖页首屏、提示和状态结构。
- `apps/miniapp/src/pages/student-takeout/index.scss`：实现紧凑首屏、轻提示和诚实空状态。
- `apps/miniapp/src/pages/profile/index.tsx`：补充资料头部可读性和头像状态表达。
- `apps/miniapp/src/pages/profile/index.scss`：优化头像、名称、校园信息与编辑入口比例。
- `apps/miniapp/src/pages/edit-profile/index.tsx`：保存后重新读取后端用户资料，再更新会话。
- `apps/miniapp/src/pages/edit-profile/index.scss`：明确头像上传、审核和可保存状态。

### Task 1: 建立失败的头像与页面合同检查

**Files:**
- Create: `scripts/check-student-entry-avatar-contract.cjs`
- Modify: `package.json`

- [ ] **Step 1: 写入失败检查**

检查必须断言：外卖页仍调用 `listExternalTakeoutMerchants` 且不含虚构商家；编辑资料页调用 `chooseAndUploadImages('avatar'`、`waitForMediaApproval`、`updateProfile`、`currentUser`；资料页使用服务端 `profile.avatar`；新页面类名存在。

- [ ] **Step 2: 运行检查确认失败**

Run: `node scripts/check-student-entry-avatar-contract.cjs`

Expected: 因 `currentUser` 刷新或新结构类名尚不存在而失败。

- [ ] **Step 3: 接入完整验证**

在 `package.json` 增加 `check:student-entry-avatar-contract`，并放入 `verify` 的界面合同检查序列。

- [ ] **Step 4: 提交测试约束**

Run: `git add scripts/check-student-entry-avatar-contract.cjs package.json && git commit -m "测试：约束学生入口与头像闭环"`

### Task 2: 完成头像服务端真相闭环

**Files:**
- Modify: `apps/miniapp/src/pages/edit-profile/index.tsx`
- Modify: `apps/miniapp/src/pages/edit-profile/index.scss`

- [ ] **Step 1: 保存后读取正式资料**

从 `services/a-user.ts` 引入 `currentUser`。`updateProfile` 成功后调用 `currentUser()`，以读取结果更新 `useSessionStore`；读取瞬时失败时使用 PATCH 返回值，避免保存成功被误报为失败。

- [ ] **Step 2: 明确头像状态**

选择期间禁用重复点击；显示“上传中 / 审核中 / 已通过，保存后生效”；只有审核通过的 `assetId` 进入 PATCH。任何失败清空待保存头像并保留旧头像。

- [ ] **Step 3: 运行合同检查和类型检查**

Run: `node scripts/check-student-entry-avatar-contract.cjs && npm --workspace apps/miniapp run typecheck`

Expected: PASS。

- [ ] **Step 4: 提交头像闭环**

Run: `git add apps/miniapp/src/pages/edit-profile/index.tsx apps/miniapp/src/pages/edit-profile/index.scss && git commit -m "修复：打通头像保存与回显闭环"`

### Task 3: 精修外卖页首屏和空状态

**Files:**
- Modify: `apps/miniapp/src/pages/student-takeout/index.tsx`
- Modify: `apps/miniapp/src/pages/student-takeout/index.scss`

- [ ] **Step 1: 重组首屏信息**

保持 `CapsulePageHeader` 与搜索路由，把目录标题、动态数量和说明合并为一个可读区，避免标题、搜索和副标题互相挤压。

- [ ] **Step 2: 优化提示和空状态**

把第三方跳转说明改为轻量提示；空状态使用纯 CSS 目录图形、标题与说明，加载错误保持独立重试态；不增加图片和商家数据。

- [ ] **Step 3: 验证真实目录边界**

Run: `npm run check:external-takeout-directory && node scripts/check-student-entry-avatar-contract.cjs`

Expected: PASS，且不存在硬编码商家。

- [ ] **Step 4: 提交外卖页精修**

Run: `git add apps/miniapp/src/pages/student-takeout/index.tsx apps/miniapp/src/pages/student-takeout/index.scss && git commit -m "优化：精修学生外卖目录入口"`

### Task 4: 精修“我的”资料头部

**Files:**
- Modify: `apps/miniapp/src/pages/profile/index.tsx`
- Modify: `apps/miniapp/src/pages/profile/index.scss`

- [ ] **Step 1: 调整资料语义层级**

保留头像、昵称、身份、用户号、学校/校区和编辑入口；将学校与校区组合成独立副信息，避免“未设置”重复和文本拥挤。

- [ ] **Step 2: 调整视觉比例**

头像使用圆形正式裁切，昵称和身份标签保持一行，编辑入口降噪；为长昵称、长学校名和无头像兜底设置溢出规则。

- [ ] **Step 3: 运行资料页合同检查**

Run: `npm run check:profile-center-ui && node scripts/check-student-entry-avatar-contract.cjs && npm --workspace apps/miniapp run typecheck`

Expected: PASS。

- [ ] **Step 4: 提交资料头部精修**

Run: `git add apps/miniapp/src/pages/profile/index.tsx apps/miniapp/src/pages/profile/index.scss && git commit -m "优化：重整个人资料头部层级"`

### Task 5: 全量验证与候选包

**Files:**
- Verify only: `apps/miniapp/src/**`
- Create outside repository: `D:\Codex-build-verify\campus-circle-rc-<commit>\miniapp-upload`

- [ ] **Step 1: 运行完整验证**

Run: `npm run verify`

Expected: 所有合同检查、类型检查和安全后端测试通过。

- [ ] **Step 2: 运行生产构建**

Run: `npm run build:miniapp`

Expected: Taro 微信小程序生产构建成功；只记录已有体积警告，不把警告冒充失败。

- [ ] **Step 3: 生成唯一候选目录与 ZIP**

复制 `apps/miniapp/dist` 到带当前提交短哈希的独立目录并生成 ZIP；不得覆盖此前候选包。

- [ ] **Step 4: 校验候选包**

Run: 对候选目录执行文件清单、`project.config.json` AppID、ZIP SHA-256 和源构建目录哈希对比。

Expected: 哈希一致，AppID 保持项目配置值，工作区无新增未提交业务改动。
