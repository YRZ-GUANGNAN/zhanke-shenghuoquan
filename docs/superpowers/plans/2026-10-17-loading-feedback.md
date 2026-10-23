# 加载与刷新反馈实施计划

> 用户已授权自主完成，本会话执行，不派代理。遵循现有正常版代码和审批边界。

**Goal:** 改善现有页面加载、下拉刷新、登录、失败图标与重试可靠性。

**Architecture:** 共享 StatusGlyph/AppState/AppSkeleton 呈现状态；RefreshScrollView 只管理原生下拉反馈、生命周期与去重，业务请求继续由页面负责。首页/二手返回真实刷新结果，不成功时保留已加载内容。原有会话和功能权限逻辑不改。

**Tech Stack:** Taro、React、TypeScript、SCSS、现有本地 SVG、Node 行为检查、官方微信开发者工具。

- [x] `components/ui/status-glyph.tsx/.scss`、`assets/icons/status-*.svg`：统一加载、登录、断网、失败、权限、空态和成功符号；`app-state` 接入，减少动效时静止。
- [x] `app-skeleton` 增加 `feed` 双列骨架；`runtime-session-gate` 使用登录符号、网络失败分型，无登录逻辑改动。
- [x] `components/refresh-scroll-view.tsx/.scss`：原生 refresher，阈值 60px；请求只运行一次，成功/失败持留后释放，原生 abort 不伪造请求取消；卸载/频道关闭后忽略旧回调。
- [x] `pages/home/index.tsx`：发现/工具/私信接入刷新；保留原双数据源、分页和权限。显式 reset 保留旧卡片至成功替换；失败源保留数据和游标；重试后仍正确分页。`student-secondhand/content.tsx` 复用刷新函数、分类与阅读位置。
- [x] `feature-page-gate` 接入共享状态和可重试读取提示；权限判定表达式保持不变。`student-takeout` 的加载、失败、空态统一到共享组件。
- [x] 行为测试检查刷新并发、成功失败、卸载、关闭再打开、旧响应、失败保留内容；更新现有 test loader 支持新增 UI 依赖。
- [x] 分享统一内置默认 PNG，后台预览默认图并提供恢复默认；审核、路由、CAS 和权限不变。
- [x] 工具更新分组、用途层级和打开状态；后台补齐既有字段与目标校验；线上只读核查和后端导航测试。
- [x] 后台 typecheck/build、分享回退、真实工具数据流与导航权限检查。
- [x] 运行相关 `check:*`、小程序 typecheck、正常构建、`git diff --check`；在独立产物上检查微信渲染，记录未完成真机项。
- [x] 仅显式暂存本轮文件，中文提交；报告变化、验证、剩余问题，未经本轮上传要求不覆盖微信开发包。
