# UI-21b-2c 二手真实写入闭环实施计划

> 状态：已完成设计确认，待执行代码实施。
> 范围：仅 `apps/miniapp/src/**` 前端；后端 API、迁移与领域逻辑冻结不动。

## 1. 目标与不可突破的边界

本模块把二手的三条关键写入链路从静态演示接到既有真实 API：

1. 发布二手商品：图片上传、金额转换、创建商品与幂等重试；
2. 商品详情：收藏切换与联系卖家；
3. 二手私信：既有腾讯云 IM 的真实文字收发与收消息订阅。

必须保留已冻结的新版视觉结构、页面路径和交互层级；不得跳回旧 `pages/chat`，不得改动后端或 API 契约。通用二手私信图片发送本轮明确不实现：当前只有配送单图片拥有参与方授权与私有媒体解析协议，不能以本地预览或不受控上传伪造该能力。

## 2. 影响文件与职责

| 文件 | 计划改动 |
| --- | --- |
| `apps/miniapp/src/services/client-request-id.ts` | 新增通用、无业务域依赖的 ULID 格式 `clientRequestId` 生成器。 |
| `apps/miniapp/src/services/c-trade.ts` | 为 `createProduct` 增加请求标识输入；请求体传 `client_request_id`，请求头传 `Idempotency-Key`，保持既有 API 错误状态语义。 |
| `apps/miniapp/src/pages/publish-secondhand/index.tsx` | 用 `chooseAndUploadImages('product_image', ...)` 替换本地假选图；将元转换为整数分；真实提交、加载态、失败提示与幂等重试。 |
| `apps/miniapp/src/pages/secondhand-detail/index.tsx` | 收藏接 `toggleProductFavorite`；联系卖家接 `contactSeller` 后跳转新版二手聊天页。 |
| `apps/miniapp/src/pages/secondhand-chat/index.tsx` | 以 `productId` 重新获取商品并创建/获取真实 IM 上下文；接腾讯云 IM 的文字拉取、发送和订阅；图片入口明确提示暂未开放。 |
| `apps/miniapp/src/**/*.test.*`（按现有测试组织） | 为纯转换、幂等键和关键页面行为补最小覆盖；若项目不存在适用页面测试基座，则以服务层单测与 TypeScript 构建验证为准。 |
| `docs/` | 实施完成后新增开发、验收和残留报告，记录真实 API 验收状态及任何外部依赖阻塞。 |

## 3. 先实现可复用的请求标识能力

1. 新增 `client-request-id.ts`，复用项目已验证的 Crockford Base32 规则，生成 26 字符、可被后端 `ValidULID` 接受的标识。
2. 此帮助器不导入 delivery、trade 或页面代码，避免为了二手发布反向依赖配送服务。
3. 为固定时间和随机源保留可测试入口，验证长度、字符集与多次调用的差异性。

## 4. 对齐二手创建商品契约

1. 为 `createProduct` 定义显式输入类型：标题、描述、整数分 `price`、分类、`imageAssetIds`、`clientRequestId`。
2. 请求继续使用既有 `request` 封装，向 `POST /trade/products` 同时发送：
   - body `client_request_id`；
   - header `Idempotency-Key`。
3. 不吞掉 `ApiError` 的 HTTP `status`；页面据此将 409、认证失败、参数失败、网络/服务端失败分别呈现。
4. 不改列表、详情、收藏和私信的既有读取 API。

## 5. 发布二手商品的真实流程

1. 将图片状态由单纯本地路径改成“预览路径 + 上传得到的 asset public id”的页面状态。
2. 点击添加图片时调用 `chooseAndUploadImages('product_image', remainingCount)`；进度或上传失败使用既有小程序提示，成功后将实际 asset id 保存在表单状态。删除图片只删除当前编辑态引用，不声称删除媒体资产。
3. 提交前继续沿用现有标题、价格、分类、描述校验；价格仅接受可精确转换为分的正数，并用整数分提交，禁止浮点金额进入 API。
4. 校验通过时惰性生成一次 `clientRequestId` 并写入 ref。提交按钮禁用且显示加载中；同一次网络失败或 5xx 后重试复用该 ref，避免重复建商品。
5. 用户编辑表单、服务端已确认成功、或服务端返回已处理的业务结果（例如 409）后清除该 ref；下一次不同内容的提交生成新 key。
6. 成功后显示成功提示并返回二手列表/刷新可见状态；失败不伪造成功，也不丢失用户填写内容。

## 6. 商品详情的真实收藏与联系

1. 收藏按钮调用 `toggleProductFavorite(product.id)`，用服务端返回的最终布尔值更新局部 UI，避免前端自行猜测状态。
2. 操作中禁用重复点击；401/403、网络错误与其他 API 错误显示明确提示且保留原收藏状态。
3. 联系卖家弹层保留新视觉与“平台不参与线下交易”提示；点击“发起私聊”时调用 `contactSeller(product.id)`。
4. 成功后跳转 `pages/secondhand-chat/index`，只传 `productId`。聊天页自己重建上下文，不能信任 URL 中伪造的会话 id 或卖家 id。

## 7. 新版二手聊天页的真实文字 IM

1. 页面进入时校验 `productId`，重新调用 `getProduct(productId)` 与 `contactSeller(productId)`，得到当前受权的 `IMConversationContext`。
2. 使用 `loadTencentMessages(context)` 拉取现有文字消息，并在页面卸载时清理 `subscribeTencentMessages(context, ...)` 的订阅。
3. 增量消息以腾讯消息唯一 ID 去重，按时间和发送方向映射为当前视觉气泡。仅渲染文字消息；未知消息类型安全忽略，不以伪内容代替。
4. 点击发送时校验非空文本，调用 `sendTencentText(context, text)`；成功后由 SDK 返回/订阅同步 UI，失败时保留草稿并给出可重试提示。
5. 图片按钮保留为视觉入口，但点击只提示“二手私信图片发送尚未开放”，不调用上传、不生成本地假图片、不复用配送私有图片端点。
6. 任何商品加载、联系卖家或 IM 初始化失败均留在新版页面提供明确错误提示，不跳转旧 `pages/chat`。

## 8. 验证顺序与验收证据

1. 运行 `npm run typecheck` 与 `npm run build:weapp`，修正 TypeScript、Taro 和小程序构建问题。
2. 检查本模块 diff：只允许 `apps/miniapp/src/**`、本模块文档与必要测试；禁止 `backend/**`、迁移、旧外卖、配送媒体协议改动。
3. 在真实后端可用时，以真实登录 token 验收：
   - 选图上传后发布二手商品，确认请求携带 asset id、整数分金额和两个幂等键位置；
   - 网络/重试场景确认同一逻辑请求复用 key；
   - 详情收藏真实更新、联系卖家进入新版聊天；
   - 两个用户之间发送、拉取并实时收到文字消息；
   - 图片按钮不产生上传或假消息。
4. 若真实 IM、媒体或后端环境不可用，只报告为外部验收阻塞；不得以 mock 伪造“真实写入通过”。
5. 完成后写入 Development / Acceptance / Residue 报告，提交并确认 feature worktree 干净，等待 owner 验收和串行 ff 合并。
