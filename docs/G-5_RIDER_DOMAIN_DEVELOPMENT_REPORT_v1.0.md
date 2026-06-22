# G-⑤ Rider Domain Development Report v1.0

日期：2026-06-22

分支：`feat/g5-rider`

开发基线：`master@ae83ef88`

G-③ 冻结点：`g3-order-baseline@4f0981a4`

实现提交：`745b633c981ccd092a52b507258d79296dbc5492`

结论：开发完成，范围止于 G-⑤ Rider/Fulfillment 后端领域核心；未进入其它模块。

## 1. 前置门禁

| 检查项 | 实际结果 | 结论 |
| --- | --- | --- |
| master 顶部 | `ae83ef88 docs(discipline): v1.2` | PASS |
| G-③ 冻结 | `4f0981a4 docs(G-3)` 是 master 直接祖先，`g3-order-baseline` 指向该提交 | PASS |
| 五个基线标签 | 全部存在 | PASS |
| 迁移锚点 | 016～019 存在，020 不存在 | PASS |
| 启动期 DDL | `db.AutoMigrate(` 为 0 | PASS |
| 主工作区 | clean，位于 master | PASS |

附件原预期 G-③ 为 master 顶部；实际多出的一次提交仅修改 `docs/DEVELOPMENT_DISCIPLINE.md`，且正是本授权要求遵循的配置治理 v1.2，因此以 `ae83ef88` 作为合法开发基线。

## 2. 实现内容

在 `backend/internal/module/takeout` 新增：

- `Rider`、`Fulfillment`、`RiderConfig` 领域实体；
- Rider 与线性 Fulfillment 状态机；
- `RiderRepository`、`RiderOrderOps` 与同域事务交汇适配器；
- Rider 注册、状态管理、可配置在途上限、抢单池、抢单和履约 Service；
- 单元、边界和真实 MySQL 8.4 并发/回滚测试。

既有 `repository.go` 仅从冻结 G-③ 代码末尾追加 G-⑤ 实现，G-①～G-③ 原有代码与行为未修改。

## 3. 状态机

Rider：

```text
active -> resting | suspended
resting -> active | suspended
suspended -> active
```

Fulfillment：

```text
ASSIGNED -> TO_STORE -> PICKED -> TO_USER -> DELIVERED
```

所有状态推进均使用 `expected_status + expected_row_version` CAS。目标态重试幂等；非法边、stale write 与终态后推进均 fail-closed。

## 4. 抢单事务与可配置上限

`ClaimOrder` 单一外层事务执行：

1. `SELECT ... FOR UPDATE` 锁定骑手行；
2. 已有同骑手同订单履约时幂等返回，其他骑手已占用时拒绝；
3. 校验新抢单骑手为 `active`；
4. 读取 `takeout_rider_config/global`；
5. 经复合索引统计该骑手 `status != DELIVERED` 的在途数；
6. 达上限返回 `ErrInFlightLimit`；
7. 使用事务内 Repository 调用冻结 G-③ `StartDelivering`；
8. 创建唯一订单履约记录 `ASSIGNED,row_version=1`。

同骑手并发由骑手行锁串行化；不同骑手互不因骑手锁阻塞，争抢同一订单时由 G-③ 订单 CAS 决出唯一成功者。系统没有派单或自动分配算法。

## 5. 送达交汇事务

`CompleteFulfillment` 在单一外层事务中先 CAS `TO_USER→DELIVERED`，再使用同一事务内 Repository 调用冻结 G-③ `CompleteDelivery`。G-③ 的内部事务作为 savepoint 执行，最终由 G-⑤ 外层事务统一提交或回滚。

真实故障注入证明：当订单仍为 `ready_for_pickup` 导致 G-③ 完成失败时，履约仍保持 `TO_USER`，没有半完成状态。

## 6. 配置治理

- 配置表：`takeout_rider_config`；
- 单行 scope：`global`；
- 默认种子：`max_in_flight_per_rider=1000`；
- 生产代码没有硬编码 1000；
- 更新使用 `expected_row_version` CAS；
- 配置变化仅影响之后的抢单，不回写既有履约。

## 7. 迁移 020

- 正向：`backend/migrations/020_takeout_rider.sql`
- 回滚：`backend/migrations/rollback/020_takeout_rider.sql`
- 新表：`takeout_riders`、`takeout_fulfillments`、`takeout_rider_config`
- 三表全部为 G 自有表，外键数 0
- 迁移 001～019 未修改

SHA-256：

```text
020 forward  b426c985b37c06cfdcc5a4512b0fc07f4a6a790967b901e901e50013f8d3ac1d
020 rollback 4ea912ca2a65a91a9ecedf9ee2b44735c20919ce49aa5bba1cf652ffd0cc6304
```

## 8. 边界

- 无 HTTP、Gin Route、Controller、Admin 或 MiniApp 改动；
- 无支付、F、E、Media、A～F import/调用；
- 仅通过 G-③ `StartDelivering`、`CompleteDelivery` 和 ready 池读取三个同域交汇能力；
- 无 Outbox/Consumer、派单算法或自动分配；
- 无金额字段或金额逻辑。
