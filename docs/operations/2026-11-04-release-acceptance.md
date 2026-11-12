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

### 1. 人工提现尚未在生产开启

本工作树已补齐“已结算收益 → 来源分配 → 后台领取/登记已打款/拒绝回充”的人工提现闭环，包含 056 迁移、处理权、不可变审计、幂等/CAS、来源证据和与旧逐单结款/待打款队列的互斥。默认 `ENABLE_DELIVERY_RIDER_WITHDRAWAL=false`，生产仍未应用 056、未开放申请或后台资金写入。

本实现不调用微信自动转账；只有完成停写备份、迁移演练、启动 `CheckWithdrawalReadiness` 和独立生产验收后，才能在明确授权下开启人工处理。实际线下打款及凭证登记仍需后台人员执行，结果未知时保持占用，不回充、不重复转账。

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
- 本轮提现专项审查运行 `go test ./internal/module/delivery ./internal/module/admin/deliveryrider -run TestWithdrawal -count=1` 通过；隔离 MySQL 覆盖来源部分分配、claim/release/exception、claim/reject 竞争、通知失败回滚、午夜结算竞争、共享充值退款竞争和回滚审计保护。
- 本轮独立复验 `npm run verify` 已结束，完整走完 preverify、verify 与 postverify；Admin/MiniApp typecheck、安全 Go 测试及24项上线回归均报告通过，24项结果逐项为 `exit 0`。执行器最终顶层 `exit_code` 字段未保留，会话已关闭，不能补记为已留存的顶层退出码证据；未为此重复运行。
- 本轮 `git diff --check` 通过；新增 056 迁移契约测试及只读 054/055/056 发布预检，未执行生产写入、真实支付、真实通知或打款。

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

## 2026-11-06 资金收口与发布预检补充

- 新增 `056_delivery_rider_withdrawal_allocations.sql` 及受事实保护的回滚脚本；迁移契约测试 `go test ./migrations -run 'TestMigration05(4|5|6)' -count=1` 通过。
- 新增只读 `deploy/native/inspect-rider-release.py`：核验 054/055/056 表结构、唯一索引/检查约束、release 与 `/version` 对齐及广播/提现/队列开关冲突；不会写库、改环境或启停服务。`npm run check:rider-release-preflight`（6 个 fixture 场景）通过。
- 最新本地候选 `npm run verify:release` 已通过：25 项上线回归、Go fresh/vet、隔离 MySQL 资金与通知、只读预检、MiniApp/Admin 构建和 `git diff --check`；构建仅有既有 CSS order/webpack 体积警告。Mini Program 产物主包 **1,977,014 bytes**、总包 **2,138,594 bytes**。
- 这仍是未发布候选：没有生产迁移、体验版设置、真实微信支付、真实订阅/私信送达、真机验收或次日实际到账证据；所有外部开关继续按 fail-closed 处理。

## 2026-11-08 本地候选最终复核

- `npm run verify:release` 已取得顶层 `exit 0`。该命令串行完成 `verify`、fresh backend tests、`go vet ./...`、骑手资金隔离 MySQL、通知隔离 MySQL、骑手发布预检、MiniApp/Admin 构建及 `git diff --check`。
- `npm run test:rider-money:mysql`：PASS；所有隔离 MySQL 场景实际执行并通过，脚本拒绝 `SKIP`。
- `npm run test:notification:mysql`：PASS；8 个通知隔离场景实际执行并通过，脚本拒绝 `SKIP`。
- `npm run check:rider-release-preflight`：PASS；18 项预检通过，覆盖布尔开关、订阅状态、054/055/056 表结构/索引/检查约束、通知投递状态和 IM 回执状态。
- MiniApp 构建产物检查通过：主包 **1,977,014 bytes**、总包 **2,138,594 bytes**；Admin 生产构建通过。Webpack 仅报告既有 CSS order 与体积建议警告，不影响 exit 0。
- 本轮清理了验收脚本生成的 `deploy/native/__pycache__/` 与 `scripts/__pycache__/`；工作区未包含这些缓存。
- 本地候选仍未等同于上线：056 生产迁移、提现开关、055 广播开关、微信支付真实交易、微信订阅/私信真实送达、腾讯 IM 回调、体验版设置、审核/正式发布、真机验收和次日实际到账仍需在外部系统逐项完成并留证。当前不执行生产写入、真实付款或发布操作。

## 2026-11-08 追加：补齐商家数据库验收与候选身份门禁

此前 `test:backend:fresh` 会清除共享数据库 DSN，商家资金的旧数据库用例因此跳过；骑手隔离测试不能替代这些用例。本轮增加专用商家隔离 runner，并纳入 `verify:release`，不修改支付业务或调用微信真实资金接口。

| 命令/检查 | 本轮结果 |
| --- | --- |
| 扩充后的 `npm run verify:release` | 顶层 exit 0；包含前端/契约回归、fresh Go、vet、三套隔离数据库验收、预检、MiniApp/Admin 构建及 diff |
| `npm run test:merchant-money:mysql` | exit 0；15 个顶层测试实际执行，充值/扣费/退款/取消充值/支付恢复/迁移均通过，SKIP=0 |
| `npm run test:rider-money:mysql` | exit 0；35 个数据库顶层测试及其子测试，另有 CHECK 支持测试；使用 37 个隔离临时数据库，无 SKIP |
| `npm run test:notification:mysql` | exit 0；7 个顶层测试（含 UTC/Shanghai 子测试），使用 8 个隔离临时数据库，无 SKIP |
| `npm run check:rider-release-preflight` | 完整回归中 28/28；追加损坏/缺失 fixture 非零退出与脱敏断言后，单独复验 29/29，exit 0 |
| 两个资金 runner 的跳过检测 | 最终脚本重新执行 exit 0；顶层/空格/Tab 缩进 SKIP 合成断言通过，SQL `SKIP LOCKED` 不误报 |
| MiniApp/Admin 构建 | exit 0；小程序主包 1,977,014 bytes、总包 2,138,594 bytes；只含既有 warning |

