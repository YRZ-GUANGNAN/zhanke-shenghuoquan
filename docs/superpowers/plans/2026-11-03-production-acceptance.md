# 生产通知与骑手结算验收执行计划

> **For agentic workers:** 按本线程分工执行，生产写入仅由主代理进行；独立代理审查与测试，不重复操作线上。

**Goal:** 完成已授权的生产迁移与北京时间次日00:00结算启用，准备真实模板、授权和手机送达验收。

**Architecture:** 保留既有 Local/+08 DATETIME 口径，不修改历史时间与余额。先修复通知数据库时钟混用、显式订阅跳转版本；唯一冻结候选按052→053→054迁移，微信发送保持关闭至真实模板和本人授权验证通过。

**Tech Stack:** Go、MySQL8.0生产/MySQL8.4隔离测试、systemd、Taro、微信官方API/CLI、Python标准库部署脚本。

## 已核实的基线

- 本地工作树 `experience-ui-wallet-20261101`，原候选 `da9f851658eb`。
- 生产 `2026.10.30.preflight.2 / 7d22416948be`；单API进程、92表、约6.6MB；无DB事件/触发器/存储过程。
- 缺052 `cancel_requested_at`、053五张通知表、054两列及due索引。
- 骑手earning/wallet/ledger/withdrawal/reservation/payout_queue均0；配送单1且待支付；支付3笔均closed。
- 系统Asia/Shanghai，服务TZ未覆盖，DSN `loc=Local`，MySQL SYSTEM/CST。保持这些历史解释，只显式设置每连接 `time_zone='+08:00'`。
- 微信gettemplate成功返回空列表；类目为跑腿1259、信息资讯1363。不能用示例ID冒充真实模板。

## 执行与验收

- [x] 修复通知DB执行时钟与loc不一致；仅启用通知时校验显式fixed-offset契约；UTC与Shanghai过期/正向/并发实库测试。
- [ ] 订阅发送显式formal/trial/developer与zh_CN；默认formal不变，非法值拒绝。
- [ ] `deploy/native/release-notification-settlement.py` 默认只读预检；严格绑定旧版本/候选hash/迁移hash、备份与前后账本digest，拒绝不符现场。
- [ ] 全套Go fresh/vet、通知MySQL、资金MySQL、release regression通过，提交中文候选并重新构建Linux二进制。
- [ ] 上传仅本次资产到独立stage；sudo预检；停API后确认进程和数据库写连接均消失。
- [ ] 全库备份并完整gzip校验，恢复到专用验证库逐表比对；在验证库演练052/053/054。验证库不授予应用权限，验证后仅删除该精确库。
- [ ] 生产按序迁移，对比所有原表数据（支付新增列忽略、earning为空），验证due索引及NOT NULL；保留备份。
- [ ] 新版本先daily=false、user notifications=true、wechat subscribe=false、sending=false启动；保留支付和全部mock禁用策略，验证readyz/version/受保护入口。
- [ ] 单独启用daily=true，复核进程环境与账本无变；不开放提现，不执行微信资金请求。
- [ ] 官方选用订阅模板后gettemplate回读，配置真实字段与白名单；本人主动授权；真实事件、接口受理、手机收件与跳转分开记录。
- [ ] 官方CLI生成当前构建预览二维码，用户手机验收钱包/通知/固定透明底栏；不代替本人授权，不伪造扫码结果。

## 失败处理

迁移遇错立即停，保留备份和现场。054后不可只回滚旧binary，也不可执行down删除字段；产生新业务事实后只能前向修复或保持新候选daily=false。只有证明停写后未产生新事实时才能评估全量恢复，脚本不自动进行破坏性回滚。模板/本人授权/真机未完成必须保持PENDING，不写成PASS。
