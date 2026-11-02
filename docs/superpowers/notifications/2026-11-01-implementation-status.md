# 订单与私信通知实施状态

日期：2026-11-01

## 已落地

- 订单状态在 delivery 同一事务写入独立通知事件，事件展开器以租约/CAS 幂等展开到站内收件箱和 `in_app` 投递；站内新通知默认跟随 `ENABLE_USER_NOTIFICATIONS=false` 关闭。
- 商家与订单当前骑手的状态通知只使用公开单号、状态和受保护路由；订单状态已经变化或账号无效时丢弃迟到通知。骑手新单池暂不广播，因为当前模型没有可验证的在岗、服务范围和容量字段。
- 小程序应用级 IM 未读快照、角色/账号代次隔离、当前可见会话已读、通知页订单收件箱和主动点击订阅授权已经接入；不会把当前页数量冒充全量未读。
- 微信订阅 sender、模板字段校验、一次性额度预留/释放/消耗、未知结果禁止自动重发、租约过期额度终结已实现，但 sender 默认关闭。
- 收益记录、骑手提现受理/后台打款/拒绝、商家充值和退款状态已在原资金事务内追加元数据通知事件；事件失败会回滚资金事务，退款重复 `PROCESSING` 轮询不会重复提醒。通知只使用公开业务 ID、金额 cents 和状态。
- Tencent IM C2C 回调已提供公网 Handler：签名/SDKAppID/结果校验、账号与用户关系复核、双向拉黑/活跃状态/消息开关复核、receipt+event 原子幂等和敏感正文丢弃均已接入。GROUP 回调不会被提升为普通私信。
- 组合根已注册 `/api/v1/notification/tencent-im/callback`；仅配置 `NOTIFICATION_IM_CALLBACK_TOKEN` 且 `ENABLE_USER_NOTIFICATIONS=true` 时启用。私信事件默认只生成小程序内通知；只有显式配置已审核的私信模板字段、测试用户白名单并开启发送开关时，才会生成 metadata-only 微信订阅任务，正文、provider ID 和联系方式不会进入快照。

## 仍需真实配置/验收

- 必须提供已审核的真实模板 ID、字段映射、测试用户白名单，并分别开启 `ENABLE_USER_NOTIFICATIONS`、`ENABLE_WECHAT_SUBSCRIBE_NOTIFICATIONS`、`ENABLE_NOTIFICATION_SENDING`；未配置时不会请求微信接口。
- “微信接口 accepted”只表示接口受理，不表示用户已读；本地 fake、静态检查和体验版上传不能证明真实送达。
- 私信微信订阅只完成安全的受控试运行链路：没有模板、白名单、授权或实时关系复核时自动降级为站内通知；当前没有服务端离线/当前会话不可见信号，不能宣称已完成面向全量用户的微信私信送达验收。
- 骑手新单池仍未广播：现有骑手模型没有可验证的在岗、服务范围和容量字段，不能把审核通过或 active 账号当作可接单资格。
- 本轮未执行真实 MySQL 迁移、真实微信发送、生产部署或体验版上传。

## 本轮验收

- `node scripts/check-notification-contract.cjs`：PASS。
- `node scripts/test-notification-contract.cjs`、`node scripts/test-notification-runtime.cjs`、`node scripts/check-private-messaging.cjs`：PASS。
- `npm --workspace apps/miniapp run typecheck`、`npm run test:backend`、`go test ./... -count=1`、`go vet ./...`：PASS。
- `npm run build:miniapp`：构建可运行；仅保留既有 CSS order 和 bundle 体积警告，未把警告当作失败。
