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

## 6. 二手页面同步更新

- [x] `student-secondhand/index.tsx` / `index.scss`：规则两列商品网格，真实图片、价格、卖家/分类元数据；保持分类请求隔离，返回静默刷新已加载页，保留同一滚动容器与已显示内容。
- [x] `secondhand-detail/index.tsx` / `index.scss`：复用纯媒体失败处理，固定比例完整图集，分开商品读取与个人收藏状态读取；同步锁保护收藏、联系、状态变更；底栏占据真实布局空间。
- [x] `secondhand-detail/product-comments.tsx` / `.scss`：请求版本与读写锁，失败不清空已有留言，定位包含回复的目标留言，锁定提交快照，审核文案沿用服务器结果。
- [x] `publish-secondhand/index.tsx` / `index.scss`：展示全部已上传附件并标记封面，保留移除封面操作；上传和发布锁，禁用提交中编辑，失败内联提示与安全重试；字段和底栏窄屏适配。
- [x] 增加 `scripts/check-secondhand-presentation.cjs` 真实 React hooks 行为测试：分类乱序、返回保留节点、分页去重、失败图索引、留言读写冲突/迟到响应、发布快照与幂等重试；无网络及生产写入。
- [x] 增加 `scripts/check-gallery-presentation.cjs`：覆盖 320/375/430 逻辑宽度、短窗口、横屏、安全区、商品正方形媒体，以及页码 200ms 淡入/5000ms 保持/200ms 淡出所需的清理与迟到回调隔离。
- [x] 执行 `npm.cmd run check:secondhand-presentation`、`npm.cmd run check:post-presentation`、`npm.cmd run check:gallery-presentation`、`npm.cmd run check:campus-info`、`npm.cmd run check:feature-policy`、`npm.cmd run verify`、`npm.cmd run build:miniapp`，均退出 0。三个行为检查已接入 `preverify`，后续完整验证自动执行。

## 7. 依据最新版手册的追加边界

已核对资料库最新提交 `55edc38`；本轮采用前序 `67d8805`（`补充模拟器适配对照与离线开发规则`）的 D01、D02、D05、D07、D10：图集横向分页和正文滚动分离；媒体高度只使用逻辑窗口、附件比例夹取与安全区，不把截图像素当设备常量；页码计时器在交互、隐藏和销毁时清理；业务状态和动画状态分离并以内容 key 隔离迟到结果；返回继续保留原路由和内容状态。`55edc38` 仅补外部资料筛选及 GH01/GH02 待核实线索，不提供新增实测参数，不把第三方仿写代码/素材或猜测的持久化/重试机制接入产品。

D03 的社区双列原则只用于原社区帖子流；二手坚持规则两列商品网格，不能混用瀑布流。D04 的进入/退出时长属于参考入口观察，本项目继续交给微信原生路由，不新增自定义转场。D06 的视频抽屉不适用本项目现有图文评论；D08/D09 不新增筛选或复制参考产品排序不同步现象。模拟器 E 系列仅作为尺寸和状态边界依据，16 项真机补测仍标记为待执行。

## 本轮最终检查

- 新增行为测试还覆盖首页栏目返回滚动恢复、切栏目不重载、无关开关不刷新、关闭来源时旧分页不回写，以及发帖/商品已创建但跳转失败时只重试导航。图集 hook 在后台媒体到达、再次显示、窗口改变、降为单图及卸载时均有独立测试。
- 完整 `verify`（含类型、真实服务契约、安全后端测试）和微信构建通过；没有新增 CSS 顺序警告，仅有既有 vendors 约 731 KiB 和 NoAsyncChunksWarning。二手 SCSS 已按项目多行风格机械格式化，并逐文件比较格式化前后编译 CSS 完全相同。
- 相对 `323f894`：校园指南/咨询、共享标题/导航/UI 状态组件、色彩令牌、身份守卫、路由、社区/二手 API 服务和后端零差异。`services/taro.ts` 与 `taro-shim.d.ts` 仅补类型，不改服务运行逻辑。
- 原项目仍为 `codex/rider-refresh-home-feed` / `e79e168`，三个既有未跟踪计划文档保留。隔离工作区两个既有项目配置改动保留且不纳入本轮提交。
- 本轮没有发布、推送或改线上开关；微信开发者工具未完成可用的帖子/二手真实页面验收，故不宣称新 UI 已经真机验收。后续需在已有合法功能配置及真实账号/内容下核对视觉、软键盘、安全区与返回锚点。
