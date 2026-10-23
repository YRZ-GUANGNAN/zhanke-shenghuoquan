# 默认封面切换与旧动效修正实施计划

**Goal:** 让后台明确选择默认分享封面来源，修复旧动效的重开跳变、双重旋转和减少动效遗漏。

**Architecture:** 沿用 ShareSettingsConsole 的真实接口和 CAS；封面来源与未保存地址只属于编辑草稿，不增加服务端字段。共享弹层保持 enter/open/exit/closed 状态机，只调整尚未卸载的反转。

**Tech Stack:** React、Taro、TypeScript、SCSS/CSS、现有 Node 行为检查与微信开发者工具。

用户已授权自主完成，本会话直接执行，不另行委派。已有技能目录不包含 executing-plans，按以下任务逐项执行并记录验证。

## 1. 封面来源切换

- [x] 在 `apps/admin/src/ShareSettingsConsole.tsx` 增加 `CoverMode = 'builtin' | 'custom'` 与草稿中的自定义地址；读取服务器时从 `defaultImageUrl` 推导模式。切换内置提交空值，切换自定义恢复草稿地址。
- [x] 自定义模式地址为空返回明确错误；保持合法域名、图片后缀、标题、真实图片优先及 CAS 校验。
- [x] 在 `apps/admin/src/share-settings.css` 沿用现有字号/颜色，提供两项原生 radio，至少 44px 操作区、选中和键盘焦点反馈，小屏换行。
- [x] 扩展 `scripts/check-admin-share-default.cjs`，验证修改地址到保存的真实 payload、切换后保留输入、空自定义不能保存、恢复内置使用空值和正确版本。

## 2. 弹层快速反转

- [x] `scripts/check-layer-presence.cjs` 在 `visible: false → true` 后、下一绘制帧之前断言 `phase === 'open'`，先确认旧实现失败。
- [x] `apps/miniapp/src/hooks/use-layer-presence.ts` 记录退场期；未卸载时反转到 open，不重新进入 enter，清理旧定时器；首次打开和卸载后打开保持双帧。
- [x] `apps/miniapp/src/components/ui/app-dialog.tsx` 的交互条件改为 `visible && phase !== 'exit'`。
- [x] 运行 `npm.cmd run check:layer-presence` 和 `npm.cmd run check:content-motion`，确认快速关闭、重开、折叠和点赞行为。

## 3. 旧样式清理

- [x] `apps/admin/src/styles.css` 去除 legacy 加载父标记旋转，保留 completion 内部圆环。减少动效明确停止动画及过渡，覆盖伪元素。
- [x] `apps/miniapp/src/components/ui/app-icon-button.scss`、`app-button.scss`、`pages/edit-profile/index.scss` 和 `pages/settings/index.scss` 补齐减少动效；设置开关只取消过渡，不清除表示开关状态的平移。
- [x] 从公共图标按钮、`app-dialog.scss` 和 `app-action-sheet.scss` 去掉常驻 will-change，不改现有过渡时长、视觉颜色和功能。

## 4. 验证与交付

- [x] 运行分享、刷新、图集、频道横滑、边缘返回、触控与两端 UI 检查，检查相关行为没有回归。
- [x] 运行 `npm.cmd run typecheck`、`npm.cmd run build:admin` 与 `npm.cmd run build:miniapp`；检查产物为正常包，不混入独立检查页。
- [ ] 用隔离检查工程检查真实组件可渲染。已准备 `feedback-harness-20261017` 的面板场景并尝试连接开发者工具，但自动化通道在重载后超时，不能以此替代原生人工检查；待后续开发者工具/真机复核。
- [x] 检查工作区只包含本轮改动与原有未跟踪产物，不操作原项目或线上数据。
