# 055/056 骑手广播与人工提现部署准备

本文件是准备和验收边界，不是生产迁移授权，也不是已经部署的回执。当前只完成本地候选；真实微信消息、支付、真机和次日到账仍为 PENDING。

## 不能复用的旧脚本

`deploy/native/release-notification-settlement.py` 只适用于其已冻结的 `7d22416948be` / 052→053→054 空骑手账本基线。它硬编码旧 release、旧 commit、迁移集合及空账本条件；不得修改这些常量后直接在当前生产重跑，也不得把它的旧回执作为 055/056 的证据。

当前 `deploy/native/inspect-rider-release.py` 只读，不执行 DDL、环境变更、服务重启、通知发送或付款。055/056 的受保护执行器尚未实现并验收，正式迁移前仍需经批准的独立执行步骤或执行器；不能用只读预检通过替代备份恢复及停写演练。

## 本地安全演练工具（不是生产执行器）

`deploy/native/rehearse-rider-release.py` 是纯离线证据包校验器：只读本地文件，无 DSN、网络、服务管理、环境写入或 SQL 执行入口。必须显式提供 stage、evidence、两个输入 SHA256、旧 commit 和完整 40 位候选 commit。它校验冻结包身份和资产 hash、Linux ELF 文件头、055→056 顺序、停写证据输入、默认关闭及支付/日结开关保持、备份 gzip CRC/完成标记、全体原表目录与恢复/演练前后 digest，以及迁移后预检。

证据 JSON 的 `mode` 只允许 `fixture` 或 `isolated_mysql`，`production`/`live` 明确拒绝。成功回执始终为 `productionReady=false`、`deployed=false`、`featuresActivated=false`、`externalEvidenceAuthenticated=false`；fixture 另为 `simulationOnly=true`。本工具只能证明输入证据包内部一致，不认证输入的现场真实性；ELF 文件头和文件 hash 也不证明 binary 可运行。不得把手工构造的零写者数字或相等 digest 当成真实生产停写/恢复证据。

输入契约：

- `manifest.json`：`schemaVersion=1`，`previousCommit`，`candidateCommit`，`assets`（仅 `campus-api` 与两份准确 SQL 文件名各自的 64 位 SHA256），`migrations`（严格 055、056 顺序），`originalTables`（冻结基线中全部原表的去重排序目录，不能仅列资金表子集）。
- evidence JSON：相同 schema/commit 和 `manifestSha256`；`stoppedWriters` 的 `scope` 必须匹配 mode，`apiProcesses`、`workerProcesses`、`applicationConnections`、`uninspectedWriters` 必须显式整数 0；`flagsBefore/flagsAfter`、`backup`、三个原表快照、`afterPreflight`。
- `backup`：stage 内不含路径跳转的 `file`、文件 `sha256`、`completed=true`；工具通读 gzip 校验 CRC，解压上限 256MiB，要求 SQL 建表内容和 mysqldump 完成标记。
- `beforeMigration`、`restoredOriginalTables`、`afterMigrationOriginalTables`：以全部原表名为键，值为 `{rows: 非负整数, sha256: 64位小写摘要}`；目录和逐表值必须完全一致，不允许预先含 055/056 表。
- `afterPreflight`：与 `inspect-rider-release.py` 相同的脱敏快照，绑定候选 commit，055/056 结构与默认关闭 candidate 门禁通过；初始 availability 和 056 四表为 0 行，global config 为 1 行。

CLI 参数全部必填，错身份/hash、残留写者、默认开关非关闭、备份损坏/缺完成标记、目录/金额摘要漂移、部分迁移或不完整结构均 exit 1。错误输出只返回固定原因码，不打印输入行、密钥或备份内容。

本地可重复验证：

```powershell
npm run check:rider-release-rehearsal
npm run test:rider-money:mysql
```

