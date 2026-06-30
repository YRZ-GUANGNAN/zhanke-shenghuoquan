# D-3 配送单资金只读查询 Acceptance Report

日期：2026-06-30

实现提交：`0d0ae9f41e8ca0ee76374b5c266d5a56ab47ab06`

结论：**PASS**

## 1. 验收矩阵

| 项目 | 结果 | 证据 |
|---|---|---|
| 未支付空态 | PASS | HTTP 200，三个资金投影字段显式为 `null` |
| 钱包支付 | PASS | `settlement.status=paid`、`source=wallet`、金额 150 分 |
| 钱包退款 | PASS | `refund.status=succeeded`、`settlement.status=refunded` |
| 微信退款等待回调 | PASS | `refund.status=refund_pending`、关联 payment 为 `wechat/paid` |
| Owner fence | PASS | 他人商家查询返回 404 |
| 不存在配送单 | PASS | 合法但不存在的 public ID 返回 404 |
| 鉴权 | PASS | 无 token 返回 401 |
| 支付门控 | PASS | `ENABLE_DELIVERY_PAYMENT=false` 时端点返回 404 |
| 只读无副作用 | PASS | 连续三次 GET 前后六类资金事实与订单快照完全一致 |
| DTO 安全 | PASS | 不暴露内部 ID、凭证、密钥、prepay ID 或回调原文 |
| 数据残留 | PASS | 专项用户、订单及资金记录清理后均为 0 |

## 2. 真实环境证据

专项测试：

```text
TestD3OrderFundsReadOnlyRealHTTPMySQL
real_mysql_version=8.4.0
D-3 funds readonly=PASS
unpaid_null+wallet_paid+wallet_refunded+wechat_refund_pending+
owner_404+auth_401+gate_404+no_mutation
```

专项测试使用：

- 真实 MySQL 8.4.0；
- 真实 Gin 路由；
- 真实 JWT 鉴权中间件；
- 真实 Delivery Repository、FundsService 和 Handler；
- 无 mock Repository、无 fake Handler、无绕过 owner fence。

微信退款的 `refund_pending` 数据由真实资金领域服务写入真实 MySQL，GET
端点随后只读该事实。此只读补丁不依赖外部微信云调用。

## 3. 回归结果

| 命令 | 结果 |
|---|---|
| `go test ./... -count=1`（注入真实 MySQL 8.4 DSN） | PASS |
| `npm run check:boundaries` | PASS |
| `npm run verify` | PASS |
| `npm run build` | PASS |
| `rg "db\.AutoMigrate\(" backend/cmd/server` | 0 命中 |
| `git diff --check` | PASS |

miniapp 构建仅输出既有 `vendors.js` 体积告警，没有新增构建错误。

## 4. 资金写逻辑零改动证明

- 未修改现有 `PayOrderWithWallet`、`PayOrderWithMock`、
  `CreditMockRecharge`、`ConfirmWechatPayment`、`RequestRefund`、
  `CompleteRefund`。
- 未修改 delivery 状态机、CAS、迁移、配置、mock 安全门或权限中间件。
- 生产改动仅为只读 Repository 方法、只读 Service 方法、只读 DTO 和 GET
  Handler。
- 多次 GET 前后数据库事实快照相同，证明查询没有隐式创建钱包、流水、
  结算、退款或支付记录。

## 5. 剩余风险

- 真实微信支付和退款云环境联调仍属于原 D-3 后端基线的外部待办，不影响
  本只读端点对数据库事实的正确投影。
- 本端点返回读取瞬间的一致快照；前端仍需对 `refund_pending` 做有限轮询，
  并以后端返回为唯一事实源。

## 6. 最终判定

配送单资金全貌只读闭环已补齐，D-3 前端现在可以在页面刷新后重新查询最新
支付、结算和退款状态。本补丁可提交 owner 验收，但不得自行进入 D-3 前端
或 D-4。
