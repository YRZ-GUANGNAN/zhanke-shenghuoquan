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

- 用户确认真机已看到液态玻璃效果后，底部导航由约 `95.3PX` 的叠高布局收为“`44PX` 可点击内容 + 安全区”。学生端固定玻璃导航仍保留 `24PX` 圆角，工作台导航保留 `18PX` 圆角；保留发布键与图标的原视觉尺寸，仅收紧底栏外高、留白和激活胶囊比例。
- 修复微信上传编译器拒绝的通用子选择器 `.delivery-workspace-nav__item > *`，改为明确子元素选择器；底栏契约新增禁止 `> *` 的检查。
- 修复 Taro 将商家/骑手消息页样式提升到 `common.wxss` 后不再生成页面 `.wxss` 的问题。商家消息、骑手消息与二手市场现在各自保留真实页面级样式入口，干净构建后 32 个页面均具备 `index.wxss`。
- 上传构建显式使用 `TARO_APP_ENABLE_DELIVERY_PAYMENT=true`、`TARO_APP_ENABLE_DELIVERY_PAYMENT_MOCK=false`。`npm --workspace apps/miniapp run typecheck`、`npm run check:release-regression`（25/25）、`check-private-messaging`、静态路由、`npm run build:miniapp` 与 `git diff --check` 均通过。
- 官方 CLI 成功上传 `2026.11.12.3`，说明“液态玻璃底栏比例与页面样式修复”。上传总包 `2,187,683` bytes，主包 `2,021,075` bytes；回执 `.codex-tmp/upload-2026.11.12.3-glass-proportion.json` SHA256 `44f35f8044d487f20bae133fa0374aa34e5b95df54d3abfb5d8818e97a9baf84`。
- `.release/20261112-1c0ba1d-upload` 为 `.3` 当时的候选快照；后续全量验收发现发布键和图标视觉尺寸契约仍需保留原值，因此该快照已被 `.4` 取代，不再作为当前最终候选。
- 上传成功不等于已设为体验版。公众平台当前仍停在登录页，需账号本人扫码后在“版本管理”选择最终版本 `2026.11.12.4` 为体验版。
- 生产后端只读复核仍为 `2026.11.12.candidate.2` / `1b919e5ce501`，`/healthz` 与 `/readyz` 均 200。通知发送仍为 `false`；`NOTIFICATION_IM_CALLBACK_TOKEN` 缺失，腾讯云控制台也停在登录页，因此私信真实回调、手机通知收件及点击跳转仍未验收。

### 最终候选 `2026.11.12.4`

- 恢复发布键 `84rpx × 64rpx`、内部加号视觉 `72rpx × 64rpx`、导航图标 `44rpx × 44rpx` 的既有尺寸契约；底栏整体仍保持收紧后的高度与固定玻璃效果。
- `npm run verify:release` 完整通过：前端与 25 项上线回归、fresh Go 全量测试、`go vet`、商家资金 15 项、骑手资金与日结/广播/提现全套隔离 MySQL、通知隔离 MySQL、发布预检 31/31、离线演练 17/17、MiniApp/Admin 构建和 diff 检查均成功。
- 支付开启构建通过：主包 `1,985,503` bytes、本地总包 `2,147,083` bytes；上传官方总包 `2,187,683` bytes、主包 `2,021,075` bytes。所有 32 个页面均具备独立 `index.wxss`。
- 官方 CLI 成功上传 `2026.11.12.4`，说明“底栏比例最终修复与全量发布验收”；回执 `.codex-tmp/upload-2026.11.12.4-final.json` SHA256 `44f35f8044d487f20bae133fa0374aa34e5b95df54d3abfb5d8818e97a9baf84`。
- 冻结候选 `.release/20261112-0e032c4-upload` 对应源码 commit `0e032c42f82469de4846c543ed84a11804741861`：150 个文件、2,147,083 bytes、目录树摘要 `f77109200ac0038570a738988b35ae64c4ea50ea5d980be6b6abe4d39d858b84`。
- 最新开发预览二维码为 `.codex-tmp/preview-2026.11.12.4-home.png`，SHA256 `adbeb15916ebaa230b794fe4e4d7f40cdbe669b753523fa9cafafb554139ae39`；它用于手机先行验收，不替代体验版设置。
- 配送微信支付所需的商户号、API v3 Key、商户私钥、平台公钥、支付回调与退款回调均已配置；仍没有真实扣款、取消和退款样本，不能把配置检查当成真实交易验收。生产数据库当前仅有 1 笔 `pending_fee_payment` 订单，商家/骑手钱包余额与待结算收益均无记录；通知、回调、提现事实表均无记录。

