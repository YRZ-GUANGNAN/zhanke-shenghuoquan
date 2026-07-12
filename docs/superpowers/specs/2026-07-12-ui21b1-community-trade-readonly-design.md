# UI-21b-1 社区与二手只读真实数据接线设计

## 状态

- 设计状态：已获 owner 批准
- 基线：`master@18dc0d28`
- 前置冻结：`ui21a-runtime-identity-baseline`
- 实施方式：设计文档提交后，另建隔离 worktree；不得直接在 `master` 开发

## 目标

在不改变 UI-20 冻结视觉、不恢复旧页面、不修改后端契约的前提下，把新版学生端的四条只读主链从静态样例切换为真实数据：

1. 首页“发现”瀑布流；
2. 帖子详情与评论读取；
3. 二手列表；
4. 二手详情。

完成后，用户从真实列表进入的详情必须与列表 ID 一致；网络失败、空数据和 404 必须诚实显示，任何情况都不得回退到芋泥波波、自行车等演示数据。

## 明确不做

- 不接点赞、收藏、关注、评论提交、发布和私信；这些属于 UI-21b-2。
- 不接“我的内容”、Profile 统计、社区统一搜索、消息列表和工具页数据。
- 不修改 `backend/**`、迁移、`apps/admin/**`、支付、Delivery、Media 状态机或腾讯 IM。
- 不新增依赖，不修改 `package.json` 或 lockfile。
- 不重新注册旧 `market`、`product-detail`、`conversations`、`chat`、`takeout*` 页面。
- 不用本地 state、Toast 或静态数组冒充服务端写入成功。
- 不提交新的仓库守卫脚本；真实性检查在验收阶段以只读 `rg` 命令执行，避免突破 `apps/miniapp/src/**` 边界。

## 允许改动范围

允许修改：

- `apps/miniapp/src/pages/home/**`
- `apps/miniapp/src/pages/post-detail/**`
- `apps/miniapp/src/pages/student-secondhand/**`
- `apps/miniapp/src/pages/secondhand-detail/**`
- `apps/miniapp/src/services/b-community.ts`
- `apps/miniapp/src/services/c-trade.ts`
- 必要的 `apps/miniapp/src/components/**` 只读数据状态组件
- 本模块的设计、实施和验收文档

禁止修改：

- `apps/miniapp/src/app.tsx`
- `RuntimeSessionGate`、session store、身份切换和身份守卫
- `backend/**`、`apps/admin/**`、迁移、配置、依赖清单和 lockfile
- 受保护分支/工作树 `feat/p0-identity-switch`

## 后端读取契约

所有成功响应使用 `{ code: 0, message: "ok", data: ... }`；失败响应使用 `{ code: HTTP状态码, message: string }`。

| 用途 | 端点 | 参数 | 返回 | 错误语义 |
| --- | --- | --- | --- | --- |
| 社区发现流 | `GET /api/v1/community/posts` | `topic?`、`keyword?`、`sort?`、`page?`、`pageSize?` | `Page<PostView>` | 公开读取；服务异常 500 |
| 帖子详情 | `GET /api/v1/community/posts/:id` | 正整数 ID | `PostView` | 非法 ID 400；不存在、未审核通过或已下架 404 |
| 帖子评论 | `GET /api/v1/community/posts/:id/comments` | 正整数 ID、`page?`、`pageSize?` | `Page<CommentView>` | 非法 ID 400；合法但不存在的帖子当前返回 200 空页 |
| 二手列表 | `GET /api/v1/trade/products` | `keyword?`、`category?`、`status?`、`page?`、`pageSize?` | `Page<ProductView>` | 非法状态 400；关键词搜索不可用时 503 |
| 二手详情 | `GET /api/v1/trade/products/:id` | 正整数 ID | `ProductView` | 非法 ID 400；不存在、未审核通过或已下架 404 |

分页统一为：

```ts
interface PageResult<T> {
  items: T[]
  page: number
  pageSize: number
  total: number
}
```

默认 `page=1`、`pageSize=20`，后端最大 `pageSize=100`。社区默认按置顶和创建时间排序；二手无关键词时按创建时间与 ID 倒序。有关键词的二手搜索依赖 Meilisearch，但本模块的二手主列表不主动传关键词。

这五个 GET 均为公开读取端点，不挂 `AuthRequired`。小程序仍保留 UI-21a 的运行时登录门禁，但不得为了验收制造不存在的读取 401。若出现 401，视为误调用了写接口或错误媒体端点，必须停止排查。

## DTO 与前端归一化

### Community

`PostView` 至少包含：数字 `id/userId`、`topic/title/content`、`pinned`、三种计数、`images`、时间戳和 `author`。`CommentView` 包含数字 `id/postId/userId`、正文、时间戳和 `author`。

前端继续复用 `CommunityPost`、`CommunityComment`，在 service 层完成：

- 数字 ID 转字符串，仅用于前端 key 和路由；
- `images ?? []`；
- 作者昵称、头像和学校字段归一化；
- 日期格式化；
- 保留真实计数，不补造评论点赞数或关注状态。

