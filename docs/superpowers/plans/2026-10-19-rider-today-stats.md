# 骑手今日收入与完成单量 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为骑手工作台接入真实的北京时间今日完成单量与骑手净收入摘要，并在统计不可用时保持订单工作台可用。

**Architecture:** 在配送域 `GormRepository` 增加只读聚合查询；`deliveryrider.Handler` 通过可选 stats reader 注册 `/stats/today`，保持既有 `NewHandler` 调用兼容。小程序服务新增独立请求，页面在现有摘要下显示两个轻量指标，订单加载与统计加载相互隔离。

**Tech Stack:** Go、Gin、GORM、MySQL、Taro React、TypeScript、SCSS、Node.js 静态契约测试。

---

### Task 1: 后端真实日统计查询与接口

**Files:**
- Modify: `backend/internal/module/delivery/fee_wallet_repository.go`
- Modify: `backend/internal/module/admin/deliveryrider/handler.go`
- Modify: `backend/internal/module/admin/deliveryrider/types.go`
- Modify: `backend/cmd/server/main.go`
- Create: `backend/internal/module/admin/deliveryrider/today_stats_test.go`

- [ ] 增加 `RiderTodayStats` 聚合类型和 `GormRepository.ReadRiderTodayStats`，接收 rider ID 与 UTC `[start,end)`，只查 `delivery_rider_earning_records`，使用 `COUNT(*)` 和 `COALESCE(SUM(rider_fee_cents),0)`。
- [ ] 在 deliveryrider handler 增加可选 `TodayStatsReader`、`WithTodayStats` 和 `WithNow`，默认 `time.Now`；`/stats/today` 先校验当前骑手身份，再用 `time.LoadLocation("Asia/Shanghai")` 计算当日边界并转 UTC。
- [ ] 新增 `TodayStatsView` 字段 `date/timezone/todayOrders/todayIncomeCents/definition`，reader 未配置返回 503，身份/查询错误沿用现有错误映射。
- [ ] 在 `registerDeliveryRoutes` 将 `runtime.riderWallets` 通过适配器传入 stats reader；不修改现有 handler 构造参数和身份守卫。
- [ ] 添加单测验证上海时区跨日窗口、当前骑手过滤、收入求和、空结果与未配置 reader。

### Task 2: 小程序服务与骑手摘要

**Files:**
- Modify: `apps/miniapp/src/services/delivery-rider.ts`
- Modify: `apps/miniapp/src/pages/delivery-rider/index.tsx`
- Modify: `apps/miniapp/src/pages/delivery-rider/index.scss`

- [ ] 新增 `DeliveryRiderTodayStats` 类型与 `getDeliveryRiderTodayStats()`，请求 `/delivery/rider/stats/today`。
- [ ] 页面新增 `todayStats` 和独立错误/加载状态；加载工作台时并行请求统计，统计失败只清空/保留指标占位，不覆盖订单错误。
- [ ] 在现有 `rider-grab-context` 后渲染 `今日单量` 与 `今日收入`，成功数据使用 `todayOrders` 和 `todayIncomeCents / 100`，未成功显示 `—`。
- [ ] 保持现有蓝色 token、横滑、下拉刷新、身份守卫和订单分页；不复用 `completedTotal`。
- [ ] 增加紧凑指标样式、长数字不溢出和 reduced-motion 规则。

### Task 3: 契约测试与验证

**Files:**
- Modify: `scripts/check-rider-workbench-clarity.cjs`
- Modify: `scripts/test-delivery-stage-workbench.cjs`（仅新增 stats mock/占位覆盖）

- [ ] 静态检查接口路径、`getDeliveryRiderTodayStats`、`todayOrders`、`todayIncomeCents` 和“—”占位；断言源码不以 `completedTotal` 渲染“今日单量”。
- [ ] 隔离测试覆盖 stats 成功、stats 失败仍显示订单、统计加载期间占位。
- [ ] 执行 `gofmt`、后端相关 `go test`、`npm --workspace apps/miniapp run typecheck`、骑手/配送静态检查、`npm run build:miniapp` 与 `git diff --check`。

### Task 4: 交付审计

- [ ] 检查 `git diff --stat` 只包含本任务文件和设计文档，不触碰用户已有脏改动。
- [ ] 报告真实数据来源、统计口径、时区、失败行为和验证结果；明确不上传微信/GitHub。
