# 微信订阅消息跳转版本兼容修复

## 已确认范围

这是既有真实订阅消息验收链路的兼容修复，不新增通知场景，不改 UI、模板字段、授权额度、发送白名单、生产配置或真实发送行为。

当前 sender 未发送 `miniprogram_state` 和 `lang`。微信默认跳转正式版，可能使体验版验收打开旧页面。采用最小服务端配置方案：`WECHAT_SUBSCRIBE_MINIPROGRAM_STATE` 默认 `formal`，只接受 `developer`、`trial`、`formal`；发送时显式附加该值及固定 `lang=zh_CN`。直接构造的零值配置仍按 `formal` 兼容；非空非法值启动失败。版本参数归 sender 所有，不接受客户端或单条通知覆盖，不改数据库快照。

不采用硬编码 `trial`（会影响上线后的用户），也不新增客户端版本参数（会扩大信任边界）。既有通知开关全部保持不变。

## 验收

- 配置加载默认值、三个允许值及非法值都有单测。
- 使用内存 HTTP transport 验证公开构造器到真实 JSON 字段的传递，且 `lang` 固定为 `zh_CN`；无真实微信请求。
- `go test ./internal/config ./cmd/server ./internal/module/notification -count=1`、`go vet ./internal/config ./internal/module/notification` 和 `git diff --check` 通过。
- 代码通过不代表手机送达或正确跳转已验收；后续仍需实际模板、用户授权、受控事件与真机证据。

依据：[微信官方发送订阅消息文档](https://developers.weixin.qq.com/miniprogram/dev/server/API/mp-message-management/subscribe-message/api_sendmessage.html)，2026-11-03 读取官方 Markdown 确认。
