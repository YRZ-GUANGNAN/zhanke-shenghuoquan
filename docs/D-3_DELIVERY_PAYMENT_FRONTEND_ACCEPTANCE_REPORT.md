# D-3 配送费支付、钱包与退款前端 Acceptance Report

## 1. 验收结论

**PASS**

D-3 前端已通过真实后端、真实 MySQL 8.4、微信开发者工具及全量回归验收。资金状态始终以后端事实为准，D-2 免费确认桥在 D-3 启用时已隐藏。

验收实现提交：

- `19ca42b62898f53c9e06e34f7dd0b9f0adc7caa7`

## 2. 验收环境

- 后端：真实 `cmd/server`，监听 `127.0.0.1:8080`。
- 数据库：项目 `backend/.env` 指向的真实 MySQL 8.4。
- 搜索依赖：本地真实 Meilisearch。
- 小程序：微信开发者工具 Stable `2.01.2510290`。
- D-3 本地验收门：
  - `ENABLE_DELIVERY=true`
  - `ENABLE_DELIVERY_PAYMENT=true`
  - `ALLOW_MOCK_DELIVERY_PAYMENT=true`
  - `ALLOW_MOCK_DELIVERY_FEE_CONFIRMATION=false`
  - `TARO_APP_ENABLE_DELIVERY=true`
  - `TARO_APP_ENABLE_DELIVERY_PAYMENT=true`
  - `TARO_APP_ENABLE_DELIVERY_PAYMENT_MOCK=true`

未使用 Mock 后端、Mock 数据库或绕过 HTTP 路由的前端验收。

## 3. 端到端验收矩阵

| 场景 | 真实结果 | 判定 |
|---|---|---|
| 钱包初始读取 | 微信开发者工具显示余额 ¥0 | PASS |
| Mock 充值 | 充值 ¥20 后，后端余额更新为 ¥20，页面出现 `+¥20 · 充值` 流水 | PASS |
| 钱包支付 | 待支付配送单使用钱包支付后进入 `ready_for_claim`，页面显示结算“已支付 · 钱包” | PASS |
| 重复支付 | 后端幂等返回既有结算，不产生第二次扣款 | PASS |
| 钱包退款 | `ready_for_claim` 且无骑手的订单退款成功，结算为 `refunded`，钱包余额退回，订单取消 | PASS |
| 微信 Mock 支付 | 点击微信支付后未调起真实支付框；后端返回 `provider=mock,status=paid`，前端重查后显示“已支付 · 本地模拟” | PASS |
| 余额不足 | 余额为 0 的真实账号调用钱包支付，后端返回 HTTP 422；开发者工具明确提示“钱包余额不足，请先充值” | PASS |
| CAS 冲突 | 使用错误 `expectedRowVersion` 的真实 HTTP 请求返回 409；前端已映射为状态变化提示并重新读取订单/资金 | PASS |
| 退款规则 | 仅 `ready_for_claim` 且无骑手可退款；其他已支付状态按钮不可用，后端仍为最终裁决 | PASS |
| D-2 桥隐藏 | D-3 开启时页面无“本地开发·确认费用”按钮 | PASS |
| 默认关闭 | 无构建变量时，编译产物中 PAYMENT=false、PAYMENT_MOCK=false，资金 UI 不开放 | PASS |

## 4. 前端不自改资金状态证明

- 充值结束后调用钱包与流水查询。
- 钱包支付结束后重新读取订单和 `GET /orders/:id/funds`。
- 微信支付：
  - Mock Provider：依据后端 `provider=mock` 跳过 `requestPayment`，再读后端状态。
  - 真实 Provider：依据后端 `wechat` 参数调用 `Taro.requestPayment`，随后轮询 `GET /payments/:id`。
- 退款结束后重新读取 `GET /orders/:id/funds`；异步退款采用有限轮询。
- 页面未通过本地 `setState` 伪造 `paid`、`refunded` 或钱包余额。

## 5. 构建与自动化结果

| 检查项 | 结果 |
|---|---|
| `go test ./... -count=1` | PASS |
| `npm run check:boundaries` | PASS |
| Admin TypeScript | PASS |
| MiniApp TypeScript | PASS |
| `npm run verify` | PASS |
| `npm run build` | PASS |
| `npm run build:miniapp` | PASS |
| `git diff --check` | PASS |
| AutoMigrate 负向搜索 | 0 命中 |
| 后端代码 diff | 0 |
| 依赖清单 diff | 0 |

仅保留既有 `vendors.js` 731 KiB 告警。

## 6. 权限、幂等与安全

- 所有请求沿用 `domainA.token` 与既有 API 信封处理。
- 钱包、流水、支付、退款均由后端按当前商家身份隔离。
- 写请求使用新 ULID 幂等键并在操作中禁用按钮，防止重复点击。
- 409/CAS 冲突不在前端覆盖状态，而是提示并刷新。
- 422 余额不足不会被前端误判成功。
- Mock 前端开关不决定支付结果；资金结果只信任后端 Provider 与状态。

## 7. 最终判定

D-3 配送费支付、商家钱包与退款的小程序前端达到专项验收标准，状态为 **PASS**。完成后应冻结本模块，等待 owner 执行串行 fast-forward 合并并打 `d3-delivery-frontend-baseline`；本分支不得自行进入 D-4。
