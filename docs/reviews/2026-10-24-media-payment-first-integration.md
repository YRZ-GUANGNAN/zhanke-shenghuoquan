# 动态媒体与配送支付首批接入审查

日期：2026-10-24

## 本次完成

### 帖子动态媒体

- 保留既有“封面照片 + 短视频片段”的实况表现，不宣称原生 Live Photo。
- 预览真正开始后才记录本会话已播放；离开视口后不自动重播，未开始播放的卡片仍可再次进入候选。
- 播放事件增加会话、媒体与播放代次隔离，旧 `onTimeUpdate`、`onEnded`、`onError` 不再结束新播放器。
- 切换账号时清空旧活动项和失败状态，避免同一帖子 ID 跨账号串状态。
- 网络状态事件会淘汰此前未完成的 `getNetworkType()` 查询，迟到的 Wi-Fi 结果不能覆盖较新的断网事件。
- 非法时长不创建自动预览播放器；离线、减少动态效果、页面隐藏与卸载继续回退静态封面。

### 配送微信支付 Provider

- 增加 `WECHAT_PAY_PLATFORM_KEY_ID`，平台公钥/证书文件必须与可信公钥 ID/证书序列号成对配置。
- 微信通知与 API 应答统一校验唯一的时间戳、nonce、签名和 `Wechatpay-Serial`，并限制五分钟时间窗。
- 所有微信 2xx 应答必须先通过 RSA-SHA256 验签，再解析 `prepay_id` 或退款结果；缺签、错签、未知 Key ID 和篡改正文均按“不可信应答”处理。
- 支付通知严格核对事件、加密算法、资源类型、`appid`、`mchid`、`JSAPI`、`CNY` 与有效 `success_time`。
- 退款创建应答核对原交易号、退款号、金额、币种和受支持状态；退款通知核对商户、原交易号、金额与币种。
- 请求与通知正文上限为 1 MiB；畸形 AES-GCM nonce/密文可控返回错误，不再可能触发 panic。
- 默认 HTTP 客户端禁止跟随重定向，避免携带支付 `Authorization` 到其他目标。
- `paid`、`closed`、`failed` 等终态支付不会再次请求微信预支付。

## 验证结果

- `npm run test:post-motion`：PASS。
- `npm run check:content-motion`：PASS。
- `npm run check:media-review-policy`：PASS。
- `npm --workspace apps/miniapp run typecheck`：PASS。
- `go test ./internal/module/admin/deliverypayment ./internal/config`：PASS。
- `npm run test:backend`：PASS。
- `npm run verify`：PASS。
- `npm run build`：PASS；仅出现仓库既有的 CSS 顺序和包体积警告。
- `git diff --check`：PASS。

## 明确未完成与未启用

- 未启用 `ENABLE_DELIVERY_PAYMENT`，未读取真实支付密钥，未访问微信真实资金接口。
- 未部署、未上传微信、未迁移数据库、未执行真实扣款或退款。
- 本批不是“四类支付全部打通”：学生多商户商品收款模型、资金来源追踪、骑手提现/自动结款、主动查单/关单/退款查询、补偿 worker 和对账仍需后续独立实施。
- 退款通知已经携带并校验微信侧原交易号和金额，但尚未新增本地退款记录的原交易号/金额并发比对；这属于后续领域完整性改造。
- 自动预览通过组件行为测试与小程序构建；原生 iOS/Android 视频层、真实网络切换和 IntersectionObserver 首次回调仍需开发者工具及真机验收。
