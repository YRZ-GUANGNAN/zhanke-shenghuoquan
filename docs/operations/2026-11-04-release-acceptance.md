# 2026-11-04 上线候选与真实验收记录

本记录区分代码、生产配置、开发上传、体验版、真实收件与资金验收。没有证据的项目保持未完成。

## 已冻结的小程序候选

- 分支：`codex/experience-ui-wallet-20261101`。
- 代码提交：`5d2efd015e2a60e403b155d85752bb2095f1fe12`。
- 官方 CLI 上传版本：`2026.11.04.1`，说明：`支付开关与设置二级修复`。
- `upload` 回执成功，进程退出 0；本轮复核本地回执存在。
- 上传回执：`.codex-tmp/upload-2026.11.04.1-payment-settings.json`。
- 回执 SHA256：`12925d4636501e8b58b8d663c5a4faf738372ea5ab22e3fbddb7804e9e0c5394`。此文件只含包大小，版本/提交关联来自上传命令及本记录，不应仅凭该 JSON 推断版本号。
- 官方编译主包 2,003,724 bytes，总包 2,168,162 bytes；商家详情、发单、两钱包保持原路由普通分包。
- 构建时显式设置 `TARO_APP_ENABLE_DELIVERY_PAYMENT=true`、`TARO_APP_ENABLE_DELIVERY_PAYMENT_MOCK=false`；未删除许可证、未额外激进压缩。
- 当前真实钱包、发单、订单详情使用 `useDeliveryPayment()` 读取 `/delivery/capabilities`。真实能力由服务端决定；不能把构建常量开启等同于支付成功，也不能把普通构建的常量默认值等同于这些页面必然关闭。

本轮官方 CLI 再次成功生成同包体开发预览：

- `.codex-tmp/device-acceptance-20261104-v1/preview.png`，SHA256 `653536476095a244c7e81e263fa1f44f1c7e60bb5622b7293119dc0d342a1b63`。
- `.codex-tmp/device-acceptance-20261104-v1/preview.json`，SHA256 与上传大小回执相同。
- 这只是开发预览，不是已设置体验版，不证明手机页面通过。

公众平台页面在本轮仍无法经浏览器工具读取，未点击设置体验版、提交审核或发布。已请用户在版本管理核对 `2026.11.04.1` 并设为体验版，尚未取得确认。

## 生产复核（只读）

- `campus-api.service`：active。
- current：`/opt/campus-circle/releases/20261103-notification-settlement-972d3f3`。
- loopback `/healthz`、`/readyz`、`/version`：均 200。
- 线上版本：`2026.11.03.candidate.3`，commit `972d3f343745`。
- 上轮冻结 binary SHA256：`70d5f389eeb0b46a98704e3b369d0e6e0da84ba7300af7d02a795e727aca84a8`。
- 052 `cancel_requested_at` 存在；054 `settle_at` 非空、`settled_at` 可空；due index 顺序为 `settlement_status, settle_at, id`。
- 053 订阅表可读，订单/私信各有 1 次 accepted、未占用的授权额度；未打印用户标识、OpenID 或 nonce。
- 骑手收益、钱包、提现均无记录；唯一配送单仍待支付；通知投递表无记录。

从运行进程环境读取的开关：

| 配置 | 当前值 |
| --- | --- |
| `ENABLE_DELIVERY_PAYMENT` | true |
| `ENABLE_DELIVERY_RIDER_DAILY_SETTLEMENT` | true |
| `ENABLE_USER_NOTIFICATIONS` | true |
| `ENABLE_WECHAT_SUBSCRIBE_NOTIFICATIONS` | true |
| `ENABLE_NOTIFICATION_SENDING` | false |
| `WECHAT_SUBSCRIBE_MINIPROGRAM_STATE` | formal |
| 测试白名单人数 | 0 |
| `NOTIFICATION_IM_CALLBACK_TOKEN` 已配置 | 否 |

此前已完成停写备份、独立验证库恢复比对、052→053→054演练及生产迁移。备份保留在 `/var/backups/campus-circle/20261103-notification-settlement-972d3f3/database.sql.gz`，上轮记录 SHA256 `191bf8993811fcbafab0104f48105ab5d0c34f87bf03a21f5894bb6e3822a5c2`；本轮未重新恢复、删除或修改备份。

