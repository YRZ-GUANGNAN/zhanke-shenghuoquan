# 订单与私信双通道通知实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改变现有订单、支付、钱包、配送状态机和 UI 风格的前提下，落地站内通知与用户主动授权的小程序订阅消息，并为离线/未读私信建立可信的通知桥接。

**Architecture:** 新增独立 `notification` 后端领域，保存通知事件、站内收件箱、渠道投递任务、订阅授权和 Tencent IM 回调去重元数据。订单事务只通过 delivery 内部 port 写入领域事件；组合根装配收件人、用户、IM 和微信能力，事务外 worker 分别投递站内与微信渠道。小程序只做应用级 IM 未读快照、主动订阅授权和受保护路由跳转。

**Tech Stack:** Go、Gin、GORM、MySQL migration、Tencent Cloud Chat callback、微信小程序 `wx.requestSubscribeMessage`、Taro/React、现有 Node contract scripts。

---

## 任务前文件映射

**创建：**

- `backend/migrations/053_delivery_notifications.sql`：通知事件、inbox、投递任务、订阅授权、IM 回调去重表和索引。
- `backend/internal/module/notification/model.go`：通知领域状态、事件、inbox、delivery、subscription、callback receipt 模型。
- `backend/internal/module/notification/repository.go`：GORM repository、幂等插入、租约认领、CAS 状态转换、授权额度占用。
- `backend/internal/module/notification/service.go`：事件展开、收件规则、静音/过期、渠道任务生成。
- `backend/internal/module/notification/handler.go`：认证 inbox、偏好/授权上报、微信 IM 回调路由。
- `backend/internal/module/notification/subscribe_sender.go`：微信 access token 缓存和订阅消息 HTTP sender。
- `backend/internal/module/notification/consumer.go`：通知任务 worker、重试、未知结果和死信。
- `backend/internal/module/notification/ports.go`：收件人资格、账号状态、IM 会话和用户偏好的组合根接口。
- `backend/internal/module/notification/*_test.go`：单元、迁移、HTTP、worker 和回调测试。
- `backend/internal/module/delivery/notification_port.go`：delivery 事务可调用的领域事件 port 和不可变事件快照。
- `apps/miniapp/src/services/notification.ts`：inbox、授权模板、授权结果、未读角标和受保护跳转 API。
- `apps/miniapp/src/hooks/use-notification-runtime.ts`：登录后唯一 IM 监听、回前台快照和账号代次隔离。

**修改：**

- `scripts/check-boundaries.cjs`：登记 `notification` 独立领域并保留 delivery/notification 禁止反向依赖规则。
- `backend/internal/config/config.go`、`config_test.go`：订阅消息开关、模板键、IM 回调配置和 fail-closed 校验。
- `backend/internal/module/delivery/repository.go`、`dispatch_repository.go`、`funds_repository.go`、`proof_repository.go`：在既有事务内追加领域事件 port 调用。
- `backend/internal/module/admin/deliverynotify/consumer.go`、`sender.go`、相关测试：停止与用户通知任务争抢原始 outbox，保留企业微信独立渠道。
- `backend/cmd/server/main.go`：组合根装配 ports、路由、worker 和独立 channel sender。
- `backend/internal/module/im/handler.go`、`service.go`：提供可信 Tencent 回调所需的账号/关系查询能力，不保存消息正文。
- `apps/miniapp/src/app.tsx`、`services/e-im.ts`、`services/delivery-im.ts`：应用级监听、快照、已读和会话代次处理。
- `apps/miniapp/src/pages/settings/index.tsx`：用户点击触发订阅授权，展示“已订阅/未授权/不可用”，不把开关当成永久授权。
- `apps/miniapp/src/pages/notifications/index.tsx`、`services/b-community.ts`：展示层聚合独立 delivery inbox，保持社区通知枚举与旧接口兼容。
- `apps/miniapp/src/components/student-bottom-nav.tsx`、`components/delivery-workspace-nav.tsx`、`pages/home/index.tsx`、`pages/merchant-messages/content.tsx`：仅接入未读角标和受控刷新，不改现有布局。

### 任务 1：通知数据库与领域模型

**Files:**

