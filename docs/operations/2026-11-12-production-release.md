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
- 微信开发者工具登录后已通过官方 CLI 上传 `2026.11.12.1`，描述“骑手广播、人工提现、通知额度与体验优化”。上传回执为 `.codex-tmp/upload-2026.11.12.1-rider-wallet.json`，SHA256 `69fb718f010e073fd08847a0e84698df97a5eb03f88d9dc24180068cc419e128`。官方上传包总包 2,180,144 bytes、主包 2,013,536 bytes。
- 真机反馈底栏仍为不透明白底后，确认微信 WebView 没有执行 `@supports(backdrop-filter)` 分支。已在 `3e8fa3e` 将底栏基础层改为半透明，模糊仅在支持时增强；上线回归 25/25 通过，并以 `2026.11.12.2` 覆盖体验版。上传回执为 `.codex-tmp/upload-2026.11.12.2-glass-fix.json`，SHA256 `c148be1057407d32220f5bb7f6fb3f4650b725001e30aa262d2073f0b66bd74f`，官方包总包 2,180,576 bytes、主包 2,013,968 bytes。
- 上传前修正了开发者工具误开旧工作树的问题，现从当前 `apps/miniapp/dist` 加载。当前分支的液态玻璃基线提交为 `b0b1d6b52be537d18cc7ce8ee8c59b75b19c06fa`，后续 `c08b2df` 按要求将底栏改为固定透明毛玻璃；最终支付开启构建的本地文件树 SHA256 为 `42a493ebf9f9b31282ae54aae3d89f80bb09a5fbed5c558b689ffbb9ac5e01bf`。
- 真机白屏、钱包、身份切换、固定底栏、支付/取消/退款、私信双账号和人工提现真实凭证仍需在外部平台或真机上验收。
- 真实配送后的北京时间次日 00:00 到账尚无真实样本。

## 2026-11-12 液态玻璃底栏比例与体验版 `2026.11.12.3`

- 用户确认真机已看到液态玻璃效果后，底部导航由约 `95.3PX` 的叠高布局收为“`44PX` 可点击内容 + 安全区”。学生端固定玻璃导航仍保留 `24PX` 圆角，工作台导航保留 `18PX` 圆角；发布键、图标、文字和激活胶囊同步收紧比例。
- 修复微信上传编译器拒绝的通用子选择器 `.delivery-workspace-nav__item > *`，改为明确子元素选择器；底栏契约新增禁止 `> *` 的检查。
- 修复 Taro 将商家/骑手消息页样式提升到 `common.wxss` 后不再生成页面 `.wxss` 的问题。商家消息、骑手消息与二手市场现在各自保留真实页面级样式入口，干净构建后 32 个页面均具备 `index.wxss`。
- 上传构建显式使用 `TARO_APP_ENABLE_DELIVERY_PAYMENT=true`、`TARO_APP_ENABLE_DELIVERY_PAYMENT_MOCK=false`。`npm --workspace apps/miniapp run typecheck`、`npm run check:release-regression`（25/25）、`check-private-messaging`、静态路由、`npm run build:miniapp` 与 `git diff --check` 均通过。
- 官方 CLI 成功上传 `2026.11.12.3`，说明“液态玻璃底栏比例与页面样式修复”。上传总包 `2,187,683` bytes，主包 `2,021,075` bytes；回执 `.codex-tmp/upload-2026.11.12.3-glass-proportion.json` SHA256 `44f35f8044d487f20bae133fa0374aa34e5b95df54d3abfb5d8818e97a9baf84`。
- 冻结候选为 `.release/20261112-1c0ba1d-upload`，最终来源 commit `6561ec13d6dcf0529d23bcbdd685155ec3bf7f95`。复制目录与 `apps/miniapp/dist` 完全一致：150 个文件、2,147,082 bytes、树摘要 `a9ccd44c7f5caeff1daec7c972edd7871ecc3be0d2adb3c2fc68a6114f679f77`。最终夜间主题清单修正后重新构建的目录摘要与该上传产物保持一致，因此 `2026.11.12.3` 仍对应当前候选内容。
- 上传成功不等于已设为体验版。公众平台当前仍停在登录页，需账号本人扫码后在“版本管理”选择 `2026.11.12.3` 为体验版。
- 生产后端只读复核仍为 `2026.11.12.candidate.2` / `1b919e5ce501`，`/healthz` 与 `/readyz` 均 200。通知发送仍为 `false`；`NOTIFICATION_IM_CALLBACK_TOKEN` 缺失，腾讯云控制台也停在登录页，因此私信真实回调、手机通知收件及点击跳转仍未验收。
