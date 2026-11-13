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
- 使用测试用户 1 → 15 的真实用户会话再次发送消息；回调、去重回执、站内通知和微信订阅发送链路均成功。
- 本次站内通知状态为 `delivered`，微信订阅发送状态为 `accepted`。
- 站内通知跳转为 `/pages/secondhand-chat/index?relationId=9`。
- 测试用户 15 的 `private_message` 单次订阅额度已正常消耗，状态变为 `exhausted`。

## 通知发送

- `ENABLE_USER_NOTIFICATIONS=true`。
- `ENABLE_WECHAT_SUBSCRIBE_NOTIFICATIONS=true`。
- `ENABLE_NOTIFICATION_SENDING=true`。
- 订单与私信模板 ID、字段映射、测试用户白名单和两项订阅授权额度均已存在。
- 当前仅向 `NOTIFICATION_TEST_USER_IDS` 中的测试用户发送。
- 私信通知已完成真实站内送达和微信接口接受；手机通知栏实际显示仍需以最终设备证据确认。
- 商品会话回调会被安全拒绝，不能冒充普通私信通知。

## COS 验收

- 真实 STS 签发、上传、读取、签名 GET 和删除生命周期测试通过。
- 运行时子用户只能执行 COS 数据操作和 STS，不具备修改桶配置的权限；版本控制通过控制台开启。
- Gate-0“同一签名 PUT 重复覆盖必须被拒绝”测试在已开启版本控制的桶上未通过；同一 PUT 会生成新版本。
- 该覆盖保护项不能标记为完整 PASS，需另做应用层限制或专用验收入口。
- 切换期间旧桶与新桶的对象清单均为 0，没有可迁移的历史媒体对象。

## 微信服务器域名补齐

- 已将旧的 `zkshq-1436381937-1436381937.cos.ap-guangzhou.myqcloud.com` 从小程序 `request合法域名` 移除。
- 已加入新桶 `https://zkshq-1507095774.cos.ap-guangzhou.myqcloud.com`。
- 已加入腾讯云 IM 统计域名 `https://events.im.qcloud.com`。
- 已加入 `wss://wss.im.qcloud.com`、`wss://wssv6.im.qcloud.com`、`wss://wss.tim.qq.com` 和 `wss://wss.im.tencent.cn` 到 `socket合法域名`。
- 微信公众平台保存成功，域名修改次数由 50 变为 49。
- 新桶真实图片写入返回 200，私有读取返回 200，匿名读取返回 403，删除返回 204，删除后读取返回 404；测试对象已清理。

## 当前仍需真实样本

- 手机确认真实用户私信通知的微信收件展示与点击打开。
- 真实支付订单：验证付款、取消、退款与回调去重。
- 真实骑手配送：验证新单广播、预计收入、北京时间次日 00:00 单次结算和金额守恒。
- 真机：白屏、底栏、钱包、身份切换、支付与通知点击必须继续做设备验收。