- Create: `backend/migrations/053_delivery_notifications.sql`
- Create: `backend/internal/module/notification/model.go`
- Create: `backend/internal/module/notification/repository.go`
- Create: `backend/internal/module/notification/ports.go`
- Create: `backend/internal/module/notification/migration_053_test.go`
- Modify: `scripts/check-boundaries.cjs`

- [ ] **Step 1: 写迁移契约测试**

在 `migration_053_test.go` 中读取迁移源文本，断言存在以下表和约束：`delivery_notification_events`、`delivery_notification_inbox`、`delivery_notification_deliveries`、`notification_subscriptions`、`im_notification_receipts`；事件版本唯一键、事件×收件人×渠道×purpose 唯一键、recipient/channel/status 索引、租约索引、openid 不可出现在正文列。

- [ ] **Step 2: 运行迁移契约测试确认失败**

运行：

```powershell
Push-Location backend
try { go test ./migrations -run TestMigration053 -count=1 }
finally { Pop-Location }
```

预期：因迁移文件和模型尚不存在而失败。

- [ ] **Step 3: 写数据库迁移**

`053_delivery_notifications.sql` 使用 `utf8mb4`，为事件表保存 `event_public_id`、`event_type`、`aggregate_public_id`、`aggregate_version`、脱敏 JSON payload、状态和时间；inbox 保存 recipient、purpose、标题/摘要、聚合坐标、read_at；delivery 保存 channel、template_key、模板快照、attempt/lease、provider_message_id、last_error_code；subscription 保存 appid/openid/template_key、授权状态、一次性额度和 nonce；receipt 保存 sdk_app_id、provider_message_id、sender/recipient、callback hash。所有状态使用 `CHECK` 和 `UNIQUE`，不允许把原始私信正文列入任何表。

- [ ] **Step 4: 实现模型与 repository**

在 `model.go` 中定义 `DeliveryStatus`、`Channel`、`NotificationPurpose` 和表模型；在 `repository.go` 中实现以下原子方法：

```go
AppendEventTx(tx *gorm.DB, event Event) error
EnsureInboxTx(tx *gorm.DB, item InboxItem) error
EnsureDeliveryTx(tx *gorm.DB, item DeliveryItem) error
ClaimNextDelivery(ctx context.Context, now time.Time, lease time.Duration) (DeliveryLease, bool, error)
CompleteDelivery(ctx context.Context, lease DeliveryLease, status DeliveryStatus, now time.Time) error
FailDelivery(ctx context.Context, lease DeliveryLease, code, safeMessage string, next time.Time, max uint32, now time.Time) (DeliveryStatus, error)
ReserveSubscription(ctx context.Context, userID uint64, appID, templateKey, nonce string, now time.Time) (SubscriptionReservation, error)
RecordCallbackOnce(ctx context.Context, receipt CallbackReceipt) (bool, error)
```

幂等冲突返回已有记录；租约状态更新必须带 row version 与 lease token；`accepted` 和 `unknown` 不回退微信额度。

- [ ] **Step 5: 运行迁移与边界测试**

运行：`Push-Location backend; try { go test ./migrations -run TestMigration053 -count=1 }; finally { Pop-Location }` 和 `node scripts/check-boundaries.cjs`，预期均 PASS。

- [ ] **Step 6: 提交任务 1**

```powershell
git add backend/migrations/053_delivery_notifications.sql backend/internal/module/notification scripts/check-boundaries.cjs
git commit -m "通知：建立订单与私信通知数据模型"
```

### 任务 2：订单事务事件与单一展开器

**Files:**

- Create: `backend/internal/module/delivery/notification_port.go`
- Create: `backend/internal/module/notification/service.go`
- Create: `backend/internal/module/notification/expander.go`
- Create: `backend/internal/module/notification/service_test.go`
- Modify: `backend/internal/module/delivery/repository.go`
- Modify: `backend/internal/module/delivery/dispatch_repository.go`
- Modify: `backend/internal/module/delivery/funds_repository.go`
- Modify: `backend/internal/module/delivery/proof_repository.go`

- [ ] **Step 1: 写事件 port 单测**

定义 delivery 不依赖 notification 包的接口：

```go
type NotificationEventAppender interface {
    AppendNotificationEventTx(*gorm.DB, NotificationEvent) error
}
```

测试 `TransitionToReadyAndEnqueue`、`PayOrderWithWallet`、`ConfirmWechatPayment`、`Transition`、`CreateProofAndTransition` 和 `RequestRefund` 在事务提交时各追加一次版本化事件，事务失败时事件数量为零。

