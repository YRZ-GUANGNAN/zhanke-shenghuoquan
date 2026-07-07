# UI-07 商家钱包静态页交付报告

## Development Report

本次在独立 worktree `C:\Users\33141\Documents\ui-merchant-wallet`、分支 `feat/ui-merchant-wallet` 上完成 UI-07。

实现内容：

- 新建静态商家钱包页：
  - `apps/miniapp/src/pages/merchant-wallet/index.tsx`
  - `apps/miniapp/src/pages/merchant-wallet/index.scss`
  - `apps/miniapp/src/pages/merchant-wallet/index.config.ts`
- 在 `apps/miniapp/src/app.config.ts` 注册 `pages/merchant-wallet/index`。
- 在 `apps/miniapp/src/pages/merchant-home/index.tsx` 将商家工作台的“充值 / 账单”快捷入口改为纯前端 `Taro.navigateTo({ url: '/pages/merchant-wallet/index' })`。

页面按授权图 4/5/6 实现：

- 蓝色顶部商家钱包页：返回、标题、余额 `¥86.50`、说明文案、白色 `+ 充值` 按钮。
- 白色账单流水卡：支付配送费、取消退款、充值三类本地假数据。
- 充值弹窗：6 档金额，默认 `¥100`，选中态蓝色描边，底部绿色“微信支付 ¥金额”按钮。
- 充值成功态：本地 state 更新余额、插入“充值 · 微信支付 / 刚刚 / +金额”流水，并显示 toast。

## 前置核实结论

- `apps/miniapp/src/pages/delivery-wallet` 是既有真实资金页，已接 D-3 后端资金 API 与真实/模拟支付链路。
- 本次没有覆盖、重命名或改动 `delivery-wallet`。
- 本次新建的是独立静态页 `merchant-wallet`，只服务 UI 阶段一静态还原。
- `merchant-home` 原“充值 / 账单”入口为占位 toast，本次仅改为跳转到静态 `merchant-wallet`。
- 样式使用 `apps/miniapp/src/styles/design-tokens.scss` 既有 token 派生色、阴影、圆角与背景变量；未修改 token 文件。

## Acceptance Report

已执行：

```powershell
npm --workspace apps/miniapp run typecheck
npm run check:boundaries
npm run build:miniapp
```

结果：

- `typecheck`：PASS
- `check:boundaries`：PASS
- `build:miniapp`：PASS
- `build:miniapp` 仅出现既有 `vendors.js` 体积 warning，不影响本次静态页构建。

安全边界验证：

```powershell
rg -n "requestPayment|startDeliveryWalletRecharge|deliveryPayment|Taro\.request|request\(" apps/miniapp/src/pages/merchant-wallet apps/miniapp/src/pages/merchant-home -S
git diff -- apps/miniapp/src/pages/delivery-wallet backend apps/miniapp/src/services apps/miniapp/src/styles/design-tokens.scss --stat
rg -n "db\.AutoMigrate\(" backend/cmd/server
rg -n "#[0-9a-fA-F]{3,8}" apps/miniapp/src/pages/merchant-wallet/index.scss apps/miniapp/src/pages/merchant-home/index.scss -S
```

结论：

- 新 `merchant-wallet` 与本次 `merchant-home` 集成未命中真实支付 / 后端请求调用。
- `delivery-wallet`、`backend`、`services`、`design-tokens.scss` 无 diff。
- `backend/cmd/server` 中 `db.AutoMigrate(` 仍为 0。
- 本次涉及 SCSS 未新增硬编码色值。

## Residue Report

- 本次未启动后端、数据库或真实支付流程。
- 本次未改后端、迁移、服务层、真实资金页。
- 本次用于构建校验的 `node_modules` / `apps/miniapp/node_modules` 为临时 junction，提交前移除，不纳入 Git。
- 测试数据全部为组件内本地假数据，刷新后还原。

## 文件范围

- `apps/miniapp/src/pages/merchant-wallet/index.tsx`
- `apps/miniapp/src/pages/merchant-wallet/index.scss`
- `apps/miniapp/src/pages/merchant-wallet/index.config.ts`
- `apps/miniapp/src/pages/merchant-home/index.tsx`
- `apps/miniapp/src/app.config.ts`
- `docs/UI-07-MERCHANT-WALLET-STATIC-REPORT.md`