### 主包瘦身 `2026.11.12.5`

- 保持 32 个公开路由和所有页面功能不变，将搜索、帖子详情、二手详情、用户主页、身份申请、我的内容、编辑资料、设置、通知中心、商家工作台、骑手工作台、配送沟通 12 个低频页面迁入分包。
- 主包页面数保持 16 个，分包页面共 16 个；新增首页、服务、二手、我的、商家消息和骑手消息入口的分包预下载规则，避免正常的按需首开延迟。
- 本地主包由 `1,985,503` bytes 降至 `1,569,973` bytes，减少 `415,530` bytes；本地总包 `2,185,538` bytes。官方上传主包 `1,585,828` bytes、总包 `2,228,002` bytes，所有单包仍在 2 MiB 内。
- `npm run verify:release` 完整通过：前端契约、25 项上线回归、fresh Go、`go vet`、商家/骑手/通知隔离 MySQL、发布预检 `31/31`、离线演练 `17/17`、MiniApp/Admin 构建与 diff 检查均成功。
- 12 个迁移页面已通过微信开发者工具逐页直达检查；最后停留页为 `pages/delivery-conversation/index?orderId=1`，标题“配送沟通”。同名公开 URL 未改变。
- 官方 CLI 成功上传 `2026.11.12.5`，说明“主包分包瘦身与完整路由回归”；回执 `.codex-tmp/upload-2026.11.12.5-subpackages.json` SHA256 `bb6018e658146db29d0312b6355bbf913d46d7c382ea4256ab4a1be275717549`。
- 冻结候选 `.release/20261112-60d8f7b-subpackages` 对应源码 commit `60d8f7bd2b88712e677a2072abae27e5316bc370`：188 个文件、2,185,538 bytes、目录树摘要 `8f21ec51eda9254fa0873071e4f872da60e169859f3aa240daca97f57971f5ae`。
- 最新开发预览二维码 `.codex-tmp/preview-2026.11.12.5-subpackages.png` SHA256 `6fd94cddc3e2b670e6d0be418c3fc624af2289af2acfb39654bc57c2def92544`。公众平台登录后需选择 `2026.11.12.5` 为体验版；上传成功本身不等于体验版已切换。

### 最终底栏恢复 `2026.11.12.6`

- 用户指出 `.5` 的底栏比例仍是重排版本。已撤回底栏比例重做，`student-bottom-nav.scss` 和 `delivery-workspace-nav.scss` 恢复为 `3e8fa3e` 中用户认可的原始尺寸、内边距、图标、发布键和激活背景，仅继续保留液态玻璃。
- 分包结构、预下载、URL、业务组件和页面内容均未回退；仍为 16 个主包页面与 16 个分包页面，共 32 个路由。
- 本地主包 `1,569,442` bytes、总包 `2,185,007` bytes；官方上传主包 `1,585,201` bytes、总包 `2,227,375` bytes。主包比 `.4` 的官方主包减少 `435,874` bytes。
- 最终底栏恢复后重新执行 `check:release-regression`，25/25 通过；`check-miniapp-package-size`、底栏玻璃、触控几何、UI 系统和类型检查均通过。
- 官方 CLI 成功上传 `2026.11.12.6`，说明“恢复原底栏比例并保留分包瘦身”；回执 `.codex-tmp/upload-2026.11.12.6-original-bottom.json` SHA256 `ffe3c4f6ef452b43b5614b68deb8834d4d97af779d4767c27fbd38ec5c5fb00a`。
- 冻结候选 `.release/20261112-ea90d7f-original-bottom` 对应源码 commit `ea90d7f0f21afd49c7abb9a560d4b5b541ca4141`：188 个文件、2,185,007 bytes、目录树摘要 `e8e811d3599b84d80d24e321ca9fa132c821dbd65804f9e3030227c2a694848f`。
- 最新开发预览二维码 `.codex-tmp/preview-2026.11.12.6-original-bottom.png` SHA256 `1fee050621c93b136e761ac56c4aac9de9769d470b0c8e38422cfa62b04ddfbe`。应只扫码使用 `.6`，`.5` 不再作为当前候选。
