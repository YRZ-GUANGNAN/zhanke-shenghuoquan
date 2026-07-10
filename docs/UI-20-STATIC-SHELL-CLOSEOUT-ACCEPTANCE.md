# UI-20 新版静态壳路由收口 Acceptance Report

## Acceptance

| 验收项 | 结果 | 证据 |
|---|---|---|
| 小程序 TypeScript | PASS | `npm --workspace apps/miniapp run typecheck` |
| 静态路由白名单 | PASS | `npm run check:miniapp-static-routes` 输出 `PASS (24 routes)` |
| 小程序构建 | PASS | `npm run build:miniapp`，Taro 4.2.0，Webpack compiled successfully |
| 仓库综合验证 | PASS | `npm run verify`：boundary、admin/miniapp typecheck、Go tests 全通过 |
| diff 格式 | PASS | `git diff --check` / `git diff --cached --check` |
| AutoMigrate | PASS | `backend/cmd/server` 中 `db.AutoMigrate(` 命中 0 |
| 后端与后台边界 | PASS | `backend/**`、`apps/admin/**` diff 为空 |
| 锁文件 | PASS | `package-lock.json` diff 为空 |

## Acceptance Evidence

### 1. 路由收口

- `app.config.ts` 与构建产物 `dist/app.json` 均为同一组 24 个新版页面。
- 自动路由检查确认注册页和共享导航不存在指向未注册页面的静态跳转。
- 旧页面未进入构建产物的页面注册表。

### 2. 一级导航

- 首页、外卖、二手、我的统一使用 `StudentBottomNav`。
- 中央发布按钮统一进入新版发布类型页。
- 二手可直接进入，不再需要先点击“我的”触发跳转。

### 3. 内容与身份坐标

- 当前可见帖子 ID 均能命中对应详情。
- 当前帖子作者 ID 均能命中对应用户主页。
- 当前可见二手商品 ID 均能命中对应详情；私信展示对应商品与卖家，不再固定为自行车/向北。
- 用户主页中的帖子和宝贝卡片均能进入匹配内容。

### 4. 配送订单闭环

- 商家端订单卡片将真实 `orderId` 传入详情。
- 骑手端 `A2044`、`A2039` 将真实 `orderId` 传入私信。
- 详情与私信共同读取 `delivery-static-data.ts`，订单号、商家、骑手、地址、状态和时间线保持同一坐标。
- 未知订单显示“配送单信息待接入”，不会串到 `B1079`。

### 5. 独立只读复核

最终只读审查结论为 PASS：原 4 个 P1（配送串单、用户主页内容映射、作者主页映射、二手私信场景）均关闭；未知帖子、商品、配送单也不再错误回退。

## 构建提示

构建仅保留既有 `publish-post/index.scss` 的 Sass `@import` deprecation warning，不影响编译结果；本轮未扩大范围处理该样式技术债。
