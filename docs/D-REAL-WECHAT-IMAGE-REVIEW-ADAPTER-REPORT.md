# 真实微信图片审核适配器报告

## Development Report

本次补丁清理新 `internal/module/media` 链路中最突出的上线技术债：图片上传完成后不再只能依赖本地 mock 自动审核，而是新增真实微信 `media_check_async` 提交适配器与回调落库链路。

改动范围严格限定在新 media / miniapp flow 及组合根：

- 新增 `miniappflow.WechatAsyncReviewSubmitter`，使用新 media 自有 COS `SignedGetURL` 对 `final_key/final_version_id` 签 10 分钟短期 GET URL，然后提交微信 `/wxa/media_check_async`。
- 新增 `POST/GET /api/v1/miniapp/media/wechat/callback`，用既有 `WECHAT_CALLBACK_TOKEN` 做微信 SHA-1 签名校验，回调后按 `trace_id` 定位审核投影并写入 decision outbox。
- `CompleteUpload` 保持原签名兼容；新增 `CompleteUploadWithOpenID`，HTTP handler 传入当前用户 `OpenID`，真实微信审核路径必需 `openid`。
- `ReviewProjectionRepository/Service` 新增按 `review_task_id` 查询能力，用于回调定位投影。
- `main.go` 按现有 mock 登录规则装配：
  - 本地 mock 模式：`APP_ENV != prod && ALLOW_MOCK_WECHAT_LOGIN=true && WECHAT_APP_ID/SECRET 为空`，继续走本地 mock auto approval。
  - 非 mock/生产：装配真实微信图片审核 submitter；缺微信凭证时 fail-closed。

未做事项：

- 未修改 media 状态机、Consumer 决策语义、COS 存储逻辑、旧 media 包、配送域/外卖域业务逻辑。
- 未新增迁移、表结构或 DB 约束。
- 未暴露 COS 原始 key/version/永久 URL；微信审核只拿短期签名 URL。

## Root Cause Analysis

D-4/D-5 暴露的上线阻断点是：新 P0-4 media 链路已有上传、COS final promotion、projection/outbox/consumer 与私有/公开 resolve，但真实图片内容安全审核没有接入微信异步审核。此前本地验收依赖 `WithLocalMockAutoApproval`，可以证明业务流程，但生产环境若没有真实审核适配器，图片会长期停留在待审核/或无法满足上线合规要求。

旧 `backend/internal/pkg/media` 曾有一套微信审核实现，但属于旧 media 包。直接复用会造成新旧 media 跨接线继续扩大。因此本次在新 `internal/module/media/miniappflow` 内按同层能力补齐：

- 微信 access token 与 `media_check_async` submitter；
- 微信回调签名校验；
- 回调 suggestion 到新 media projection decision 的映射；
- 回调定位使用已有 `review_task_id` 字段，不新增表。

## Acceptance Evidence

已完成的本地验证：

- `go test ./cmd/server ./internal/module/media/... -count=1`：PASS。
- `go test ./... -count=1`（backend）：PASS。
- `npm run check:boundaries`：PASS。
- `npm run verify`：PASS。
- `npm run build`：PASS（仅保留既有 `vendors.js` 体积 warning）。
- 单元验证覆盖：
  - 本地 mock 模式下不装配真实微信 submitter；
  - 非 mock 且有微信凭证时装配 `WechatAsyncReviewSubmitter`；
  - 非 mock/生产缺凭证时 fail-closed；
  - submitter 使用短期 COS GET URL 调 `/wxa/media_check_async`；
  - `CompleteUploadWithOpenID` 会提交真实微信审核并推进 projection 到 `reviewing`；
  - 微信回调 `suggest=pass` 会写入 approved decision outbox；
  - callback token 签名校验 fail-closed。

待最终上线前真实验收：

- 在公网 HTTPS 域名下配置微信小程序图片审核回调到：
  `/api/v1/miniapp/media/wechat/callback`
- 使用正式 `WECHAT_APP_ID` / `WECHAT_APP_SECRET` / `WECHAT_CALLBACK_TOKEN` 与真实 COS 凭证上传一张图片；
- 验证微信回调到达后 projection 从 `reviewing` 变为 `approved` 或 `rejected`；
- 验证审核失败/未知 suggestion 时不会错误放行。

## Residue Report

- 本次无迁移、无测试数据写入、无数据库残留。
- 本次未启动长期后台进程。
- 本次未提交 `.env`、凭证、token、COS key、微信 secret。
- 旧 media、旧外卖、配送资金/通讯/照片链路未改动。
