# 商家与骑手当前工作台修复实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复商家和骑手当前工作台的封面衔接与页面级横向手势，让消息和我的可用，同时保持学生端、订单接口和外部回调契约不变。

**Architecture:** 保留现有 Taro/React 页面与 `ChannelPager` 订单分组分页；工作台根容器复用 `useHorizontalNavigation` 的方向判定和点击抑制，底部导航与手势共享可见入口顺序。封面只使用 `profile.cover` 真实媒体，样式在角色页内完成，不修改共享令牌和学生端。

**Tech Stack:** Taro、React、TypeScript、SCSS、Node 静态合同检查。

---

### Task 1: 先固定工作台导航合同

**Files:**
- Modify: `apps/miniapp/src/components/delivery-workspace-nav.tsx`
- Modify: `scripts/check-role-workbench-ui.cjs`
- Test: `scripts/check-channel-pager.cjs`

- [ ] 把商家底部入口保留为 `orders,publish,messages,mine`，但只将 `orders,messages,mine` 放入手势索引；骑手手势入口保持 `grab,tasks,messages,mine`，过滤后仍由同一数组计算当前索引。
- [ ] 保留 `select` 的路由锁和 `onSelect` 回调；手势使用同一个 `useHorizontalNavigation` 实例，边界不循环。
- [ ] 增加静态断言，确保消息和我的仍在两种角色入口中，且没有重复身份切换入口或 `navigateToMiniProgram` 直连逻辑。
- [ ] 运行 `node scripts/check-role-workbench-ui.cjs`，预期输出 `role workbench ui contract: ok`。

### Task 2: 修复商家首页根手势和封面层

**Files:**
- Modify: `apps/miniapp/src/pages/merchant-home/index.tsx`
- Modify: `apps/miniapp/src/pages/merchant-home/index.scss`

- [ ] 将 `workspace.swipe.handlers` 从只包裹 hero 改为工作台根容器事件入口；订单 `RefreshScrollView` 和按钮仍由其原生事件处理，不在滚动区添加 touchmove 拦截。
- [ ] 保留顶部真实头像、商家名称、地址和审核状态；若已有 `profile.cover`，渲染角色专属封面层并在加载失败时回退到中性底色。
- [ ] 让封面、资料摘要、订单标题和订单滚动区连续排列，封面底部不绘制黑线或额外白色圆底；底部导航保持安全区。
- [ ] 只保留已有订单分组 `ChannelPager`，确保工作台级手势与订单分组手势分别使用各自交互锁。

### Task 3: 修复骑手首页根手势和封面层

**Files:**
- Modify: `apps/miniapp/src/pages/delivery-rider/index.tsx`
- Modify: `apps/miniapp/src/pages/delivery-rider/index.scss`

- [ ] 将工作台手势从 hero 扩展到页面根容器，保留接单大厅、待取货、待送达三组现有数据和分页。
- [ ] 显示现有 `profile.avatar` 与 `profile.cover`，封面无图/失败时使用中性状态；不加入生成图片或第三方素材。
- [ ] 保留下拉刷新、订单卡片按钮和凭证/履约提交的原生触控优先级；手势只在横向位移满足现有阈值时切换入口。
- [ ] 保留骑手“接单 / 配送 / 消息 / 我的”入口，边界滑动不循环，切换身份和确认弹窗期间停用手势。

### Task 4: 对齐角色“我的”页背景和导航

**Files:**
- Modify: `apps/miniapp/src/pages/delivery-profile/index.tsx`
- Modify: `apps/miniapp/src/pages/delivery-profile/index.scss`
- Modify: `apps/miniapp/src/components/delivery-workspace-nav.scss`

- [ ] 使用与工作台首页一致的封面高度、裁切方式、遮罩和资料块衔接，保持 `Image` 错误回退与真实头像。
- [ ] 根容器继续接收工作台手势，编辑资料、钱包、身份切换按钮在手势保护期内不误触发。
- [ ] 检查底部导航安全区、选中态、触控尺寸和小屏换行，不修改学生端底栏。

### Task 5: 真实外部订单边界检查

**Files:**
- Inspect: `backend/internal/module/delivery/external_intake.go`
- Inspect: `backend/internal/module/admin/deliveryintake/handler.go`
- Inspect: `apps/miniapp/src/services/delivery-merchant.ts`
- Test: `scripts/check-external-takeout-directory.cjs`

- [ ] 确认工作台只读取已有真实配送订单，来源平台和取货号来自服务端字段；没有平台授权时保留未接入/空态。
- [ ] 确认外部回调继续使用签名、幂等和商家作用域，不在小程序内抓取淘宝、京东或美团页面。
- [ ] 运行 `node scripts/check-external-takeout-directory.cjs`，记录已有路由/接口合同，不新增第三方适配器。

### Task 6: 验证与交付

- [ ] 运行 `npm --workspace apps/miniapp run typecheck`。
- [ ] 运行 `node scripts/check-role-workbench-ui.cjs`、`node scripts/check-channel-pager.cjs`、`node scripts/test-delivery-workspace.cjs`、`node scripts/test-delivery-stage-workbench.cjs`。
- [ ] 运行 `npm run build:miniapp`，记录既有 CSS 顺序和包体积警告。
- [ ] 运行 `git diff --check`，确认只改角色工作台、导航和对应检查文件；不提交或覆盖其他用户脏改动。
- [ ] 使用当前本地预览检查商家/骑手有封面、无封面、失败回退、消息/我的手势和底部导航，最后生成新的可查看预览。
