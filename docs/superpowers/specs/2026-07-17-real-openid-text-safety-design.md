# 真实 OpenID 文本安全接入设计

日期：2026-07-17
状态：方案已批准

## 背景

社区发帖、社区评论、二手商品创建和二手商品更新已经调用微信 `msg_sec_check`，但四处 Handler 均向 `SafetyClient.CheckText` 传入空 OpenID。认证中间件其实已经把数据库中的真实 `user.User` 放入 Gin 上下文；继续传空值会让微信请求缺少必要身份，并使审核风险记录退化为平台错误。与此同时，Handler 直接依赖 `*review.SafetyClient`，难以在单元测试中证明请求使用了当前登录人的 OpenID。

现有审核闭环保持有效：写入对象先处于 `submitted`，文本检查结果写入 `review_tasks.risk`，只有人工批准后才可公开。此次只修复文本安全输入和可测试性，不改变该闭环。

## 目标

1. 四个文本写入入口都从认证上下文读取服务端可信的 `user.User.OpenID`，不接受客户端提供 OpenID。
2. Handler 依赖可注入的 `review.TextSafetyChecker` 接口，生产仍使用 `*review.SafetyClient`，测试可使用记录参数的假实现。
3. 微信超时、令牌失败、非成功响应或响应解析失败时，业务写入仍进入人工审核，不因临时平台故障丢失用户提交，也不绕过审核。
4. 空 OpenID 不向微信发请求，并在审核风险中明确标记为人工处理。
5. 保持数据库结构、前端 API、HTTP 状态码、请求响应 JSON 和幂等语义不变。

## 非目标

- 不新增数据库迁移、审核状态或自动审批规则。
- 不允许安全检查结果直接公开内容；`wechat pass` 仍只是人工审核的风险参考。
- 不增加微信失败重试队列、令牌缓存重构或新的环境变量。
- 不修改图片/媒体审核、二手私信、收藏、搜索或管理端审核界面。
- 不把 OpenID、正文、微信 access token 或微信原始错误正文写入日志、风险字段或 API 响应。

## 架构

在 `backend/internal/module/admin/review/safety.go` 定义最小接口：

```go
type TextSafetyChecker interface {
	CheckText(context.Context, string, string) string
}
```

参数依次为请求上下文、可信 OpenID 和待检查文本，返回值仍是现有审核任务使用的风险摘要。`*SafetyClient` 继续实现该方法，因此 `main.go` 中创建一次 `review.NewSafetyClient(cfg)` 并注入社区与二手 Handler 的生产装配方式不变。

`community.Handler` 与 `trade.Handler` 的 `safety` 字段及 `NewHandler` 参数从具体指针改为 `review.TextSafetyChecker`。接口只位于审核包，避免两个业务包重复定义同名契约，也不把微信 HTTP 细节泄漏到业务层。

每个受保护入口在绑定并校验请求后只读取一次：

```go
currentUser := user.CurrentUser(c)
risk := h.safety.CheckText(c.Request.Context(), currentUser.OpenID, content)
```

同一个 `currentUser.ID` 用于业务写入，避免用户 ID 与 OpenID 来自不同上下文读取。具体组合文本保持现状：社区帖子为 `title + "\n" + content`，评论为 `content`，商品创建和更新为 `title + "\n" + description`。

## 四个入口

| 入口 | HTTP 路由 | 审核对象 | OpenID 来源 |
| --- | --- | --- | --- |
| 社区发帖 | `POST /community/posts` | `post` | `user.CurrentUser(c).OpenID` |
| 社区评论 | `POST /community/posts/:id/comments` | `comment` | `user.CurrentUser(c).OpenID` |
| 商品创建 | `POST /trade/products` | `product` | `user.CurrentUser(c).OpenID` |
| 商品更新 | `PUT /trade/products/:id` | 既有 `product` 重新提交 | `user.CurrentUser(c).OpenID` |

带幂等键与不带幂等键的创建路径使用同一套风险摘要规则。`TextSafetyChecker` 仍在每次 HTTP 请求入口执行，但幂等重放继续返回第一次保存的对象和审核任务，不重复媒体预留、对象创建或审核任务写入，也不改写既有终态审核任务。

## 风险摘要与错误处理

