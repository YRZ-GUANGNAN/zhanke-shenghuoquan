# 手势连续性自审实施计划

**Goal:** 解决横滑后切后台可能点不动、发布连点重复进入，以及减少动效遗漏。

**Architecture:** 原生 Swiper 继续处理横滑；现有交互保护对象增加生命周期清理。发布入口只加请求身份锁，返回复用现有 `navigateBackOrHome`。检查器保留真实组件，不修改业务模型。

**Tech Stack:** React、Taro、TypeScript、SCSS/CSS、现有隔离 Node 检查。

用户已授权自主优化，按 brainstorming 和 writing-plans 梳理设计与步骤后直接在当前会话执行；不委派，不重新询问已确定的方向。

## 1. 恢复当前页面的检查装配

- [x] `scripts/check-post-presentation.cjs` 装配 product-price、chat-avatar、RefreshScrollView、ToolDirectory；纯函数用实际源文件，恢复真实 ChannelPager，UI 替身补 AppSkeleton。
- [x] `scripts/check-miniapp-regression-contract.cjs` 验证首页使用 RefreshScrollView，以及该组件真实导入并渲染 ScrollView、透传 props；保留分页、滚动位置、会话隔离断言。
- [x] 运行 `npm.cmd run check:post-presentation`、`npm.cmd run check:miniapp-regression-fixes`。
- [x] 修正校园服务检查对已不用的加载图标的要求，改查实际 AppSkeleton、AppState、重试动作和分类空态，头像链路断言保留。

## 2. 横滑与生命周期

- [x] `scripts/check-channel-pager.cjs` 增加 useDidHide/useDidShow 真实 Hook 注册替身。先用滑动后隐藏/显示及漏发 touchend 的用例复现旧实现失败。
- [x] `apps/miniapp/src/components/channel-pager.tsx` 对隐藏和卸载执行清理，前台恢复后释放旧点击锁；隐藏期事件无效，旧 change 不能切换栏目，新触摸重置事件归属。保持原生时长/缓动、子树保活。
- [x] 运行频道、帖子、个人主页相关行为检查，确认既有滚动和导航不变。

## 3. 发布与减少动效

- [x] `scripts/check-tactile-navigation.cjs` 扩展实际发布页的连点/失败/返回生命周期检查，先复现重复进入。
- [x] `apps/miniapp/src/pages/publish-entry/index.tsx` 使用请求对象 ref 防重、同步/异步失败释放当前锁并提示；隐藏后旧回执不影响新入口；返回复用现有安全返回函数。
- [x] `apps/miniapp/src/pages/publish-entry/index.scss` 为 reduced-motion 关闭过渡与 pressed 缩放；`apps/admin/src/styles.css` 两个旧层减少动效块直接禁用 animation/transition，并取消按钮装饰位移但保留开关状态平移。
- [x] 运行触控导航/发布检查，编译检查所改 SCSS/CSS；补测同步失败重试和打开编辑页时返回互斥。

## 4. 验证与交付

- [x] 执行 `preverify`、项目 `verify` 主脚本及剩余检查、类型检查、两端构建。`preverify` 已通过；本轮修改后相关频道/发布/帖子/个人主页检查分别重跑通过。`verify` 除下述基线分层问题外均通过，不宣称整链全绿。
- [ ] 基线已有的 `check:boundaries` 失败：`backend/internal/module/delivery/external_intake.go` 在 repository 之外持有数据库并执行查询。源文件与检查器均与 `bde6a01` 相同，保留后续后端分层整理，本轮不修改接口或后端文件。
- [x] 两端构建后的产物核验：30 个业务页面，无检查页，API 默认地址不变；发布和后台减少动效规则已进入产物。原项目仍为 `e79e168`，仅保留原有三个未跟踪计划。
- [x] `git diff --check` 通过；只将本轮源文件、检查和说明纳入本地提交，保留旧产物和其他计划，不推送、不部署。
