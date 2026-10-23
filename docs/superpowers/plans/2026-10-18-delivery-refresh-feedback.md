# 配送订单刷新与反馈实施计划

**Goal:** 依照最新资料完善两端订单刷新、手势与状态反馈，保护学生端正在进行的修改。

**Architecture:** 页面复用原生 `RefreshScrollView`，专属 `delivery-order-feedback` 负责等待、骨架和状态文字。现有服务、身份守卫、结果未知保护及每单写操作继续沿用。

**Tech Stack:** Taro、React、TypeScript、SCSS、隔离行为回归与实际组件布局预览。

## 1. 资料与范围

- [x] 核对工作树 `delivery-workspace-redesign`、基线 `df115f2` 和最新参考库 `deecfe4`。
- [x] 阅读共享刷新组件及其成功/失败/隐藏/卸载/账号隔离合同。

## 2. 交互和请求

文件：`apps/miniapp/src/pages/merchant-home/index.tsx`、`apps/miniapp/src/pages/delivery-rider/index.tsx`。

- [x] 两端接入 `<RefreshScrollView onRefresh={requestRefresh} refreshKey={账号和页签} busy={当前读取或操作状态}>`，原触底加载仍接 `loadMoreOrders`。
- [x] 加载函数返回 `Promise<boolean>`，仅真实更新成功返回 `true`；拒绝重复或过期请求返回 `false`。
- [x] 商家已审核背景刷新保持页面，首次核验与真实资格失败保留守卫。
- [x] 骑手分页增加同步锁，原生下拉、手动刷新和履约/凭证动作互斥，内部动作完成后的列表同步仍可执行。

## 3. 状态视觉

文件：新增 `apps/miniapp/src/components/delivery-order-feedback.tsx` 与 `.scss`；两端页面的 `.scss` 与专属 icon 注册表。

- [x] 初始读取显示订单骨架；局部刷新/分页显示三点等待；失败给出原因与 44px 重试动作；空列表只显示正确说明。
- [x] 移除旧空态图标引用和旋转样式，固定刷新按钮及图标尺寸，补充局部淡入和 `prefers-reduced-motion`。

## 4. 验证与交付

文件：`scripts/test-delivery-stage-workbench.cjs`、两端 clarity 检查、`docs/reviews/2026-10-18-delivery-refresh-feedback.md`。

- [x] 增加延迟响应行为场景：成功/失败恢复、同帧连拉/连点、分页阻止刷新、上传阻止刷新、账号旧事件作废、刷新期间保留卡片。运行 `node scripts/test-delivery-stage-workbench.cjs`。
- [x] 运行 `node scripts/check-refresh-feedback.cjs` 和既有配送/布局合同、`npm.cmd run typecheck`、`npm.cmd run build:miniapp`。
- [x] 独立预览检查 320/390/430px 及长文本，记录刷新按钮边界和空态/错误/加载截图；检查改动范围和 `git diff --check`。
- [x] 本地保存代码和证据，说明原生真机验收边界。
