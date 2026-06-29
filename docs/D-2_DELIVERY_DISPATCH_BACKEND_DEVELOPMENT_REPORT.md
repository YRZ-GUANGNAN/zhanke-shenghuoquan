# D-2 Delivery Dispatch Backend Development Report

## 1. 结论

D-2 后端已按授权完成，代码提交为 `bf386a46`。本模块只建立配送商家建单、管理员开通骑手、骑手抢单与履约、可靠 Outbox 通知和 `ENABLE_DELIVERY` 运行时接线；未实现前端、真实配送费支付、照片、IM 或 D-3 及后续内容。

## 2. 前置门禁复核

- 基线：`master@930f7647`，包含 `d1-delivery-skeleton-baseline`。
- 标签：`d0-legacy-gate-baseline`、`d1-delivery-skeleton-baseline` 均存在。
- 最大既有迁移号为 `023`，D-2 使用 `024`。
- `backend/cmd/server` 中 `db.AutoMigrate(` 命中为 0。
- D-1 Delivery 核心已有 15 条状态转换与 `status + row_version` CAS。
- 既有 `takeout.RiderService` 依赖旧订单操作，不适合作为共享身份能力；底层 `CreateRider/FindRider` 可形成无订单依赖的窄接口。
- `ENABLE_DELIVERY` 已存在但此前未装配运行时。

## 3. 实现内容

### 3.1 迁移 024

新增：

- `delivery_orders.merchant_request_id CHAR(26) NULL`；
- 唯一索引 `(merchant_user_id, merchant_request_id)`；
- 独立 `delivery_outbox`；
- 事件身份、聚合版本幂等键、状态 CHECK、租约字段、重试字段、CAS `row_version` 与领取热点索引。

迁移不包含跨域外键，提供完整 rollback。

SHA-256：

- `024_delivery_dispatch.sql`: `ADDAA0D053F9AB7C58A47A284B1F2392CAC9BA7F39651AC0BFD8997887FCFD5F`
- `rollback/024_delivery_dispatch.sql`: `27C925B47FA85111082B90854086DEA91A033C8AC5931EBC566057A7DED815C7`

### 3.2 骑手身份窄接口

在冻结 takeout 域仅增加身份专用接口：

- `RiderIdentityRepository`: 只包含 `CreateRider/FindRider`；
- `RiderIdentityService`: 只包含 `Find/RequireActive/RegisterApproved`；
- Delivery 核心只认识 `RiderEligibility.RequireActive`；
- `admin/deliveryrider.IdentityAdapter` 在组合层完成适配。

Delivery 运行时未构造旧 `OrderService`、`RiderOrderOps`、旧 `RiderService`、支付、目录或旧路由。管理员开通骑手前还会验证目标用户真实存在。

### 3.3 配送编排

新增 `DispatchService/DispatchRepository`：

- 商家按 JWT 用户身份幂等建单；
- 服务端固定费用快照：配送费 150 分、骑手 100 分、平台 50 分；
- 生产语义始终创建为 `pending_fee_payment`；
- 商家订单列表、详情、取消均限定 `merchant_user_id`；
- 抢单池只返回最小信息；
- 抢单、履约、释放回池均复用 D-1 CAS 状态机；
- 非所属骑手详情返回 404，避免泄露订单存在性；
- `TransitionToReadyAndEnqueue` 在同一事务中推进状态并写 Outbox，事件版本使用更新后的 `row_version`。

### 3.4 HTTP 接口

商家 `/api/v1/delivery/merchant`（全部 `AuthRequired`）：

- `POST /orders`
- `GET /orders`
- `GET /orders/:id`
- `PATCH /orders/:id/cancel`
- `POST /orders/:id/mock-confirm-fee`，仅精确 mock 门成立时注册

骑手 `/api/v1/delivery/rider`（全部 `AuthRequired` + active 骑手校验）：

- `GET /me`
- `GET /orders/claimable`
- `POST /orders/:id/claim`
- `GET /orders`
- `GET /orders/:id`
- `PATCH /orders/:id/to-pickup`
- `PATCH /orders/:id/picked-up`
- `PATCH /orders/:id/delivering`
- `PATCH /orders/:id/delivered`
- `PATCH /orders/:id/release`

管理员 `/api/v1/admin/delivery`（`AuthRequired + AdminRequired`）：

- `POST /riders`

### 3.5 可靠 Webhook Outbox

- `ready_for_claim` 与 Outbox 写入同一 MySQL 事务；
- `SELECT ... FOR UPDATE SKIP LOCKED` 领取；
- lease token + `row_version` CAS 防止旧 worker 覆盖；
- 指数退避、最大次数、`deadletter`；
- URL 未配置时事件进入 `skipped`，不阻塞启动；
- 外呼失败不会回滚订单；
- 企业微信 HTTP 200 后仍校验响应 `errcode`；
- 消息只包含订单号、路线摘要、配送费和事件号，不包含姓名、完整电话、地址或房间号；
- URL/key 不写入日志、错误或报告。

### 3.6 配置和门控

新增：

- `ALLOW_MOCK_DELIVERY_FEE_CONFIRMATION=false`
- `WECOM_RIDER_WEBHOOK_URL=`

安全条件：

- `APP_ENV=prod` 且 mock 配送费确认开启时，`config.Validate` 拒绝启动；
- mock 配送费确认不复用旧 `ALLOW_MOCK_PAYMENT`；
- `ENABLE_DELIVERY=false` 时不构造 Repository、身份适配、Webhook、Consumer，不启动 worker，不注册任何 Delivery 路由。

## 4. 边界证明

- Delivery 核心未导入 Gin、HTTP、takeout、media、IM 或其他业务域。
- 外部 HTTP 仅存在于 `admin/deliverynotify`。
- takeout 变更仅为身份专用新增文件，无旧订单运行时依赖。
- 社区、二手、media、IM、apps 均为零差异。
- AutoMigrate 仍为 0。
- ADR-002 正式入库仍是独立文档待办，不阻塞本模块代码验收。

## 5. 已知边界

- 企业微信群机器人真实 URL 未写入仓库，也未在验收中向生产群发送消息；Sender 的 HTTP、状态码与 `errcode` 行为由隔离测试服务器验证。
- 动态费率、真实配送费支付、照片、IM 和前端明确留给后续独立授权。
