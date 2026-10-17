# 公开主页边缘返回 Implementation Plan

> 在现有隔离分支按用户已给出的方向实施；本任务内联完成，不派生代理、不重复请求批准。

**Goal:** 当前公开主页支持左边缘右滑返回，并保留帖子／二手横滑、透明返回键和已有导航兜底。

**Architecture:** 纯状态工具识别触摸；页面 Hook 负责生命周期、系统手势归属和返回；透明边缘作为 Swiper 的兄弟节点防止同一手势切栏目。iOS 有页面栈时系统拥有返回，其余环境使用应用手势。菜单／对话框开启时移除边缘节点。

**Tech Stack:** Taro、React、SCSS；现有 Node/JSDOM 行为测试、微信构建和本地真实组件预览。

## 实施与验收

- [x] 新建 `apps/miniapp/src/utils/edge-back.ts`：`createEdgeBack` 返回 `start/move/end/cancel`；起点 0–24px，方向容差 8px，右移需大于纵移 1.5 倍，完成距离 `Math.min(120, Math.max(72, width * .24))`；触点、方向或生命周期无效立即取消。
- [x] 新建 `apps/miniapp/src/hooks/use-edge-back.ts`：检测 iOS 已有返回栈时使用系统手势；应用手势调用 `navigateBackOrHome`；处理禁用、hide/show、resize、卸载和手势结束清理。
- [x] 修改 `apps/miniapp/src/pages/user-profile/index.tsx` 与 `index.scss`：加载、失败、成功状态均接入；左边缘透明触摸区位于 Swiper 外；箭头跟手最多 12px，取消归位；菜单／确认框期间禁用。保留 44px 返回触控区及所有业务处理。
- [x] 新增 `scripts/check-profile-edge-back.cjs` 并加入 `preverify`：覆盖短滑、反向、多指、重复事件、无栈兜底、iOS 系统归属、弹窗与生命周期取消；补充已有页面测试的真实 Hook 依赖。
- [x] 运行 `npm run check:profile-presentation`、`npm run check:profile-edge-back`、`npm run check:miniapp-back-navigation`、`npm run check:channel-pager`、`npm run check:reference-layout`、小程序 TypeScript 与 `npm run build:miniapp`，全部通过；构建仅保留既有 vendors 体积建议。
- [x] 更新 5197 真实组件预览与本地触摸验证：短滑、纵向滑、中央横滑、标题栏短滑和取消均留在主页；菜单不穿透；左边缘触摸和鼠标拖动均可回到二手页。预览仅使用原有 fixture，未进入生产包。
