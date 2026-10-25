# 骑手钱包与平台待打款队列 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不调用微信转账的前提下，落地骑手只读钱包、商家钱包恢复能力，以及真实持久化的待打款队列、余额占用、取消释放和线下结款并发互斥。

**Architecture:** `delivery` 领域新增队列、占用和审计三张表，所有资金写操作在数据库事务中按“预读骑手 → 锁钱包 → 锁收益 → 锁队列/占用 → 复核来源”统一锁序执行；`available_balance_cents` 保持账面口径，占用额由有效 reservation 聚合。骑手接口只读本人 DTO，Admin 只查询及幂等取消；MiniApp 沿用现有蓝色视觉和守卫，商家充值只恢复原单。生产扫描开关默认关闭，微信外发、生产迁移、部署和上传不在本计划中。

**Tech Stack:** Go、Gin、GORM、MySQL 8 migration、Taro React、TypeScript、SCSS、Node contract scripts。

---

### Task 1: 创建 050 队列、占用与审计迁移

**Files:**
- Create: `backend/migrations/050_delivery_rider_payout_queue.sql`
- Create: `backend/migrations/rollback/050_delivery_rider_payout_queue.sql`
- Create: `backend/migrations/migration_050_test.go`

- [ ] **Step 1: 写迁移失败测试**

测试必须断言三张表、唯一键、状态 CHECK、金额 CHECK、取消幂等键唯一约束、依赖顺序回滚，并拒绝任何历史 `INSERT/UPDATE` 回填：

```go
func TestMigration050CreatesQueueReservationsAndEventsWithoutBackfill(t *testing.T) {
    source := readMigration050(t, migration050File)
    for _, token := range []string{
        "CREATE TABLE delivery_rider_payout_queue",
        "UNIQUE INDEX uq_delivery_rider_payout_queue_earning",
        "CREATE TABLE delivery_rider_payout_reservations",
        "UNIQUE INDEX uq_delivery_rider_payout_reservation_earning",
        "CREATE TABLE delivery_rider_payout_events",
        "UNIQUE INDEX uq_delivery_rider_payout_event_idempotency",
    } {
        if !strings.Contains(source, token) { t.Fatalf("migration 050 missing %q", token) }
    }
}
```

- [ ] **Step 2: 运行测试并确认失败**

Run: `cd backend; go test ./migrations -run Migration050 -count=1`

Expected: FAIL，原因是 050 文件尚不存在。

- [ ] **Step 3: 编写只建结构、不回填的迁移**

```text
delivery_rider_payout_queue:
  payout_queue_public_id UNIQUE, earning_public_id UNIQUE, delivery_order_public_id,
  rider_user_id, amount_cents, currency='CNY', status blocked|queued|cancelled,
  reason_code, source_policy_version, source_evidence_hash,
  row_version, created_at, updated_at, cancelled_at

delivery_rider_payout_reservations:
  reservation_public_id UNIQUE, payout_queue_public_id UNIQUE, earning_public_id UNIQUE,
  wallet_public_id, rider_user_id, amount_cents, status active|released,
  row_version, reserved_at, released_at

delivery_rider_payout_events:
  event_public_id UNIQUE, payout_queue_public_id,
  event_type blocked|queued|cancelled|source_invalidated,
  actor_type system|admin, operator_user_id NULL, reason_code,
  idempotency_key NULL UNIQUE, created_at
```

回滚先确认三张新表为空，再按 event → reservation → queue 删除；不加跨域外键，不执行数据回填。

- [ ] **Step 4: 运行迁移测试并提交**

Run: `cd backend; go test ./migrations -run Migration050 -count=1`

Expected: PASS。

```powershell
git add backend/migrations/050_delivery_rider_payout_queue.sql backend/migrations/rollback/050_delivery_rider_payout_queue.sql backend/migrations/migration_050_test.go
git commit -m "配送：增加骑手待打款队列迁移"
```

### Task 2: 实现来源核验、整笔占用和幂等取消

**Files:**
- Create: `backend/internal/module/delivery/rider_payout_queue_model.go`
- Create: `backend/internal/module/delivery/rider_payout_queue_repository.go`
- Create: `backend/internal/module/delivery/rider_payout_queue_service.go`
- Create: `backend/internal/module/delivery/rider_payout_queue_test.go`
- Create: `backend/internal/module/delivery/zz_rider_payout_queue_mysql_test.go`

- [ ] **Step 1: 写纯领域失败测试**

