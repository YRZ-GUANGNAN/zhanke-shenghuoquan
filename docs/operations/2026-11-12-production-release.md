# 2026-11-12 骑手资金与工作台生产发布记录

本记录只描述当前服务器、数据库与运行开关的实际状态。微信体验版、真实通知送达、真实支付和真机验收仍未完成时继续保持 PENDING。

## 已部署后端

- 当前 release：`/opt/campus-circle/releases/20261112-rider-wallet-1b919e5`。
- 运行时版本：`2026.11.12.candidate.2`。
- 运行时 commit：`1b919e5ce501`（完整候选提交 `1b919e5ce5017373cdf10a22da1f503f5e1f5c50`）。
- 后端 binary SHA256：`ec5c8695db133cc3c98b78e5ba2cbf8aef82cb3fde1b843e04190e3fd2b2eb61`。
- 候选 manifest SHA256：`984b749ac302e761aaf2eb2d274b80e6e9657a5213d8f0f375808ef3a6add4bf`。
- 2026-11-12 下午复验：服务器回环 `/healthz`、`/readyz`、`/version` 正常，release 身份与运行时提交一致。

## 数据库迁移与恢复证据

- 已应用 `055_delivery_rider_availability.sql`、`056_delivery_rider_withdrawal_allocations.sql`。
- 迁移前备份：`/var/backups/campus-circle/20261112-rider-wallet-1b919e5/database-20261112T052022Z.sql.gz`。
- 备份 SHA256：`cb52e440ec49b8739c622b8caf331cfe0f284419e54bee04163cfd6254c37422`。
- 备份解压后大小：`364439` bytes。
- 停写后完整 dump 已恢复到独立验证库，并重放 055/056；97 张原表摘要、来源关系及行数据在演练前后保持一致。
- 055 仅新增骑手在岗表和全局容量配置，初始容量 3；056 四张提现事实表初始均为 0 行。
- 生产库未执行自动 down 或全库恢复。若后续回退 binary，055/056 新表与审计事实必须保留，不得为兼容旧 binary 删除资金事实。

## 已启用运行开关

| 配置 | 当前值 |
| --- | --- |
| `ENABLE_DELIVERY` | true |
| `ENABLE_DELIVERY_PAYMENT` | true |
| `ENABLE_DELIVERY_RIDER_DAILY_SETTLEMENT` | true |
| `ENABLE_DELIVERY_RIDER_BROADCAST` | true |
| `ENABLE_DELIVERY_RIDER_WITHDRAWAL` | true |
| `ENABLE_DELIVERY_RIDER_PAYOUT_QUEUE` | false |
| `ENABLE_USER_NOTIFICATIONS` | true |
| `ENABLE_WECHAT_SUBSCRIBE_NOTIFICATIONS` | true |
| `ENABLE_NOTIFICATION_SENDING` | false |

两项骑手功能激活门禁 `broadcast`、`withdrawal` 均通过。人工提现仍采用“用户申请 + 后台人工登记真实转账凭证”；旧自动排队打款保持关闭。

## Binary 回滚边界

- 上一 release：`/opt/campus-circle/releases/20261103-notification-settlement-972d3f3`。
- 上一 runtime commit：`972d3f343745`。
- 上一 binary SHA256：`70d5f389eeb0b46a98704e3b369d0e6e0da84ba7300af7d02a795e727aca84a8`。
- 当前 `/healthz`、`/readyz`、`/version` 通过；如新 binary 后续故障，可原子切回上一 release 并保留已迁移表。

## 仍未完成

- 微信订阅消息测试白名单已预置唯一已授权测试账号，但 `ENABLE_NOTIFICATION_SENDING=false`；腾讯 IM 回调令牌和控制台 `C2C.CallbackAfterSendMsg` 尚未配置。
- 还没有真实订单通知或私信通知的手机收件、点击跳转和回调去重证据。
- 微信开发者工具当前未登录，支付开关开启的 `2026.11.12.1` 候选尚未上传体验版。
- 真机白屏、钱包、身份切换、固定底栏、支付/取消/退款、私信双账号和人工提现真实凭证仍需在外部平台或真机上验收。
- 真实配送后的北京时间次日 00:00 到账尚无真实样本。
