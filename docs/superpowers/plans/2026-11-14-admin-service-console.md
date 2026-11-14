# 服务型后台本地原型 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** 在现有管理后台项目中新增独立本地预览入口，以真实业务取舍重建服务型运营工作台。

**Architecture:** 使用现有 React 18/Vite/TypeScript 和 Zustand，在 `apps/admin/src/preview` 中建立独立数据、状态、组件和页面。`preview.html` 作为第二入口，生产 `index.html` 不修改。

**Tech Stack:** React 18、TypeScript、Vite、Zustand、CSS。

---

### Task 1: 建立预览契约检查

**Files:**
- Create: `scripts/check-admin-preview.cjs`
- Modify: `package.json`

- [x] 创建检查脚本，验证 `preview.html`、预览入口、数据 store、服务导航、通知模板、Webhook、配送、审核和设置文件存在，并验证关键中文文案。
- [x] 在根 `package.json` 增加 `check:admin-preview`。
- [x] 运行 `npm run check:admin-preview`，确认因预览文件尚不存在而失败。
- [x] 提交契约检查。

### Task 2: 建立本地数据与状态层

**Files:**
- Create: `apps/admin/src/preview/data.ts`
- Create: `apps/admin/src/preview/store.ts`

- [x] 定义订单、审核、用户、素材、通知模板、Webhook、发送记录、反馈、认证、服务入口和设置的数据类型。
- [x] 提供覆盖所有模块的种子数据，以及金额、时间、脱敏和状态工具。
- [x] 使用 Zustand `persist` 保存模板、Webhook、发送记录、订单状态、审核状态、用户状态、设置和导航修改。
- [x] 提供 `resetPreview` 操作并保留确认提示。

### Task 3: 建立视觉基础与通用组件

**Files:**
- Create: `apps/admin/src/preview/ui.tsx`
- Create: `apps/admin/src/preview/styles.css`

- [x] 实现 `StatusPill`、`StatCard`、`Panel`、`Toolbar`、`EmptyState`、`Modal`、`Drawer`、`Toast`、`Toggle`、`ProgressBar` 和 `Segmented`。
- [x] 建立深紫/薄荷/暖白设计令牌、响应式布局、键盘焦点和减弱动画支持。
- [x] 用一张临时示例页验证组件编译。

### Task 4: 建立服务型全局壳

**Files:**
- Create: `apps/admin/preview.html`
- Create: `apps/admin/src/preview/main.tsx`
- Create: `apps/admin/src/preview/App.tsx`

- [x] 实现分组侧栏、顶部命令栏、全局搜索、移动端抽屉菜单和“本地预览”标识。
- [x] 所有 15 个现有模块均可导航，不存在空路由。
- [x] 增加重置本地数据入口。
- [x] 运行 `npm run typecheck`。

### Task 5: 经营总览

**Files:**
- Create: `apps/admin/src/preview/views/DashboardView.tsx`

- [x] 实现今日订单、配送收入、待审核、通知异常等状态卡。
- [x] 实现“今日待办”与模块快捷跳转。
- [x] 实现订单趋势与渠道健康可视化。
- [x] 验证按钮可跳转到对应页面。

### Task 6: 通知工作台

**Files:**
- Create: `apps/admin/src/preview/views/NotificationsView.tsx`

- [x] 实现运行概览、消息模板、Webhook、发送记录四视图。
- [x] 实现模板新增/编辑、变量插入、实时预览、启停和测试发送。
- [x] 实现 Webhook 新增/编辑、地址校验、脱敏显示、启停和测试。
- [x] 实现发送记录筛选和失败重试。
- [x] 验证刷新后新增数据和状态保持。

### Task 7: 配送与结算

**Files:**
- Create: `apps/admin/src/preview/views/DeliveryView.tsx`

- [x] 实现筛选、汇总卡、订单表、详情抽屉。
- [x] 实现订单状态推进和骑手分配。
- [x] 实现结算与提现提示，不执行真实资金操作。

### Task 8: 审核与内容运营

**Files:**
- Create: `apps/admin/src/preview/views/OperationsView.tsx`

- [x] 实现审核中心、社区内容、二手商品、举报处理、素材中心。
- [x] 审核通过/拒绝必须填写原因并更新本地状态。
- [x] 内容下架和素材异常处理均有反馈。

### Task 9: 认证、反馈和用户

**Files:**
- Create: `apps/admin/src/preview/views/PeopleView.tsx`

- [x] 实现认证审核、意见反馈、用户与权限。
- [x] 支持状态筛选、处理说明、账号状态变更。
- [x] 所有高风险操作使用明确文案，不触发真实数据变更。

### Task 10: 平台设置与旧版外卖

**Files:**
- Create: `apps/admin/src/preview/views/SettingsView.tsx`
- Create: `apps/admin/src/preview/views/LegacyTakeoutView.tsx`

- [x] 实现服务入口新增/停用。
- [x] 实现分享设置和功能开关保存。
- [x] 实现旧版外卖只读概览与功能开关提示。

### Task 11: 全站验证与视觉收口

**Files:**
- Modify: `apps/admin/src/preview/styles.css`
- Modify: `apps/admin/src/preview/App.tsx`
- Modify: `apps/admin/src/preview/views/*.tsx`

- [x] 运行 `npm run check:admin-preview`。
- [x] 运行 `npm run typecheck`。
- [x] 运行 `npm run build:admin`。
- [x] 启动 `npm run dev:admin`，在浏览器逐项验证设计文档中的验收闭环。
- [x] 修复所有布局溢出、空态、交互死角和类型错误。
- [x] 保存最终页面截图。
- [x] 提交完整本地原型。