以上测试的临时数据库剩余均为 0，独立实例与临时目录清理通过。这里更正数量口径：此前“37 个骑手场景/8 个通知场景”是隔离数据库数量，不能当作顶层测试数量。真实微信支付/收件不包含在这些本地测试中。

预检新增 `--expected-commit` 与 `--require-ready`，阻止旧自洽版本冒充指定候选；默认关闭候选还要求广播、提现、内部队列、通知发送都关闭。门禁不通过或检查器自身失败均非零退出。操作顺序与限制见 `docs/operations/2026-11-08-rider-release-preparation.md`，旧 052–054 迁移执行器不可直接复用到 055/056。

本轮公网只读复查：`/healthz` HTTP 200，`/version` HTTP 200 且仍为 `2026.11.03.candidate.3` / `972d3f343745`；公网 `/readyz` 为 HTTP 403，未复查服务端回环 readyz，不把公网拒绝解读为内部健康失败。本轮没有复读生产进程环境或数据库，前述开关/余额/授权额度仍是旧现场记录，不可据此推断最新值。

通知只读代码审查未发现新的 P0/P1；外部 IM 控制台、模板、授权额度、发送白名单、真实双账号收件/跳转继续 PENDING。设置页“开发预览”和一次性授权文案已在 `203af31` 修正，但服务端授权余额回显仍未实现，不宣称该缺口已解决。商家商品收益仍按已确认边界 `external_order_source_unavailable` 保持关闭，不把配送充值余额当成商品收益；骑手独立保证金充值也未实现，不能混入已实现的收入提现功能。

## 2026-11-08 追加：通知额度回显与安全演练收口

本节更新上文的本地代码状态，不刷新 2026-11-04 的公众平台或生产现场记录。

- 设置二级页现从受认证接口读取当前账号每个模板的 `remainingQuota` / `reservedQuota`，只展示一次性授权额度，并继续明确“实际送达以微信为准”。接口不返回 OpenID、授权 nonce 或供应商身份；按用户、AppID、模板及当前模板标记隔离，过期授权不计入。前端对旧服务端缺字段保持“待确认”，不伪造为 0。
- 授权成功后重新读取服务端额度；账号 epoch 与请求序列双重守卫确保切号、卸载和较早慢请求不能覆盖新状态。独立审查未发现 P0/P1，发现的旧请求覆盖 P2 已修复。
- `npm run check:rider-release-rehearsal` 已纳入 `verify:release`。纯离线工具只校验显式绑定的 commit/hash、停写输入、默认关闭、gzip 完整性、完整原表目录及恢复/迁移摘要；拒绝 production/live mode，成功回执也始终为 `productionReady=false`、`deployed=false`。隔离 MySQL 用例真实执行 mysqldump→gzip→另一新库恢复→055→056，20 张原表及来源库摘要保持不变；该结果不认证生产备份或生产停写。
- 商家商品收益继续 `merchantEarnings=false` / `external_order_source_unavailable`。官方 Provider 尚未提供真实应收、支付/结算、退款/冲正、订单版本、门店授权归属和回查凭证；本轮不新建收益账本、不开放提现，也不复用配送费钱包冒充收益。
- 骑手独立配送保证金仍未实现且不属于本次提现收口。当前没有保证金充值入口、账户、流水、退款接口或专用迁移，也没有“缴保证金才能接单”的已批准规则；不得把商家微信充值资金来源改名或混入骑手收入余额。若未来需要保证金，必须另行确认金额、扣罚/退款、资格、关号、对账及争议规则后独立设计。

2026-11-12 完整 `npm run verify:release` 已重新执行并以顶层 `exit 0` 收口：全量前端/契约回归、fresh Go、`go vet ./...`、商家资金 15 项、骑手资金 36 个顶层测试（39 个隔离数据库实例）、通知 7 个顶层测试（8 个隔离数据库实例）、发布预检 29/29、离线演练 17/17、MiniApp/Admin 构建及 `git diff --check` 均通过。三套 MySQL runner 的遗留测试数据库均为 0，临时进程/目录清理通过。小程序构建产物主包 **1,978,644 bytes**、总包 **2,140,224 bytes**；Admin 产物 4 个文件、**390,118 bytes**。构建仅有既有 CSS 顺序和 webpack 体积警告。

上述验收只证明本地测试数据与构建产物，不认证生产停写、生产备份、真实微信收件、真机支付或真实次日到账。旧 `.release/20261108-e0297648f5a5` 仍只对应旧提交 `e0297648f5a5`，不得当作本次最终候选上传或部署。

生产迁移、广播/提现/通知发送开关、腾讯 IM 控制台、微信模板与真实授权送达、真实支付/退款、真机白屏与页面验收、真实配送后的次日 00:00 到账、人工转账凭证、体验版设置、审核和正式发布均继续 PENDING。
