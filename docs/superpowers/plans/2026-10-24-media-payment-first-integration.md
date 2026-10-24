# 动态媒体与支付首批接入 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不启用生产支付、不恢复旧商城的前提下，把已批准的媒体预览规则补强，并完成配送支付适配器的首批可信通信校验。

**Architecture:** 复用主线已有 PostMotionMedia 与预览协调器，不新增媒体类型。支付遵循 `2026-10-24-delivery-payment-integrity-design.md`：本批先封堵 Provider 的未验签应答和非法通知，不修改资金账本或数据库。学生商品收款、骑手出款、查单补偿与资金来源追踪分别验收，本批不能标记“全部支付已打通”。

**Tech Stack:** Taro / React / TypeScript、Node 行为测试、Go 标准库 RSA / AES-GCM / httptest。

用户 2026-10-24 “去装入吧”授权接入此前确认的媒体行为与四类支付方向。沿用已批准设计；实施子技能在当前可用技能目录中缺失，改用现有子代理、TDD 和主代理复审。保留三个历史未跟踪计划文件；不 push、不上传、不部署、不访问真实资金接口。

## Task 1: 媒体行为回归

**Files:** `apps/miniapp/src/utils/motion-preview.ts`、`apps/miniapp/src/components/post-motion-media.tsx`、`apps/miniapp/src/hooks/use-post-motion-preview.ts`、`scripts/test-post-motion.cjs`。

- [ ] 添加真实组件回归，覆盖开始播放后滑出又回来、旧媒体事件晚到、新会话与离线通知晚于网络查询、卡住的观察器取消。只有证明现有实现不满足旧设计才修改相应文件。
- [ ] 针对“每条每会话只预览一次”执行失败用例：

```js
await visible('one', .8)
await React.act(async () => feed.container.querySelector('[data-kind="Video"]').taroProps.onPlay())
await visible('one', 0)
await visible('one', .8)
assert.equal(feed.container.querySelector('[data-kind="Video"]'), null)
```

- [ ] 仅将已真正开始预览的媒体在离开当前播放槽后计为已播，不让 onPlay 当场挤掉当前播放器；未播放的候选仍可播放；完成/失败/超时继续回封面。
- [ ] 旧 URL / 会话 / 已卸载播放器的事件不得结束或污染新播放器；拒绝非法时长启动预览；网络变更事件优先于早先未完成的网络查询。
- [ ] 执行 `npm run test:post-motion`、`npm run check:content-motion`、`npm run check:media-review-policy`、`npm --workspace apps/miniapp run typecheck`，均须 exit 0。

## Task 2: 微信支付 Provider 信任链

**Files:** `backend/internal/module/admin/deliverypayment/provider.go`、该目录独立验签实现和测试、`backend/internal/config/config.go`、配置测试、`backend/.env.example`（仅公开配置名）。

- [ ] 先加失败测试：同一平台密钥签名但错误 `Wechatpay-Serial` 必须拒绝；缺少签名的 200 不能返回预支付参数；无效 GCM nonce 只能报错不能 panic；错误 appid/mchid/CNY、事件类型、算法或成功时间必须拒绝。
- [ ] 增加 `WechatPayPlatformKeyID` / `WECHAT_PAY_PLATFORM_KEY_ID`，对应配置可信的支付公钥 ID 或平台证书序列号。不能把商户证书序列号当平台序列号，不能相信通知传入的任意 ID。关闭状态不强制新增凭据；实际启用的配送支付要求该值，防止静默不验签。
- [ ] 抽出共用应答/通知验签：验证唯一且非空的时间戳、nonce、签名、serial；以原始字节构造签名串，验证时限与 RSA；响应体上限 1 MiB，超限显式拒绝。未知公钥 ID 拒绝，不从任意远端下载 key。

```go
message := timestamp + "\n" + nonce + "\n" + string(body) + "\n"
digest := sha256.Sum256([]byte(message))
err := rsa.VerifyPKCS1v15(platformKey, crypto.SHA256, digest[:], signature)
```

- [ ] HTTP 成功响应通过验签才解析；缺签名/验签失败为未知而非确定业务拒绝。合法错误应答不透传原始 body；重定向不能向其他目标泄露 Authorization。
- [ ] 通知先验签再按受支持事件解密。确认 `AEAD_AES_256_GCM`、resource 类型、appid/mchid、CNY、JSAPI、交易标识与有效时间。退款校验官方存在的主体字段，不臆造必须有 appid。
- [ ] `decryptResource` 在 `gcm.Open` 前验证 nonce 长度与密文长度，所有畸形输入可控返回错误。
- [ ] `CreateRefund` 成功应答要核对原交易号、退款号、金额/币种，不能凭一个 `SUCCESS` 字符串相信退款成功；对缺字段严格拒绝。
- [ ] 本批不假装完成付款人快照、本地退款归属/金额核对、查单 worker 或出款：它们需要领域/迁移/并发的独立实现。支付开关保持关闭，验签通过也不是生产启用证明。
- [ ] 用 `httptest.Server` 和测试生成 RSA 密钥验证，禁止生产 HTTP、真实证书或读取 `.env`。运行 `go test ./internal/module/admin/deliverypayment ./internal/config`；全后端使用 `npm run test:backend` 的安全环境。

## Task 3: 集成审查与交付

**Files:** 本计划和 `docs/reviews/2026-10-24-media-payment-first-integration.md`。

- [ ] 主代理复查各修改，不把验证 stub 改宽来通过测试；保持依赖、路由、账本、业务开关不变。
- [ ] 运行 `npm run verify`、`npm run test:post-motion`、`npm run build`、`git diff --check`。MySQL/微信真机/真实支付均另记未验证，禁止冒充通过。
- [ ] 记录媒体已经接入与支付只完成首批的具体边界；四类支付余项包括多商户商品收款模型、资金来源校验、骑手提现与自动结款、查单补偿与对账。
- [ ] 精确暂存本次文件，中文提交；不提交用户已有文件，不 push。

## 官方依据（2026-10-24 核对）

- https://pay.wechatpay.cn/doc/v3/merchant/4013053420 — 平台证书应答验签。
- https://pay.wechatpay.cn/doc/v3/merchant/4013053249 — 支付公钥与公钥 ID。
- https://pay.wechatpay.cn/doc/v3/merchant/4012791902 — 小程序支付通知、验签及解密。
