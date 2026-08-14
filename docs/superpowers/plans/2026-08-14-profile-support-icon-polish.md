# Profile Support Icon Polish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 统一“我的”页面账户与支持四枚图标的网格、线宽、尺寸与视觉重心，重点修复设置齿轮。

**Architecture:** 保留现有 `AppIcon` 组件与图标名称映射，只替换四个专用 SVG 资源并微调资料页局部尺寸。通过现有 `check-profile-center-ui.cjs` 增加资产级契约，避免后续重新混用不同网格或线宽。

**Tech Stack:** Taro 4、React、SCSS、SVG、Node.js 静态契约

---

### Task 1: 锁定统一图标契约

**Files:**
- Modify: `scripts/check-profile-center-ui.cjs`

- [ ] **Step 1: 写入失败契约**

为 `action-settings.svg`、`action-document.svg`、`action-feedback.svg`、`action-info.svg` 增加断言：统一 `viewBox="0 0 48 48"`、`stroke-width="3.4"`、`stroke-linecap="round"` 和 `stroke-linejoin="round"`，且不使用文字、位图或外部引用；同时确认资料页仍映射 `settings/document/feedback/info`，`AppIcon` 仍导入并注册这些资源。

- [ ] **Step 2: 运行失败检查**

Run: `npm run check:profile-center-ui`

Expected: FAIL，因为现有 SVG 尚未统一圆角描边属性，设置齿轮仍混用 3.2/3.4 描边。

### Task 2: 重绘四枚专用图标并调整光学尺寸

**Files:**
- Modify: `apps/miniapp/src/assets/icons/action-settings.svg`
- Modify: `apps/miniapp/src/assets/icons/action-document.svg`
- Modify: `apps/miniapp/src/assets/icons/action-feedback.svg`
- Modify: `apps/miniapp/src/assets/icons/action-info.svg`

- [ ] **Step 1: 替换 SVG 路径**

使用同一 48×48 网格、`#2E7DD4` 颜色、3.4 圆角描边规范四枚图标；校正设置齿轮外轮廓中心，其余图标保持文档、对话与信息语义。

- [ ] **Step 2: 运行专项验证**

Run: `npm run check:profile-center-ui`

Expected: PASS，输出 `profile center ui: ok`。

Run: `npm --workspace apps/miniapp run typecheck`

Expected: PASS，无 TypeScript 错误。

### Task 3: 构建与交付边界验证

**Files:**
- Verify only: `apps/miniapp/dist/`

- [ ] **Step 1: 构建小程序**

Run: `npm run build:miniapp`

Expected: PASS，构建产物包含更新后的四枚 SVG；`dist` 仅用于本机审查，不提交。

- [ ] **Step 2: 检查差异边界**

Run: `git diff --check`

Expected: PASS；提交仅包含设计、计划、四枚 SVG 和专项契约，不包含用户自有未跟踪文件。

- [ ] **Step 3: 白名单提交**

逐文件暂存本计划列出的文件并使用中文提交信息，禁止 `git add .`。