## 尚未完成的硬门槛

### 1. 人工提现尚未可用

不是“已经能申请，只差人工打款”。当前 `deliveryrider.Handler.withdrawalWritesReady=false`，申请、后台确认、拒绝回充全部禁用，且没有生产开启配置。

缺少提现金额到具体已结算、来源核验通过收益的分配记录，与旧逐单线下结款/内部待打款队列没有完整互斥。直接开放可能重复登记同笔收益。须补分配与并发测试后才可另行部署启用；不接入自动微信转账、不执行真实打款。

### 2. 通知尚未真实送达

- 已选官方模板并获得本人一次性授权；没有真实微信投递结果。
- 私信需在腾讯 IM 配置 `C2C.CallbackAfterSendMsg` 到 `https://zhanduiwang.top/api/v1/notification/tencent-im/callback`，服务端和控制台配置一致的保密鉴权 Token，SDKAppID 匹配。
- 配置不能通过聊天传递密钥；需在正确账号/控制台确认具体变更，配置后验证签名、拒绝无签名请求、双账号真实消息及去重。
- 测试发送先限定本人账号白名单；开发预览目标 developer、体验版 trial、正式版 formal。当前 formal 且发送关闭，未更改。
- 必须在白名单及授权准备好后产生真实业务事件；不补造订单、不插入假事件、不伪造供应商回调。
- 微信 `accepted` 只代表 API 受理；手机收到且点击到正确页面后才完成收件/跳转验收。
- 私信暂无线下/当前会话可见性权威信号，不能宣称全量离线提醒完成。

### 3. 骑手新单广播已形成代码候选，尚未部署

- 本工作树已实现全校园骑手池、骑手主动开始/停止接单、后台 `1..100` 容量配置、通知候选及发送前复核；默认容量为 3。
- 迁移 `055_delivery_rider_availability.sql` 新增权威在岗状态和全局容量配置。资格 revision 变化会持久停工，资格恢复后必须由骑手重新开始接单。
- 抢单事务按 availability→全局配置→用户/骑手资格→进行中订单→目标订单稳定锁序复核。功能开关关闭、迁移缺失、未开工、已满载、资格撤销、订单被抢或过期均 fail-closed；停止接单不影响已有配送推进。
- 隔离 MySQL 8.4 验证：8 个并发抢单在容量 1 下只成功 1 个；双骑手抢同单只成功 1 个；通知写入失败回滚订单；撤销再恢复资格不会自动复工；修改过的配置和已有在岗事实阻止回滚。临时数据库、MySQL 进程和数据目录均已清理。
- 本地 `npm run verify`、Go 全量测试、前端 typecheck 和通知定向测试通过。2026-11-06 收口与提交记录见文末；本实现未部署，未打开 `ENABLE_DELIVERY_RIDER_BROADCAST`，不能宣称生产已有广播或真实送达。

### 4. 真实支付与次日结算尚未验收

无真实已完成配送收益样本。现有 isolated MySQL 测试与生产日结开关通过不代表真实次日到账。验收需本人完成真实支付、骑手真实配送，确认未支付订单不进骑手池、预计收入不提前入余额、北京时间次日00:00结算一次且金额守恒。

真实付款/退款/线下转账要单独确认账号、金额及操作，不由测试脚本替用户发起。

### 5. 真机 UI 与正式发布

设置二级页、钱包、固定透明底栏、分包首次进入、冷热启动、支付取消和网络失败都需在本次版本真机验收。设置页仍无服务端授权额度回显；中性文案不等于该缺口已解决。

另有两条文案应在下一次设置页修订时纠正：develop 应明确标记开发预览而非体验版；“一次授权一次送达”应表达一次性通知额度而非保证送达。此处只登记问题，未在已上传包之外偷偷改源码。

体验版设置、审核提交、审核通过、正式发布分开记录，当前均未取得完成证据。

## 测试证据