覆盖 `blocked` 不占用、`queued` 必须恰有 active reservation、`cancelled` 必须已释放，金额整笔等于收益金额；`available-reserved` 下溢返回 `ErrFundingSourceMismatch`。

- [ ] **Step 2: 定义模型与仓库契约**

```go
type RiderPayoutQueueStatus string
const (
    RiderPayoutQueueBlocked RiderPayoutQueueStatus = "blocked"
    RiderPayoutQueueQueued RiderPayoutQueueStatus = "queued"
    RiderPayoutQueueCancelled RiderPayoutQueueStatus = "cancelled"
)

type RiderPayoutQueueRepository interface {
    ListRiderPayoutCandidates(context.Context, RiderPayoutCandidateCursor) ([]RiderEarningRecord, error)
    ReconcileRiderPayoutQueue(context.Context, string, time.Time) (RiderPayoutQueueResult, error)
    CancelRiderPayoutQueue(context.Context, string, uint64, uint64, string, string, time.Time) (RiderPayoutQueueResult, error)
    ReadRiderPayoutSummary(context.Context, uint64) (RiderPayoutSummary, error)
}
```

模型禁止 `processing/succeeded`；创建和取消都不更新 `RiderWallet.AvailableBalanceCents`、不创建 `RiderLedgerPayout`。

- [ ] **Step 3: 写严格来源核验**

```text
earning → delivery order → fee settlement
direct wechat: payment purpose/currency/amount/merchant/status/transaction id/refund
wallet: funding allocation → funding lot → original payment, amount conservation, no release/refund
```

仅真实微信、终态已付、主体/金额/币种一致且未退款返回 verified；mock、历史未知、混合来源、缺链、退款中/已退款和证据变化均返回白名单 reason code 的 blocked，不猜测可付款比例。

- [ ] **Step 4: 实现事务与统一锁序**

扫描单笔候选时：无锁预读收益取得 rider ID；事务内依次锁钱包、收益、来源证据、queue 和 reservation；复核收益未 `paid_offline`、余额足够且同收益不存在 active reservation。verified 创建/恢复为 `queued + active reservation + queued event`；其余创建/更新为 `blocked` 且无 reservation。收益唯一键保证重复扫描返回原记录；`cancelled` 不自动重入。

取消要求合法 queue ULID、合法幂等 ULID、`expectedRowVersion>0`、非空原因。相同键同参返回原结果；同键异参或过期版本冲突。queued 在同一事务把 reservation 改 released 并把 queue 改 cancelled；blocked 只改 queue；均只写一次审计事件。

- [ ] **Step 5: 运行单测**

Run: `cd backend; go test ./internal/module/delivery -run 'PayoutQueue|FundingVerification' -count=1`

Expected: PASS。

- [ ] **Step 6: 运行隔离 MySQL 并发测试**

Run: `$env:DELIVERY_PAYOUT_QUEUE_MYSQL_DSN='<isolated-dsn>'; cd backend; go test ./internal/module/delivery -run PayoutQueueAgainstRealMySQL -count=1`

Expected: 多 worker 重复扫描只产生一份 active 占用；取消/扫描竞争无残留；事务注入失败后三表无半成品。没有独立 DSN 时记录 SKIP/BLOCKED，禁止使用生产库。

- [ ] **Step 7: 精确提交领域实现**

```powershell
git add backend/internal/module/delivery/rider_payout_queue_model.go backend/internal/module/delivery/rider_payout_queue_repository.go backend/internal/module/delivery/rider_payout_queue_service.go backend/internal/module/delivery/rider_payout_queue_test.go backend/internal/module/delivery/zz_rider_payout_queue_mysql_test.go
git commit -m "配送：实现待打款余额占用与释放"
```

### Task 3: 让线下结款与有效占用原子互斥

**Files:**
- Modify: `backend/internal/module/delivery/fee_wallet_model.go`
- Modify: `backend/internal/module/delivery/fee_wallet_repository.go`
- Modify: `backend/internal/module/delivery/fee_wallet_repository_test.go`
- Modify: `backend/internal/module/admin/deliveryconsole/handler.go`
- Modify: `backend/internal/module/admin/deliveryconsole/handler_test.go`

- [ ] **Step 1: 写冲突和幂等失败测试**

覆盖：active reservation 时 `MarkRiderEarningPaidOffline` 返回冲突且余额/收益/流水不变；取消后可以结款；结款与入队竞争最终只有占用或 payout 之一；已完成线下结款的相同幂等请求仍返回原结果。

- [ ] **Step 2: 改为统一锁序**

