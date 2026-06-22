# G-⑥a-1 外卖后台传输层骨架 Development Report v1.0

日期：2026-06-22

分支：`feat/g6a1-takeout-admin`

基线：`87de169a`

实现提交：`4a76f887f48db568af12c22c47e6a75693621e30`

## 1. 前置门禁

- master 顶部：`87de169a docs(G-5): record rider domain acceptance`；
- `g1-merchant-baseline`、`g2-catalog-baseline`、`g3-order-baseline`、`g5-rider-baseline`、`media-purpose-takeout`、`infra-automigrate-retired`：全部存在；
- 020 存在，021 开发前不存在；
- `backend/cmd/server` 中 `db.AutoMigrate(`：0；
- 主工作区：干净。

## 2. 真实代码预读结论

- `admin/mediaconsole` 使用 `Handler + QueryService + Repository + DTO`，由 composition root 提供已鉴权的 `gin.RouterGroup`；
- A 域真实中间件签名为 `(*user.Service).AuthRequired()` 和 `user.AdminRequired()`；
- 响应统一使用 `response.OK/Created/Fail`，信封为 `{code,message,data}`；
- G-①～⑤分别导出 `MerchantService`、`CatalogService`、`OrderService`、`RiderService`，但此前没有后台所需分页查询；
- 当前 boundary 只禁止 `internal/module/takeout` 本体引入 HTTP/A～F，未禁止 `admin` 导入 `takeout`；
- 在仅新增 `admin/takeoutconsole` 骨架后立即执行 `npm run check:boundaries`：PASS，不需要修改或放宽 boundary 脚本。

## 3. 实现内容

### 3.1 迁移 021

新增 `takeout_withdraw_requests`：

- ULID 公共身份；
- rider 逻辑引用，无外键；
- `amount_cents BIGINT UNSIGNED`；
- `pending/paid/rejected` CHECK；
- `row_version` CAS；
- public ID 唯一索引、rider/status 索引；
- rollback 只删除 G 自有新表。

SHA-256：

- forward：`d36850e96d351bd7ecb0cdd50adc793234354e183f7492711a476b6f582bd41f`
- rollback：`74bfcf98b4eaa5d96146b0bb5dca1a818d13a69a979a6f58c09d151ffae3467f`

### 3.2 提现领域能力

- `WithdrawStatus` 与 `CanTransitionWithdraw`；
- `CreateWithdrawRequest`；
- `MarkWithdrawPaid`；
- `RejectWithdraw`；
- `ListWithdrawRequests`；
- `FindWithdrawByID`；
- payment 与全部 pending earning settlement 位于同一 MySQL 事务；
- earning 逐条使用 `settlement_status + row_version` CAS；
- 重复支付返回已存在的 paid 事实，不重复结算。

### 3.3 后台只读查询

新增纯领域 `AdminQueryService` 和 Repository 原语：

- 商家分页/状态筛选；
- 分类分页/商家筛选；
- 菜品分页/商家及状态筛选；
- 订单分页/状态筛选；
- 骑手分页/状态筛选；
- 按骑手汇总 pending settlement。

### 3.4 HTTP 传输层

新增 `backend/internal/module/admin/takeoutconsole`，包含：

- Handler；
- camelCase 只读 DTO；
- 统一分页 DTO；
- 领域错误到 HTTP 状态映射；
- 无 GORM、无 F review、无业务规则复制。

路由挂载于 `/api/v1/admin/takeout`：

- `GET /merchants`、`GET /merchants/:id`；
- `PATCH /merchants/:id/suspend|unsuspend`；
- `GET /categories`、`GET /categories/:id`；
- `GET /dishes`、`GET /dishes/:id`；
- `GET /orders`、`GET /orders/:id`；
- `GET /riders`、`PATCH /riders/:id/status`；
- `GET|PUT /fee-config`；
- `GET|PUT /rider-config`；
- `GET /settlement-report`；
- `GET /withdraw-requests`、`GET /withdraw-requests/:id`；
- `PATCH /withdraw-requests/:id/pay|reject`。

### 3.5 Composition root

`main.go` 仅追加：

- G 各 Service 构造；
- `AdminQueryService`、`WithdrawService`；
- `takeoutconsole.NewHandler`；
- `AuthRequired + AdminRequired` 路由组。

既有路由、worker、媒体、A～F wiring 未重排或改写。

## 4. 范围隔离

- F review 读取/修改/接入：0；
- Admin 前端页面：0；
- Apps 变更：0；
- 001～020 迁移变更：0；
- boundary 脚本变更：0；
- takeout 生产代码 Gin/net/http/A～F import：0；
- AutoMigrate：0；
- 支付、E、Media 接线：0。

## 5. 已知约束

- 当前列表为 offset 分页，符合现有 Admin 契约；大规模运营查询可在另行授权后评估游标分页；
- 按冻结授权，创建提现只校验 `amount_cents > 0`；实际可提现余额校验和骑手发起 HTTP 留给 G-⑦；
- `MarkWithdrawPaid` 按授权结算该骑手当前全部 pending earning；重复 paid 为幂等返回；
- 本模块不包含 F 审核、Admin 页面或骑手端入口。
