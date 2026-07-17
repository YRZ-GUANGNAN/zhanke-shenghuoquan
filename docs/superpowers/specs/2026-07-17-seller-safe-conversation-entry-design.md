# 二手卖家安全私信入口设计

> 状态：方案已确认，待实施计划
> 日期：2026-07-17

## 1. 背景与目标

现有 `PUT /trade/products/:id/contact` 与 `productId` 聊天入口只适合买家；首页“私信”仍是静态样例，卖家无法进入真实关系。旧 `pages/chat`、`pages/conversations` 会信任 URL 会话坐标，不能恢复为正式入口。本阶段完成：

1. 新增参与者受限的 `GET /api/v1/im/conversations/:relationId`，让关系中的买家或卖家安全恢复会话上下文。
2. 首页“私信”改为真实二手会话列表，点击后只携带 `relationId`。
3. `pages/secondhand-chat` 同时支持买家 `productId` 入口和关系参与者 `relationId` 入口；腾讯 IM 使用的全部坐标都来自本次服务端响应。

成功标准：买家发信后，卖家可从首页进入同一 C2C 会话并回复；第三方枚举 `relationId` 只能得到 `404`。

## 2. 设计原则

- URL 参数只是资源定位符，不是授权依据。前端不得从 URL 读取或拼装 `conversationId`、`peerUserId`、`buyerId`、`sellerId`。
- 权限过滤下沉到数据库查询：按 `relation_id + 当前用户是 buyer/seller + reference_type=product` 一次定位，避免先查记录再在应用层暴露存在性。
- 不存在与无权访问对外统一为 `404`，防止关系枚举。
- 单关系读取是纯读操作，不导入腾讯账号、不创建关系、不刷新关系时间。
- 复用现有 `im_conversations`、`im_user_mappings` 和 `GET /im/conversations`，不新增迁移。

## 3. 后端架构与契约

### 3.1 HTTP 接口

在现有鉴权路由组注册：

```text
GET /api/v1/im/conversations/:relationId
Authorization: Bearer <token>
```

成功返回现有 `IMConversationContext`：`relationId`、`conversationId`、`peerUserId`、`productId`、`buyerId`、`sellerId`。`conversationId` 与 `peerUserId` 必须按当前调用者重新计算：买家对端是卖家，卖家对端是买家；不得直接使用表中历史 `conversation_id`。

状态码固定如下：

- 未登录：`401`。
- `relationId` 不是正整数：`400`。
- 关系不存在、不是 `product` 关系或当前用户不是参与者：统一 `404`，使用相同对外错误文案。
- 映射缺失或数据库异常：`500`，返回通用文案，不透出 SQL 或关系详情。

### 3.2 Repository 与 Service 边界

`Repository` 增加按 `id + reference_type=product + 当前用户为 buyer/seller` 单次过滤的方法，以 `(Conversation, found, error)` 表达未命中。`Service` 据关系确定对端，再用现有 `FindMapping` 取得 IM 标识。列表与单项读取共用“按调用者计算对端”的转换函数。

Handler 仅做参数解析、读取 `CurrentUserID` 和错误映射。`404` 使用 IM 域哨兵错误，不复用 trade 错误或比较错误字符串。

## 4. 前端数据流

### 4.1 首页真实会话列表

`services/e-im.ts` 保留 `listIMConversations()`，新增 `getIMConversation(relationId)`。首页删除静态 `conversations` 数组，在“私信”页签首次展示以及页面重新可见时读取第一页真实关系，首期固定 `page=1&pageSize=20`。

首页不伪造消息摘要、未读红点或消息时间。每项以 `relationId` 为 key，默认展示“二手商品 #<productId> / 二手交易私信”；商品补充请求成功时替换为真实标题与首图，失败时保留通用文案，不隐藏已授权关系。

列表具备 `loading / empty / ready / error` 四态。普通业务请求的 `401` 不会被现有请求层自动刷新，因此首页捕获 `ApiError(401)` 后必须调用 `useSessionStore.getState().retry()`，等待成功后只重放一次会话列表请求；同一次加载不得循环刷新。恢复失败进入错误态并保留手动重试入口。其他失败可重试且不回退静态样例。点击只能导航到：

```text
/pages/secondhand-chat/index?relationId=<服务端返回的 relationId>
```

### 4.2 聊天页双安全入口

页面只接受两种互斥入口；双参数、缺参数或格式非法时进入错误态，不初始化腾讯 IM。

**买家入口：** 详情页传 `productId`。聊天页调用现有 contact 接口，再按响应中的 `productId` 取商品，并用响应中的 IM 坐标初始化；忽略 URL 中其他会话字段。

