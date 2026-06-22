# G-⑥a-ui Takeout Admin Development Report v1.0

日期：2026-06-23
分支：`feat/g6a-ui-takeout-admin`
基线：`master@42d7926e`

## 1. 结论

已在现有管理端架构内完成外卖域 G 的 8 个运营视图，并按追加裁定完成两个最小只读契约补丁：菜品 `categoryId` 可选筛选、订单阶段时间戳透出。未引入路由库、新状态库、迁移或新鉴权路径。

## 2. 开发前真实代码审查

- `store.ts`：Zustand 仅保存 `section`，以 `setSection` 切换；本次只扩展既有 `AdminSection` 联合类型，不改 store 结构。
- `App.tsx`：导航数组与 `Content()` 条件分发页面；本次仅新增 `takeout` 导航和对应分支，既有页面行为不变。
- `services/f-admin.ts`：使用 `/api/v1`、`localStorage['domainA.token']`、Bearer token、`{code,message,data}` 信封与 `items/page/pageSize/total` 分页；本次新建同构的 `takeout-admin.ts`，未修改原文件。
- 列表范式：复用现有列表的筛选、分页、详情侧栏结构；写操作范式在其上补齐确认、loading、409 明示和刷新动作。
- `types.ts`：现有展示契约集中管理；本次继续在此登记 G 域 DTO。

## 3. 实现内容

### 3.1 前端

- 新增 `TakeoutConsole`，以组件内 tab 承载：商家、菜品/分类、订单、骑手、费率、在途上限、结算、提现。
- 新增专属 API client，严格复用既有 token 和响应信封。
- 商家页仅展示 G 域状态，不查询或展示独立 F Task 状态。
- CAS 敏感写操作全部保留服务端 `rowVersion`，提交前确认，提交中禁用；409 显示“数据已被修改，请刷新后重试”并提供刷新。
- 配置更新成功后重新 GET 服务端值，不做乐观伪更新。

### 3.2 授权的最小后端增量

- 菜品列表新增可选 `categoryId`，省略时委派原冻结查询，行为不变。
- 订单只读 DTO 新增已有列的八个阶段时间戳。
- 无迁移、无状态机变更、无领域写行为变更。

## 4. 文件与提交

- 后端契约增量：`b2a7f622 fix(G-6a-ui): extend takeout admin read contract for UI`
- 前端实现：`2bc9611 feat(G-6a-ui): add CAS-safe takeout admin console`
- 前端生产文件：`apps/admin/src/App.tsx`、`styles.css`、`types.ts`、`TakeoutConsole.tsx`、`services/takeout-admin.ts`

## 5. 范围证明

- `package.json` 无变化；未新增 React Router 或状态管理依赖。
- 未修改 G-①～G-⑤ 状态机、迁移、金额规则或审核逻辑。
- 未触碰 P0-5、68 项 schema 漂移或其它业务域。
