# 商家与骑手身份申请界面自审记录

本轮在 `codex/delivery-workspace-redesign-20261018` 的独立工作树完成，范围是商家/骑手身份申请入口弹窗、申请详情页和对应展示验证。后端、身份 API、媒体隐私合同、权限守卫和人工审核状态机保持不变；学生端其他进行中的工作树未写入。

## 这次改动

- “我的”里的 `missing`、`pending`、`rejected` 申请入口改为状态摘要弹窗，显示角色、状态、申请编号、时间和下一步动作；`approved` 继续查看详情，身份可用、禁用、暂停和休息状态仍走原有守卫。
- 详情页改成状态头部、审核反馈、本次申请进度、可编辑/只读资料、证明网格和安全区底部操作栏。商家地址使用多行字段，提交中的字段和上传操作会锁定。
- 加入统一的骨架、三点等待、失败恢复、长文案换行和私密证明占位。历史 `assetIds` 不拼接公开 URL，提交前继续检查 `purpose === 'identity_application' && isMediaUsable(status)`，上传完成后仍由管理员人工核验。
- 详情页输入框内部补足 44px 触控高度；预览桥接只替换真实 `view`/`text`/`image` 标签选择器，避免误改 `.app-button--text` 等类名。

## 验证结果

- `node scripts/test-identity-application-ui.cjs`：两角色四状态、失败/导航重试、重复点击、隐藏/卸载/换号迟到响应、只读资料、拒绝重提、上传提交、409、刷新失败保留资料、取消返回恢复全部通过。
- `node scripts/check-identity-application-contract.cjs`、`node scripts/check-identity-application-ui.cjs`、`node scripts/check-profile-center-ui.cjs`、`node scripts/check-profile-presentation.cjs`、`node scripts/check-miniapp-ui-system.cjs`、`node scripts/check-reference-layout.cjs`、`node scripts/check-feature-policy.cjs`、`node scripts/check-feature-gate-feedback.cjs`、`node scripts/check-miniapp-capsule-layout.cjs` 全部通过。
- `npm.cmd --workspace apps/miniapp run typecheck` 通过。
- `npm.cmd run build:miniapp` 退出码为 0。日志仍有现有 `mini-css-extract-plugin` 公共样式顺序提示和 `vendors.js` 约 732 KiB 的建议，不影响构建产物。
- 独立预览共 106 组：商家/骑手、入口/详情、320/390/430px，覆盖加载、失败、审核中、已通过、拒绝、上传中、上传失败、提交中、刷新失败、长审核意见、放大文字和减少动效。`metrics.json` 中触控尺寸、横向溢出、弹窗居中、角色图标中心、底栏固定和内容可达性问题为 0。

## 审核边界

预览使用真实 TSX/SCSS 与隔离服务夹具，按钮不会调用真实业务；浏览器预览不能替代微信开发者工具或真机对键盘、安全区、字体倍率、原生滚动阻尼和真实账号审核链路的验收。本轮未上传、部署、送审、发布或合并学生端改动。

预览入口：`C:/Users/33141/.codex/visualizations/2026/10/18/01a14d4a-5356-7931-ab05-d68af7d7f48d/identity-application-refresh/index.html`
