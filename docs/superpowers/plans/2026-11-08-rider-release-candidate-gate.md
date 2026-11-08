# 骑手发布候选绑定 Implementation Plan

> **For agentic workers:** 按当前主代理已确认范围在本线程完成；不启动额外代理、不执行生产写入。

**Goal:** 防止旧版本自洽状态被误当作本次候选发布验收通过。

**Architecture:** 保留无参数只读盘点。`evaluate(snapshot, expected_commit=None)` 在传入候选时要求 12..40 位小写十六进制，并要求运行时 commit 至少同长且匹配该前缀；不匹配时所有 ready 门禁关闭。CLI 增加 `--require-ready candidate|broadcast|withdrawal`，该模式必须同时指定 `--expected-commit`，门禁失败退出 1。默认关闭 candidate 门禁还要求广播、提现、旧 payout queue 和真实通知发送均关闭，不误阻断分别授权的广播/提现预检。

**Tech Stack:** Python 标准库、unittest、现有 JSON fixture；不增加依赖。

## Task 1：候选身份及命令门禁

Files: `deploy/native/inspect-rider-release.py`、`scripts/test-rider-release-preflight.py`。

- [x] 增加匹配、错版、短前缀、非法值、runtime 缺失的 fixture 测试。
- [x] 增加 CLI 子进程测试：只读盘点不宣称验收；没有 expected 的 require 拒绝；已绑定成功 exit 0；错版或缺 schema exit 1。
- [x] 实现上述参数和 JSON `expectedCommitMatches` 字段；不输出密钥、用户标识或行内容。
- [x] 执行 `python -B scripts/test-rider-release-preflight.py`，29 项全部通过，exit 0；其中独立 fixture 和 CLI 覆盖 payout queue/通知发送不能通过默认关闭 candidate 门禁，以及损坏/缺失 fixture 非零退出且不泄露内容或路径。

## Task 2：055/056 部署准备交付

File: `docs/operations/2026-11-08-rider-release-preparation.md`。

- [x] 明确旧 052/053/054 部署脚本不可复用到本次迁移。
- [x] 逐项记录候选冻结、停写、备份/恢复验证、055/056 演练、资金守恒、默认关闭部署和分阶段授权条件。
- [x] 将微信平台配置、真实通知/支付、真机、次日结算保留 PENDING，不用本地 fixture 代替。
- [x] 执行 `git diff --check`，exit 0，提交权保留给主代理统一处理。

## Task 3：补齐商家资金数据库验收

Files: `scripts/test-merchant-money-mysql.cjs`、`package.json`。

这是既有资金验收遗漏修复，不改变资金产品或业务接口。`test:backend:fresh` 清空 DSN，原商家资金测试会跳过；骑手隔离 runner 不包含这些测试，不能替代它们。

- [x] 为 `zz_funds_mysql_test.go`、`zz_recharge_cancel_mysql_test.go` 的全部 `AgainstRealMySQL84` 测试增加专用隔离 runner；测试名从源文件收集，必须逐个出现 PASS。
- [x] 清除继承的 MYSQL/DSN 环境变量，使用独立临时 datadir 和 loopback 端口；验证 `@@datadir`、MySQL 8.4 及清理边界；既有共享数据库不参与。
- [x] 运行 `npm run test:merchant-money:mysql`：15 个测试实际执行、无 SKIP、临时数据库剩余 0、临时实例和目录清理通过。
- [x] 将该命令纳入 `verify:release`，不增加依赖。
- [x] 扩充后的 `npm run verify:release` 完整通过，顶层 exit 0；最后增加的异常 fixture 用例另以完整预检 29/29 复验。独立审查通过，统一提交与更新验收记录；本地通过不替代真实微信支付。
- [x] 两个资金 runner 同时拦截顶层及缩进子测试 SKIP；提取实际脚本的表达式以合成输出验证，不误报 SQL `SKIP LOCKED`。
