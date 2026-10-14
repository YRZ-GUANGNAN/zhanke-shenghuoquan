# 帖子排版与整体 UI 实施计划

**目标：** 在新分支保留现有蓝色色调和所有业务能力，完成帖子浏览链路与共享 UI 的排版、状态和操作反馈更新。

**架构：** Taro / React 页面继续调用原服务；纯展示模型处理真实附件、文字、时间。共享组件负责统一尺寸，页面负责数据权限和滚动状态。

**技术栈：** Taro 4.2、React 18、TypeScript、SCSS、Node 断言脚本。本会话直接执行，不新建子代理；未安装的 executing-plans 技能用当前逐项执行与检查替代。

## 1. 帖子流

- [ ] 修改 `apps/miniapp/src/pages/home/index.tsx` 和 `index.scss`：移除编号哈希高度；保留原聚合、接口分页、重试和开关。
- [ ] 用真实 `post.avatar` / `product.sellerAvatar`、附件数、话题/分类及价格；图片缺失使用正文，失败标记按 `target:id:imageUrl` 隔离。
- [ ] 加载与空态复用 `AppState`；不把系统状态绘成帖子。

## 2. 详情展示与交互

- [ ] 创建 `apps/miniapp/src/utils/post-presentation.ts`：图片校验保序、尺寸约束、显示日期与重复标题处理。
- [ ] 修改 `apps/miniapp/src/pages/post-detail/index.tsx` 和 `index.scss`：展示已有全部附件，纵向内容与横向图集独立；保留 `navigateBackOrHome()`。
- [ ] 将评论数点击变为评论锚点；输入有内容时显示明确发送按钮。保留幂等请求与真实审核结果，使用同步请求锁防止连续点击。
- [ ] 修改图片对应的旧静态断言，保留原无图/失败、权限、分页、返回和关注热区保障。

## 3. UI 一致性

- [ ] 复用现有共享导航、标题、按钮、状态样式；不修改已有颜色令牌值，也不改校园指南/咨询模块及其共享组件依赖。
- [ ] 对齐 `search/index.scss`、`my-content/index.scss`、`publish-post/index.scss` 的正文、列表边界、输入触控与安全区。

## 4. 验证与交付

- [ ] 执行展示模型行为测试，再执行既有页面回归、功能策略、分享、搜索、发布、UI 与胶囊检查。
- [ ] 执行 `npm.cmd --workspace apps/miniapp run typecheck` 与 `npm.cmd run build:miniapp`，预期退出码为 0。
- [ ] 在微信开发者工具读取实际页面状态并尝试截图；不提供假数据替代运行结果。
- [ ] 复查 diff，记录可查看结果及未实测项；保留原项目配置修改，不上传微信、不合并旧分支。