- [ ] **Step 2: 运行单测确认失败**

运行：`Push-Location backend; try { go test ./internal/module/delivery -run TestNotificationEvent -count=1 }; finally { Pop-Location }`，预期失败，因为 port 和 appender 尚未接入。

- [ ] **Step 3: 接入准确事务插点**

在既有事务中调用 appender，不在 HTTP handler、微信支付回调、reconciler 或前端结果中重复生成事件。支付成功沿 `PayOrderWithWallet`、`ConfirmWechatPayment` 共享结算路径进入 `dispatch_repository.go:transitionToReadyAndEnqueueTx`；普通状态覆盖 `repository.go:Transition`，凭证覆盖 `proof_repository.go:CreateProofAndTransition`，退款覆盖 `funds_repository.go:RequestRefund`。事件键由订单公开 ID、row version 和 event type 构成。

- [ ] **Step 4: 实现收件规则与过期复核**

在 `notification/service.go` 实现 `ExpandEvent`：`RiderPoolOpened` 只查询 active/接单/范围/容量/偏好均满足的骑手；商家状态只发订单所属商家；私信事件按 recipient+peer 合并；新单在任务投递前再次查询可抢状态，取消/已抢/过期变为 `skipped`。所有跨域查询通过 `ports.go` 接口传入组合根。

- [ ] **Step 5: 让原始 delivery outbox 只有一个认领者**

`expander.go` 负责认领并展开 `delivery_outbox`，成功创建独立 inbox/delivery 任务后再完成原始事件；历史 `succeeded/skipped/deadletter` 不回补。`admin/deliverynotify` 不再直接和 expander 争抢同一 outbox，企业微信改为独立 delivery channel。

- [ ] **Step 6: 运行订单与通知测试**

运行：`Push-Location backend; try { go test ./internal/module/delivery ./internal/module/notification -count=1 }; finally { Pop-Location }`，预期事务回滚、重复事件、状态过期、跨角色收件测试全部 PASS。

- [ ] **Step 7: 提交任务 2**

```powershell
git add backend/internal/module/delivery backend/internal/module/notification
git commit -m "通知：接入订单事务事件与幂等展开"
```

### 任务 3：站内投递、微信订阅 sender 与配置

**Files:**

- Create: `backend/internal/module/notification/consumer.go`
- Create: `backend/internal/module/notification/subscribe_sender.go`
- Create: `backend/internal/module/notification/handler.go`
- Create: `backend/internal/module/notification/consumer_test.go`
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/config/config_test.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/internal/module/admin/deliverynotify/consumer.go`
- Modify: `backend/internal/module/admin/deliverynotify/sender.go`

- [ ] **Step 1: 写 sender/配置失败测试**

覆盖：模板 ID/openid/access token 缺失返回 `blocked`；微信返回明确拒绝、模板错误、token 失效和限流分别映射脱敏 error code；网络超时映射 `unknown` 且不自动释放额度；同一 lease 重复执行不重复发送。

- [ ] **Step 2: 增加 fail-closed 配置**

在 `Config` 增加 `EnableWechatSubscribeNotifications`、订单/私信模板键、IM callback token 和发送状态开关，默认关闭；`Validate` 禁止生产环境缺少 `WECHAT_APP_ID/SECRET` 时打开 sender，禁止把 secret/template secret 写入日志。配置测试覆盖默认关闭、成对凭据和生产校验。

- [ ] **Step 3: 实现 access token 缓存和微信发送**

`subscribe_sender.go` 只从服务端配置读取 AppID/AppSecret，缓存带提前刷新时间的 access token，POST `/cgi-bin/message/subscribe/send`，严格校验 `touser`、template_id、page 和字段长度。返回只保留微信错误码、任务 ID 和脱敏响应，不打印 token、openid 或原始 body。

- [ ] **Step 4: 实现独立 channel consumer**

`consumer.go` 认领 notification delivery，按 channel 调用站内或微信 sender，使用指数退避和死信；`accepted/delivered/read/unknown/blocked/skipped` 按设计转换。改造 `deliverynotify` 使其不再认领同一用户通知事件，保留企业微信运维 webhook 的独立任务/开关。

- [ ] **Step 5: 实现认证 inbox 与订阅上报 API**

`handler.go` 注册：`GET /notification/inbox`、`POST /notification/inbox/:id/read`、`GET /notification/subscribe/templates`、`POST /notification/subscribe/authorize`。授权上报只接模板键、结果和一次性 nonce，服务端重新验证当前用户，不接受客户端传入 openid/AppSecret；未授权时仍返回站内通知。

- [ ] **Step 6: 运行后端配置、HTTP 和 worker 测试**

运行：`Push-Location backend; try { go test ./internal/config ./internal/module/notification ./internal/module/admin/deliverynotify ./cmd/server -count=1 }; finally { Pop-Location }`；预期覆盖 sender、租约、限额、HTTP 权限和历史 outbox 不回补。

- [ ] **Step 7: 提交任务 3**

```powershell
git add backend/internal/config backend/internal/module/notification backend/internal/module/admin/deliverynotify backend/cmd/server/main.go
git commit -m "通知：接入站内投递与微信订阅通道"
```

### 任务 4：Tencent IM 可信回调与私信通知

**Files:**

- Create: `backend/internal/module/notification/im_callback.go`
- Create: `backend/internal/module/notification/im_callback_test.go`
- Modify: `backend/internal/module/im/handler.go`
- Modify: `backend/internal/module/im/service.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: 写回调安全测试**

