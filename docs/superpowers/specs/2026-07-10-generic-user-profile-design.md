# 通用个人主页设计

## 背景

学生端“我的”页面右上角当前是“编辑资料”，只能进入资料编辑页。产品需要将该入口改为“主页”，并建立一个可被“我的”、帖子作者和二手卖家共同复用的个人主页，而不是为“阿泽”单独制作页面。

当前前端仍处于静态页面联调阶段。现有后端提供帖子列表、帖子详情、关注、二手列表和本人二手列表，但没有公开用户资料、按指定用户筛选帖子、按指定用户筛选二手的完整 API。因此本任务只完成通用前端页面、路由契约和静态交互，不新增或修改后端接口。

## 目标

- 将“我的”页面右上角按钮从“编辑资料”改为“主页”。
- 新增统一路由 `/pages/user-profile/index?userId=<id>&source=<source>`。
- 页面根据 `userId` 渲染统一的用户资料结构，不在组件中绑定“阿泽”。
- 同一页面支持“帖子”和“二手”两个内容标签、数量展示和瀑布流卡片。
- 区分本人视角和他人视角，避免本人关注、私信、举报或拉黑自己。
- 保留后续接入真实用户、帖子和二手 API 的清晰数据边界。

## 非目标

- 不修改 backend、数据库迁移或现有配送域。
- 不在本任务中新增公开用户资料或按用户筛选内容的 API。
- 不实现真实关注、私信、举报、拉黑、复制链接或分享能力；静态阶段使用可验证的页面跳转、状态切换或提示反馈。
- 不重做“我的”、首页、帖子详情和二手详情的整体视觉。

## 页面与路由

新增 `pages/user-profile/index` 并注册到 `app.config.ts`。

路由参数：

- `userId`：目标用户标识，是页面数据选择和未来 API 请求的唯一主键。
- `source`：可选来源标识，例如 `profile`、`post`、`secondhand`，仅用于返回和交互上下文，不参与用户身份判断。
- `tab`：可选初始标签，取值 `posts` 或 `secondhand`，默认 `posts`。

“我的”页面入口在静态阶段使用稳定标识 `userId=current-user`；数据适配层将其解析为当前用户视图并设置 `isSelf=true`。其他用户使用各自稳定 ID，组件只消费统一的 `PublicProfileView`，不按用户名或来源参数写分支。后续接入登录态后，用真实当前用户 ID 替换 `current-user`，路由和页面组件保持不变。

## 数据模型

页面使用一个前端视图模型：

```ts
interface PublicProfileView {
  id: string
  nickname: string
  account: string
  avatarText: string
  school: string
  grade?: string
  bio: string
  followingCount: number
  followerCount: number
  likedCount: number
  isSelf: boolean
  isFollowing: boolean
  posts: PublicProfilePost[]
  products: PublicProfileProduct[]
}
```

`PublicProfilePost` 和 `PublicProfileProduct` 只包含卡片渲染与详情跳转所需字段。静态数据与页面渲染分离，后续可以将数据适配层替换为真实 API，而不改变页面结构和交互组件。

## 视角与权限

本人视角：

- 主操作显示“编辑资料”和“分享主页”。
- “编辑资料”进入现有 `/pages/edit-profile/index`。
- 更多菜单只显示“复制主页链接”；不显示举报和拉黑。

他人视角：

- 主操作显示“关注/已关注”和“私信”。
- 关注按钮在静态页面内切换状态和粉丝数。
- 私信进入现有会话入口；缺少真实会话参数时给出明确静态提示。
- 更多菜单显示“复制主页链接”“举报该用户”“拉黑该用户”和“取消”。

## 布局与交互

- 顶部采用参考图中的蓝色资料区，包括返回、更多菜单、头像、昵称、账号、学校、简介和社交统计。
- 中间操作区按本人/他人视角显示对应按钮。
- 内容区使用“帖子 N / 二手 N”双标签，切换时不改变页面尺寸和顶部布局。
- 帖子使用双列瀑布流卡片，显示图片占位、标题和点赞数；点击进入现有帖子详情页。
- 二手使用双列卡片，显示图片占位、标题、价格和状态；点击进入现有二手详情页。
- 空标签显示稳定的空状态，不渲染无意义占位卡片。
- 更多菜单使用页面内遮罩和底部操作面板，点击遮罩或“取消”关闭。

## 错误与降级

- 缺少或无法识别 `userId` 时，数据适配层返回通用示例用户并展示非阻断提示，页面不白屏。
- 静态阶段未接入的操作统一使用 `Taro.showToast` 反馈。
- 详情页尚未支持真实 ID 时仍携带卡片 ID 生成稳定路由，现有详情页可忽略该参数。

## 预计改动范围

- 修改 `apps/miniapp/src/pages/profile/index.tsx`，替换右上角入口文案和路由。
- 视样式需要调整 `apps/miniapp/src/pages/profile/index.scss` 中入口宽度。
- 修改 `apps/miniapp/src/app.config.ts`，注册通用个人主页。
- 新增 `apps/miniapp/src/pages/user-profile/index.tsx`。
- 新增 `apps/miniapp/src/pages/user-profile/index.scss`。
- 新增 `apps/miniapp/src/pages/user-profile/index.config.ts`。
- 可新增同目录静态数据文件，使页面组件不直接绑定具体用户名。

## 验收标准

- 从“我的”页面点击“主页”可进入通用个人主页。
- 页面组件以 `userId` 和统一视图模型驱动，没有“阿泽专用页”或按昵称判断逻辑。
- 本人视角不会出现关注、私信、举报或拉黑自己的操作。
- 他人视角具备关注状态切换、私信入口和完整更多菜单。
- “帖子/二手”可切换，计数、空状态和卡片内容一致。
- 帖子和二手卡片均可进入现有详情页。
- 页面在常见小程序手机宽度下无文字重叠、横向溢出或底部面板遮挡错误。
- `npm --workspace apps/miniapp run typecheck` 和小程序构建通过。
- `master` 保持不变，所有改动仅存在于独立 worktree 分支。
