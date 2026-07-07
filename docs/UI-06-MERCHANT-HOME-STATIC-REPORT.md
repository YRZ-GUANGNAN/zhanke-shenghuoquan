# UI-06 商家工作台静态复刻报告

日期：2026-07-07  
分支：`feat/ui-merchant-home`  
提交：本报告随最终提交入库，最终提交号以 `git log --oneline -1` 为准。

## Development Report

本次按 UI-06 授权与最新 Claude Design 截图，新建独立商家工作台静态页：

- 新增页面：
  - `apps/miniapp/src/pages/merchant-home/index.tsx`
  - `apps/miniapp/src/pages/merchant-home/index.scss`
  - `apps/miniapp/src/pages/merchant-home/index.config.ts`
- 在 `apps/miniapp/src/app.config.ts` 注册 `pages/merchant-home/index`。
- 在 `apps/miniapp/src/pages/profile/index.tsx` 的身份切换中，将“商家”入口接到 `/pages/merchant-home/index`。

页面内容为纯静态复刻：

- 顶部蓝色商家身份头图：`茶知味·大学城店`、`商家`、`营业中 · 大学城二食堂`、`切换身份`。
- 钱包余额与经营数据：`¥86.50`、`今日单量 12`、`配送中 2`。
- 快捷入口：`发布配送单`、`充值`、`账单`、`消息`。
- 三个订单 tab：`待接单`、`配送中`、`已完成`。
- 三类静态订单卡：
  - 待接单：显示待骑手接单、配送地址、联系人、备注、配送费、取消并退款、详情。
  - 配送中：显示骑手信息、私聊/电话占位按钮、配送状态、详情。
  - 已完成：显示送达时间、完成状态、配送信息。

所有操作按钮仅触发 `Taro.showToast` 静态占位，不请求后端、不调用真实 API、不修改资金/订单状态。

## Acceptance Report

已执行校验：

```text
npm --workspace apps/miniapp run typecheck
PASS

npm run check:boundaries
PASS

npm run build:miniapp
PASS
```

`build:miniapp` 仅保留既有体积提示：

- `vendors.js` 超过推荐体积；
- Webpack 建议按需拆包。

本次静态页不引入新依赖，不改变这些既有提示。

范围核查：

```text
git diff -- backend apps/miniapp/src/services apps/miniapp/src/styles/design-tokens.scss
空输出
```

结论：

- 未修改 `backend/**`。
- 未修改 `apps/miniapp/src/services/**`。
- 未修改 `apps/miniapp/src/styles/design-tokens.scss`。
- 未修改真实后端联调页面 `delivery-merchant` / `delivery-wallet` / `delivery-rider`。
- 新建 `merchant-home` 为独立静态页面。

旧名核查：

```text
rg -n "后端|接口|数据库|CAS|assignment|腾讯云|广海空间|海大|request|fetch|getDeliveryWallet|requestPayment" apps/miniapp/src/pages/merchant-home apps/miniapp/src/pages/profile apps/miniapp/src/app.config.ts
```

结果：

- `merchant-home` 新页无上述技术词、接口词、旧学校/旧产品名。
- 唯一命中为 `app.config.ts` 中历史全局 `navigationBarTitleText: '广海空间'`，本任务未改全局标题，避免扩大 UI-06 范围。

## Residue Report

- 为在独立 worktree 构建，小程序依赖曾通过 Junction 临时复用主仓 `node_modules`。
- 验收后已删除：
  - `C:/Users/33141/Documents/ui-merchant-home/node_modules`
  - `C:/Users/33141/Documents/ui-merchant-home/apps/miniapp/node_modules`
- 未启动后端。
- 未写入测试数据。
- 未新增临时数据库/迁移/配置。

残留：0。
