# UI-21b-1 社区与二手只读真实数据接线 Development Report

## 结论

UI-21b-1 已在隔离工作树 `feat/ui21b1-community-trade-readonly` 完成。基线为 `master@6979762d`，代码最终提交为 `6a580889`。

本模块只接通社区与二手的真实只读数据，不修改后端、接口、迁移、登录门禁、身份切换或任何写业务。

## 提交记录

- `35db69de feat(UI-21b-1): 增加社区二手分页读取契约`
- `9b7abd4f feat(UI-21b-1): 接通首页社区二手发现流`
- `e7b01446 feat(UI-21b-1): 接通帖子详情与评论读取`
- `1e51db44 feat(UI-21b-1): 接通二手列表与分类分页`
- `6a580889 feat(UI-21b-1): 接通二手详情只读数据`

## 改动范围

共修改 10 个既有前端文件：

- `apps/miniapp/src/services/b-community.ts`
- `apps/miniapp/src/services/c-trade.ts`
- `apps/miniapp/src/pages/home/index.tsx`
- `apps/miniapp/src/pages/home/index.scss`
- `apps/miniapp/src/pages/post-detail/index.tsx`
- `apps/miniapp/src/pages/post-detail/index.scss`
- `apps/miniapp/src/pages/student-secondhand/index.tsx`
- `apps/miniapp/src/pages/student-secondhand/index.scss`
- `apps/miniapp/src/pages/secondhand-detail/index.tsx`
- `apps/miniapp/src/pages/secondhand-detail/index.scss`

`backend/**`、`apps/admin/**`、迁移、`app.tsx`、session、身份切换及受保护工作树均为零改动。

## 页面与接口映射

| 页面 | 真实读取端点 |
| --- | --- |
| 首页发现流 | `GET /api/v1/community/posts?page=&pageSize=&sort=recent` |
| 首页发现流 | `GET /api/v1/trade/products?page=&pageSize=&status=selling` |
| 帖子详情 | `GET /api/v1/community/posts/:id` |
| 帖子评论 | `GET /api/v1/community/posts/:id/comments?page=&pageSize=` |
| 二手列表 | `GET /api/v1/trade/products?page=&pageSize=&status=selling&category=` |
| 二手详情 | `GET /api/v1/trade/products/:id` |

服务层新增兼容的分页返回结构，同时保留既有导出行为：

```ts
interface PageResult<T> {
  items: T[]
  page: number
  pageSize: number
  total: number
}
```

## 核心实现

### 首页发现流

- Community 与 Trade 分别维护页码、总量和错误状态。
- 使用 `post:<id>`、`secondhand:<id>` 作为去重键，避免两个域的数字 ID 碰撞。
- 两类数据按固定顺序交错，卡片色调和高度由稳定 ID 映射生成，不使用随机数。
- 只有两类请求全部失败时才进入整页错误态；一边成功时保留真实内容，并允许单独重试失败源。
- 分页失败不清空已加载内容；重试只请求失败的数据源。
- 使用既有页面滚动容器和 `IntersectionObserver` 加载下一页。

### 帖子详情与评论

- 路由 `id` 必须是正整数，不再使用静态默认帖子。
- 帖子详情与首屏评论并行读取。
- 评论翻页按评论 ID 去重，到达 `total` 后停止请求。
- 404 映射为“不存在或已下架”，网络错误保留点击重试。
- 评论 DTO 没有点赞数字字段，因此不展示伪造计数。

### 二手列表

- 固定请求 `status=selling`。
- 分类切换向后端传原始分类值；“全部”省略 `category`。
- 分类切换时重置页码、列表和总量。
- 使用 `queryVersion` 丢弃旧分类或页面卸载后的迟到响应。
- 使用加载锁避免同页并发重复请求，按商品 ID 去重，到达 `total` 后停止。
- 左右列按稳定索引拆分，刷新后不随机跳位。

### 二手详情

- 路由 `id` 必须是正整数。
- 展示真实价格、状态、标题、描述、分类、成色、时间、卖家昵称和校区。
- 后端金额按整数分返回，由 `c-trade.ts` 统一转换为元，页面不猜测金额单位。
- 不伪造浏览量、成交量、微信号或卖家统计。

### 图片链路

- 直接使用 Community/Trade DTO 中的 `images[].url`。
- 首图通过 `<Image mode="aspectFill">` 渲染。
- 不拼装新 media resolve 地址，也不把业务数字 ID 当 media public ID。
- 无图或图片加载失败时，在原媒体容器内显示占位，保持冻结 UI 的几何尺寸。

### 写操作边界

关注、点赞、收藏、评论提交、联系卖家等写控件保留原视觉，但只提示“下一阶段接入”：

- 不调用写接口；
- 不本地修改计数或选中态；
- 不创建伪造评论或会话；
- 不使用会改变冻结视觉的原生 `disabled` 样式。

## 审查修正

实现审查中发现并修复两项 P2：

1. 首页双源首屏状态判断曾可能把“一边空成功、一边失败”误判为整页错误，现已改为只有所有被请求数据源均失败才进入整页错误态。
2. 二手分类请求在页面卸载后存在迟到响应风险，现已在 cleanup 中推进 `queryVersion`，拒绝旧响应写入。

最终独立只读审计未发现 P1/P2/P3 问题。

## 明确未做

- 社区点赞、关注、收藏、评论写入；
- 二手收藏、联系卖家和 IM 创建；
- 搜索真实接线；
- 用户主页写能力；
- 后端、迁移、权限、登录或身份切换修改；
- UI-21b-2 及后续模块。
