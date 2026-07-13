# UI-21b-2b 社区与二手真实写交互设计（先社区、后二手）

> 状态：设计方向已确认，待 owner 审阅正式规格后进入实施计划  
> 基线：`master@83ca3962`  
> 前置标签：`ui21b2a-write-contract-baseline`  
> 日期：2026-07-13

## 1. 背景

UI-01 至 UI-20 已完成小程序静态壳与主要页面闭环，UI-21b-2a 已冻结社区和二手写接口的确定态、幂等与查询契约。当前页面仍以静态展示或旧式 toggle 调用为主，尚未把以下真实写链路安全接入新 UI：

- 社区：点赞、收藏、关注、评论、发帖和图片绑定；
- 二手：收藏、联系卖家、发布商品和图片绑定。

本阶段只负责把冻结契约接入现有新 UI，不修改后端，也不复活已废弃的旧前端结构。

## 2. 目标

1. 所有布尔写操作使用目标态 `PUT/DELETE`，不再使用 toggle。
2. 所有资源创建使用稳定的 `Idempotency-Key` 与同值 `clientRequestId`。
3. 发帖和发布二手商品时，将实际上传得到的 media asset public id 传给后端，不再提交空绑定。
4. 写操作以服务端确定结果为准，避免重复点击、网络重试和乱序响应造成状态翻转或重复资源。
5. 保持现有新 UI 的视觉和路由，不引入旧页面、旧导航或新架构。

## 3. 非目标

- 不修改 `backend/**`、迁移、状态机、Repository、HTTP 路由或 API 契约。
- 不实现新的内容审核、媒体状态机或 COS 能力。
- 不重构全局请求层、会话层、状态管理或导航体系。
- 不恢复 `pages/publish`、`pages/product-detail` 等旧 UI 作为正式入口。
- 不在本阶段处理搜索、推荐、私信 SDK 或其它与本次写交互无关的功能。

## 4. 总体决策：拆成两个可独立验收的增量

### 4.1 UI-21b-2b-1：社区真实写交互

建议分支：`feat/ui21b2b1-community-writes`  
建议冻结标签：`ui21b2b1-community-writes-baseline`

仅处理：

- 帖子点赞/取消点赞；
- 帖子收藏/取消收藏；
- 作者关注/取消关注；
- 发布评论；
- 发布帖子；
- 帖子图片上传与 `imageAssetIds` 真实绑定。

### 4.2 UI-21b-2b-2：二手真实写交互

前置：UI-21b-2b-1 已合并并冻结。  
建议分支：`feat/ui21b2b2-trade-writes`  
建议冻结标签：`ui21b2b2-trade-writes-baseline`

仅处理：

- 商品收藏/取消收藏；
- 联系卖家并进入稳定会话；
- 发布二手商品；
- 商品图片上传与 `imageAssetIds` 真实绑定。

按“先社区、后二手”拆分可以把每次真实环境验收控制在单域内，失败时也能独立回滚，不会把两个域的写风险混在一个提交中。

## 5. 冻结 HTTP 契约

### 5.1 社区

目标态写操作：

- `PUT /api/v1/community/posts/:id/like`
- `DELETE /api/v1/community/posts/:id/like`
- `PUT /api/v1/community/posts/:id/favorite`
- `DELETE /api/v1/community/posts/:id/favorite`
- `PUT /api/v1/community/users/:id/follow`
- `DELETE /api/v1/community/users/:id/follow`

资源创建：

- `POST /api/v1/community/posts`
- `POST /api/v1/community/posts/:id/comments`

创建请求必须同时携带：

- Header：`Idempotency-Key: <clientRequestId>`
- Body：`clientRequestId: <同一个值>`

发帖请求继续使用后端既有字段：`title`、`content`、`topic`、`imageAssetIds`、`clientRequestId`。

### 5.2 二手

目标态写操作：

- `PUT /api/v1/trade/products/:id/favorite`
- `DELETE /api/v1/trade/products/:id/favorite`
- `PUT /api/v1/trade/products/:id/contact`

资源创建：

- `POST /api/v1/trade/products`

