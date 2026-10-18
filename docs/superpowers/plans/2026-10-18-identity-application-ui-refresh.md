# 商家骑手身份申请改版 Implementation Plan

**Goal:** 落地已批准的状态摘要弹窗与申请详情改版，并验证权限、状态恢复和窄屏布局。

**Architecture:** 原 API 和媒体链路继续使用；增加申请专属展示组件和入口弹窗。“我的”只改身份入口分流，详情页保留同步锁和取消恢复。所有文件写入当前配送工作树，在本会话逐步实现和验证。

**Tech Stack:** Taro 4、React 18、TypeScript、SCSS、Node.js 隔离组件行为测试。

## Task 1: 申请摘要与入口

- [x] 新建 `components/identity-application-summary.tsx/.scss`，统一角色、空申请/pending/approved/rejected 文案、固定画布角色图标、三点等待与北京时间。仅处理已有申请数据。
- [x] 新建 `components/IdentityApplicationEntryModal.tsx/.scss`，使用 `AppDialog`；404 为开始申请，pending 为查看详情，rejected 为修改资料，approved 为查看详情，其他读取错误为重试。使用同步请求/导航锁和会话校验，关闭/隐藏/卸载后旧结果失效；长审核原因在原生滚动容器内展示。
- [x] `pages/profile/index.tsx` 仅接入上述弹窗：missing/pending/rejected 可查看申请，available 继续原身份切换，disabled/suspended/resting 不绕过现有规则。保留身份申请功能开关和原路由。

## Task 2: 申请详情

- [x] `pages/identity-application/index.tsx` 用状态头部、本次进度、分区表单/只读资料、证明网格和安全区操作栏替代旧布局。
- [x] 姓名/电话/商家信息使用 `AppField`，只读模式使用完整文本而非禁用输入框；地址用多行字段。校验错误留在字段附近，提交/上传时锁定编辑。
- [x] 复用 `AppSkeleton`、`AppState`、`AppNotice` 和 `AppButton`，仅在申请作用域内调整样式。动作等待显示三点和准确动词，取消、上传部分失败、409 刷新保留现有语义。
- [x] 校验账号隔离及同步请求锁，详情刷新失败保留已读取内容；失败或未就绪时不允许写入；历史私密证明不解析公开 URL。
- [x] 重写局部 SCSS：44PX 点击目标、固定图标尺寸、长文本换行、320px 布局、底部安全区、减少动效。

## Task 3: 行为与合同验证

- [x] 新建 `scripts/test-identity-application-ui.cjs`，复用现有隔离 hook renderer 加载真实组件，覆盖两角色、404/失败重试、重复点击、导航失败、迟到响应、账号变化、pending/approved 只读、拒绝重提版本、私有证明、媒体人工核验合同和取消导航恢复。所有服务均为隔离夹具。
- [x] 新建 `scripts/check-identity-application-ui.cjs`，检查局部作用域、安全区和权限入口；加入现有 `verify`。只更新受入口承载变化影响的旧合同断言，不删除业务验证。
- [x] 执行 `node scripts/test-identity-application-ui.cjs`、`node scripts/check-identity-application-contract.cjs`、`node scripts/check-identity-application-ui.cjs`、`npm.cmd --workspace apps/miniapp run typecheck`。预期全部退出码 0。

## Task 4: 预览与交付

- [x] 在独立可视化目录中渲染真实 TSX/SCSS，生成 320/390/430px 弹窗与页面，覆盖空申请、加载、错误、审核中、已通过、拒绝长文案、上传和失败。检查滚动内容、按钮触控尺寸、图标居中、字体放大及减少动效。
- [x] 执行身份相关、个人中心、功能开关、胶囊与 UI 合同以及 MiniApp 构建。构建警告与真机未验证项如实记录。
- [x] `git diff --check`，确认学生端工作树、后端、服务和全局样式未写入；记录本轮唯一的学生页面调整是隔离分支上的申请入口接线。
- [x] 写入 `docs/reviews/2026-10-18-identity-application-ui-refresh.md`，按明确文件列表提交代码，打开独立预览。完成边界为本地代码与预览，不上传、发布、部署或合并。

## 自审

用户接口不提供审核历史，所以进度仅展示本次提交及结果。证明材料沿用最新人工核验合同。新测试验证真实组件行为，不复制状态机。已获用户继续实施授权，按本计划执行。
