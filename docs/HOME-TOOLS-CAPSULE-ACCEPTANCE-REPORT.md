# 首页工具胶囊验收报告

- 验收日期：2026-07-19
- 验收基线：`9b0199a`
- 旧入口清理提交：`9b101b4`
- 本次范围：只清理旧工具入口并验证现有首页工具胶囊，不进行 UI 美化，不修改 `apps/miniapp/src/app.config.ts`

## 入口清理结果

| 检查项 | 结果 | 说明 |
| --- | --- | --- |
| 注册路由唯一性 | PASS | `app.config.ts` 的 24 个注册路由中不存在 `pages/tools/index`；首页的“工具”页签是当前唯一可达入口。 |
| 旧工具页面 | PASS | 已删除未注册的 `pages/tools/index.tsx`、`index.scss` 和 `index.config.ts`，避免留下孤立页面。 |
| 旧底栏失效入口 | PASS | 已从 legacy `BottomNav` 移除指向 `/pages/tools/index` 的工具项。 |
| 共享组件保留 | PASS | `components.tsx` 仍被旧 `market`、`takeout`、`product-detail` 源码引用，因此保留 `BottomNav` 和 `AvatarMark`，未做扩大删除。 |
| 当前学生底栏 | PASS | `StudentBottomNav` 仍只有首页、外卖、二手、我的，没有第二个工具入口。 |
| 运行时残留引用 | PASS | `apps/miniapp/src` 与 `scripts` 中不存在 `/pages/tools/index` 或 `<BottomNav active='tools' />`。 |

## 自动验收

| 命令 | 结果 | 证据 |
| --- | --- | --- |
| `npm.cmd run check:miniapp-static-routes` | PASS | `miniapp static route check PASS (24 routes)` |
| `npm.cmd run check:miniapp-business-closure` | PASS | `miniapp business closure check PASS (24 registered routes)` |
| `npm.cmd run check:home-real-tools-contract` | PASS | `home real tools contract: ok` |
| `npm.cmd --prefix apps/miniapp run typecheck` | PASS | `tsc --noEmit` 退出码为 0 |
| `npm.cmd --prefix apps/miniapp run build:weapp` | PASS | Taro/webpack 编译成功，耗时约 15.17 秒 |

构建仍报告既有非阻断警告：`vendors.js` 约 731 KiB，以及 `NoAsyncChunksWarning`。本任务没有调整拆包或界面结构。

## 真机与真实后台验收

以下项目无法由静态检查或本机构建替代，必须保持为 `PENDING`：

| 场景 | 状态 | 验收要求 |
| --- | --- | --- |
| 微信胶囊在不同机型的避让 | PENDING | 至少使用 iPhone 12/13 尺寸和一台窄屏 Android 真机，确认顶部页签、搜索按钮与微信胶囊不重叠。 |
| 真实后台工具数据 | PENDING | 后台只返回占位或缺少 `appId/path` 时不展示伪工具；配置真实目标后应显示真实工具。 |
| 真实小程序跳转 | PENDING | 使用真实 `appId` 和 `path` 验证 `navigateToMiniProgram` 成功、失败提示及返回后的状态。 |
| Hero 上半区交互 | PENDING | 真机验证可点击区域、连续点击只触发一次跳转、失败后可重试、返回首页后可再次进入。 |
| 真机视觉与手感 | PENDING | 需要真实设备截图和人工体验确认；本报告不把代码 PASS 误写为真机 PASS。 |

## 结论

旧工具页面和失效底栏入口已安全移除，首页工具胶囊成为唯一可达入口；静态路由、业务闭环、首页真实工具契约、类型检查与微信构建均通过。真实后台跳转和真机体验仍为 `PENDING`，因此当前结论是“代码与本地构建验收通过”，不是“发布验收完成”。
