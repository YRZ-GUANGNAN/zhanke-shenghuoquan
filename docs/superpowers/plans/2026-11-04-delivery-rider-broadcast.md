# 配送骑手主动接单与新单广播实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为当前 `delivery` 域增加骑手主动接单、后台可调并发上限和全校园新单广播，并让通知资格与抢单事务使用同一服务端复核。

**Architecture:** 新增配送域 availability 仓储和两张表：每个骑手一行工作状态，一行全局容量配置。`DispatchService` 通过事务型 claim port 按 availability→配置→用户/骑手资格→进行中订单→目标订单稳定锁序抢单；通知 recipient resolver 使用同一资格规则生成候选，发送前只复核当前收件人的资格和订单。仅复用既有骑手身份，不使用旧 takeout 域的工作状态、订单与容量。

**Tech Stack:** Go、GORM、MySQL 迁移、Gin、Taro TypeScript、现有 delivery notification outbox。

---

### Task 1: 迁移与领域模型

**Files:**
- Create: `backend/migrations/055_delivery_rider_availability.sql`
- Create: `backend/internal/module/delivery/rider_availability_model.go`
- Create: `backend/internal/module/delivery/rider_availability_repository.go`
- Create: `backend/migrations/migration_055_test.go`
- Create: `backend/migrations/rollback/055_delivery_rider_availability.down.sql`
- Create: `backend/internal/module/delivery/rider_availability_model_test.go`
- Create: `backend/internal/module/delivery/zz_rider_availability_mysql_test.go`

- [x] **Step 1: 添加幂等迁移**

创建 `delivery_rider_availability`（`rider_user_id` 唯一、`state`、`scope=campus`、`row_version`、时间字段）和 `delivery_rider_broadcast_config`（单行 `config_scope=global`、`max_in_flight_per_rider`、`row_version`、时间字段）。字段只使用整数/字符串，不加用户外键；配置默认 `max_in_flight_per_rider=3`，状态默认 `off_duty`，scope 固定 `campus`。为状态、scope 和配置行建立检查约束/唯一键。

- [x] **Step 2: 定义模型及校验**

定义 `RiderDutyState`、`RiderAvailability`、`RiderBroadcastConfig`，校验 `rider_user_id>0`、状态枚举、scope 为 `campus`、容量在 `1..100`、row version 和时间非零。定义 `RiderAvailabilitySnapshot`，包含当前占用数和容量。

- [x] **Step 3: 实现仓储读取/写入**

在事务中实现：按骑手 ID 创建/锁定已有 availability；按 row version CAS 切换 `on_duty/off_duty`；读取全局容量配置；按 `delivery_orders` 状态集合 `claimed,to_pickup,picked_up,delivering` 统计占用。缺表、缺配置返回 `ErrRiderAvailabilityUnavailable`，查询错误继续上抛，均不返回“可接单”。缺少状态行的 GET 返回只读 off-duty 快照（版本 0、占用为真实值），只有显式 start/stop 才创建持久行，避免首次读取的 gap lock 与并发开工互锁。资格 revision 变化时持久停工，恢复资格不自动复工。

- [x] **Step 4: 写迁移结构测试**

`migration_055_test.go` 检查表名、约束、默认容量、无外键和回滚文件存在；执行 `go test ./migrations -run 'TestMigration055' -count=1`，预期 PASS。重复执行与回滚事实保护由隔离 MySQL 测试覆盖。

### Task 2: 抢单事务与服务端路由