`MarkRiderEarningPaidOffline` 先无锁预读 rider ID，再在事务内按钱包 → 收益 → queue/reservation 加 `FOR UPDATE`；存在 active reservation 返回 `ErrRiderPayoutReserved`，不得自动取消。完成原有扣账、payout、ledger 和收益状态更新后提交。

- [ ] **Step 3: 映射 HTTP 冲突并验收**

Admin handler 将占用、CAS 和不可取消状态映射 `409`，非法字段为 `400`，未知错误为 `500`。

Run: `cd backend; go test ./internal/module/delivery ./internal/module/admin/deliveryconsole -run 'PaidOffline|PayoutQueue' -count=1`

- [ ] **Step 4: 精确提交**

```powershell
git add backend/internal/module/delivery/fee_wallet_model.go backend/internal/module/delivery/fee_wallet_repository.go backend/internal/module/delivery/fee_wallet_repository_test.go backend/internal/module/admin/deliveryconsole/handler.go backend/internal/module/admin/deliveryconsole/handler_test.go
git commit -m "配送：互斥待打款占用与线下结款"
```

### Task 4: 增加骑手本人只读钱包 API 与默认关闭的扫描开关

**Files:**
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/config/config_test.go`
- Modify: `backend/.env.example`
- Create: `backend/internal/module/delivery/rider_wallet_query.go`
- Modify: `backend/internal/module/admin/deliveryrider/types.go`
- Modify: `backend/internal/module/admin/deliveryrider/handler.go`
- Create: `backend/internal/module/admin/deliveryrider/wallet_query_test.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: 写只读权限与 DTO 失败测试**

测试 `/wallet`、`/wallet/earnings`、`/wallet/ledger`、`/wallet/payout-queue`。未登录/非骑手拒绝；客户端传 `riderUserId` 不改变会话作用域；响应不含 operator、note、idempotency、evidence、provider payload、数据库 ID。无钱包且无收益返回 `walletState=not_created`；数据库错误或缺表不得伪装为零余额。

- [ ] **Step 2: 实现只读查询**

```go
type RiderWalletSnapshot struct {
    WalletState string
    WalletPublicID string
    AvailableBalanceCents uint64
    ReservedCents uint64
    UnverifiedCents uint64
    UpdatedAt *time.Time
}
```

查询只用 `SELECT`，不调用 `lockOrCreateRiderWalletTx`；`reservedCents=SUM(active reservations)`，`unverifiedCents` 为未结款且 blocked/尚无可核验证据的收益总额。分页端点各自维护 page/pageSize/total。

- [ ] **Step 3: 注册 handler 与路由**

复用登录、骑手身份与 active/resting 守卫，从会话读取 user ID。today 复用 `ReadRiderTodayStats`，不得由客户端金额计算。

- [ ] **Step 4: 增加默认关闭的配置**

新增 `ENABLE_DELIVERY_RIDER_PAYOUT_QUEUE=false`。只有 delivery 已开启、050 表存在且配置显式为 true 时才启动仅访问本地数据库的稳定分页扫描 worker；配置错误拒绝启动 worker，不执行迁移，不调用外部 API。

- [ ] **Step 5: 运行测试并提交**

Run: `cd backend; go test ./internal/config ./internal/module/admin/deliveryrider ./internal/module/delivery ./cmd/server -run 'RiderWallet|PayoutQueue|Config' -count=1`

```powershell
git add backend/internal/config/config.go backend/internal/config/config_test.go backend/.env.example backend/internal/module/delivery/rider_wallet_query.go backend/internal/module/admin/deliveryrider/types.go backend/internal/module/admin/deliveryrider/handler.go backend/internal/module/admin/deliveryrider/wallet_query_test.go backend/cmd/server/main.go
git commit -m "配送：接入骑手钱包只读查询与队列开关"
```

### Task 5: 落地骑手钱包页面和正式入口

**Files:**
- Create: `apps/miniapp/src/services/delivery-rider-wallet.ts`
- Modify: `apps/miniapp/src/pages/rider-wallet/index.tsx`
- Modify: `apps/miniapp/src/pages/rider-wallet/index.scss`
- Modify: `apps/miniapp/src/pages/rider-wallet/index.config.ts`
- Modify: `apps/miniapp/src/pages/delivery-profile/index.tsx`
- Modify: `apps/miniapp/src/app.config.ts`
- Modify: `scripts/check-miniapp-static-routes.cjs`
- Modify: `scripts/check-role-workbench-ui.cjs`
- Modify: `scripts/check-miniapp-business-closure.cjs`
- Modify: `scripts/test-delivery-workspace.cjs`