`CheckText` 继续以字符串返回审核风险，但所有平台故障改为固定、短小、无敏感信息的人工处理标记：

- OpenID 为空或仅含空白：`manual:wechat_openid_missing`，不发起微信请求；
- AppID/AppSecret 未配置：`manual:wechat_safety_unconfigured`；
- access token 获取失败：`manual:wechat_token_unavailable`；
- 请求创建、网络或超时失败：`manual:wechat_request_failed`；
- 微信返回非 2xx：`manual:wechat_http_error`；
- 响应无法解析：`manual:wechat_response_invalid`；
- 微信业务 `errcode != 0`：`manual:wechat_api_error:<errcode>`；
- 正常结果保持 `wechat pass` 或 `wechat <suggest> label=<label>`。

空正文返回 `manual:wechat_content_empty`；正常业务校验仍会按原契约拒绝缺少必填文本的请求。风险摘要长度始终远小于现有 `review_tasks.risk` 的 240 字符限制。

微信临时失败不转换为 Handler 错误：对象仍以 `submitted` 保存，风险摘要随同步提交或 outbox 进入同一人工审核队列，在管理员明确批准前对公共读接口保持不可见。数据库、媒体预留、审核任务或 outbox 自身失败时，仍沿用现有错误映射；本设计不吞掉内部持久化错误。

## 数据流

```text
JWT 认证 -> AuthRequired 写入当前 user.User
  -> Handler 读取同一 User.ID + User.OpenID
  -> TextSafetyChecker.CheckText(ctx, OpenID, 组合文本)
  -> 生成安全、定长 risk
  -> Service 创建或更新 submitted 对象
  -> 同步 Submit 或 write outbox 创建/重置人工审核任务
  -> 管理员批准后才进入公共列表、详情与搜索
```

OpenID 只在进程内从认证上下文传到微信 HTTPS 请求体，不进入业务请求模型、数据库新字段、前端响应或结构化日志。

## 测试策略

1. `review` 包测试：使用可控 HTTP Client 验证正常请求体含预期 OpenID 与组合文本；空 OpenID 不触发 HTTP；配置缺失、令牌失败、网络失败、非 2xx、无效 JSON 和非零 errcode 分别返回固定人工标记；风险字符串不得包含 OpenID、正文、token 或原始响应。
2. `community` Handler 测试：注入记录型 checker，分别验证发帖和评论传入认证用户的真实 OpenID、正确文本及请求 context；验证返回 risk 继续进入同步和幂等审核路径。
3. `trade` Handler 测试：分别验证商品创建与更新传入卖家的真实 OpenID和正确组合文本；更新后仍重置为 `submitted`，平台失败标记仍产生人工审核任务。
4. 回归测试：覆盖创建幂等重放不重复副作用、未批准内容不可公开、人工批准后原有展示和搜索行为不变。
5. 实施门禁执行 `go test ./internal/module/admin/review ./internal/module/community ./internal/module/trade ./cmd/server -count=1`、`go test ./...`、`go vet ./...` 与 `git diff --check`。

## 代码边界

计划只修改以下后端文件及对应测试：

- `backend/internal/module/admin/review/safety.go`
- `backend/internal/module/admin/review/safety_test.go`
- `backend/internal/module/community/handler.go`
- `backend/internal/module/community/handler_test.go` 或现有等价 Handler 测试
- `backend/internal/module/trade/handler.go`
- `backend/internal/module/trade/handler_test.go` 或现有等价 Handler 测试
- `backend/cmd/server/main.go` 仅在接口类型装配需要显式编译断言时调整；默认无需改变运行配置

不得修改 `backend/migrations/**`、`apps/miniapp/src/**` 或任何对外路由与 JSON 契约。

## 完成标准

- 四个入口均以服务端认证用户的真实 OpenID 调用 checker，且测试逐一证明。
- 空 OpenID和所有微信临时故障都产生明确 `manual:*` 风险并进入人工审核，内容不会被自动公开。
- 正常微信结果、创建/更新返回、幂等重放、审核批准和公共可见性保持原契约。
- 无数据库迁移、无前端 API 变化、无敏感信息落盘或出现在日志与响应中。
- 所列测试与静态检查全部通过。