发布请求必须同样携带 Header `Idempotency-Key` 与 Body `clientRequestId`，且值完全一致。请求字段沿用冻结契约：`title`、`description`、`category`、`price`、`originalPrice`、`condition`、`campus`、`imageAssetIds`、`clientRequestId`。

`price` 与 `originalPrice` 必须由前端将用户输入转换为整数分后发送，禁止发送浮点金额。

## 6. 目标态写入规则

### 6.1 禁止 toggle

前端不得再调用旧的：

- `POST /posts/:id/like`
- `POST /posts/:id/favorite`
- `POST /users/:id/follow`
- `POST /products/:id/favorite`
- `POST /products/:id/contact`

用户点击时，前端先确定“希望最终变成什么状态”，然后选择对应方法：

- 希望为 `true`：调用 `PUT`；
- 希望为 `false`：调用 `DELETE`。

连续发送相同目标态必须安全：重复 `PUT` 不会反向取消，重复 `DELETE` 不会重新启用。

### 6.2 状态来源

- 页面初始状态来自详情和 viewer-state 查询，不根据静态样例推断。
- 写成功后优先采用服务端响应中的确定状态。
- 若发生超时、断网等“结果未知”错误，不擅自回滚或翻转；重新拉取 viewer-state 后再显示。
- 不做未经服务端确认的永久乐观更新。若为交互反馈暂时变化，失败或结果未知时必须恢复到服务端状态。

### 6.3 并发与重复点击

- 同一资源、同一动作提交期间按钮进入 loading 并禁用。
- 乱序响应不得覆盖较新的用户意图；每个动作保留本地 intent 序号或 abort 标记。
- 页面卸载后不得继续写入已卸载组件状态。

## 7. 创建操作的幂等语义

### 7.1 key 生命周期

一次明确的用户提交意图只生成一个 `clientRequestId`：

1. 用户首次点击“发布”或“评论”时生成；
2. 自动重试、网络重试、用户点击“重试”时复用同一个 id 和同一份请求载荷；
3. 用户修改业务字段后再次发起，视为新意图，生成新 id；
4. 服务端明确成功后清除本次 id；
5. 不能在每次 HTTP 重试时重新生成 id。

### 7.2 Header 与 Body

`Idempotency-Key` 和 `clientRequestId` 必须来自同一变量。若二者不一致，后端会按冻结契约拒绝请求；前端不得维护两套来源。

### 7.3 草稿与重试

- 失败时保留用户文本、话题、分类、价格和已选图片。
- 对“请求可能已到达服务端”的失败，复用原 key 查询或重试。
- 对客户端校验失败，用户修改后生成新 key。
- 成功后只跳转一次，不能因重复响应重复导航。

## 8. 媒体上传与业务绑定

### 8.1 上传流程

复用现有 `chooseAndUploadImages`/media service：

1. 用户选图；
2. 按既有上传链路完成 COS 上传与 media complete；
3. 读取每张成功结果的 `UploadedMedia.assetId`；
4. 按用户选择顺序形成 `imageAssetIds`；
5. 全部必要上传完成后才允许提交业务资源。

### 8.2 purpose

- 社区帖子：`post_image`
- 二手商品：`product_image`

不得混用 takeout/delivery 等其它 purpose。

### 8.3 失败规则

- 选了图片但未拿到 asset id 时，禁止把空数组当作成功提交。
- 部分上传失败时保留已选列表并明确标记失败项，用户可重试或删除失败项。
- 发布请求只携带成功上传且仍被用户保留的 asset id。
- 最大图片数沿用现有 UI 的 9 张限制。

## 9. 页面与文件边界

### 9.1 社区阶段

主要落点：

- `apps/miniapp/src/pages/post-detail/**`
- `apps/miniapp/src/pages/publish-post/**`
- `apps/miniapp/src/services/b-community.ts`
- 必要的现有小程序共享类型或轻量工具

首页卡片若无法可靠取得 viewer-state，只负责进入详情，不在卡片上伪造可写状态。

### 9.2 二手阶段

主要落点：

- `apps/miniapp/src/pages/secondhand-detail/**`
- `apps/miniapp/src/pages/publish-secondhand/**`
- `apps/miniapp/src/services/c-trade.ts`
- 必要的现有小程序共享类型或轻量工具