- [ ] **Step 1: 先更新失败契约**

新契约要求正式路由、`RuntimeSessionGate`、`useIdentityGuard('rider')`、本人 wallet/earnings/ledger/queue service、今日统计、账号/token/可见性隔离和无写操作；继续禁止遗留 `/pages/delivery-wallet/index`。

- [ ] **Step 2: 实现 service 和页面**

顺序固定：账面未结款 → 今日单量/收入 → 排队占用/来源待核验 → 收益 → 流水 → 排队记录。`queued` 显示“等待打款通道”，`blocked` 使用服务端白名单原因，`paid_offline` 显示“已登记线下结款”。不出现充值、提现、申请结款、自动打款或微信到账按钮/文案。

四个分区独立 loading/error；失败保留旧数据。复用当前返回、蓝色 token、手势保护和 44px 触控目标，不修改 `components/**`、`styles/**` 或未提交的 Taro UX 文件。

- [ ] **Step 3: 接入“我的”并验收**

骑手资金区只增加一个“收入与结算”入口并经现有 `open()`/`workspace.swipe.canActivate()` 跳转；商家入口不改。

Run: `npm run check:miniapp-static-routes; npm run check:miniapp-business-closure; npm run check:role-workbench-ui; node scripts/test-delivery-workspace.cjs; npm --workspace apps/miniapp run typecheck`

- [ ] **Step 4: 精确提交**

```powershell
git add apps/miniapp/src/services/delivery-rider-wallet.ts apps/miniapp/src/pages/rider-wallet/index.tsx apps/miniapp/src/pages/rider-wallet/index.scss apps/miniapp/src/pages/rider-wallet/index.config.ts apps/miniapp/src/pages/delivery-profile/index.tsx apps/miniapp/src/app.config.ts scripts/check-miniapp-static-routes.cjs scripts/check-role-workbench-ui.cjs scripts/check-miniapp-business-closure.cjs scripts/test-delivery-workspace.cjs
git commit -m "配送：落地骑手收入与结算页面"
```

### Task 6: 正式化商家钱包原单恢复与支付状态

**Files:**
- Create: `apps/miniapp/src/pages/merchant-wallet/wallet-state.ts`
- Modify: `apps/miniapp/src/pages/merchant-wallet/index.tsx`
- Modify: `apps/miniapp/src/pages/merchant-wallet/index.scss`
- Modify: `apps/miniapp/src/services/delivery-payment.ts`
- Modify: `apps/miniapp/src/pages/merchant-order-detail/index.tsx`
- Create: `scripts/test-wallet-recovery.cjs`

- [ ] **Step 1: 写恢复行为失败测试**

覆盖同帧双击只创建一次；取消/超时/隐藏保留原 `clientRequestId/paymentPublicId`；无 payment ID 只重放原幂等号；换账号不读取旧 key；换 token/卸载后迟到响应不写回；余额/流水分区失败保留旧值；分页去重；checking/error 不显示“未开通”。

- [ ] **Step 2: 实现账号隔离恢复对象**

storage key 使用 `delivery.wallet.recharge.${accountId}`；只保存 accountId、amountCents、clientRequestId、paymentPublicId、createdAt，不保存签名、openid 或密钥。只有后端确认 `paid/failed/closed` 才清理；未知结果查询原单或用原 key 重放。

- [ ] **Step 3: 拆分页面状态和正式标签**

余额、流水、原支付单独立请求；同步 ref 防同帧重复；流水分页按 `ledgerPublicId` 去重；使用 `deliveryPaymentStatusLabel`/`deliveryRefundStatusLabel`。能力检查中、检查失败和确实未开通三种状态分别呈现。

- [ ] **Step 4: 运行测试并提交**

Run: `node scripts/test-wallet-recovery.cjs; npm run check:delivery-stage-workbench; npm --workspace apps/miniapp run typecheck`

```powershell
git add apps/miniapp/src/pages/merchant-wallet/wallet-state.ts apps/miniapp/src/pages/merchant-wallet/index.tsx apps/miniapp/src/pages/merchant-wallet/index.scss apps/miniapp/src/services/delivery-payment.ts apps/miniapp/src/pages/merchant-order-detail/index.tsx scripts/test-wallet-recovery.cjs
git commit -m "支付：正式化商家钱包原单恢复"
```

### Task 7: 接入 Admin 队列查询、审计和幂等取消

