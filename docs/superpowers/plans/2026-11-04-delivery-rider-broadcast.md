# 配送骑手主动接单与新单广播实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为当前 `delivery` 域增加骑手主动接单、后台可调并发上限和全校园新单广播，并让通知资格与抢单事务使用同一服务端复核。

**Architecture:** 新增配送域 availability 仓储和两张表：每个骑手一行工作状态/容量，一行全局容量配置。`DispatchService` 通过可选的事务型 claim port 在钱包式稳定锁序下锁 availability 与订单；通知 recipient resolver 通过同一资格 port 生成候选，发送前再次校验订单版本和骑手容量。旧 takeout 域不参与。

**Tech Stack:** Go、GORM、MySQL 迁移、Gin、Taro TypeScript、现有 delivery notification outbox。

---

### Task 1: 迁移与领域模型

**Files:**
- Create: `backend/migrations/055_delivery_rider_availability.sql`
- Create: `backend/internal/module/delivery/rider_availability_model.go`
- Create: `backend/internal/module/delivery/rider_availability_repository.go`
- Create: `backend/internal/module/delivery/migration_055_test.go`

- [ ] **Step 1: 添加幂等迁移**

创建 `delivery_rider_availability`（`rider_user_id` 唯一、`state`、`scope`、`max_in_flight`、`row_version`、时间字段）和 `delivery_rider_broadcast_config`（单行 `config_scope=global`、`max_in_flight_per_rider`、`row_version`、时间字段）。字段只使用整数/字符串，不加用户外键；配置默认 `max_in_flight_per_rider=3`，状态默认 `off_duty`，scope 固定 `campus`。为状态、scope 和配置行建立检查约束/唯一键。

- [ ] **Step 2: 定义模型及校验**

定义 `RiderDutyState`、`RiderAvailability`、`RiderBroadcastConfig`，校验 `rider_user_id>0`、状态枚举、scope 为 `campus`、容量在 `1..100`、row version 和时间非零。定义 `RiderAvailabilitySnapshot`，包含当前占用数和容量。

- [ ] **Step 3: 实现仓储读取/写入**

在事务中实现：按骑手 ID `FOR UPDATE` 创建/读取 availability；按 row version CAS 切换 `on_duty/off_duty`；读取全局容量配置；按 `delivery_orders` 状态集合 `claimed,to_pickup,picked_up,delivering` 统计占用。缺表、缺配置和查询错误统一返回 `ErrRiderAvailabilityUnavailable`，不返回“可接单”。

- [ ] **Step 4: 写迁移结构测试**

`migration_055_test.go` 检查表名、约束、默认容量、无外键、重复执行安全和回滚文件存在；执行 `go test ./internal/module/delivery -run 'TestMigration055' -count=1`，预期 PASS。

### Task 2: 抢单事务与服务端路由

**Files:**
- Modify: `backend/internal/module/delivery/dispatch_service.go`
- Modify: `backend/internal/module/delivery/dispatch_repository.go`
- Modify: `backend/internal/module/admin/deliveryrider/handler.go`
- Modify: `backend/internal/module/admin/deliveryrider/types.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/internal/module/delivery/dispatch_boundary_test.go`

- [ ] **Step 1: 添加可选 availability port**

新增 `RiderAvailabilityPort`：`Read(ctx,riderID) (Snapshot,error)`、`SetDuty(ctx,riderID,state,expectedVersion) (Snapshot,error)`、`Config(ctx) (Config,error)`、`Claim(ctx,riderID,orderID,expectedOrderVersion,now) (Order,error)`、`Candidates(ctx,now) ([]Snapshot,error)`。保留 `NewDispatchService*` 构造签名，用 `WithRiderAvailability` 注入；未注入的单元测试保持旧行为，生产组合根必须注入并在缺表时启动失败。

- [ ] **Step 2: 实现 GORM claim 事务**

按“骑手 availability 行 → 全局配置行 → 订单行”顺序锁定；重验 `on_duty`、active 资格、`ready_for_claim`、`rider_user_id IS NULL`、row version 和当前 in-flight `< max_in_flight`，再执行唯一状态更新。返回 `ErrRiderAvailabilityUnavailable`、`ErrRiderAtCapacity` 或 `ErrConcurrentUpdate`；不在通知 worker 内改订单。

- [ ] **Step 3: 暴露骑手状态与配置接口**

在 `/api/v1/delivery/rider` 增加 `GET /availability`、`POST /availability/start`、`POST /availability/stop`；响应状态、scope、capacity、inFlight、rowVersion。后台 `/api/v1/admin/delivery/rider-config` 增加读取和行版本更新 `maxInFlightPerRider`，仅允许 `1..100`。身份、账号状态和管理员权限沿用现有中间件。

