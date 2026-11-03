# 上线候选收尾与回归验收计划

> **For agentic workers:** 按 writing-plans 分任务执行；当前未提供 superpowers:subagent-driven-development / superpowers:executing-plans，使用现有子代理分别审查、主代理复核并统一验收。步骤以复选框记录。

**Goal:** 收口用户已经批准的 UI、钱包、通知实现，补候选遗漏和可复现缺陷，给出可复验的上线准备结论。

**Architecture:** 在 `codex/experience-ui-wallet-20261101`、起点 `c08b2df` 上最小补丁集成；对照另一工作树但不修改它、不整体覆盖分支。保留固定透明底栏、当前页面布局和所有资金/通知默认关闭门禁。测试使用本地隔离数据，不连接生产账本或调用真实微信支付/通知。

**Tech Stack:** Taro / React / TypeScript、Go / GORM、隔离 MySQL 8.4、Node 验收脚本；不增加依赖。

---

## 依据与不变边界

- `docs/superpowers/plans/2026-11-02-rider-daily-settlement-closeout.md`：北京时间次日 00:00、预计收入、人工提现、迁移及余额保护。
- `docs/superpowers/specs/2026-11-01-order-private-notifications-design.md` 与实施状态：站内加订阅双通道；默认关闭、真实模板/授权/回调验收单列。
- 用户确认商家收益仅来自官方外部订单真实应收；接口未接通不入账。
- 不执行生产迁移、正式发布、真实扣款退款转账，不自动启用资金或订阅发送开关。浏览器预览不作为手机验收。

## Task 1：候选遗漏与小程序路由验收

**审查文件：** `apps/miniapp/src/app.config.ts`、`apps/miniapp/src/pages/notifications/index.tsx`、`scripts/check-miniapp-static-routes.cjs`、`scripts/check-miniapp-dark-theme.cjs`、`scripts/generate-miniapp-dark-theme.cjs`；与 `8d6257b`、`4c43bb4`、`f6be51c`、`dfbca09` 比较。

- [ ] 用 `git log --left-right --cherry-pick --oneline HEAD...codex/service-chat-refinement-20261018` 及逐文件 diff 区分遗漏与候选主动修改，不恢复底栏收起逻辑。
- [ ] 先运行 `node scripts/check-miniapp-dark-theme.cjs` 复现分包路径错误；核对通知入口到路由、登录守卫、主题生成和分包产物。
- [ ] 仅移植已经批准且候选缺少的修复；每项对应一个失败/回归断言。候选已有的新资金/通知改动不被另一分支覆盖。
- [ ] 运行路由、主题、通知契约、固定底栏、工作台手势检查；预期全部 exit 0。

## Task 2：通知发送边界

**审查文件：** `backend/cmd/server/user_notification_im.go`、`backend/cmd/server/user_notification_templates.go`、`backend/cmd/server/user_notifications.go`、对应测试及 `backend/internal/config/config_test.go`。

- [ ] 审查过期、目的不符、错模板、快照变化、拉黑、停用、静默、白名单等拒绝分支，区分可达漏洞与已有下层保护。
- [ ] 对确认缺陷先补失败用例，再作最小安全修复；测试不得发送真实微信请求。
- [ ] `node scripts/check-notification-contract.cjs`、`node scripts/test-notification-contract.cjs`、`node scripts/test-notification-runtime.cjs`、`npm run test:backend` 全部 exit 0。

## Task 3：资金与迁移回归

**审查文件：** `scripts/test-rider-money-mysql.cjs`、`backend/internal/module/delivery/rider_daily_settlement_repository.go`、`backend/internal/module/delivery/rider_daily_settlement_service.go`、`backend/migrations/054_delivery_rider_daily_settlement.sql`。

- [ ] 先确认 runner 只使用临时数据目录、专用 loopback 端口，并在删除前验证边界；不继承共享数据库 DSN。
- [ ] 执行 `node scripts/test-rider-daily-settlement.cjs` 与 `node scripts/test-rider-money-mysql.cjs`，记录实际数据库测试数量、SKIP 情况和清理结果。
- [ ] 复核余额占用/释放/并发、真实付款凭据、退款阻断、历史迁移、00:00 时间边界及提现写门禁；不为通过验收移除门禁。

## Task 4：完整验收与交付

**文件：** 新增本轮 `docs/operations/2026-11-03-release-readiness.md`；必要时在 `scripts/` 增加可重复的本地验收入口，不改依赖。

- [ ] 执行 `npm run verify`，保留完整退出码；其安全 Go runner 清除共享 DSN，数据库用例结果由 Task 3 单列。
- [ ] 执行钱包页面/恢复、手工订单支付、通知、暗色、底栏和 UI 产物专项；未纳入 verify 的检查不能遗漏。
- [ ] 在 `backend` 执行无缓存 `go test ./... -count=1`（清空共享 DSN）及 `go vet ./...`。
- [ ] 执行 `npm run build:miniapp`、`npm run build:admin`，之后运行 `node scripts/check-miniapp-ux-artifact.cjs`，避免使用旧 dist 假通过。
- [ ] `git diff --check`、审查变更清单；中文提交并记录 hash。不 push、不部署、不上传。
- [ ] 报告逐项测试退出码、构建警告、版本差异处理和仍需生产/真机验收项目；不把本地通过称为全部上线完成。
