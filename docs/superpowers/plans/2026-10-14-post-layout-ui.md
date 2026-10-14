# 帖子排版与整体 UI 实施计划

**目标：** 在新分支保留现有蓝色色调和所有业务能力，完成帖子浏览链路与共享 UI 的排版、状态和操作反馈更新。

**架构：** Taro / React 页面继续调用原服务；纯展示模型处理真实附件、文字、时间。共享组件负责统一尺寸，页面负责数据权限和滚动状态。

**技术栈：** Taro 4.2、React 18、TypeScript、SCSS、Node 断言脚本。本会话直接执行，不新建子代理；未安装的 executing-plans 技能用当前逐项执行与检查替代。

## 1. 帖子流

- [x] 修改 `apps/miniapp/src/pages/home/index.tsx` 和 `index.scss`：移除编号哈希高度；保留原聚合、接口分页、重试和开关。
- [x] 用真实 `post.avatar` / `product.sellerAvatar`、附件数、话题/分类及价格；图片缺失使用正文，失败标记按 `target:id:imageUrl` 隔离。
- [x] 加载与空态复用 `AppState`；不把系统状态绘成帖子。

## 2. 详情展示与交互

- [x] 创建 `apps/miniapp/src/utils/post-presentation.ts`：图片校验保序、尺寸约束、显示日期与重复标题处理。
- [x] 修改 `apps/miniapp/src/pages/post-detail/index.tsx` 和 `index.scss`：展示已有全部附件，纵向内容与横向图集独立；保留 `navigateBackOrHome()`。
- [x] 将评论数点击变为评论锚点；输入有内容时显示明确发送按钮。保留幂等请求与真实审核结果，使用同步请求锁防止连续点击。
- [x] 修改图片对应的旧静态断言，保留原无图/失败、权限、分页、返回和关注热区保障。

## 3. UI 一致性

- [x] 复用现有共享导航、标题、按钮、状态样式；不修改已有颜色令牌值，也不改校园指南/咨询模块及其共享组件依赖。
- [x] 对齐 `search/index.scss`、`my-content/index.scss`、`publish-post/index.scss` 的正文、列表边界、输入触控与安全区。

## 4. 验证与交付

- [x] 执行展示模型行为测试，再执行既有页面回归、功能策略、分享、搜索、发布、UI 与胶囊检查。
- [x] 执行 `npm.cmd --workspace apps/miniapp run typecheck` 与 `npm.cmd run build:miniapp`，预期退出码为 0。
- [x] 在微信开发者工具读取实际页面状态并尝试截图；实际为校园指南模式，帖子页面视觉验收待可用真实会话，不提供假数据替代运行结果。
- [x] 复查 diff，记录可查看结果及未实测项；保留原项目配置修改，不上传微信、不合并旧分支。

## 5. 第二轮细节与行为复查

首轮实现已提交 `994251b`。下面仅增量修改现有页面和验证脚本，咨询文件、共享组件、服务契约、项目配置均不修改。

- [x] `utils/post-presentation.ts`：增加按附件身份移除失败图片的纯函数；当前图仍存在时保留它，失败当前图时选邻图，最后一张失败时回到空集合。扩展 `scripts/check-post-presentation.cjs` 验证首/中/末图失败及迟到的重复错误。
- [x] `post-detail/index.tsx`：统一图集项目与索引状态；评论首屏、分页及重试独立加载，并在重新读取/卸载时作废旧响应。`index.scss` 扣除底栏真实占高，长计数和作者不挤出屏幕。
- [x] `search/index.tsx`：同步请求锁、查询版本、推荐态退出清理；跳转成功后才记录本会话已打开的帖子。使用已有 `images` 的真实缩略图，错误仍保留缩略图容器。`index.scss` 控制图文比例、两行标题/摘要和元数据截断。
- [x] `publish-post/index.tsx`：上传/提交同步锁，固定提交期间标签与输入状态，错误保留在编辑区。`index.scss` 附件与添加入口对齐、规范间距与禁用反馈。
- [x] 用页面行为测试检查搜索乱序/快速分页、评论失败重试不重载正文、发布连续点击不重复提交。执行 `npm.cmd run check:post-presentation`、`npm.cmd run check:campus-info`、`npm.cmd run verify`、`npm.cmd run build:miniapp`。
- [x] 核对咨询及共享文件与基线一致；检查本次差异，定向暂存并提交。可视化只用真实运行页面，无法真机验收的情况明确标记。

## 验证记录与限制

- 展示逻辑：24 项断言通过；页面行为测试使用真实 React hooks 和隔离服务替身，覆盖查询乱序、同步防重、列表节点保留、分页错误位置、评论独立重试与定位、提交期间编辑锁定及失败重试幂等。
- `verify` 全部通过，包含 TypeScript、路由、分享返回、发布搜索契约、边界与后端安全测试。`check:campus-info` 和微信构建通过。
- 构建保留既有 `vendors.js` 约 731 KiB 和 `NoAsyncChunksWarning` 提示。
- 开发者工具实际首页为校园指南模式；保留截图 `C:/Users/33141/.codex/visualizations/2026/07/28/019fa89e-0db5-7d30-bbae-fcf9d7f9fcc7/post-layout-20261014-round2-runtime.png`。它仅证明实际模式，不是本轮帖子 UI 的展示图。自动化跳转没有完成可用的帖子页验收，没有修改线上开关、账号或登录状态。
- 待真机验收：320/375/430 逻辑宽度、软键盘和底部安全区、搜索返回的像素位置、多图失败后当前图的原生翻页表现。上述场景的代码/状态测试不能替代实际视觉与触控验收。
