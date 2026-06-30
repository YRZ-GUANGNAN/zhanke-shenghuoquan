# D-3 配送单资金只读查询 Development Report

日期：2026-06-30

分支：`feat/d3-order-funds`

基线：`master@3793d91f`

实现提交：`0d0ae9f41e8ca0ee76374b5c266d5a56ab47ab06`

## 1. 范围

本补丁只新增商家按配送单读取资金全貌的能力：

```text
GET /api/v1/delivery/merchant/orders/:id/funds
```

返回该配送单的可选结算、退款和关联支付状态。未支付配送单返回显式
`settlement: null`、`refund: null`、`payment: null`。

本补丁没有修改支付、充值、钱包扣款、退款、微信回调、状态机、CAS、
配置、权限中间件、迁移或任何前端代码。

## 2. 前置核实

- `master` 为 `3793d91f`，包含 D-3 支付后端基线。
- 标签 `d3-delivery-payment-backend-baseline` 存在。
- `backend/cmd/server` 中 `db.AutoMigrate(` 命中数为 0。
- 主工作区干净。
- 既有只读端点只有 owner-scoped `GET /payments/:id`。
- `FundsRepository` 已有退款读取原语，但没有按配送单聚合结算、退款和支付的公开读取。
- D-2/D-3 的商家归属校验采用
  `delivery_order_public_id + merchant_user_id`，不存在或越权统一使用 not-found
  语义。本补丁复用了同一 owner fence。

## 3. 实现

### Repository

新增：

```go
FindOrderFundsOwned(ctx, orderPublicID, merchantUserID)
```

方法在无锁一致性读事务中执行：

1. 按配送单 public ID 和 JWT merchant user ID 读取订单；
2. 读取可选 `delivery_fee_settlements`；
3. 读取可选 `delivery_refunds`；
4. 若结算关联支付，则读取对应 `delivery_payment_attempts`。

所有查询都附带 merchant owner 条件。方法没有 `Create`、`Update`、`Delete`、
CAS、行锁或外部调用。

### Service

新增 `FundsService.OrderFunds`，只负责：

- 校验 merchant user ID；
- 校验配送单 public ID 为合法 ULID；
- 委派 Repository 只读聚合。

### HTTP / DTO

`OrderFundsView` 仅暴露：

- `deliveryOrderPublicId`
- `settlement`
- `refund`
- `payment`

内部数据库 ID、微信密钥、证书、回调原文、prepay ID 和微信 transaction ID
均未暴露。路由挂在既有 `AuthRequired` merchant group 中，并且只有
`ENABLE_DELIVERY_PAYMENT=true`、资金运行时完成装配时才会注册。

## 4. 修改文件

- `backend/internal/module/delivery/funds_repository.go`
- `backend/internal/module/delivery/funds_service.go`
- `backend/internal/module/admin/deliverypayment/handler.go`
- `backend/internal/module/admin/deliverypayment/types.go`
- `backend/cmd/server/delivery_order_funds_http_mysql_test.go`

没有新增或修改迁移，没有修改 `main.go`、`config.go`、delivery 状态机、
既有资金写方法或前端文件。

## 5. 测试设计

真实 HTTP + MySQL 8.4 测试覆盖：

- 未支付空态；
- 钱包支付后的 `settlement=paid/source=wallet`；
- 钱包退款后的 `refund=succeeded/settlement=refunded`；
- 微信退款等待回调时的 `refund=refund_pending`；
- 可选关联 payment 状态；
- 无 token 401；
- 他人配送单和不存在配送单 404；
- 支付门控关闭时路由 404；
- 连续三次 GET 前后订单、钱包、流水、支付、结算、退款完整快照一致。

## 6. 边界结论

本补丁为纯查询补丁。D-3 资金写路径、mock 隔离、状态机、CAS 和数据库契约
均保持冻结状态，未进入 D-3 前端或 D-4。
