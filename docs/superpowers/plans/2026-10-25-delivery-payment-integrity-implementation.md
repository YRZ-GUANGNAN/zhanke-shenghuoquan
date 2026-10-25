# 配送微信支付完整性 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不启用生产资金开关的前提下，让配送服务费支付与退款具备可信查单、未知结果补偿、付款人/金额归属核对和可恢复状态，为后续骑手结款提供真实资金依据。

**Architecture:** 保持 `delivery` 领域负责账本与状态转换，`admin/deliverypayment` 仅负责微信 API 签名、验签与状态归一化。所有外部调用发生在数据库事务之外；补偿器先以租约认领本地记录，再查询原商户订单号或原退款单号，最终复用同一领域入口提交结果。生产开关保持关闭，真实资金联调与代码验收分开。

**Tech Stack:** Go 1.25、Gin、GORM/MySQL 8.4、微信支付 API v3、Taro/TypeScript、Node 契约检查。

---

## Task 1: 微信查单、关单与退款查询适配器

**Files:**
- Modify: `backend/internal/module/admin/deliverypayment/provider.go`
- Modify: `backend/internal/module/admin/deliverypayment/provider_test.go`

- [x] **Step 1: 为 Provider 写失败测试**

覆盖 `QueryPayment`、`ClosePayment`、`QueryRefund`：请求必须使用原 `out_trade_no` / `out_refund_no`，所有 2xx 应答先验签；错误 `appid`、`mchid`、`trade_type`、金额、币种、付款人或业务号必须拒绝。

- [x] **Step 2: 扩展 Provider 契约**

```go
type PaymentQueryResult struct {
    OutTradeNo, WechatTransactionID, PayerOpenID string
    State string
    AmountCents uint64
    PaidAt *time.Time
}

type RefundQueryResult struct {
    OutTradeNo, OutRefundNo, WechatRefundID string
    State string
    AmountCents, TotalCents uint64
    SuccessAt *time.Time
}

type Provider interface {
    // existing methods remain
    QueryPayment(context.Context, string) (PaymentQueryResult, error)
    ClosePayment(context.Context, string) error
    QueryRefund(context.Context, string) (RefundQueryResult, error)
}
```

- [x] **Step 3: 实现官方 API v3 路径**

```text
GET  /v3/pay/transactions/out-trade-no/{out_trade_no}?mchid={mchid}
POST /v3/pay/transactions/out-trade-no/{out_trade_no}/close
GET  /v3/refund/domestic/refunds/{out_refund_no}
```

`SUCCESS` 才提供可入账支付事实；`NOTPAY`、`USERPAYING` 保持待核实；`CLOSED`、`REVOKED`、`PAYERROR` 归一化为确定终态。退款保留 `PROCESSING`、`ABNORMAL`、`CLOSED` 与 `SUCCESS`，不得用布尔值抹平。

- [x] **Step 4: 运行适配器测试**

Run: `go test ./internal/module/admin/deliverypayment`

Expected: PASS，测试仅使用 `httptest.Server` 与临时 RSA 密钥，不访问微信生产网络。

## Task 2: 支付与退款可恢复状态

**Files:**
- Create: `backend/migrations/048_delivery_payment_recovery.sql`
- Create: `backend/migrations/rollback/048_delivery_payment_recovery.sql`
- Create: `backend/migrations/migration_048_test.go`
- Modify: `backend/internal/module/delivery/funds_model.go`
- Modify: `backend/internal/module/delivery/funds_repository.go`
- Modify: `backend/internal/module/delivery/funds_service.go`
- Modify: `backend/internal/module/delivery/zz_funds_mysql_test.go`

- [x] **Step 1: 先写迁移和状态竞争失败测试**

支付记录新增付款人快照、供应商状态、下次核实时间、租约、尝试次数和脱敏错误码；退款记录新增原交易号、供应商状态及相同的补偿字段。既有记录字段允许 `NULL`，不得把历史记录猜成真实成功。

- [x] **Step 2: 添加前向迁移**

