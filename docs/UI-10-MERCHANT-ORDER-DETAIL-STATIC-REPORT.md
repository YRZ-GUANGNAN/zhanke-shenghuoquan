# UI-10 商家端配送单详情静态页报告

## Development Report

- 分支: `feat/ui-merchant-order-detail`
- 基线: `ff1dc349 feat(ui): 静态复刻发布配送单`
- 基线标签: `ui-merchant-publish-baseline`
- 目标: 新建独立静态页 `pages/merchant-order-detail/index`，按设计图还原商家端配送单详情。
- 集成点: `pages/merchant-home/index.tsx` 的订单卡「详情」按钮改为纯前端 `Taro.navigateTo({ url: '/pages/merchant-order-detail/index' })`。

## 前置核实结论

- 既有已接后端页面和服务已核实并保持隔离:
  - `apps/miniapp/src/pages/delivery-merchant/*`
  - `apps/miniapp/src/pages/delivery-rider/*`
  - `apps/miniapp/src/pages/delivery-wallet/*`
  - `apps/miniapp/src/pages/delivery-conversation/*`
  - `apps/miniapp/src/services/delivery-*.ts`
- 本任务没有覆盖、不改造、不 reskin 这些已接后端页面和服务。
- `merchant-home` 详情按钮原为 toast 占位，本次仅改为跳转新静态页。
- `design-tokens.scss` 已有品牌蓝、金额橙派生基础、背景、文本、卡片阴影、占位斜纹等变量，本次新 SCSS 仅通过 token 和 `sass:color` 派生视觉值，未改 tokens。

## Acceptance Report

页面内容:

- 顶部白色栏: 返回箭头 + `配送单详情`。
- 状态卡: `配送中`、`¥1.50`、`钱包支付`、`骑手已取货，正在前往送达点`、`#B1079 · 12:31 下单`、`送达点: 3栋 4楼 412室`。
- 骑手信息卡: `李伟 · 骑手 · 12:35 接单`、`137 5560 8814`、私聊/电话圆形占位按钮。
- 配送进度: 7 步时间线，5 步完成、当前配送中、待送达。
- 配送照片: 取货照静态占位、送达照待上传静态占位、隐私标签 `锁 仅本单商家与骑手可见`。
- 全部交互为纯前端占位:
  - 返回: `Taro.navigateBack`
  - 私聊/电话/照片: `Taro.showToast`
  - 不发请求、不接上传、不接消息。

验证命令:

```powershell
npm --workspace apps/miniapp run typecheck
npm run check:boundaries
npm run build:miniapp
```

结果:

- `typecheck`: PASS
- `check:boundaries`: PASS
- `build:miniapp`: PASS，仅有既有 `vendors.js` 体积告警。
- `apps/miniapp/dist/pages/merchant-order-detail/index.js`: 已生成
- `apps/miniapp/dist/pages/merchant-order-detail/index.wxml`: 已生成
- `apps/miniapp/dist/pages/merchant-order-detail/index.wxss`: 已生成

## Boundary / Negative Checks

无网络、上传、消息、图片真实解析关键词命中:

```powershell
rg -n "Taro\.request|request\(|requestPayment|deliveryPayment|deliveryProof|deliveryIm|upload|chooseMedia|getImageInfo|compressImage|prepare|resolve|TencentCloudChat|conversation" apps/miniapp/src/pages/merchant-order-detail apps/miniapp/src/pages/merchant-home -S
```

结果: 无命中。

新页面样式无新增十六进制色值:

```powershell
rg -n "#[0-9a-fA-F]{3,8}" apps/miniapp/src/pages/merchant-order-detail/index.scss -S
```

结果: 无命中。

后端、服务、已接后端配送页、tokens、admin、迁移均无 diff:

```powershell
git diff -- backend apps/miniapp/src/services apps/miniapp/src/pages/delivery-merchant apps/miniapp/src/pages/delivery-wallet apps/miniapp/src/pages/delivery-rider apps/miniapp/src/pages/delivery-conversation apps/miniapp/src/styles/design-tokens.scss apps/admin backend/migrations --stat
```

结果: 无输出。

AutoMigrate 仍为 0:

```powershell
rg "db\.AutoMigrate\(" backend/cmd/server
```

结果: 无命中。

## Residue Report

- 临时依赖 junction 仅用于验证，提交前已清理。
- 未启动后端、未连接数据库、未产生测试数据。
- 未改 `.env`、未改迁移、未改 services、未改 tokens。
- 最终 commit hash 以 `git log --oneline -1` 和交付回复为准。
