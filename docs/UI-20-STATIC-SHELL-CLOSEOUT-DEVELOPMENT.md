# UI-20 新版静态壳路由收口 Development Report

## 基线与执行位置

- 基线：`master@9c0ca376`
- 分支：`feat/ui20-static-shell-closeout`
- 独立 worktree：`C:\Users\33141\Documents\ui20-static-shell-closeout`
- 设计记录：`0e8b3425 docs(ui): 固化静态壳路由收口设计`
- 实施计划：`31bdf379 docs(ui): 编写静态壳路由收口计划`
- 功能实现：`f36282f feat(ui): 收口新版静态壳路由`

## Development

### 1. 新版静态壳成为唯一注册入口

`apps/miniapp/src/app.config.ts` 收口为 24 个新版页面白名单。旧页面源码仍留在仓库中供后续迁移参考，但不再注册到本轮小程序运行壳，因此不会被主导航或新版页面跳转重新带回。

### 2. 学生端统一底部导航

新增 `apps/miniapp/src/components/student-bottom-nav.tsx`，统一承载：

- 首页：`/pages/home/index`
- 外卖：`/pages/student-takeout/index`
- 中央发布：`/pages/publish-entry/index`
- 二手：`/pages/student-secondhand/index`
- 我的：`/pages/profile/index`

首页、外卖、二手、我的四个一级页均改用同一组件，消除各页单独维护导致的错跳和二次点击才生效问题。

### 3. 核心静态流闭环

- 搜索：首页搜索入口、二手搜索入口、帖子标签均进入新版搜索页；查询词通过路由参数回显。
- 发布：中央 `+` 进入统一发布类型页；发帖与发布配送单进入已授权新版页面。
- 我的：编辑资料、设置、法律条款、黑名单、我的内容、学生/骑手/商家身份入口均保持新版路由。
- 帖子：信息流卡片、我的内容、用户主页卡片均携带内容 ID；详情作者可进入匹配的用户主页。
- 二手：列表、我的内容、用户主页、详情、联系卖家、私信、卖家主页均携带商品和卖家坐标。
- 配送：商家订单卡片、骑手订单卡片、详情和私信统一携带真实 `orderId`。

### 4. 静态数据坐标一致性

- 为当前所有可见帖子补齐详情映射与作者资料。
- 为当前所有可见二手商品补齐详情映射，并在私信中透传 `productId`、`sellerId`、名称和头像。
- 新增 `delivery-static-data.ts`，集中登记 8 个已展示配送单：`B1085`、`B1083`、`B1079`、`B1077`、`B1071`、`B1066`、`A2044`、`A2039`。
- 未知帖子、商品或配送单不再静默回退到奶茶帖、自行车或 `B1079`，而是明确显示“静态内容待接入”。

### 5. 自动路由守卫

新增 `scripts/check-miniapp-static-routes.cjs` 与根脚本 `check:miniapp-static-routes`，检查：

- 24 个白名单页面完整注册；
- 不允许额外旧页面混入注册表；
- 每个注册页面对应的 `.tsx` 文件存在；
- 注册页面及共享底部导航不得跳向未注册路由。

## 范围声明

- 未修改 `backend/**`、`apps/admin/**`、迁移、数据库、接口或 API 契约。
- 未引入新依赖，`package-lock.json` 无变化。
- 本轮仅完成新版静态前端壳、静态数据坐标与路由闭环。
