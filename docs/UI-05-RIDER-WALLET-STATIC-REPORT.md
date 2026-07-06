# UI-05 Rider Wallet Static Report

## Development Report

本次按 `UI-05-RIDER-WALLET-STATIC-AUTHORIZATION.md` 实现骑手端钱包静态页，范围严格限定为小程序前端静态 UI：

- 新增骑手钱包页：
  - `apps/miniapp/src/pages/rider-wallet/index.tsx`
  - `apps/miniapp/src/pages/rider-wallet/index.scss`
  - `apps/miniapp/src/pages/rider-wallet/index.config.ts`
- 在 `apps/miniapp/src/app.config.ts` 注册 `pages/rider-wallet/index`。
- 在 `apps/miniapp/src/pages/delivery-rider/index.tsx` 将顶部钱包按钮从占位 toast 改为纯前端跳转：
  - `Taro.navigateTo({ url: '/pages/rider-wallet/index' })`

前置核实结论：

- `master` 起点为 `73d57efb feat(ui): 静态复刻骑手我的配送`。
- 基线标签 `ui-rider-my-delivery-baseline` 存在。
- 主仓库工作区起点干净。
- 既有 `apps/miniapp/src/pages/delivery-wallet` 是商家侧钱包/充值页，已接 D-3 后端资金 API；本次没有覆盖它，避免误伤商家钱包。
- 本次复用 `apps/miniapp/src/styles/design-tokens.scss`，但没有修改 design tokens。

视觉与交互实现：

- 钱包页按授权图 4/5/6 静态复刻：
  - 蓝色渐变顶部区；
  - 返回箭头 + 居中标题“我的钱包”；
  - 钱包余额 `¥128.50`；
  - 文案“ⓘ 提现由平台微信转账，1-2 天内到账”；
  - 白色胶囊“↑ 提现”按钮；
  - 白卡“流水记录”，右侧“仅展示最近 30 天”；
  - 静态收入/提现流水；
  - 底部“— 到底了 —”。
- 提现弹窗按授权静态复刻：
  - 暗色遮罩；
  - 底部白色圆角 sheet；
  - 标题“申请提现” + 关闭按钮；
  - 浅灰金额卡；
  - 浅橙提示条；
  - 蓝色渐变“申请提现 ¥128.50”按钮。
- 交互均为纯前端：
  - 骑手页钱包按钮进入钱包页；
  - 钱包页返回箭头返回上一页；
  - 提现按钮打开弹窗；
  - 弹窗关闭按钮关闭；
  - 申请提现按钮显示静态 toast 并关闭弹窗。

## Acceptance Report

已完成以下验证：

- `npm --workspace apps/miniapp run typecheck`：PASS
- `npm run check:boundaries`：PASS
- `npm run build:miniapp`：PASS
  - 构建存在既有体积/异步 chunk warning：
    - `vendors.js` 超过 500 KiB；
    - `mini.runtime` 不支持异步 chunk。
  - 无新增构建错误。
- 后端零改动：
  - `git diff -- backend` 为空。
- 服务层零改动：
  - `git diff -- apps/miniapp/src/services` 为空。
- design tokens 零改动：
  - `git diff -- apps/miniapp/src/styles/design-tokens.scss` 为空。
- 新骑手钱包页与骑手页入口未引入后端/支付调用：
  - 未命中 `getDeliveryWallet`、`listDeliveryWalletLedger`、`startDeliveryWalletRecharge`、`requestPayment`、`deliveryPayment`、`delivery-payment` 等关键字。

## Residue Report

- 不接后端。
- 不改任何 API。
- 不改数据库。
- 不接真实提现。
- 不改商家钱包页 `delivery-wallet`。
- 不改学生页、首页、外卖、二手、社区、旧外卖、配送后端、媒体、IM、admin。
- 验证期间为 worktree 创建的本地 `node_modules` junction 仅用于编译，收尾时清理，不入库。

最终提交 hash 以交付回复和 `git log --oneline -1 feat/ui-rider-wallet` 为准。