- 上轮 `npm run verify`（含24项postverify）通过；设置二级修改后 miniapp typecheck、设置契约、build/postbuild、包体边界、diff检查通过。
- 本轮重新生成官方预览通过，大小与已上传回执一致。
- 本轮独立提现审查运行 `go test ./internal/module/delivery ./internal/module/admin/deliveryrider -run TestWithdrawal -count=1` 通过。该测试验证已有硬门禁和校验，不代表提现闭环安全。
- 本轮独立复验 `npm run verify` 已结束，完整走完 preverify、verify 与 postverify；Admin/MiniApp typecheck、安全 Go 测试及24项上线回归均报告通过，24项结果逐项为 `exit 0`。执行器最终顶层 `exit_code` 字段未保留，会话已关闭，不能补记为已留存的顶层退出码证据；未为此重复运行。
- 本轮 `git diff --check` 通过；只新增本验收记录与提现安全补充设计，未修改已上传候选的业务源码或构建产物。

本轮未产生真实资金动作、通知发送、生产数据库写入或开关变更。

## 2026-11-06 本地代码收口（未发布）

本节只记录本次工作树的实现和本地测试，不刷新或推断上文 2026-11-04 的生产、公众平台状态。

完成项：

- 全校园配送池；骑手主动开始/停止接单，后台容量 `1..100`、默认 3，停工仍可只读看池。
- 迁移 055、资格 revision、稳定锁序与原子抢单，资格撤销后必须显式重新开工。
- `RiderPoolOpened` 首次展开扫描候选池；微信发送前仅复核当前收件人，101 个候选的回归测试确认 guard 不再调用全池扫描。
- 默认关闭开关直接禁止新抢单；组合根和 HTTP 测试确认返回 `ErrRiderAvailabilityUnavailable` / 503，缺迁移也不留旧抢单回退入口。
- 前端真实 parser/hook 隔离行为测试覆盖首次零值、容量边界、开始/停止、重复点击、未知写入结果只查询，以及切号/令牌变化/页面隐藏/卸载后的迟到响应；纳入 `postverify`，上线回归由 24 项增至 25 项。
- 更新旧 D-2 HTTP/MySQL 用例，适配必须主动开工的契约；未使用其外部数据库 DSN，本轮不宣称该旧整套真实 HTTP/MySQL 验收已执行。

本地验收命令（成功项均保存执行器 exit 0 结果）：

| 命令 | 结果 |
| --- | --- |
| `npm run test:backend:fresh` | PASS；安全包装器清空数据库 DSN 后执行 `go test ./... -count=1`，依赖外部 DSN 的旧测试跳过 |
| `go vet ./...`（backend） | PASS |
| `go test ./cmd/server -run 'TestDeliveryRuntime.*Broadcast\|TestRiderPool' -count=1`（backend） | PASS |
| `go test ./migrations -run 'TestMigration055' -count=1`（backend） | PASS |
| `npm --workspace apps/miniapp run typecheck` | PASS |
| `npm --workspace apps/admin run typecheck` | PASS |
| `node scripts/check-delivery-rider-availability.cjs` | PASS；契约及真实 TS/hook 行为检查 |
| `npm run verify` | PASS；含 25 项上线回归 |
| `npm run test:rider-money:mysql` | PASS；隔离 MySQL 8.4，容量 1 的 8 个并发请求仅 1 次抢单成功，同一单仅 1 位骑手成功，资格变化停工、事件写失败回滚与迁移回滚保护通过 |
| `npm run test:notification:mysql` | PASS；隔离 MySQL 8.4 的 7 项测试及 UTC/Shanghai 子项通过 |
| `npm run build:miniapp` | PASS；含产物和包体检查，仅既有 CSS 顺序、webpack 体积警告 |
| `npm run build:admin` | PASS |
| `git diff --check` | PASS |

小程序本地构建主包 **1,976,170 bytes**，总包 **2,135,658 bytes**；这不是微信官方上传回执。测试脚本已验证两套隔离实例关闭、临时数据目录删除、遗留测试数据库为 0，既有 MySQL 服务未受影响。

发布边界：本次仅代码提交，不 push、不上传、不部署、不发送通知、不执行付款或打款。后续发布须先应用迁移 055，并协调包含主动开工入口的小程序版本，再启用 `ENABLE_DELIVERY_RIDER_BROADCAST`；不能只部署默认关闭后端，否则新抢单会按设计被禁止。微信通知发送开关、白名单、模板和授权仍需单独检查。真实收件与跳转、真机支付、真实配送后的次日 00:00 到账均仍未验收；本地测试不得代替这些证据。
