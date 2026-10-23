# 个人主页融合背景实施计划

> **For agentic workers:** 使用当前会话逐项执行；不分派子代理。已确认的设计位于 `docs/superpowers/specs/2026-10-14-profile-cover-design.md`。

**Goal:** 将个人封面由独立横幅改为整块个人资料头部的背景，保持白蓝主题及全部业务不变。

**Architecture:** 复用现有真实封面字段、图片失败状态和编辑导航；照片层与资料层分离，使用共享浅色遮罩样式向白色页面融合。编辑区仅同步裁切预览与提示，不改上传保存。

**Tech Stack:** Taro / React / TypeScript / SCSS，现有 Node 行为测试及本地 Chrome 布局检查。

---

## 1. 行为约束回归

**文件：** `scripts/check-profile-presentation.cjs`

- [x] 添加实际页面断言：背景层不包含编辑按钮；资料与背景同属 hero；有效封面启用 `student-profile-hero--with-cover`；失效/清空封面取消该状态。
- [x] 执行 `npm.cmd run check:profile-presentation`，用 `5019af6` 的原页面进行内存中的基线验证，旧结构在新增断言上失败，当前实现通过。
- [x] 继续覆盖原身份切换、统计数据、功能开关、账号切换、头像失效和导航防重，不替换成静态伪页面。

## 2. 融合层与资料布局

**文件：** `apps/miniapp/src/pages/profile/index.tsx`、`index.scss`、`index.config.ts`；新增 `apps/miniapp/src/styles/profile-cover-surface.scss`。

- [x] 由同一 `hasCover = Boolean(profile.cover && profile.cover !== failedCover)` 控制真实图片及视觉状态。
- [x] 保持 `CapsuleSystemRow`，背景层绝对铺满 hero 且 `pointer-events: none`；标题和换封面入口同一行；资料层为头像与文字两列，编辑提示置于文字列下方。
- [x] 共享遮罩 mixin：上方浅色保证胶囊区可读、下方浅色保护文字、底边完全白色；不引入滤镜依赖或默认图。系统文字改黑色以匹配浅色安全区。
- [x] 昵称、用户号、学校保持单行省略；按钮至少 44px。用现有主题令牌，头像边框变细，移除负边距及独立横幅固定高度。
- [x] 下方内容入口、请求、角色守卫与业务开关不修改。

## 3. 编辑区同步

**文件：** `apps/miniapp/src/components/profile-cover-editor.tsx`、`profile-cover-editor.scss`。

- [x] 预览不再使用旧 750:280 横条；使用更接近头部的比例和同一个遮罩 mixin，空态文字在遮罩之上。
- [x] 默认说明改为“照片会铺在头像和资料背后，居中裁切，下沿自然融入页面。”不修改上传、审核、保存、清空或账号隔离。
- [x] 执行 `npm.cmd run check:profile-cover` 和 `npm.cmd run check:profile-center-ui`。

## 4. 验收与保存

**产物：** 本任务可视化目录 `profile-cover-fusion-{320,390,430}.png` 和临时布局脚本；不放入产品包。

- [x] 渲染真实 JSX / SCSS 的无图状态；检查 320/390/430px 宽度、长昵称、触控区、资料位于背景内、无横向溢出。照片分支只用明确标记的中性测试表面做几何/对比检查，不生成或引入假照片。
- [x] `npm.cmd run verify`、`npm.cmd run build:miniapp`、`git diff --check`。
- [x] 精确暂存本次文件并本地提交；两份既有配置不暂存，原项目不动；不推送、不部署、不上传微信。
- [x] 明确区分本地布局通过与真实照片/微信真机验收，附可查看的本地产物。

## 验证记录

- 以 `5019af6` 为本轮代码基线；原有两份小程序配置与 `9d485c3` 快照一致，未修改/暂存。原项目仍为 `e79e168`，仅原来的三份未跟踪计划。
- 实际页面行为检查 PASS，覆盖背景状态、失效/清空回退、共享编辑防连点及原统计/权限/身份行为；封面编辑上传/审核/保存回归 PASS。
- 基线检查最初在输出带 React 内部引用的 DOM 差异时触发 Node 内存错误；将断言改为布尔检查后，准确在“照片层只负责背景”上失败；当前版本 PASS。测试从 Git 读取旧版本到内存，没有回滚或覆盖工作区。
- `npm.cmd run verify` PASS：全部前端行为与边界检查、管理端/小程序类型检查和安全后端单元测试通过。初次类型检查发现可选封面字段未收窄，修正后完整复跑通过。
- 最终 `npm.cmd run build:miniapp` PASS，确认编译产物包含新的文字颜色、融合遮罩与黑色系统文字；仅有既存 `vendors.js` 731KiB 性能提示。`git diff --check` PASS。
- 9 组实际组件布局 PASS：320/390/430px × 无图/深色中性底/浅色中性底；覆盖 24/48px 状态栏、长昵称、标题安全区、44px 触控区、照片完整覆盖资料、失败状态不跳变。
- 对截图中的标题、封面入口、昵称、用户号、学校做 45 个对比度抽样。原次要文字对比度为 4.20，基于原令牌适度加深后最小值 4.91，深色测试底昵称为 7.48 以上。中性底不是照片，不能作为真实照片效果已经通过的证据。
- 可视化产物在本任务可视化目录：`profile-cover-fusion-{320,390,430}.png`、`profile-cover-fusion-*-{dark,light}-probe.png`、对应 HTML、`profile-cover-fusion-metrics.json`。不属于产品资源，不进入代码包；没有生成或借用照片。
- 未验收：真实个人照片的构图及实际微信渲染、手机选图上传/审核/跨设备读回。本次没有后端、数据库、线上配置、GitHub 推送或微信上传。