构造合法签名、错误签名、错误 sdkAppID、重复 provider message ID、未知参与者、双向拉黑、禁用二手 feature flag 和配送 GROUP 已撤销成员的回调；只有合法且参与关系仍存在的回调创建脱敏事件。

- [ ] **Step 2: 实现回调校验和去重**

校验腾讯配置的应用标识/签名、事件类型和发送结果；使用 `sdkAppID + providerMessageID + sender + recipient` 作为回调 receipt key。只保存收件人、peer/order、未读数量、时间和安全跳转坐标，严禁落库或日志输出原始 message body。

- [ ] **Step 3: 实现 C2C/GROUP 收件规则**

C2C 以 recipient+peer 合并多个 product/user relation；GROUP 以订单公开 ID和当前成员单独去重，重新读取 conversation 状态、参与者、active 和拉黑关系，不能从回调字段绕过 feature gate。

- [ ] **Step 4: 注册安全回调路由并验收**

在组合根注册 `POST /notification/tencent-im/callback`，复用 sanitized recovery 和跳过敏感 body 日志的路由配置；运行 `go test ./internal/module/notification ./internal/module/im ./cmd/server -count=1`。

- [ ] **Step 5: 提交任务 4**

```powershell
git add backend/internal/module/notification backend/internal/module/im backend/cmd/server/main.go
git commit -m "通知：接入腾讯 IM 私信回调"
```

### 任务 5：小程序应用级未读、主动订阅和通知展示

**Files:**

- Create: `apps/miniapp/src/services/notification.ts`
- Create: `apps/miniapp/src/hooks/use-notification-runtime.ts`
- Modify: `apps/miniapp/src/app.tsx`
- Modify: `apps/miniapp/src/services/e-im.ts`
- Modify: `apps/miniapp/src/services/delivery-im.ts`
- Modify: `apps/miniapp/src/pages/settings/index.tsx`
- Modify: `apps/miniapp/src/pages/notifications/index.tsx`
- Modify: `apps/miniapp/src/services/b-community.ts`
- Modify: `apps/miniapp/src/components/student-bottom-nav.tsx`
- Modify: `apps/miniapp/src/components/delivery-workspace-nav.tsx`
- Modify: `apps/miniapp/src/pages/home/index.tsx`
- Modify: `apps/miniapp/src/pages/merchant-messages/content.tsx`
- Create: `apps/miniapp/src/services/notification.test.ts`

- [ ] **Step 1: 写服务层契约测试**

测试 `loadNotificationInbox`、`markNotificationRead`、`requestNotificationSubscription` 和 `mergeUnreadSnapshot`：未确认快照不显示假 0；重复会话事件不重复累加；旧账号代次事件不能写入新账号。

- [ ] **Step 2: 实现 notification service**

从后端读取真实模板键，映射到构建时不含 secret 的模板 ID；只在显式点击 handler 中调用 `wx.requestSubscribeMessage`，用 nonce 上报 accept/reject/ban/close/exception，授权失败不阻塞业务。inbox 查询使用独立 API，社区通知仍走原 `b-community.ts` 接口。

