# UI-03 骑手抢单大厅静态复刻报告

## Development Report

本次在独立 worktree `C:\Users\33141\Documents\ui-rider-grab-hall`、分支 `feat/ui-rider-grab-hall` 中完成，起点为：

- `master`: `126c8785 fix(ui): 打通学生端首页与我的页导航`
- 基线标签：`ui-student-home-baseline`、`ui-student-profile-baseline` 均存在
- 小程序路由已存在：`apps/miniapp/src/app.config.ts` 注册 `pages/delivery-rider/index`

按 `UI-03-RIDER-GRAB-HALL-STATIC-AUTHORIZATION.md`，本模块只做骑手端抢单大厅图 1 的静态复刻：

- 完全替换原 `delivery-rider` 真实业务页 UI，不再复用旧真实履约页面结构；
- 页面改为纯静态 mock 数据，不调用后端、不 import `delivery-rider` service、不上传图片、不操作真实订单；
- 只保留占位 toast：切换身份、钱包、刷新、抢单；
- 默认展示 `待抢单` tab，`待取货` / `待送达` 只做本页占位，不展开 UI-04；
- 抢单前隐私规则按设计呈现：电话打码、仅显示楼栋楼层、门牌提示为抢单后显示；
- 保持骑手端无学生底部导航、无中间 FAB；
- 复用现有 `apps/miniapp/src/styles/design-tokens.scss`，未修改 token。

改动文件：

- `apps/miniapp/src/pages/delivery-rider/index.tsx`
- `apps/miniapp/src/pages/delivery-rider/index.scss`
- `apps/miniapp/src/pages/delivery-rider/index.config.ts`

## Acceptance Report

### 静态范围与旧业务清理

已确认以下旧真实业务调用在本页中为 0：

```powershell
rg "getDeliveryRiderMe|claimDeliveryOrder|listClaimable|chooseAndUploadImages|delivery-shared|loadPrivateDeliveryProofs" apps/miniapp/src/pages/delivery-rider
```

结果：无命中。

本页仍保留在既有小程序路由中：

```powershell
rg "pages/delivery-rider" apps/miniapp/src/app.config.ts
```

结果：`pages/delivery-rider/index` 已注册。

### 构建验证

由于 worktree 不在主仓库目录树下，首次运行时 `tsc/taro` 无法解析依赖。本次未安装依赖、不改锁文件，仅临时将 worktree 的 `node_modules` junction 指向主仓库已有依赖；验证结束后已删除 junction。

执行：

```powershell
npm --workspace apps/miniapp run typecheck
```

结果：PASS。

执行：

```powershell
npm run build:miniapp
```

结果：PASS。仅出现既有 `vendors.js` 体积警告：

- `AssetsOverSizeLimitWarning: vendors.js (731 KiB)`
- `NoAsyncChunksWarning`

### 边界验证

`git diff --stat` 显示仅骑手页三件套改动：

```text
apps/miniapp/src/pages/delivery-rider/index.config.ts
apps/miniapp/src/pages/delivery-rider/index.scss
apps/miniapp/src/pages/delivery-rider/index.tsx
```

未修改：

- `backend/**`
- `apps/miniapp/src/services/**`
- `apps/miniapp/src/app.config.ts`
- `apps/miniapp/src/styles/design-tokens.scss`
- 迁移、配置、后端路由、API 契约

## Residue Report

- 临时 `node_modules` junction 已删除；
- `.rej` 临时补丁文件残留为 0；
- 主仓库 `C:\Users\33141\Documents\New project` 工作区保持干净；
- worktree 仅保留本次授权相关改动与本报告，待提交；
- 本次没有启动后端、没有连接数据库、没有写入测试数据，残留数据为 0。
