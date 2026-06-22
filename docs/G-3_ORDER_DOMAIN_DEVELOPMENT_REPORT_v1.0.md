# G-③ Order Domain Development Report v1.0

日期：2026-06-22

分支：`feat/g3-order`

基线：`master@f03e8140`

实现提交：`95bdabbe411428e06eae14e107c825083c1ebc80`

结论：开发完成，范围止于 G-③ 订单领域核心；未进入 G-④。

## 1. 前置门禁

| 检查项 | 实际结果 | 结论 |
| --- | --- | --- |
| master 顶部 | `f03e8140 docs(G-2): record catalog domain acceptance` | PASS |
| 基线标签 | `g1-merchant-baseline`、`g2-catalog-baseline`、`media-purpose-takeout`、`infra-automigrate-retired` 均存在 | PASS |
| 迁移锚点 | 017、018 存在，019 不存在，最大编号 018 | PASS |
| 启动期 DDL | `backend/cmd/server` 中 `db.AutoMigrate(` 为 0 | PASS |
| 主工作区 | clean，位于 master | PASS |

## 2. 实现内容

在 `backend/internal/module/takeout` 新增：

- `Order`、`OrderItem`、`RiderEarningRecord`、`FeeConfig` 领域实体；
- `OrderStatus`、`SettlementStatus` 及冻结状态机；
- `CatalogReader` 只读接口与 `OrderRepository`；
- `OrderService` 下单、状态推进、查询、费率配置与应结能力；
- 单元、边界和真实 MySQL 8.4 集成测试。

既有 `repository.go` 仅从原文件末尾追加 G-③ Repository 实现；G-①/G-② 原有代码与行为未修改。

没有新增 HTTP、Gin Route、Controller、Admin、小程序、真实支付回调、F/E/G-⑤ 接线、Outbox/Consumer 或 Media 调用。

## 3. 金额与快照规则

- 所有金额字段为 `BIGINT UNSIGNED`，Go 类型为 `uint64`；
- 数量为 `INT UNSIGNED` / `uint32`；
- 乘法、逐行累加及总价累加均执行溢出检查；
- `CreateOrder` 不接收外部金额，只读取 G-② 菜品的名称、价格、状态及商户归属；
- `line_amount = unit_price × quantity`、`items_amount = Σline`；
- 创建订单时读取全局费率，`delivery_fee = rider_fee + platform_cut`；
- `total_amount = items_amount + delivery_fee`；
- `takeout_order_items` 创建后无更新能力，Catalog 后续改名/改价不影响订单快照。

## 4. 状态机

```text
pending_payment -> paid | cancelled
paid            -> accepted | refunded
accepted        -> preparing | refunded
preparing       -> ready_for_pickup
ready_for_pickup-> delivering
delivering      -> completed
completed / cancelled / refunded -> terminal
```

订单状态更新均携带 `expected_status + expected_row_version`，事务内先 `SELECT ... FOR UPDATE`，CAS 更新执行 `row_version = row_version + 1`。已达同一目标状态时幂等返回；支付号或骑手不一致的“伪重试”会 fail-closed。

## 5. 完成配送与应结事务

`CompleteDelivery` 在单一数据库事务中：

1. 锁定订单并校验 `delivering + row_version + rider_user_id`；
2. 锁定 `takeout_fee_config/global`；
3. CAS 推进订单到 `completed`；
4. 将当前费率快照写入 `takeout_rider_earning_records`；
5. 以 `UNIQUE(order_public_id)` 保证一单一条。

任一步失败时整个事务回滚。重复完成会校验既有应结记录并返回同一订单，不产生第二条记录。

## 6. 迁移 019

- 正向：`backend/migrations/019_takeout_order.sql`
- 回滚：`backend/migrations/rollback/019_takeout_order.sql`
- 新表：`takeout_orders`、`takeout_order_items`、`takeout_rider_earning_records`、`takeout_fee_config`
- 全部为 G 自有表，外键数 0
- 种子：`global / rider=100 / platform=50 / row_version=1`
- 迁移 001～018 未修改

SHA-256：

```text
019 forward  ffb8d7071a896c690e1f0813169c8758f6d148233e52ea16208ba7f503fa6275
019 rollback 049d73b617fd74c65150f214fee7d4530c848755ed0def2462efb1021bea1d42
```

## 7. 内部领域能力

订单：`CreateOrder`、`MarkPaid`、`Accept`、`StartPreparing`、`MarkReady`、`StartDelivering`、`CompleteDelivery`、`Cancel`、`Refund`、`FindOrder`、`ListOrderItems`。

费率与应结：`GetActiveFeeConfig`、`UpdateFeeConfig`、`FindEarningByOrder`、`MarkSettled`。

## 8. 已知事项

- buyer、merchant、rider 仅为逻辑引用，不读取 A 域或建立跨域外键；
- G-③ 只建设支付/配送相关状态边，不接真实支付、抢单或配送触发器；
- CatalogReader 的校验与订单写入之间不锁定 Catalog 行，保持只读边界；组合层未来如需库存语义必须另行授权，不得在 G-③ 偷渡；
- MiniApp 仍有既有 `vendors.js` 体积提示，但构建通过且本模块没有前端变更。