- [ ] **Step 3: 接入唯一应用级 IM 监听**

在 `use-notification-runtime.ts` 登录成功后静默连接 Tencent IM，订阅一次 `CONVERSATION_LIST_UPDATED` 与 `MESSAGE_RECEIVED`；回前台/重连读取未读快照。账号或角色切换先增加 generation、清空角标，再释放旧监听；旧 promise 拒绝回写。

- [ ] **Step 4: 修正已读和订单刷新边界**

仅当前可见且已鉴权的会话调用 `setMessageRead`；工作台通知只触发已有订单查询，保持页签、滚动、分页和操作反馈，不直接拼接订单或把未支付订单加入列表。隐藏页面停止轮询，失败按现有错误反馈处理。

- [ ] **Step 5: 接入展示层**

通知页聚合独立 delivery inbox 与社区通知，保持原视觉和路由；学生底栏、商家/骑手消息入口只增加未读角标；设置页增加两个主动订阅按钮和明确的“未授权/已授权/暂不可用”文案，不改现有消息开关含义。

- [ ] **Step 6: 运行小程序静态与类型测试**

运行：

```powershell
npm --workspace apps/miniapp run typecheck
node scripts/check-private-messaging.cjs
node scripts/test-chat-service-delivery.cjs
node scripts/check-miniapp-settings-contract.cjs
```

预期全部 exit 0，且不出现模板 ID、openid、secret 或私信正文。

- [ ] **Step 7: 提交任务 5**

```powershell
git add apps/miniapp/src
git commit -m "通知：补齐小程序未读与订阅授权"
```

### 任务 6：端到端验收与受控启用

**Files:**

- Modify: `scripts/check-boundaries.cjs`
- Create: `scripts/check-notification-contract.cjs`
- Create: `scripts/test-notification-contract.cjs`
- Modify: `docs/README.md` 或现有通知运维文档（只记录配置键和状态语义，不写真实密钥）

- [ ] **Step 1: 写静态合约检查**

检查 notification 路由、订阅必须由点击触发、secret 不进入 miniapp bundle、delivery 核心不导入 notification/user/im/community、社区 `/notifications` 契约未被替换、历史 delivery outbox 不回补。

- [ ] **Step 2: 运行完整验收命令**

```powershell
npm --workspace apps/miniapp run typecheck
node scripts/check-private-messaging.cjs
node scripts/test-chat-service-delivery.cjs
node scripts/check-boundaries.cjs
node scripts/check-notification-contract.cjs
Push-Location backend
try { go test ./... }
finally { Pop-Location }
npm run build:miniapp
git diff --check
```

每条命令必须 exit 0；webpack 既有体积/顺序警告可记录但不能把失败当成功。

- [ ] **Step 3: 保持微信 sender 默认关闭进行候选验收**

先只验证本地 inbox、IM 未读和 mock HTTP sender；不发送真实微信测试消息，不上传体验版，不部署生产。候选版本记录 commit、迁移、配置开关和测试结果。

- [ ] **Step 4: 真实测试账号受控启用**

在用户提供并确认真实模板、测试账号授权和测试窗口后，服务端以 feature flag 仅开启订单/私信各一条测试；检查微信接口结果为 `accepted` 不等于用户已读，未知结果不自动无限重发。测试完成后恢复默认关闭或按用户另行授权开启。

- [ ] **Step 5: 提交验收记录**

```powershell
git add scripts/check-notification-contract.cjs scripts/test-notification-contract.cjs docs
git commit -m "验收：完成订单与私信通知闭环检查"
```

## 依赖与停止条件

- 任务 1 完成并通过迁移/边界测试后才能开始任务 2；任务 2 完成后才能启动 worker。
- 任务 3 的微信 sender 永远以配置开关关闭为默认；没有真实模板、openid、授权和安全凭据时，只允许站内通道。
- 任务 4 必须先验证 Tencent 回调鉴权资料；没有可信回调配置时，私信只做小程序内未读，不创建微信外部任务。
- 任何迁移、订单事务、IM 回调或支付回归测试失败，都停止微信启用和体验版上传，保留站内通知候选。
- 不修改现有钱包支付实现、不改变用户 UI 视觉、不提交任何密钥或真实用户数据。