**关系入口：** 首页传 `relationId`。聊天页调用新增 GET，再按响应中的 `productId` 取商品并使用同一响应的 IM 坐标。该入口对买卖双方有效且不写关系。

取得服务端上下文后统一订阅会话、加载历史文字、标记已读并启用发送。商品读取失败时显示通用关联信息但可继续会话；授权上下文失败必须 fail-closed，不能调用腾讯 SDK。

聊天页以当前 session 用户 ID 与服务端上下文中的 `buyerId/sellerId` 判断视角。买家视角继续显示商品中的卖家名称、头像和卖家主页入口；卖家视角不得把自己显示成对端，统一显示中性“买家”或“交易对方”与占位头像，并禁用主页入口。当前用户既不是 buyer 也不是 seller 时，即使服务端异常返回了上下文也必须 fail-closed。本次不新增公开买家资料接口。

聊天页对授权上下文请求的首次 `401` 同样调用 session `retry()`，成功后只重放一次完整上下文加载；第二次 `401` 或登录恢复失败直接进入错误态并禁用发送。

详情页仍走 `productId`；首页不调用 `contactSeller`，卖家打开会话不产生写操作。

## 5. 错误处理与安全行为

- `404`：聊天页显示“会话不存在或无权访问”，提供返回首页，不区分资源不存在和越权。
- `401`：页面显式调用 session `retry()` 并至多重放一次；恢复前禁止发送，第二次失败进入错误态，避免旧 Token 无限重试。
- `400`：显示入口无效，要求从商品详情或首页重新进入。
- `500`、超时或断网：保留重试入口；每次重试重新向服务端取上下文，不复用失败前的局部坐标。
- 页面卸载或参数变化时停止状态写入并解除腾讯订阅。
- 发送仅在服务端上下文和腾讯会话 ready 后启用；失败保留草稿，不显示假成功。
- 腾讯凭证与正文不写入业务数据库、日志或错误提示。

## 6. 路由与兼容性

正式路由继续只注册 `pages/secondhand-chat/index`。现有 `pages/chat`、`pages/conversations` 源文件不删除，但不得加入 `app.config.ts`，也不得从任何正式页面导航进入。实施时增加静态路由边界检查，确保这两个旧页面保持未注册状态。

现有 contact、会话列表、API 基址和腾讯 IM 能力保持兼容；仅增加一个 GET 与安全入口。

## 7. 测试与验收

后端单元与 Handler 测试覆盖：

1. 买家和卖家读取同一 `relationId` 均成功，且各自得到正确的对端 `peerUserId` 与 `conversationId`。
2. 第三方读取和不存在关系返回完全一致的 `404`；非 product 关系同样返回 `404`。
3. 非法 ID 为 `400`，未认证为 `401`，存储故障为不泄露内部信息的 `500`。
4. 单项读取不调用账号导入、不新增关系、不修改 `created_at/updated_at`。
5. 现有分页列表继续只返回当前用户参与的关系。

当前 miniapp 没有 Jest/Vitest 等前端测试运行器，本阶段不为此单点修复引入整套测试基础设施。前端门禁分为：

1. 静态与构建：源码不再含硬编码会话数组；导航只拼接 `relationId`；`app.config.ts` 仍未注册 `pages/chat/index` 和 `pages/conversations/index`；miniapp typecheck、静态路由检查、weapp 构建、仓库 verify 与后端 `go test ./...` 全部通过。
2. 微信开发者工具自动化：验证首页 empty/error/retry/ready 状态；点击真实项只产生 `relationId` 路由；`productId` 与 `relationId` 两入口分别调用正确服务；双参数、缺参数、伪造 URL 坐标、`404` 和连续 `401` 均不会初始化或发送腾讯消息；首次 `401` 只触发一次 session 恢复和一次重放；页面卸载后订阅释放。
3. 真实双账号：验证买卖双方得到相反且正确的对端，卖家侧显示“买家/交易对方”且不能跳转到自己主页，双方能够读取历史、实时收取和发送唯一文字标记。

真实验收：买家从详情建立关系并发送唯一文字；卖家从首页经 `relationId` 进入、看到并回复；买家收到回复；第三个账号访问同一关系得到 `404`。证据不得输出 Token、UserSig 或完整凭证。

## 8. 非目标

- 不改迁移、表、索引，不存消息。
- 不做图片私信、群聊、推送、搜索、未读数、可靠摘要或会话删除。
- 不新增聚合接口，不重构请求层与身份体系。
- 不恢复、注册或改造旧 `pages/chat`、`pages/conversations`。
- 不改变商品审核、收藏、发布、媒体版本访问及 contact 写契约。
- 不处理腾讯 IM 控制台策略、内容审核或运维告警。
