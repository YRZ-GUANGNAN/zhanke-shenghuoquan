# 公开主页透明返回栏 Implementation Plan

> 按用户已明确给出的截图与自主实施授权，在当前隔离分支完成；不新增代理或重复审批。

**Goal:** 顶部仅保留透明底的返回箭头，封面清楚呈现。

**Architecture:** 沿用现有公开主页、滚动与返回逻辑。去掉导航行中的昵称及更多按钮；封面状态使用白色箭头，其余状态使用现有深色。更多操作移至资料区，保留复制、举报、拉黑功能。

**Tech Stack:** Taro、React、SCSS；现有组件行为检查、生产构建和本地真实组件预览。

## 实施

- [x] 修改 `apps/miniapp/src/pages/user-profile/index.tsx` 的 navigation，仅渲染返回按钮；按 `status === 'ready' && hasCover && !scrolledTabs[activeTab]` 设置封面态类名。原更多按钮移至 `.public-profile-user__main`，使用已有 `AppIcon name='more'` 与滑动防误触。
- [x] 修改同页 `index.scss`，返回按钮 `background: transparent`，保留 44px 触控范围；箭头 `border-color: currentColor`，封面态白色并使用轻阴影，其他状态深色；删除旧顶部昵称和更多按钮样式。
- [x] 用 `npm run check:profile-presentation`、`npm run check:reference-layout`、小程序 TypeScript 和微信构建验证。重建当前 5197 预览，17 个场景通过；透明背景、44px 触控区、封面/滚动态箭头颜色和资料操作入口均已检查。
- [x] 源码提交 `fd66d4702a857de3ae4ee2944092377720fee7e6` 已推送 GitHub；官方微信开发者工具 CLI 成功上传开发版本 `2026.10.17.2`，备注“优化体验”，上传包 1,958,726 字节。冻结快照与已验证构建一致。本次导航修正未修改后端或原工作区，未设置体验版、未送审或正式发布。