2026-11-08 本轮证据：离线 fixture/CLI **17/17，exit 0**；隔离 MySQL 8.4 的 `TestRiderReleaseBackupRestore055056AgainstIsolatedMySQL` **PASS**。该用例经既有专用 runner 的固定 loopback/端口及真实 datadir 双重核验，创建 054 基线与测试资金来源，真实 mysqldump→gzip 完整性→另一新库恢复→055/056，检查 **20 张原表** digest 和来源库不变、默认新表初始状态及 `CheckWithdrawalReadiness`；全套资金 runner **exit 0**，遗留测试数据库 0、临时进程/目录清理 PASS。测试仅使用明确隔离的测试数据，不使用生产导出，不证明生产数据恢复成功，也没有部署 candidate binary。

2026-11-12 最终本地复核：完整 `npm run verify:release` 顶层 `exit 0`，离线演练仍为 17/17，发布预检 29/29，骑手备份恢复用例再次实跑 PASS；证据计数和清理边界同上。该回执仍不能替代生产停写、生产全库备份恢复、055/056 正式迁移或运行时身份核验。

## 只读候选身份门禁

冻结候选必须包含：完整 Git commit、后端 Linux binary SHA256、055/056 SQL SHA256、小程序与 Admin 资产 SHA256、构建参数、测试命令退出码。构建期间工作树必须干净；不能在旧 dist 上仅改版本说明上传。

在生产主机的受限运维会话中可运行以下只读命令；示例 commit 对应当前已提交基线，实际发布必须替换为本次最终冻结候选，不能沿用示例认定最新版：

```bash
python3 /opt/campus-circle/release-tools/inspect-rider-release.py
python3 /opt/campus-circle/release-tools/inspect-rider-release.py --expected-commit 203af3154df3 --require-ready candidate
python3 /opt/campus-circle/release-tools/inspect-rider-release.py --expected-commit 203af3154df3 --require-ready broadcast
python3 /opt/campus-circle/release-tools/inspect-rider-release.py --expected-commit 203af3154df3 --require-ready withdrawal
```

`/opt/campus-circle/release-tools/inspect-rider-release.py` 必须先由操作者以本次冻结脚本的 SHA256 校验后置入；该路径不是仓库已部署事实。无参数命令只输出盘点，即使 ready=false 也不代表进程错误。`--expected-commit` 比较 `/version` 运行时 commit，至少 12 位小写十六进制；指定长度不得超过运行时实际提供的 commit 长度。错版、缺失或非法身份不能通过。`--require-ready` 必须同时指定 expected commit，所选门禁未通过时 exit 1；它不启用任何开关。

- `candidate`：只证明候选身份自洽、服务 ready、054 和开关组合满足默认关闭部署条件，不证明 055/056 已完整部署。
- `broadcast`：增加 055 结构条件，不证明微信订阅发送或手机真实收件。
- `withdrawal`：增加 056 结构条件和与旧队列互斥，不代替服务启动 `CheckWithdrawalReadiness` 的来源/账本一致性检查，也不证明已打款。

当前预检不核验所有运行时配置及所有数据库特征；例如模板真实语义、凭证可用性、微信控制台回调、表引擎、生产写者集合需由发布审查单独核验。不能把一个 `true` 解释成完整资金安全或正式上线验收。

## 生产迁移前必须完成的证据