### Trade

`ProductView` 至少包含：数字 `id/sellerId`、标题、描述、分类、`price/originalPrice`、成色、校区、状态、图片、收藏数、时间戳和 `seller`。

前端在 service 层完成：

- 数字 ID 转字符串；
- 金额由整数分除以 100，禁止浮点金额回写；
- `images ?? []`；
- `selling/reserved/sold` 映射为“出售中/已预订/已售”；
- 不伪造浏览数、卖家成交数或微信号。

为支持分页，service 采用纯增量方式增加保留分页元数据的读取函数；既有导出继续委托新函数，避免破坏其它调用方。

## 图片链路

Community/Trade 的 `images[].url` 是现有 legacy media 的完整业务 URL。前端直接传给 `<Image src>`，由该 URL 302 跳转到短期 COS 签名地址。

- 不拼接 `/miniapp/media/resolve`；
- 不把数字 Post/Product ID 当成 media public ID；
- 使用第一张图填充现有媒体容器，`mode="aspectFill"`；
- 没有图片或图片加载失败时，仍在相同容器中显示原占位文案，卡片高度不得坍塌。

## 页面设计

### 1. 首页发现流

数据源为社区最近帖子与在售二手商品。两类结果按固定顺序交错合并，key 使用 `post:<id>` / `secondhand:<id>`，避免数字 ID 碰撞。

- 帖子卡使用标题、作者、头像、点赞数和第一张图；
- 二手卡使用标题、卖家、头像、收藏数和第一张图；
- `tone` 与高度由 `target:id` 的稳定散列决定，不使用随机数，保证刷新后瀑布流不跳位；
- 列表分页时分别维护 Community 和 Trade 页码，合并后按 key 去重；两边都到达 `total` 后停止加载；
- 首屏使用独立请求结果：两边都失败才进入整块 `error`；一边成功时保留该边的真实卡片，并在瀑布流末尾显示“部分内容加载失败，点击重试”，重试只请求失败的数据源；
- 分页时单边失败不得清空已加载卡片，只在瀑布流末尾显示可重试状态；状态需区分首屏 `loading`、`loadingMore`、两边各自的 `hasMore/error`；
- 首页继续使用现有 CSS 滚动的 `View`，不得为分页改成 `ScrollView`。在现有最后一张卡片/末尾哨兵上使用 `Taro.createIntersectionObserver` 触发下一页，只允许增加 `id` 或 `data-*`，不改变 className 和主要层级；
- “私信”和“工具”Tab 本模块不接线，保留现有壳层行为，不得与发现流失败回退共用静态卡片。

状态插入点仅限 `student-home-content` 中发现流区域：

- `loading`：使用现有 `student-post-card` 尺寸的骨架卡；
- `empty`：显示“暂时没有内容”；
- `error`：显示“加载失败，点击重试”；
- `ready`：渲染真实瀑布流。

现有点赞图标保留，但点击只提示“互动功能下一阶段接入”，不得本地改变计数或选中态。这里采用业务层禁用，不使用会改变默认灰度与尺寸的原生 `disabled`。

### 2. 帖子详情与评论

路由参数 `id` 必须是正整数。移除默认 `milk-tea` 和 `postCopies`/静态评论回退。

- 帖子与首屏评论可并行读取；
- 评论分页按后端正序追加并按评论 ID 去重；
- 评论分页继续使用现有 CSS 滚动的 `View`；在既有评论区底部节点上使用 `Taro.createIntersectionObserver`，不得把 `post-detail-scroll` 改为 `ScrollView`；
- 第一张图进入现有 hero；无图保持相同 hero 占位；
- `topic` 映射为真实话题标签；
- 评论没有点赞数字段，评论点赞区域不得显示伪造数字；
- 404 显示“帖子不存在或已下架”；
- 合法 ID 但评论接口返回空页时显示真实空评论态。

导航、滚动区、内容块、评论区和底部 actionbar 的外层 DOM/className 均保持。关注、点赞、收藏、评论提交控件保留外观但禁用真实写入，并统一提示 UI-21b-2 接入。评论输入框可继续输入以保持视觉和键盘体验，但点击提交时不得清空草稿或追加假评论；所有写控件均采用业务层拦截，不添加会改变冻结视觉的原生 `disabled`。

### 3. 二手列表

数据源为 `status=selling` 的真实商品列表。分类改为向后端传 `category` 精确筛选，不能只过滤当前已加载页冒充全量分类。“全部”必须省略 `category`（或传空字符串），其余分类才传对应的原始中文分类值；禁止把字面值“全部”传给后端。

- 分类切换重置 `page/items/total` 并重新请求；
- 使用既有 `ScrollView.onScrollToLower` 在滚动到底时加载下一页，不更换滚动容器；
- 新页按商品 ID 去重；
- 达到 `total` 后停止请求；
- 图片和占位均使用现有 `secondhand-card__media`；
- `tone/height` 由商品 ID 稳定生成。

header、分类条、风险提示、双列瀑布流和 `StudentBottomNav` 保持不变。空分类显示“该分类暂无在售宝贝”；网络失败显示可点击重试卡片。

