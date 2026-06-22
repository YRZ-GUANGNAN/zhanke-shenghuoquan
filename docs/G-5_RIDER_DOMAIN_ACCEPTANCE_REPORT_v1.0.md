# G-⑤ Rider Domain Acceptance Report v1.0

日期：2026-06-22

验收对象：外卖域 G 骑手众包抢单与履约核心

实现提交：`745b633c981ccd092a52b507258d79296dbc5492`

真实环境：MySQL 8.4.0

专项结论：**PASS**

## 1. 验收矩阵

| 验收项 | 证据 | 结果 |
| --- | --- | --- |
| 迁移隔离 | 仅新增 020 与 rollback；001～019 未改 | PASS |
| 真实迁移 | MySQL 8.4 apply→rollback→apply | PASS |
| 表结构 | 三表字段、索引、CHECK、种子符合契约 | PASS |
| 冻结保护 | G-③ order status CHECK 前后未变 | PASS |
| 零外键 | 三表外键总数 0 | PASS |
| Rider 状态机 | active/resting/suspended 全部合法边 | PASS |
| 履约状态机 | ASSIGNED→TO_STORE→PICKED→TO_USER→DELIVERED | PASS |
| 非法/终态 | fail-closed | PASS |
| Rider CAS | stale 拒绝，目标态幂等 | PASS |
| 履约 CAS | stale 拒绝，目标态幂等 | PASS |
| 抢单池 | 仅返回 `ready_for_pickup` | PASS |
| 非 active 骑手 | 抢单拒绝，订单/履约无副作用 | PASS |
| 多骑手同单 | 仅 1 成功，其余 `ErrOrderAlreadyClaimed` | PASS |
| 订单交汇 | 成功订单转 delivering 且 rider 正确 | PASS |
| 可配置上限 | 配为 1 时第二单拒绝；调回 1000 后可抢 | PASS |
| 同骑手并发两单 | 骑手行锁保证 1 成功、1 `ErrInFlightLimit` | PASS |
| 配置 CAS | stale 拒绝，生产代码无硬编码 1000 | PASS |
| Claim 原子性 | 订单 CAS 与 fulfillment 创建同事务 | PASS |
| Complete 原子性 | fulfillment delivered、order completed、earning 同事务 | PASS |
| 交汇失败回滚 | fulfillment 保持 TO_USER、order 保持 ready | PASS |
| 重复送达 | 幂等，earning count=1 | PASS |
| 查询 | FindRider、FindFulfillmentByOrder、ready 池 | PASS |
| Boundary | 无 HTTP、跨域、支付、Media、派单算法 | PASS |
| 数据残留 | Rider/Fulfillment/G5 Orders/Items/Earnings/Dishes=0 | PASS |
| Go 回归 | `go test ./... -count=1` | PASS |
| 静态/构建 | `go vet ./...`、`go build ./...` | PASS |
| Workspace verify | boundary、双端 typecheck、backend tests | PASS |
| Production build | MiniApp、Admin | PASS |

## 2. 真实 MySQL 证据摘要

```text
real_mysql_version=8.4.0
migration_020_apply_rollback_apply=PASS tables=3 foreign_keys=0 rider_config=global:1000
g3_order_unchanged=PASS
rider_lifecycle=PASS fulfillment_linear=PASS stale_CAS=REJECTED idempotency=PASS
claim_order_same_tx=PASS ready_pool=PASS
configurable_limit=1_reject_then_1000_accept
delivery_crosspoint=PASS earning_count=1
same_rider_two_orders_lock=PASS success=1 in_flight_limit=1
multi_rider_same_order=PASS success=1 already_claimed=1
inactive_rider_claim=REJECTED order=ready_for_pickup fulfillment_count=0
complete_crosspoint_failure_rollback=PASS fulfillment=TO_USER order=ready_for_pickup
rider_residue riders=0 fulfillments=0 orders=0 items=0 earnings=0 dishes=0 rider_config=global:1000:rv1
```

## 3. 剩余风险

| 等级 | 风险 | 控制要求 |
| --- | --- | --- |
| 中 | ready 池当前为领域内只读全集，没有 HTTP 分页契约 | 对外接线时必须增加分页查询，不得直接暴露无限列表 |
| 中 | 事务交汇依赖同一 GORM 连接上的外层事务与内层 savepoint | 后续组合层不得替换为独立数据库连接；回归测试必须保留 |
| 中 | 抢单只控制在途上限，不校验地理距离或营业范围 | 这是本次冻结范围；不得用隐式算法扩展，需另行授权与可配治理 |
| 低 | 默认 1000 近似不限 | 控制机制已保留，后台接线后可通过 CAS 收紧 |
| 低 | MiniApp 既有 vendors.js 体积提示 | 与 G-⑤ 无关，不阻断领域验收 |

没有阻断 G-⑤ 验收的高风险问题。

## 4. 最终结论

**G-⑤ Rider Domain：PASS。**

开发停止在本模块；不合并 master、不打标签、不进入其它模块，等待 owner 验收。
