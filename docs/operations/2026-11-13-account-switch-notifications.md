# 2026-11-13 腾讯云账号切换与通知配置记录

本记录只描述 2026-11-13 已实际完成的资源切换和验收，不把未发生的真实订单、付款或私信送达写成 PASS。

## 小程序体验版

- 微信公众平台已确认体验版为 `2026.11.13.2`。
- 该版本包含消息页统一下拉刷新、移除顶部“刷新”文字。
- 对应源码提交：`0ff3ed4`。
- 开发预览二维码：`.codex-tmp/preview-2026.11.13.2-message-refresh.png`。

## 腾讯云资源

- 腾讯 IM SDKAppID：`1600172602`。
- 腾讯 IM 应用名称：`湛科生活圈`。
- COS 存储桶：`zkshq-1507095774`。
- COS 地域：`ap-guangzhou`。
- COS AppID：`1507095774`。
- COS 桶保持私有读写，并已在控制台开启版本控制。
- 生产服务器使用独立子用户 `campus-circle-server`，未使用主账号 API 密钥。
- 新 IM、COS 和私有回调 Token 已写入 `/etc/campus-circle/campus-api.env`。
- 临时凭据 CSV 已删除，系统剪贴板已清空，敏感页面已关闭。

## IM 私信回调

- 回调地址：`https://zhanduiwang.top/api/v1/notification/tencent-im/callback`。
- 回调鉴权已开启。
- `C2C.CallbackAfterSendMsg` 已启用。
- 配置过程中暴露过一次临时回调 Token，随后已轮换；腾讯云与生产环境新 Token 已核对一致。
- 腾讯真实消息已到达公网回调地址，Nginx 记录为 `POST /api/v1/notification/tencent-im/callback`，服务端返回 `HTTP 200`。
- 新建应用的真实账号导入测试通过。

## 通知发送

- `ENABLE_USER_NOTIFICATIONS=true`。
- `ENABLE_WECHAT_SUBSCRIBE_NOTIFICATIONS=true`。
- `ENABLE_NOTIFICATION_SENDING=true`。
- 订单与私信模板 ID、字段映射、测试用户白名单和两项订阅授权额度均已存在。
- 当前仅向 `NOTIFICATION_TEST_USER_IDS` 中的测试用户发送。
- 尚未产生“用户对用户”的真实私信事件。商品会话回调会被安全拒绝，不能冒充普通私信通知。

## COS 验收

- 真实 STS 签发、上传、读取、签名 GET 和删除生命周期测试通过。
- 运行时子用户只能执行 COS 数据操作和 STS，不具备修改桶配置的权限；版本控制通过控制台开启。
- Gate-0“同一签名 PUT 重复覆盖必须被拒绝”测试在已开启版本控制的桶上未通过；同一 PUT 会生成新版本。
- 该覆盖保护项不能标记为完整 PASS，需另做应用层限制或专用验收入口。

## 当前仍需真实样本

- 真实用户对用户私信：验证回调入库、站内通知、手机微信订阅收件与点击跳转。
- 真实支付订单：验证付款、取消、退款与回调去重。
- 真实骑手配送：验证新单广播、预计收入、北京时间次日 00:00 单次结算和金额守恒。
- 真机：白屏、底栏、钱包、身份切换、支付与通知点击必须继续做设备验收。