1. 只读重查当前 `/version`、release symlink、服务进程、数据库结构及各开关；旧 2026-11-04 现场记录可能已过时。核对 MySQL 版本、InnoDB、时区/DSN 时钟契约，不改变历史金额或时间解释。
2. 明确应用、Admin、worker、数据库事件/触发器/外部任务等全部写者；经授权进入维护/停写，确认没有遗留连接在写资金或订单。未能证明停写即终止迁移。
3. 精确绑定旧 release、新 candidate、binary、迁移文件校验和；拒绝重复、部分迁移或与预检不符的现场。先确认备份磁盘空间、权限、保留策略。
4. 全库一致性备份、gzip 完整性和 SHA256 校验；恢复到不授予应用权限的专用验证库，按稳定主键生成全部原表 digest，与停写生产快照逐表一致。
5. 在验证库按 055→056 演练。确认旧表行数及原字段 digest 不变，055 仅新增初始 global 配置，056 四张表初始无业务事实；核验列类型/非空、唯一索引顺序、CHECK 实际表达式和 ENFORCED，以及 InnoDB 事务引擎。
6. 测试缺迁移拒绝启动、默认关闭启动、广播/提现分别启用条件、与旧 payout queue 的互斥、历史资金不一致拒绝启动。对有任一新业务事实的库，055/056 down 必须拒绝；禁止为满足回滚而删除业务事实。
7. 将演练回执、digest、校验和、操作者、开始/结束时间和每步 exit code 留存在受限位置；回执不包含密钥、OpenID、IM token、支付原始 payload 或用户行内容。

仅在上述证据通过且取得生产变更授权后，才可执行实际迁移。现场漂移、备份恢复失败、DDL 部分应用或检查不符时保持停写，保留现场并前向修复；不自动 down、不自动全库恢复、不只回滚旧 binary。

## 默认关闭部署与分阶段启用

新候选初次部署保持 `ENABLE_DELIVERY_RIDER_BROADCAST=false`、`ENABLE_DELIVERY_RIDER_WITHDRAWAL=false`、`ENABLE_DELIVERY_RIDER_PAYOUT_QUEUE=false`，通知发送保持关闭；不擅自改变已有支付和日结状态。新广播守卫关闭时不能接受新抢单，必须协调维护窗口及对应前端，不能仅部署后端就声称骑手已可接单。

部署后先核对 binary SHA256、运行时 commit、readyz、进程环境和停写前资金 digest。然后通过绑定候选的只读门禁；提现还需启动来源一致性检查成功。以下启用分别需要明确授权与回执，不能互相推断：

- 广播：055、对应前端、主动开工入口和可调容量均准备好，确认停工/满载/资格变化/订单被抢的发送前与抢单复核；通知发送仍可保持关闭。
- 提现：056、来源一致性、处理权/审计、真实后台操作者和异常处理手册均通过；只开放真实申请及人工登记，不调用自动微信转账。
- 微信发送：真实模板/字段、正确 `developer|trial|formal`、本人测试白名单、有效授权额度、Tencent IM SDKAppID/回调 token 和控制台回调均核对；仅本人真实事件，不造假订单或回调。
- 支付/退款/人工打款：确认具体账号、金额和动作后由本人/授权后台操作者执行；禁止测试脚本直接发起真实资金操作。

## 真机与外部验收仍为 PENDING

| 验收项 | 必需证据 |
| --- | --- |
| 最新体验版 | 本次官方上传版本与 commit/资产关联、公众平台已设为体验版截图或回执；不是旧上传记录 |
| UI/白屏 | 本次版本冷/热启动、设置二级、两钱包、普通分包首次进入、固定透明底栏、身份切换和网络失败的真机记录 |
| 微信通知 | 真实事件→站内记录→微信 API 受理→本人手机收件→点击正确页面各自留证；accepted 不等于已送达 |
| 私信 | 控制台 C2C.CallbackAfterSendMsg、拒绝非法签名、双账号真实消息、去重、收件/跳转；不宣称已有全量离线权威信号 |
| 支付与订单池 | 商家真实支付/取消/退款；未支付订单不可被骑手发现或抢单，支付成功只形成一笔资金/订单事实 |
| 骑手次日结算 | 真实完成配送后先有预计收入、不提前入可提现余额；北京时间次日 00:00 后仅结算一次，前后金额与流水守恒 |
| 人工提现 | 来源核验、占用、后台领取、真实转账凭证、登记后金额守恒；结果未知保持占用，确认未付才释放 |

本地 `npm run verify:release`、fixture、浏览器预览和构建产物检查不能替代上表。体验版、审核提交、审核通过、正式发布必须分别记录；未取得证据的项目继续 PENDING。
