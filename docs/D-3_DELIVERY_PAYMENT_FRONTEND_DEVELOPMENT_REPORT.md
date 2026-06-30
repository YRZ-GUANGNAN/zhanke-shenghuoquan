# D-3 配送费支付、钱包与退款前端 Development Report

## 1. 结论

D-3 小程序前端已按冻结后端契约完成，开发状态为 **Development Complete**。

实现提交：

- `19ca42b62898f53c9e06e34f7dd0b9f0adc7caa7`
- `feat(D-3): add delivery wallet payment and refund miniapp flows`

本次仅修改 `apps/miniapp`，后端、数据库迁移、配送领域状态机、Repository、封存旧外卖支付前端均为零改动。

## 2. 前置门禁与真实契约核实

- 基线：`master@32b8dabf`，包含 `d3-order-funds-readonly-baseline`。
- 基线标签：
  - `d3-order-funds-readonly-baseline`
  - `d3-delivery-payment-backend-baseline`
- 工作树在开发前干净。
- D-3 后端真实端点已逐项按代码核对：
  - `GET /api/v1/delivery/merchant/wallet`
  - `GET /api/v1/delivery/merchant/wallet/ledger`
  - `POST /api/v1/delivery/merchant/wallet/recharges`
  - `GET /api/v1/delivery/merchant/payments/:id`
  - `POST /api/v1/delivery/merchant/orders/:id/pay-with-wallet`
  - `POST /api/v1/delivery/merchant/orders/:id/pay-with-wechat`
  - `POST /api/v1/delivery/merchant/orders/:id/refunds`
  - `GET /api/v1/delivery/merchant/orders/:id/funds`
- 写请求继续使用 26 位 ULID 作为 `clientRequestId` 或 `idempotencyKey`。
- Mock Provider 在后端事务内完成支付；前端根据 `payment.provider === "mock"` 跳过 `Taro.requestPayment`，随后重新读取后端事实。
- 真实微信 Provider 仍走 `Taro.requestPayment` 与有限轮询。

## 3. 实现内容

### 3.1 配送资金 API Client

新增 `apps/miniapp/src/services/delivery-payment.ts`：

- 钱包、流水、充值、钱包支付、微信支付、支付查询、退款、资金聚合查询。
- 支付状态有限轮询：最多 30 次，每次间隔 1 秒。
- 退款状态有限轮询：最多 20 次，每次间隔 1.5 秒。
- 复用 `createDeliveryClientRequestId()` 生成幂等键。
- DTO 显式声明 payment、settlement、refund、ledger 的冻结状态集合。

### 3.2 配送钱包页

新增 `pages/delivery-wallet`：

- 展示后端钱包余额与更新时间。
- 展示 append-only 资金流水及余额快照。
- 支持分页加载更多流水。
- 支持 ¥10、¥20、¥50 快捷充值及自定义金额。
- 金额校验为 `0 < amountCents <= 10,000,000`。
- Mock Provider 不调用微信支付 UI；真实 Provider 调用 `Taro.requestPayment`。
- 充值结束后重新读取钱包与流水，不在前端增加余额。

### 3.3 配送单支付与退款

在现有配送商家页增加：

- 配送钱包入口。
- `pending_fee_payment` 状态下的钱包支付与微信支付入口。
- 支付后重新读取订单与 `GET /orders/:id/funds`。
- 资金状态展示：结算来源、支付 Provider、支付状态、退款状态。
- `ready_for_claim` 且无骑手时允许申请退款；其他已支付状态显示不可退款按钮。
- 钱包余额不足、退款规则拒绝、Provider 未配置、Provider 拒绝、409/CAS 冲突均有明确提示。
- D-2 桥接按钮显示条件严格改为：
  - `deliveryMockFeeEnabled && !deliveryPaymentEnabled`

因此 D-3 启用时不存在第二套免费确认入口。

### 3.4 构建期门控

新增：

- `TARO_APP_ENABLE_DELIVERY_PAYMENT`，默认 `false`。
- `TARO_APP_ENABLE_DELIVERY_PAYMENT_MOCK`，默认 `false`。

Mock 前端开关仅控制本地提示文字，不参与资金成功判定。成功与否始终读取后端 `provider/status/settlement/refund`。

## 4. 边界证明

- `backend/**`：零改动。
- `backend/migrations/**`：零改动。
- `internal/module/delivery`：零改动。
- 封存旧外卖前端：零改动。
- 社区、二手、媒体、IM：零改动。
- `package.json` 与 workspace 依赖：零改动。
- 未新增 Router、状态库或第三方依赖。
- 未进入 D-4。

## 5. 已知非阻断项

- 小程序构建继续出现既有 `vendors.js` 731 KiB 体积告警；本模块未新增依赖，告警不属于 D-3 功能阻断。
- 真实微信支付仍依赖后续正式商户配置；本阶段真实分支已经保留 `requestPayment + query polling`，本地验收使用严格隔离的后端 Mock Provider。
