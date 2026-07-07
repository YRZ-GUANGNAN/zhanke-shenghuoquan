# UI-08 商家消息静态页交付报告

## Development Report

本次在独立 worktree `C:\Users\33141\Documents\ui-merchant-messages`、分支 `feat/ui-merchant-messages` 上实现。

实现内容：

- 新增静态页面 `apps/miniapp/src/pages/merchant-messages/`：
  - 白色顶部栏：返回、居中标题“消息”、右侧“全部已读”。
  - 白色圆角消息卡：骑手私聊 2 条、系统通知 3 条。
  - 未读红点由组件内 local state 控制。
  - “全部已读”仅清除本地未读红点并提示。
  - 点击骑手私聊或系统通知仅显示 `Taro.showToast` 占位。
- 更新 `apps/miniapp/src/app.config.ts` 注册 `pages/merchant-messages/index`。
- 更新 `apps/miniapp/src/pages/merchant-home/index.tsx`：
  - “消息”快捷入口从静态 toast 改为 `Taro.navigateTo({ url: '/pages/merchant-messages/index' })`。

明确未做：

- 未接后端。
- 未接腾讯 IM。
- 未改 `services/**`。
- 未改 D-5 IM 页面。
- 未改 `design-tokens.scss`。
- 未改 backend/admin/迁移。

## 前置核实结论

- 当前基线：`master` 为 `0f115e58 feat(ui): 静态复刻商家钱包`，tag `ui-merchant-wallet-baseline` 存在。
- `backend/cmd/server` 中 `db.AutoMigrate(` 搜索结果为 0。
- 既有 IM/聊天页已确认存在并保持隔离：
  - `apps/miniapp/src/pages/chat/`
  - `apps/miniapp/src/pages/conversations/`
  - `apps/miniapp/src/pages/delivery-conversation/`
  - `apps/miniapp/src/services/e-im.ts`
  - `apps/miniapp/src/services/delivery-im.ts`
- 本次新建 `merchant-messages`，未覆盖、不导入、不改动上述 D-5 IM 或 C2C IM 相关页面和服务。
- `merchant-home` 中消息入口原为 `showPlaceholder`，本次仅改成纯前端页面跳转。
- 颜色、阴影、字体、背景均通过 `design-tokens.scss` 变量或基于 token 的 `sass:color` 派生值实现；未修改 token。

## Acceptance Report

已执行验收命令：

```powershell
npm --workspace apps/miniapp run typecheck
npm run check:boundaries
npm run build:miniapp
```

结果：

- `typecheck`：PASS
- `check:boundaries`：PASS
- `build:miniapp`：PASS
  - 仅出现既有 `vendors.js` 体积告警，无编译错误。

范围与隔离证据：

```powershell
rg -n "requestPayment|startDeliveryWalletRecharge|deliveryPayment|Taro\.request|request\(|Tencent|IM|WorkGroup|conversation|UserSig|Chat|delivery-im|e-im" apps/miniapp/src/pages/merchant-messages apps/miniapp/src/pages/merchant-home -S
```

结果：无真实网络请求、无 IM/Tencent/WorkGroup/UserSig/Chat 关键字命中。

```powershell
git diff -- apps/miniapp/src/pages/delivery-conversation apps/miniapp/src/pages/chat apps/miniapp/src/pages/conversations apps/miniapp/src/services backend apps/miniapp/src/styles/design-tokens.scss apps/admin --stat
```

结果：无 diff。

```powershell
rg -n "db\.AutoMigrate\(" backend/cmd/server
```

结果：0。

## Residue Report

- 测试过程中临时创建了 worktree 本地 `node_modules` junction，用于复用主仓库依赖跑验收；已在提交前清理。
- 未启动后端。
- 未写数据库。
- 未新增测试数据。
- 未改 `.env`。
- 未改迁移。
- 未留后台进程。

最终 commit hash 以 `git log --oneline -1` 与交付回复为准。
