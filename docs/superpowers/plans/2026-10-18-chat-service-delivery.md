# 私信服务与配送端体验 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 让私信可先输入并对失败消息可重试，收敛服务入口，补齐工作台手势和真实资料展示，同时保持支付与外部订单的真实状态边界。

**Architecture:** 以现有 Taro/React 组件和腾讯云 IM 服务为基础，在聊天页增加本地待发送状态；以 `ChannelPager`/触控事件复用订单分组手势；服务页移除重复工具目录。后端不添加未授权的第三方适配器，现有支付、外部收件箱和通知 outbox 只做验证与状态说明。

**Tech Stack:** Taro、React、TypeScript、SCSS、Go、GORM、MySQL。

---

### Task 1: 私信后台连接与失败重试

**Files:**
- Modify: `apps/miniapp/src/pages/secondhand-chat/index.tsx`
- Modify: `apps/miniapp/src/pages/chat/index.tsx`
- Modify: `apps/miniapp/src/pages/delivery-conversation/index.tsx`
- Modify: corresponding chat SCSS files

- [ ] 让输入框不因 IM 初始化状态禁用；移除连接中展示。
- [ ] 增加本地 pending/failed 消息，成功后用云端消息合并，失败保留原消息并重试。
- [ ] 保持身份校验、会话授权和云端发送契约不变。

### Task 2: 服务页收敛

**Files:**
- Modify: `apps/miniapp/src/pages/student-takeout/index.tsx`
- Modify: `apps/miniapp/src/pages/student-takeout/index.scss`

- [ ] 删除服务页对 `ToolDirectory` 的加载与渲染。
- [ ] 保留真实合作商家目录、营业筛选、跳转失败和空态。
- [ ] 保留 `/pages/takeout/index` 的真实商品/订单流程，不变更路由和接口。

### Task 3: 工作台手势与资料

**Files:**
- Modify: `apps/miniapp/src/pages/merchant-home/index.tsx`
- Modify: `apps/miniapp/src/pages/delivery-rider/index.tsx`
- Modify: `apps/miniapp/src/pages/delivery-profile/index.tsx`
- Modify: related SCSS

- [ ] 订单分组容器增加左右滑动切换，点击标签保持兼容，边界不循环。
- [ ] 统一身份切换入口，不在导航重复呈现。
- [ ] 使用 `profile.avatar`/`profile.cover`，缺失时回退到中性首字母。

### Task 4: 验证支付、订单和通知边界

**Files:**
- Inspect/validate: `backend/internal/module/delivery/external_intake*.go`
- Inspect/validate: `backend/internal/module/admin/deliverypayment/*`
- Inspect/validate: `backend/internal/module/admin/deliverynotify/*`
- Add: `scripts/check-chat-service-delivery.cjs`

- [ ] 运行现有支付、外部收件箱、通知和 MiniApp 类型/构建检查。
- [ ] 检查没有把未授权三方平台标记为已连接。
- [ ] 记录真实接入所需的官方授权、签名和回调配置。

### Task 5: 构建与交付记录

- [ ] `npm --workspace apps/miniapp run typecheck`
- [ ] 运行范围内静态检查和 `npm run build:miniapp`
- [ ] `git diff --check`，记录隔离分支、构建产物和未完成的真实授权项。
