# Tencent IM 回调协议核验记录

日期：2026-11-01

## 官方证据

- [即时通信 IM 回调配置](https://cloud.tencent.com/document/product/269/32431)：开启回调鉴权后，回调 URL 带 `Sign` 与 `RequestTime`；签名为 `sha256(Token + RequestTime)`。`RequestTime` 与当前时间相差超过 1 分钟必须拒绝，以防重放。
- [发单聊消息之后回调](https://cloud.tencent.com/document/product/269/2716)：命令为 `C2C.CallbackAfterSendMsg`；消息发送者和接收者分别在 `From_Account`、`To_Account`，消息标识可使用 `MsgKey`/`MsgId`，并提供 `MsgSeq`、`MsgTime`、`SendMsgResult`、`UnreadMsgNum`。回调也包含 `MsgBody`，本项目不读取其内容。
- [群内发言之后回调](https://intl.cloud.tencent.com/zh/document/product/1047/34375)：命令为 `Group.CallbackAfterSendMsg`；群组、发送者和消息序列号分别在 `GroupId`、`From_Account`、`MsgSeq`，并提供 `MsgTime`、`MsgBody`。本项目不读取 `MsgBody`。
- 腾讯的回调示例和多条回调说明要求服务端校验请求 URL 中的 `SDKAppID` 是否属于本应用。本项目只接受 URL 查询参数中的 `SdkAppid`/`SDKAppID`，并与服务端配置精确匹配；不会从客户端正文推断应用身份。

## 安全边界

腾讯签名只覆盖配置的 Token 与时间戳，不覆盖 HTTP body。因此解析器还会限制 body 大小、严格校验命令和元数据字段，并丢弃正文消息元素；签名通过不代表正文可信。`SendMsgResult != 0`、缺少消息标识、账号或群组标识时不会产生通知元数据。

`UnreadMsgNum` 是供应商回调快照，不能证明用户离线或当前会话不可见。缺少本地会话可见性、active、参与者和双向拉黑核验时，只允许记录可信回执元数据；不创建微信订阅任务。

## 实现状态

本轮仅实现 fail-closed 的鉴权/解析器和单元测试，不注册公网路由、不发送真实消息、不保存正文。持久化前必须由组合根提供账号关系、会话状态、feature gate 和去重 repository ports。