联系卖家成功后使用后端返回的稳定会话坐标进入既有聊天页；重复点击必须落到同一会话，不创建重复关系。

### 9.3 绝对边界

- 只允许修改 `apps/miniapp/src/**`、对应前端测试和 `docs/**`。
- `backend/**`、迁移、配置、接口与其它应用保持零 diff。
- 不新增依赖，不引入路由库或新状态管理方案。
- 不删除历史页面源码；正式入口保持新 UI 路由，旧 UI 不再接回。

## 10. 交互状态模型

所有写操作使用统一的四态：

- `idle`：允许提交；
- `submitting`：按钮禁用并显示 loading；
- `succeeded`：使用服务端结果更新并执行一次后续导航；
- `failed`：保留输入，展示明确错误并允许安全重试。

页面同时存在多个独立动作时，各动作使用独立状态，例如点赞 loading 不应锁死评论输入，但同一点赞动作不得并发。

## 11. 错误处理

前端请求错误必须保留 HTTP status，不能全部扁平化为普通字符串：

- `400`：字段或 Header/Body 幂等键不一致，提示检查输入；
- `401`：登录失效，进入既有登录恢复流程；
- `404`：资源不存在或按权限隐藏，提示内容已不可用；
- `409`：冲突，刷新服务端状态后提示用户；
- `422`：业务校验失败，显示后端可读信息并保留草稿；
- `503/504` 或网络失败：结果可能未知，保留原 key，允许查询或幂等重试。

不得在失败后把 UI 误判为成功，也不得吞掉错误。

## 12. 验收设计

### 12.1 静态与构建门禁

每个子阶段都必须通过：

- `apps/miniapp` TypeScript typecheck；
- `npm run build:miniapp`；
- 根目录 boundary/verify；
- `package.json` 无依赖变化；
- 后端与迁移 diff 为零。

由于本机 C 盘曾长期空间紧张，创建 worktree 或执行构建前必须确认至少有 5 GB 可用空间；不足时停止并报告，不以删除项目或用户文件的方式强行腾挪。

### 12.2 社区真实环境验收

使用真实后端、真实 MySQL、真实鉴权和真实 media 上传，证明：

1. 点赞、收藏、关注只调用目标态 `PUT/DELETE`；
2. 重复发送相同目标态不会反向翻转；
3. 评论重复提交同一幂等 key 只生成一条评论；
4. 发帖重复提交同一幂等 key 只生成一个帖子；
5. 带图发帖请求的 `imageAssetIds` 非空，media reference 实际建立；
6. 401、冲突和网络重试不造成 UI 假成功；
7. 验收数据和进程清理完成。

### 12.3 二手真实环境验收

使用真实后端、真实 MySQL、真实鉴权和真实 media 上传，证明：

1. 收藏只调用目标态 `PUT/DELETE`；
2. 重复联系卖家得到同一稳定会话并正确进入聊天页；
3. 发布商品重复提交同一幂等 key 只生成一个商品；
4. 价格以整数分发送，服务端存储与页面显示一致；
5. 带图商品请求的 `imageAssetIds` 非空，media reference 实际建立；
6. 失败时保留表单，重试不重复创建；
7. 验收数据和进程清理完成。

## 13. 交付与收尾

每个子阶段分别交付：

- Development Report；
- Acceptance Report；
- Acceptance Evidence；
- Residue Report；
- 最终 commit hash；
- 干净 worktree 证明。

完成后提交到对应 feature 分支并停止，等待 owner 验收。合并只允许在主仓库串行 `--ff-only`，合并后再创建对应 baseline tag；不得自动进入下一个子阶段。

## 14. 风险与回滚

- 目标态接错：回滚该子阶段前端提交即可，后端冻结契约不受影响。
- 媒体上传成功但业务提交失败：保留草稿和 asset id，按原幂等 key 重试；孤立 media 的清理由既有生命周期策略处理，不在前端私自删除。
- 网络超时导致结果未知：先查询服务端，再决定展示；禁止生成新 key 盲目重发。
- 旧入口回流：通过路由核对和残留搜索阻止 `pages/publish`、`pages/product-detail` 成为正式入口。

本规格只固化设计，不授权实施。实施前还需基于本设计编写逐步实施计划并经 owner 确认。
