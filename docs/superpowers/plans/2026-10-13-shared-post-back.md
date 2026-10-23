# 分享帖子返回修复实施计划

> 在当前隔离工作区内执行，不新增子代理。用户已要求自主完成修复，不重复请求设计批准。

**目标：**分享直达帖子后能返回首页，站内打开仍返回上一页。

**架构：**小程序导航工具先调用 `navigateBack({ delta: 1 })`，失败后 `reLaunch` 首页；最终失败提示重试。帖子页面只替换返回按钮的处理函数。

**技术栈：**Taro 4、React、TypeScript、Node.js 行为回归测试、微信开发工具自动化。

## 执行步骤

- [x] 在 `scripts/check-miniapp-back-navigation.cjs` 增加行为测试：真实导入导航实现，用隔离的页面栈替身验证单页、站内返回、失败兜底、失败提示及再次重试，不访问生产接口。先运行得到失败，确认能捕获缺失修复。
- [x] 新增 `apps/miniapp/src/utils/navigation.ts`，导出 `navigateBackOrHome(): Promise<void>`；嵌套捕获返回失败及首页失败，最终提示“暂时无法返回，请重试”。提示异常收敛，不创建额外导航循环。
- [x] `apps/miniapp/src/pages/post-detail/index.tsx` 导入函数，将顶部 `onClick` 改为 `() => void navigateBackOrHome()`，保留所有现有业务处理。
- [x] 将导航检查注册为 `check:miniapp-back-navigation`，加入 `verify`。运行导航检查、`check:share-cards`、`check:miniapp-regression-fixes`、`check:miniapp-static-routes`、`check:boundaries` 和小程序类型检查。
- [x] 运行 `npm.cmd run build:miniapp`，重新加载当前开发工具项目；用现有真实帖子验证分享直达返回首页以及站内进入返回上一页。页面选择器、后续截图接口超时，改用自动化工具读取真实 Taro 渲染节点并触发其原生 `tap` 事件处理函数，回读页面栈验证。已保存修复前页面截图及文字证据；未修改生产开关或身份。
- [x] 复核差异、保留两个既有项目配置修改，记录交付状态。已上传开发版本 `2026.10.13.3`，备注“优化体验”，微信后台回读确认；未提交审核或发布。验证及产物记录在 `docs/operations/2026-10-13-shared-post-back-fix.md`。

## 自审

验收映射到单页、站内、失败恢复及真实开发工具验证；只修改导航相关文件，无后端变更。项目没有可用的 `superpowers:executing-plans` 技能，按本计划在当前任务直接执行。