**Files:**
- Modify: `backend/internal/module/admin/deliveryconsole/query_repository.go`
- Modify: `backend/internal/module/admin/deliveryconsole/types.go`
- Modify: `backend/internal/module/admin/deliveryconsole/types_test.go`
- Modify: `backend/internal/module/admin/deliveryconsole/handler.go`
- Modify: `backend/internal/module/admin/deliveryconsole/handler_test.go`
- Modify: `backend/internal/module/admin/deliveryconsole/http_mysql_test.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `apps/admin/src/services/delivery-admin.ts`
- Modify: `apps/admin/src/DeliveryConsole.tsx`
- Modify: `apps/admin/src/styles.css`
- Create: `apps/admin/src/utils/client-request-id.ts`
- Create: `scripts/check-rider-payout-queue-admin.cjs`

- [ ] **Step 1: 写 Admin 权限、DTO 和取消失败测试**

覆盖 list/detail/events/cancel；未登录 401、普通用户 403、管理员 200；非法 ULID/reason/version 400；过期版本/同 key 异参/已取消新请求 409；相同 key 同参返回原结果。JSON 不含 evidence 原文、幂等键、锁、租约或数据库 ID。

- [ ] **Step 2: 接入路由和 DTO**

新增 `GET /rider-payout-queue`、`GET /rider-payout-queue/:id`、`POST /rider-payout-queue/:id/cancel`，留在现有 `/api/v1/admin/delivery` 的 `AuthRequired + AdminRequired` 下。写操作只委托 delivery 领域 repository；query repository 不直接释放占用。

- [ ] **Step 3: 实现后台 UI**

新增“待打款队列”页签，展示队列/收益/订单/骑手/金额/来源核验/状态/占用/更新时间。只为 blocked/queued 提供“取消排队”，文案明确“仅占用，未调用微信转账”；不得出现批准、打款成功或微信已付。

把现有 `d6c-...` 非 ULID 生成器替换为合法 Crockford ULID，线下结款与队列取消共用；一次操作失败重试复用原 key。

- [ ] **Step 4: 运行测试并提交**

Run: `cd backend; go test ./internal/module/admin/deliveryconsole ./cmd/server -run 'PayoutQueue|DeliveryConsole' -count=1`

Run: `npm --workspace apps/admin run typecheck; node scripts/check-rider-payout-queue-admin.cjs; npm run build:admin`

```powershell
git add backend/internal/module/admin/deliveryconsole/query_repository.go backend/internal/module/admin/deliveryconsole/types.go backend/internal/module/admin/deliveryconsole/types_test.go backend/internal/module/admin/deliveryconsole/handler.go backend/internal/module/admin/deliveryconsole/handler_test.go backend/internal/module/admin/deliveryconsole/http_mysql_test.go backend/cmd/server/main.go apps/admin/src/services/delivery-admin.ts apps/admin/src/DeliveryConsole.tsx apps/admin/src/styles.css apps/admin/src/utils/client-request-id.ts scripts/check-rider-payout-queue-admin.cjs
git commit -m "后台：接入骑手待打款队列取消与审计"
```

### Task 8: 全量验收和范围审查

**Files:**
- No source changes unless a failure is directly caused by Tasks 1-7.

- [ ] **Step 1: 运行 Go 验收**

```powershell
cd backend
go test ./...
go vet ./cmd/server ./internal/module/delivery ./internal/module/admin/deliveryrider ./internal/module/admin/deliverypayment ./internal/module/admin/deliveryconsole
```

- [ ] **Step 2: 运行 MiniApp/Admin 验收**

```powershell
npm run check:miniapp-static-routes
npm run check:miniapp-business-closure
npm run check:role-workbench-ui
npm run check:delivery-stage-workbench
npm run check:private-messaging
npm --workspace apps/miniapp run typecheck
npm --workspace apps/admin run typecheck
npm run build:miniapp
npm run build:admin
git diff --check
```

- [ ] **Step 3: 审查范围和事实边界**

`git status --short --branch` 必须保留并排除用户已有 `components/**`、`styles/**`、`.codex-preview/**` 和 Taro UX 修改；禁止 `git add .`。确认未修改遗留 `delivery-wallet`、`delivery-merchant`、帖子、二手和学生端。

- [ ] **Step 4: 汇报证据**

分别报告：自动化 PASS；独立 MySQL 并发验收 PASS 或无 DSN 时 SKIP/BLOCKED；微信真实转账 PENDING；生产迁移、生产开关、部署和上传均 PENDING。不得把单元测试、内部 queued 状态或后台取消报告成真实到账。
