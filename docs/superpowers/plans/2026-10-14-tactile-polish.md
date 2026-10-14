# 小程序轻回弹与交互稳定性实施计划

> **For agentic workers:** 使用本任务内代理分工，主代理逐项审阅。subagent-driven-development/executing-plans 技能当前不可用，采用原生协作与检查点等价执行。继承已确认的自主实施范围，不另开用户任务。

**Goal:** 不增加业务能力，完善导航触控、输入避让、弹层收放和屏幕变化的稳定表现。

**Architecture:** 沿用现有 Taro 页面、CSS 令牌与服务；交互辅助模块只维护呈现状态，不调用新业务接口。参考库最新提交 5278dd8 的第七轮证据只用于状态边界：不新增自定义拖拽阈值、弹簧公式或全页转场。并行文件所有权互斥，最后由主代理统一测试并显式提交文件。

**Tech Stack:** Taro/React/TypeScript/SCSS，Node + React DOM/jsdom 行为测试，Sass/PostCSS 与浏览器几何检查。

## 1. 导航触控

文件：components/student-bottom-nav.tsx、student-bottom-nav.scss；scripts/check-tactile-navigation.cjs。不修改其他组件或 package.json。

- [ ] 新增实际组件测试：同帧点击发布两次仅调用一次 navigateTo；reject 后可重试；useDidShow 解锁；active tab 不导航；关闭服务/二手/发布时入口保持策略映射。
- [ ] 实施内部轻反馈，外部 min-width/min-height 44PX，保持旧蓝色；只使用约 120ms opacity/scale，不移动点击区；用 useRef 同步锁并捕获错误，保留全部现有 URL。
- [ ] `node scripts/check-tactile-navigation.cjs` 通过，并报告构建后 320/390/430 的点击区边界。

## 2. 键盘与输入

文件：pages/secondhand-chat、pages/delivery-conversation、pages/post-detail/index.tsx/index.scss（仅输入生命周期）；必要新增 hooks/use-keyboard-inset.ts、utils/keyboard-inset.ts；scripts/check-keyboard-inset.cjs。其他脚本变更报主代理合并。

- [ ] 先编写输入/视口算式及组件行为测试，覆盖 0/280px、blur、hide/show、resize、非法高度和低高度窗口。
- [ ] 实施单一键盘避让源，保证 scroll 视口同步扣除遮挡并保留草稿；不改发送协议或权限，不双重使用自动与手动顶起；不改变现有返回路由层级。
- [ ] `node scripts/check-keyboard-inset.cjs` 与原评论/二手行为检查通过，记录真机尚未验收。

## 3. 弹层呈现

文件：components/ui/app-dialog.tsx/.scss、app-action-sheet.tsx/.scss、app-button.scss、app-icon-button.scss；hooks/use-layer-presence.ts；scripts/check-layer-presence.cjs。

- [ ] 先测试 visible=true→false 延后卸载、false→true 取消旧任务、卸载取消、loading 禁止关闭、同帧确认不重复。
- [ ] 实施进入/打开/退出的呈现生命周期：`useLayerPresence(visible)` 返回 `mounted` 与 `phase`，闭合期禁用所有命中，不更改回调签名。
- [ ] 按钮内部视觉反馈和退出动效只用 transform/opacity，减少动效覆盖；不复制参考样本的遮罩色值或阶段时长；`node scripts/check-layer-presence.cjs` 通过。

## 4. 窗口与可读性（主代理）

文件：hooks/use-viewport-refresh.ts、components/feature-page-gate.tsx（只接入视觉刷新，不改授权表达式）、styles/reference-layout.scss；scripts/check-viewport-refresh.cjs、check-reference-layout.cjs。

- [ ] 测试同一页窗口变化与前台恢复会重新渲染但不卸载业务页；相同尺寸不重复更新；清理后不回写。
- [ ] 页面 wrapper 接入刷新 hook，尺寸快照只含几何不含业务。原图集 hook 不变。reference-layout 的 type 按角色采用 `clamp()` 与比例行高；保留旧声明作为不支持 clamp 的回退。
- [ ] 测试标题/正文/评论/作者/辅助字号在 320–768 宽度存在明确上下限；咨询页不新增引用。

## 5. 汇总验证

- [ ] 把新检查纳入 preverify；保留已有图集生命周期检查，不增加自定义拖拽阈值；`npm.cmd run verify`、`npm.cmd run build:miniapp`、`git diff --check` 全通过。
- [ ] 实际组件截图与几何验收，检查按压/退出/窄屏状态；没有真机则明确 pending。
- [ ] 实现回执、原工作区核对、白名单暂存与中文提交。不部署、不上传，不提交两份用户 project 配置。
