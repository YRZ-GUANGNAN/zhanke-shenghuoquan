# UI-21b-2c 二手真实写入闭环设计

## 背景

`UI-21b-2b` 已将社区发帖、点赞、评论和收藏接到冻结的后端契约。二手视觉页面仍保留三个断点：发布二手只显示静态提示、详情页的收藏与联系卖家未调用真实能力、视觉私信页的文本与图片均为静态演示。

本阶段只补齐已经存在且可安全调用的前端能力：创建二手商品、收藏切换、建立二手会话和腾讯 IM 文本收发。后端、路由契约和资金逻辑均保持冻结。

## 目标

1. `pages/publish-secondhand` 能上传商品图片并调用既有 `POST /trade/products` 创建二手商品。
2. `pages/secondhand-detail` 能调用既有收藏切换和联系卖家接口，并跳转到视觉风格的二手私信页。
3. `pages/secondhand-chat` 以既有会话上下文连接腾讯 IM，展示、发送和订阅真实文本消息。
4. 保留当前静态设计、页面路由和视觉规范；真实交互仅替换静态提示，不回退到旧版页面样式。

## 非目标与硬边界

- 不修改 `backend/**`、迁移、HTTP 路由、请求或响应契约。
- 不修改支付、配送、媒体审核或旧外卖封存逻辑。
- 不新增或伪造通用二手 IM 图片消息。当前图片安全链路仅服务于配送会话，二手私信没有受参与方鉴权保护的图片上传、审核和短时 URL resolve 契约。
- 私信页图片按钮保留视觉位置，但点击明确提示“二手私信图片发送尚未开放”，不选择本地图片、不发送腾讯原生图片、不把临时本地路径写入消息。
- 不将二手私信重定向到旧的 `pages/chat`；始终留在当前视觉 `pages/secondhand-chat`。

## 已冻结的前端与服务契约

### 创建二手商品

既有 `c-trade.createProduct` 调用 `POST /trade/products`。后端接受：

- `clientRequestId`：26 位 ULID；
- `title`、`description`、`category`；
- `price`：整数分；
- `imageAssetIds`：新 media 上传所得的 asset public id 数组。

请求同时带 `Idempotency-Key`，以便服务端在网络重试时保证同一逻辑发布不重复创建。创建成功为 201，幂等重放为 200；非法输入为 400，幂等键与不同请求体冲突为 409，依赖暂不可用为 503/504。

### 收藏与联系卖家

- `toggleProductFavorite(productId)` 已返回目标收藏状态。
- `contactSeller(productId)` 会建立或返回 `IMConversationContext`，包含腾讯 IM 文本会话所需的合法上下文。
- 二手详情页仍通过 `getProduct(productId)` 读取当前商品；进入聊天页后须再次按 `productId` 调用 `contactSeller`，不能信任 URL 中伪造的会话参数。

### 文本 IM

既有 `e-im` 服务提供 `ensureTencentIMReady`、`loadTencentMessages`、`sendTencentText` 与 `subscribeTencentMessages`。视觉聊天页在得到 `IMConversationContext` 后调用这些服务，并在页面卸载时取消订阅。

## 页面行为

### 1. 发布二手：`pages/publish-secondhand`

1. 图片选择改为调用 `chooseAndUploadImages('product_image', remaining)`；页面保存返回的 asset public id 与预览信息。
2. 保持“最多 9 张、第一张为封面”的静态视觉和表单校验。
3. 点击发布时将展示价（元）精确转换为整数分；不使用浮点金额传给服务端。
4. 每一次逻辑发布生成一个前端 ULID，并同时写入 `clientRequestId` 与 `Idempotency-Key`。网络失败后的原样重试复用同一个 key；成功、用户修改发布内容或收到 409 冲突后，丢弃旧 key 并生成新的逻辑请求。
5. 成功后提示“发布成功，等待审核”，并返回二手列表或“我的内容”。400、409、503/504 分别给出明确且不误导的提示；提交中禁用重复点击。

### 2. 二手详情：`pages/secondhand-detail`

1. 收藏按钮调用 `toggleProductFavorite`，以服务端返回状态更新图标与文案；失败时保留原状态并提示错误。
2. “联系卖家”底部操作保留现有联系面板。选择“发起私聊”时调用 `contactSeller(productId)`，成功后关闭面板并进入 `pages/secondhand-chat?productId=<id>`。
3. 用户取消、无权限、商品不存在或会话创建失败均不假定成功，并展示可理解的错误反馈。

### 3. 二手私信：`pages/secondhand-chat`

1. 页面按 `productId` 读取商品和重新获取 `IMConversationContext`；加载期间显示当前设计体系内的加载/错误状态。
2. 历史消息由 `loadTencentMessages` 提供，实时消息由 `subscribeTencentMessages` 追加；仅渲染当前会话的文本消息。
3. 输入非空文本后调用 `sendTencentText`。发送过程中禁用发送按钮，成功后清空输入框，失败时保留文本以便用户重试。
4. 显示商品关联卡、交易风险提示、文本消息气泡和底部输入区，保持当前二手私信视觉稿。
5. 图片入口显示清楚的“暂未开放”反馈；不调用临时上传、不发送未经审核或没有参与方授权的媒体。

## 前端组织

- 仅修改或新增 `apps/miniapp/src/**`：`services/c-trade.ts`、可能新增独立的前端请求 ID 工具、三个二手页面及其样式/配置。
- 不从配送商家服务导入请求 ID 工具；若抽取 ULID 生成器，放入通用 miniapp 工具或二手服务，避免不相关领域耦合。
- 不改变现有 `api.ts` 的请求、鉴权和错误模型；利用它已经保留的 HTTP 状态映射错误文案。

## 验收标准

1. 真实后端可用时：发布二手上传图片后服务端收到真实 asset public ids、价格为整数分、同一幂等键不会重复创建。
2. 二手详情收藏状态由真实响应驱动；联系卖家后能够进入视觉私信页。
3. 文本私信能加载、发送并接收腾讯 IM 文本消息；异常时不显示“已发送”的假成功。
4. 点击二手私信图片入口只显示“尚未开放”，不会产生媒体写入或图片消息。
5. `npm run typecheck`、`npm run build:weapp` 通过；后端代码、接口契约及支付/配送域无 diff。

## 风险与后续

二手 IM 图片需要独立授权：定义私有媒体 purpose、上传绑定、审核投影、商品会话参与方鉴权和短时 resolve。该能力完成前，禁用图片发送是正确的 fail-closed 行为。