### 4. 二手详情

路由参数 `id` 必须是正整数。移除默认 `bike`、`productCopies` 和 `missingProductCopy` 的静态替代内容。

- 第一张真实图片进入 hero；
- 真实价格、状态、标题、描述、分类、成色、时间、卖家昵称和校区映射到原位置；
- 后端没有浏览数、卖家成交统计或微信号时，不显示伪造值；
- 404 显示“宝贝不存在或已下架”；
- 卖家主页、收藏、联系卖家和加微信均不在本模块接线。收藏只提示下一阶段接入；联系卖家弹层的纯展示开关可以保留，但“发起私聊/加微信”只提示，不调用写接口、不复制静态微信号；
- 不调用 `contactSeller`，不创建 IM 关系，不跳转静态聊天冒充真实私信。

header、滚动区、hero、info、seller、warning、actionbar 和 contact sheet 的几何结构保持不变。

## 加载状态与并发

每页采用统一状态：

```ts
type LoadStatus = 'loading' | 'ready' | 'empty' | 'error'
```

- 页面首次进入请求一次；同一轮重复触发合并或忽略；
- 路由 ID 变化时废弃旧请求结果，禁止旧详情覆盖新详情；
- 首屏状态与分页状态分离；分页加载中忽略再次触底请求；
- 重试只重放当前查询条件；
- 失败后保留错误态，不注入演示数组；
- 组件卸载后不得继续写 state。

`ApiError.status===404` 映射为“不存在或已下架”；`status===0` 映射为网络错误；其它状态显示安全的失败文案，并保留点击重试。

## 视觉冻结

- 四页根容器、header、底栏、瀑布流双列、详情 actionbar 和既有滚动区域不变；
- 已有 className 和主要 DOM 层级不重命名、不搬迁；
- loading/empty/error 只能嵌入原内容容器，不新增全屏遮罩；
- 不用旧页面 JSX 覆盖新版页面；
- 不改 UI-20/21a 已冻结的导航、登录和身份行为；
- 视觉变化仅限真实图片、真实文字长度和诚实状态文案。

## 真实性守卫

由于本模块坚持只改 `apps/miniapp/src/**`，不提交新脚本。验收阶段以只读命令确认：

- 四个目标页不再包含 `discoverCards`、`postCopies`、`productCopies` 或目标功能的静态 `products/comments`；
- 不再包含 `milk-tea`、`bike`、芋泥波波、固定 A20xx/B10xx 等演示回退；
- 页面无直接 `Taro.request`；
- 页面从 `b-community.ts` / `c-trade.ts` 获取数据；
- `catch` 中不存在 demo 数组或静态 fallback；
- 静态路由检查仍为仓库当前真实值 25 routes。

## 验收

### 自动门禁

1. `npm run check:boundaries`
2. `npm run check:miniapp-static-routes`
3. UI-21b-1 只读真实性 `rg` 检查
4. `npm --workspace apps/miniapp run typecheck`
5. `npm run build:miniapp`
6. `npm run verify`
7. `git diff --check`
8. `backend/**`、`apps/admin/**`、迁移、`package.json` 和 lockfile diff 为 0

### 真实环境

使用真实 Gin、MySQL 8.4、Meilisearch 和现有 JWT/session，在微信开发者工具验证：

1. 社区列表字段、计数和数据库一致；
2. 从列表 ID 进入对应帖子详情，评论来自真实接口；
3. 二手列表价格按整数分正确显示，分类请求是后端全量筛选；
4. 从列表 ID 进入对应商品详情，不串到默认自行车；
5. 第二页无重复、无丢失，到达 total 后停止；
6. 空库/空分类显示空态，不出现样例卡；
7. 未知 ID 显示 404 状态，不回退静态内容；
8. 断开后端或制造 5xx 后显示错误并可重试；
9. 无图片和图片加载失败保持原占位尺寸；
10. 写操作按钮不产生本地假状态，也不发写请求。

## 残留与回滚

- 验收夹具使用独立前缀并 scoped 清理；
- 恢复验收前 token；
- 清理对应 Meili 文档/outbox 残留；
- 关闭后端、Meilisearch 和 DevTools 自动化进程；
- 不提交 `.env`、凭证、日志、`dist` 或 DevTools 自动改写的 `project.config.json`；
- 最终 worktree clean；主仓库与 `feat/p0-identity-switch` 不受影响。

回滚只需回退 UI-21b-1 提交，不涉及数据库或迁移回滚。

## 设计自检

- 范围只有四条只读主链，无支付、上传、IM 或写操作偷渡。
- 后端契约不足时停止并报告，不补假字段、不顺手改后端。
- 公开 GET 的真实鉴权语义已写清，没有伪造 401 验收。
- 分类使用服务端筛选，避免“仅过滤当前页”造成错误结果。
- 金额继续遵守整数分规则。
- 图片沿用 B/C 既有公开 URL，不误接新 media resolve。
- 错误、空态和 404 均不会回退演示数据。
- 视觉冻结、独立 worktree、边界、真实环境和残留要求均有唯一解释。