**Files:**
- Modify: `backend/internal/module/delivery/dispatch_service.go`
- Modify: `backend/internal/module/admin/deliveryrider/handler.go`
- Create: `backend/internal/module/admin/deliveryrider/availability.go`
- Create: `backend/internal/module/admin/deliveryrider/availability_test.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/cmd/server/delivery_runtime_boundary_test.go`
- Modify: `backend/cmd/server/delivery_http_mysql_test.go`
- Create: `backend/cmd/server/delivery_rider_availability_runtime_test.go`
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/config/config_test.go`
- Modify: `backend/.env.example`

- [x] **Step 1: 添加可选 availability port**

新增 `RiderAvailabilityRepository`：`ReadRiderAvailability`、`SetRiderDuty`、`ReadRiderBroadcastConfig`、`UpdateRiderBroadcastConfig`、`ListOnDutyRiderAvailability`；写抢单接口为 `RiderAvailabilityClaimRepository.ClaimOrderWithAvailability`。保留 `NewDispatchService*` 构造签名，用 `WithRiderAvailability` 注入；未注入的单元测试保持旧行为。生产组合根无论开关是否开启都必须注入，关闭时注入 `UnavailableRiderAvailabilityClaim`，开启但缺少迁移时拒绝启动。

- [x] **Step 2: 实现 GORM claim 事务**

按“骑手 availability 行 → 全局配置行 → 用户/骑手资格 → 进行中订单（ID 升序）→ 目标订单”顺序锁定；重验 `on_duty`、active 资格、`ready_for_claim`、`rider_user_id IS NULL`、row version 和当前 in-flight `< max_in_flight`，再执行唯一状态更新。完成/释放不反向锁 availability，保持现有订单→proof→钱包路径。返回明确的不可用、停工、满载或版本冲突错误；不在通知 worker 内改订单。

- [x] **Step 3: 暴露骑手状态与配置接口**

在 `/api/v1/delivery/rider` 增加 `GET /availability`、`POST /availability/start`、`POST /availability/stop`；响应 enabled、riderUserId、state、scope、maxInFlight、inFlight、rowVersion。后台 `/api/v1/admin/delivery/rider-config` 增加读取和行版本更新 `maxInFlightPerRider`，仅允许 `1..100`。身份、账号状态和管理员权限沿用现有中间件。

- [x] **Step 4: 接入组合根**

`main.go` 启动时验证 055 表和配置，创建 availability repository，注入 dispatch service 和 rider handler；迁移缺失时 fail-closed，不启动当前候选广播/抢单 writer。现有已分配订单查询与配送状态推进不受影响。

- [x] **Step 5: 更新服务端 HTTP/并发测试**

停工或未开始接单的骑手仍可只读查看 claimable 池；真正 claim 必须在岗且未满载，否则拒绝。功能关闭时也必须拒绝新抢单，已分配订单查询和履约推进不受影响。组合根测试覆盖关闭开关的接单硬门禁，隔离 MySQL 测试覆盖容量冲突、两个骑手并发只有一个成功，以及停止接单不取消已有订单。运行 `go test ./cmd/server -run 'TestDeliveryRuntime.*Broadcast' -count=1`、`npm run test:rider-money:mysql`。

旧 D-2 HTTP/MySQL 验收用例同步启用新开关，先断言停工只能读池不能抢单，再显式开工后参加并发抢单；清理仅删除该用例自己创建的在岗记录。该旧外部数据库用例本轮未设置 `DELIVERY_D2_MYSQL_DSN`，不计为本轮已运行；组合根 HTTP 503 测试和隔离 MySQL 并发测试已运行。

### Task 3: 通知候选与发送前复核

**Files:**
- Modify: `backend/cmd/server/user_notifications.go`
- Modify: `backend/cmd/server/user_notification_templates.go`
- Modify: `backend/cmd/server/user_notification_rider_pool_test.go`
- Create: `backend/cmd/server/user_notification_rider_pool.go`

- [x] **Step 1: 注入候选解析器**

`orderNotificationRecipients` 增加 availability port；`RiderPoolOpened` 不再直接返回空列表，改为读取当前 `ready_for_claim` 订单、候选骑手快照、用户通知偏好和 active 账号，按 rider ID 去重。范围固定 `campus`，不从地址文本推断区域。

- [x] **Step 2: 增加发送前 guard**

在 `orderSubscribeGuard` 中重新读取订单版本/状态和当前任务 `RecipientUserID` 的 availability/in-flight，不再为每个收件人重复扫描整个骑手池；仅首次事件展开调用 `ListOnDutyRiderAvailability`。订单已抢、取消、过期、骑手停工或满载时返回空收件人并将任务标为 `skipped`。不改变支付、订单和钱包事实。

- [x] **Step 3: 覆盖通知测试**

保留“缺少 availability 必须跳过”测试，增加 off-duty/满载/非 active/偏好关闭跳过、on-duty 且未满载生成候选、发送前状态变化跳过与单收件人 guard 不扫全池测试；重复事件只生成一组任务由通知隔离 MySQL 测试验证。运行 `go test ./cmd/server -run 'TestRiderPool|Test.*Notification.*Order' -count=1`。

### Task 4: 小程序接入与后台容量设置

**Files:**
- Modify: `apps/miniapp/src/services/delivery-rider.ts`
- Modify: `apps/miniapp/src/pages/delivery-rider/index.tsx`
- Modify: `apps/miniapp/src/pages/delivery-rider/index.scss`
- Modify: `apps/admin/src/DeliveryConsole.tsx`
- Modify: `apps/admin/src/services/delivery-admin.ts`
- Create: `apps/miniapp/src/hooks/use-rider-availability.ts`
- Create/Modify: `scripts/check-delivery-rider-availability.cjs`
- Modify: `scripts/check-release-regression.cjs`
- Modify: `scripts/test-delivery-stage-workbench.cjs`
- Modify: `package.json`（仅增加验收命令，不增依赖）

- [x] **Step 1: 添加服务端类型和调用**

增加 availability snapshot、start/stop 和后台 config 的严格响应规范化；缺字段、未知状态、非安全整数和请求失败均保持不可接单。

- [x] **Step 2: 在现有骑手工作台接入开始/停止接单**

沿用现有 v10 卡片和底栏，不改变订单 DOM 契约；把“可接单”状态改为服务端 availability，增加开始/停止操作、容量/占用摘要和失败重试。开始后刷新 claimable，停止不清空已有配送。

- [x] **Step 3: 后台配置组件级接入**

在现有配送管理区域增加最大同时配送单数输入、当前值、行版本冲突提示和保存反馈；不复用旧 takeout 配置页面。

- [x] **Step 4: 前端契约测试**

脚本执行真实 TypeScript parser 和 hook，覆盖 start/stop 请求、容量边界、首次状态的零值、旧服务端缺字段 fail-closed、同帧重复点击、未知 POST 结果只补 GET，以及切号/令牌变化/隐藏/卸载的迟到响应隔离。原工作台测试继续覆盖订单加载与既有履约操作。运行 `node scripts/check-delivery-rider-availability.cjs`，并纳入 `postverify`。

### Task 5: 集成验收与提交

**Files:**
- Modify: `docs/operations/2026-11-04-release-acceptance.md`
- Modify: `docs/superpowers/specs/2026-11-04-delivery-rider-availability-broadcast-design.md`（仅在实施事实需要修正时）

- [x] **Step 1: 运行定向测试**

运行 `go test ./internal/module/delivery ./cmd/server -count=1`、`npm --workspace apps/miniapp run typecheck`、`npm --workspace apps/admin run typecheck`、`node scripts/check-delivery-rider-availability.cjs`、`git diff --check`。

- [x] **Step 2: 运行现有回归**

运行 `npm run verify`、`npm run test:notification:mysql`、`npm run test:rider-money:mysql`、`npm run build:miniapp`；构建只记录现有体积/顺序警告，不改支付或通知密钥。

- [x] **Step 3: 更新验收记录并提交**

记录迁移、默认关闭/启动门禁、测试结果和未完成的真实微信送达/支付/次日结算条件。提交信息使用中文，例如 `功能：接入骑手主动接单与新单广播`；不部署、不改生产配置、不发送真实通知，直到真实账号和手机验收单独通过。