```sql
ALTER TABLE delivery_payment_attempts
  ADD COLUMN payer_openid VARCHAR(128) NULL,
  ADD COLUMN provider_status VARCHAR(32) NOT NULL DEFAULT '',
  ADD COLUMN next_verify_at DATETIME(3) NULL,
  ADD COLUMN verify_lease_until DATETIME(3) NULL,
  ADD COLUMN verify_attempt_count INT UNSIGNED NOT NULL DEFAULT 0,
  ADD COLUMN last_error_code VARCHAR(64) NOT NULL DEFAULT '';

ALTER TABLE delivery_refunds
  ADD COLUMN out_trade_no VARCHAR(64) NULL,
  ADD COLUMN provider_status VARCHAR(32) NOT NULL DEFAULT '',
  ADD COLUMN next_verify_at DATETIME(3) NULL,
  ADD COLUMN verify_lease_until DATETIME(3) NULL,
  ADD COLUMN verify_attempt_count INT UNSIGNED NOT NULL DEFAULT 0,
  ADD COLUMN last_error_code VARCHAR(64) NOT NULL DEFAULT '';
```

同时添加 `(status, next_verify_at, verify_lease_until)` 索引；回滚只移除本迁移字段和索引。迁移测试只允许独立临时库。

- [x] **Step 3: 保存付款人和退款原交易号**

`MarkPrepayIssued` 同时保存创建预支付时使用的 `openid`。创建微信退款时从关联支付记录写入 `out_trade_no`，后续回调与查询必须同时匹配退款单号、原交易号、退款金额及币种。

- [x] **Step 4: 用显式结果替换退款 success bool**

```go
type RefundObservation struct {
    OutRefundNo, OutTradeNo, WechatRefundID string
    AmountCents uint64
    ProviderStatus string
    ObservedAt time.Time
}
```

`SUCCESS` 原子完成退款；`PROCESSING` 保持锁定；`ABNORMAL` 进入人工异常；`CLOSED` 明确结束本次退款但不把钱恢复成已退款。重复和乱序观察不得覆盖可信成功。

- [x] **Step 5: 运行领域与 MySQL 测试**

Run: `go test ./internal/module/delivery`

Expected: 无隔离 DSN 时 MySQL 用例明确 SKIP；配置独立临时库时覆盖迁移 apply/rollback/apply、租约竞争、重复回调、金额/原交易号错配。

## Task 3: 补偿协调器与受限主动刷新

**Files:**
- Create: `backend/internal/module/admin/deliverypayment/reconciler.go`
- Create: `backend/internal/module/admin/deliverypayment/reconciler_test.go`
- Modify: `backend/internal/module/admin/deliverypayment/handler.go`
- Modify: `backend/internal/module/admin/deliverypayment/handler_test.go`
- Modify: `backend/cmd/server/main.go`

- [x] **Step 1: 写租约、重启恢复和查单/回调竞争测试**

同一记录只能被一个实例认领；worker 崩溃后租约到期可恢复；查询成功与回调并发只能入账一次；网络未知不得换业务号或标记失败。

- [x] **Step 2: 实现 Reconciler**

```go
type Reconciler struct {
    funds *delivery.FundsService
    provider Provider
    leaseTTL time.Duration
    now func() time.Time
}

func (r *Reconciler) Run(ctx context.Context, interval time.Duration) error
func (r *Reconciler) ReconcilePayment(ctx context.Context, paymentPublicID string) error
func (r *Reconciler) ReconcileRefund(ctx context.Context, refundPublicID string) error
```

退避有上限；明确配置错误暂停重试并记录错误码；日志和 API 不输出 OpenID、密钥、签名、原始微信应答。

- [x] **Step 3: 让商户查询触发受限核实**

`GET /delivery/merchant/payments/:id` 和订单资金详情在本地状态非终态、且达到 `next_verify_at` 时仅触发一次受限查单；HTTP 请求不等待无限重试。前端继续轮询本地事实，不相信 `requestPayment` 返回值。

- [x] **Step 4: 在服务生命周期启动和停止 worker**

仅当 `ENABLE_DELIVERY_PAYMENT=true` 且是真实 `WechatProvider` 时启动；mock、未配置或支付开关关闭时不启动。进程关闭要取消 context 并等待退出。

- [x] **Step 5: 运行 HTTP 与恢复测试**

Run: `go test ./internal/module/admin/deliverypayment ./cmd/server`

Expected: PASS；没有真实资金请求。

## Task 4: 充值来源批次与配送费消费分配