- [ ] **Step 4: 接入组合根**

`main.go` 启动时验证 055 表和配置，创建 availability repository，注入 dispatch service 和 rider handler；迁移缺失时 fail-closed，不启动当前候选广播/抢单 writer。现有已分配订单查询与配送状态推进不受影响。

- [ ] **Step 5: 更新服务端 HTTP/并发测试**

扩展 `delivery_http_mysql_test.go`：未开始接单的 claimable/claim 被拒绝；开始后可见；同一骑手超过容量返回 409；两个骑手并发只有一个成功；停止接单不取消已有订单。运行对应 `go test ./cmd/server -run 'Test.*Delivery.*Availability|Test.*Delivery.*Claim' -count=1`。

### Task 3: 通知候选与发送前复核

**Files:**
- Modify: `backend/cmd/server/user_notifications.go`
- Modify: `backend/cmd/server/user_notification_templates.go`
- Modify: `backend/cmd/server/user_notification_rider_pool_test.go`
- Modify: `backend/cmd/server/user_notifications_test.go`

- [ ] **Step 1: 注入候选解析器**

`orderNotificationRecipients` 增加 availability port；`RiderPoolOpened` 不再直接返回空列表，改为读取当前 `ready_for_claim` 订单、候选骑手快照、用户通知偏好和 active 账号，按 rider ID 去重。范围固定 `campus`，不从地址文本推断区域。

- [ ] **Step 2: 增加发送前 guard**

在 `orderSubscribeGuard` 中重新读取订单版本/状态和骑手 availability/in-flight；订单已抢、取消、过期、骑手停工或满载时返回空收件人并将任务标为 `skipped`。不改变支付、订单和钱包事实。

- [ ] **Step 3: 覆盖通知测试**

更新原“缺少 availability 必须跳过”测试为：off-duty/满载/非 active/偏好关闭跳过，on-duty 且未满载生成候选，发送前状态变化跳过；重复事件只生成一组任务。运行 `go test ./cmd/server -run 'Test.*Notification.*Rider|Test.*Notification.*Order' -count=1`。

### Task 4: 小程序接入与后台容量设置

**Files:**
- Modify: `apps/miniapp/src/services/delivery-rider.ts`
- Modify: `apps/miniapp/src/pages/delivery-rider/index.tsx`
- Modify: `apps/miniapp/src/pages/delivery-rider/index.scss`
- Modify: `apps/admin/src/DeliveryConsole.tsx`
- Modify: `apps/admin/src/services.ts`（若现有服务集中定义接口）
- Create/Modify: `scripts/check-delivery-rider-availability.cjs`

- [ ] **Step 1: 添加服务端类型和调用**

增加 availability snapshot、start/stop 和后台 config 的严格响应规范化；缺字段、未知状态、非安全整数和请求失败均保持不可接单。

- [ ] **Step 2: 在现有骑手工作台接入开始/停止接单**

沿用现有 v10 卡片和底栏，不改变订单 DOM 契约；把“可接单”状态改为服务端 availability，增加开始/停止操作、容量/占用摘要和失败重试。开始后刷新 claimable，停止不清空已有配送。

- [ ] **Step 3: 后台配置组件级接入**

在现有配送管理区域增加最大同时配送单数输入、当前值、行版本冲突提示和保存反馈；不复用旧 takeout 配置页面。

- [ ] **Step 4: 前端契约测试**

脚本覆盖 start/stop 请求、容量边界、旧服务端缺字段 fail-closed、开始后刷新接单池和停止保留已有订单。运行 `node scripts/check-delivery-rider-availability.cjs`。

### Task 5: 集成验收与提交

**Files:**
- Modify: `docs/operations/2026-11-04-release-acceptance.md`
- Modify: `docs/superpowers/specs/2026-11-04-delivery-rider-availability-broadcast-design.md`（仅在实施事实需要修正时）

- [ ] **Step 1: 运行定向测试**

运行 `go test ./internal/module/delivery ./cmd/server -count=1`、`npm --workspace apps/miniapp run typecheck`、`npm --workspace apps/admin run typecheck`、`node scripts/check-delivery-rider-availability.cjs`、`git diff --check`。

- [ ] **Step 2: 运行现有回归**

运行 `npm run verify`、`npm run test:notification:mysql`、`npm run test:rider-money:mysql`、`npm run build:miniapp`；构建只记录现有体积/顺序警告，不改支付或通知密钥。

- [ ] **Step 3: 更新验收记录并提交**

记录迁移、默认关闭/启动门禁、测试结果和未完成的真实微信送达/支付/次日结算条件。提交信息使用中文，例如 `功能：接入骑手主动接单与新单广播`；不部署、不改生产配置、不发送真实通知，直到真实账号和手机验收单独通过。
