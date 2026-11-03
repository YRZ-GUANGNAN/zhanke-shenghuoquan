# 微信订阅消息跳转版本兼容修复实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让既有订阅通知显式跳到受控小程序版本，避免体验验收误开旧正式版。

**Architecture:** 配置层加载并校验单一版本枚举；sender 构造器再次校验并固定语言，编码请求时附加版本和语言。业务消息、数据库快照与授权逻辑不变。

**Tech Stack:** Go、现有 config loader、net/http、testing。

---

### 任务 1：最小配置与 sender 兼容修复

**Files:**
- 修改 `backend/internal/config/config.go`、`backend/.env.example`。
- 修改 `backend/internal/module/notification/subscribe_sender.go`。
- 测试 `backend/internal/config/config_test.go`、`backend/internal/module/notification/subscribe_sender_test.go`。

- [x] 先补失败测试：配置加载 `"" → "formal"`，接受 `developer/trial/formal`，拒绝 `release/Trial/空格`；公开 sender 构造器经内存 HTTP transport 断言请求字段 `miniprogram_state` 和 `lang`。
- [x] 运行 `go test ./internal/config ./internal/module/notification -run 'TestWechatSubscribeMiniprogramState|TestSubscribeSenderMiniprogramState' -count=1`，确认缺少新字段导致编译失败；实现后同命令 exit 0。
- [x] 配置新增 `WechatSubscribeMiniprogramState string`，loader 使用 `env("WECHAT_SUBSCRIBE_MINIPROGRAM_STATE", "formal")`；Validate 对 `"", "developer", "trial", "formal"` 放行，其余报静态错误。示例配置默认 `formal`。
- [x] sender 配置和实例增加同名版本值；空值归一为 `formal`，其他值严格校验；wire payload 使用匿名组合，保留原消息字段并增加 `MiniprogramState string`（JSON `miniprogram_state`）和 `Lang string`（JSON `lang`），后者固定 `zh_CN`。
- [x] 使用 `gofmt`，`go test ./internal/config ./cmd/server ./internal/module/notification -count=1`、`go vet ./internal/config ./internal/module/notification`、`git diff --check` 均 exit 0；已清空当前测试进程的 `MYSQL_DSN`、`NOTIFICATION_MYSQL_DSN`，未连接生产数据库。
- [x] 向主代理汇报改动、测试结果与真机边界；遵照本次分工未自行提交、未改生产配置。

### 任务 2：35508 订单状态 phrase 兼容

**Files:** `backend/cmd/server/user_notification_templates.go`、`user_notification_templates_test.go`、新增 `user_notification_order_phrase_test.go`。

- [x] 先添加所有合法迁移的表驱动测试：通过现有 resolver 生成站内标题和 `phrase3` 快照，断言8种短状态、26位订单编号、北京时间、guard重新验证一致；断言待支付没有通知。
- [x] 添加未知标题（含不足5字的未知文案）、未知事件/状态、新单池、篡改短状态和私信未改动的回归测试。运行 `go test ./cmd/server -run 'TestOrderNotificationPhrase|TestPrivateMessagePhrase|TestNotificationTemplateMappingPreserves' -count=1` 验证红灯后实现并转绿。
- [x] 仅在订单 snapshot 的 `status` 分支、目标字段前缀为 `phrase` 时调用纯白名单函数；每个已知原文返回明确短状态，默认返回 `notification.ErrSubscribePayload`；原 `thing` 与 private snapshot 不动。
- [x] 将原测试中「订单phrase必定失败」改为「已知状态可兼容」；运行 `gofmt`、`go test ./cmd/server ./internal/module/notification ./internal/config -count=1`、`go vet ./cmd/server ./internal/module/notification ./internal/config`、`git diff --check` 均 exit 0。
- [x] 汇报父代理审查，不改生产配置，不选用真实模板，不提交。