**Files:**
- Create: `backend/migrations/049_delivery_funding_sources.sql`
- Create: `backend/migrations/rollback/049_delivery_funding_sources.sql`
- Create: `backend/migrations/migration_049_test.go`
- Create: `backend/internal/module/delivery/funding_source_model.go`
- Create: `backend/internal/module/delivery/funding_source_repository.go`
- Create: `backend/internal/module/delivery/funding_source_test.go`
- Modify: `backend/internal/module/delivery/funds_repository.go`

- [x] **Step 1: 写同一充值不可重复消费的并发测试**

两个订单同时消费余额时，批次分配总额不得超过已确认充值；退款恢复原分配，不能生成新资金；mock 和历史未知余额永远不能成为 `verified`。

- [x] **Step 2: 添加资金批次与分配表**

`delivery_funding_lots` 记录真实支付来源、总额、已分配、已退款和核验状态；`delivery_funding_allocations` 记录订单结算使用的批次金额。唯一引用和金额 CHECK 保证守恒。

- [x] **Step 3: 在现有事务内分配/回滚**

微信充值成功时创建 `verified` 批次；钱包支付订单时按创建时间稳定分配；直接微信支付的订单结算直接关联原支付。历史余额保持 `unknown` 并在后台差异报告中显示，迁移不补造批次。

- [x] **Step 4: 运行资金守恒测试**

Run: `go test ./internal/module/delivery -run 'Funding|Funds'`

Expected: PASS 或仅独立数据库缺失导致的明确 SKIP。

## Task 5: 前端状态兼容与运维可见性

**Files:**
- Modify: `apps/miniapp/src/services/delivery-payment.ts`
- Modify: `apps/miniapp/src/pages/delivery-merchant/index.tsx`
- Modify: `backend/internal/module/admin/deliveryconsole/types.go`
- Modify: `backend/internal/module/admin/deliveryconsole/query_repository.go`
- Modify: `backend/internal/module/admin/deliveryconsole/handler.go`
- Modify: `apps/admin/src/services/delivery-admin.ts`
- Modify: `apps/admin/src/DeliveryConsole.tsx`

- [ ] **Step 1: 增加不误导的状态映射测试**

网络未知、微信处理中、退款异常和关闭分别显示；不得把 `prepay_issued`、`PROCESSING` 或请求返回 2xx 显示为支付/退款完成。

- [ ] **Step 2: 保持现有 UI，仅补状态与重试入口**

商家可主动刷新原单；后台查看脱敏错误、补偿积压与资金来源差异。后台不提供手工把记录改成 `paid`/`refunded` 的按钮。

- [ ] **Step 3: 运行类型和构建**

Run:

```powershell
npm --workspace apps/miniapp run typecheck
npm --workspace apps/admin run typecheck
npm run build:miniapp
npm run build:admin
```

Expected: exit 0；既有体积/CSS 顺序警告可记录但不可新增错误。

## Task 6: 安全验收与第一阶段交付

**Files:**
- Create: `docs/reviews/2026-10-25-delivery-payment-integrity.md`
- Modify: `backend/.env.example`（只增加公开配置名或注释，不写值）

- [ ] **Step 1: 跑完整自动化验收**

```powershell
npm run verify
npm run test:backend
npm run typecheck
npm run build
node scripts/check-miniapp-package-size.cjs
git diff --check
```

- [ ] **Step 2: 执行凭据与产物检查**

确认 Git index、前端源码、构建产物和日志不含商户私钥、APIv3 Key、AppSecret、真实 OpenID。生产密钥只使用服务器安全配置和受限文件权限。

- [ ] **Step 3: 记录边界并提交**

精确暂存本阶段支付文件，中文提交。报告自动化 PASS、数据库 SKIP/PASS、真实微信联调 PENDING。未完成小额真实支付、退款和账单核对前保持 `ENABLE_DELIVERY_PAYMENT=false`，不得部署为已启用支付。

## 后续独立计划

完成本计划后再分别执行：

1. `2026-10-24-rider-auto-payout-design.md`：骑手提现、默认自动审核、人工备用和商家转账。
2. `2026-10-24-delivery-notification-design.md`：站内事件、公众号绑定与通知渠道。
3. 生产备份、前向迁移、灰度开关、小额真实联调、账单对账、后端部署和小程序上传。代码上传、微信审核、正式发布与真机资金验收分别报告。

