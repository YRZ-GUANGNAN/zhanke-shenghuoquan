# UI-09 商家发布配送单静态复刻报告

## Development Report

- 分支：`feat/ui-merchant-publish`
- 基线：`c01f2058 feat(ui): 静态复刻商家消息`
- 目标：新增商家端「发布配送单」静态页面，并从商家工作台「发布配送单」入口进入。
- 范围：仅 `apps/miniapp/src/pages/merchant-publish/**`、`apps/miniapp/src/pages/merchant-home/index.tsx`、`apps/miniapp/src/app.config.ts`。

本次新增页面：

- `apps/miniapp/src/pages/merchant-publish/index.tsx`
- `apps/miniapp/src/pages/merchant-publish/index.scss`
- `apps/miniapp/src/pages/merchant-publish/index.config.ts`

页面内容按授权实现：

- 顶部白色导航：返回 + 居中标题「发布配送单」；
- 取货信息卡：默认店铺信息，可编辑联系人、电话、取货地址；
- 收货信息卡：收货人、联系电话、送达地址、备注；
- 配送费卡：固定展示 `¥1.5`，提示「平台统一定价,无需填写,发布时支付」；
- 底部主按钮：「发布并支付」；
- 支付底部弹层：钱包余额 / 微信支付二选一，默认钱包；
- 确认后仅展示 toast：「发布成功,等待骑手接单」。

## Preflight / 文件级核实结论

1. 主仓库已有 `ui-merchant-messages-baseline`，当前 UI-09 worktree 从 `c01f2058` 新建。
2. 既有 `delivery-merchant`、`delivery-wallet` 是 D-2/D-3 已接后端与真实资金能力的页面，本次未修改。
3. `merchant-home` 原「发布配送单」入口为静态占位，本次仅改为跳转到新静态页 `/pages/merchant-publish/index`。
4. 样式复用 `design-tokens.scss`，未修改 token 文件。

## Acceptance Report

验证命令：

```powershell
npm --workspace apps/miniapp run typecheck
npm run check:boundaries
npm run build:miniapp
```

结果：

- `typecheck`：PASS
- `check:boundaries`：PASS
- `build:miniapp`：PASS
- 构建产物确认：`apps/miniapp/dist/pages/merchant-publish/index.js`、`index.wxml`、`index.wxss` 均已生成。

静态边界扫描：

- 新页面与入口源码未出现真实支付、配送创建、后端请求调用；
- `apps/miniapp/src/services/**` 无 diff；
- `apps/miniapp/src/pages/delivery-merchant/**` 无 diff；
- `apps/miniapp/src/pages/delivery-wallet/**` 无 diff；
- `apps/miniapp/src/styles/design-tokens.scss` 无 diff；
- `backend/**`、`apps/admin/**`、`backend/migrations/**` 无 diff；
- `backend/cmd/server` 内 `db.AutoMigrate(` 仍为 0。

## Residue Report

- 验证期间临时使用 worktree 依赖 junction，验证后已删除；
- 未启动/停止后端；
- 未访问真实数据库；
- 未调用任何真实支付能力；
- 未改任何后端、服务层、迁移、admin、旧外卖或配送真实业务页。

最终提交 hash 以交付回复中的 `git log --oneline -1` 为准。
