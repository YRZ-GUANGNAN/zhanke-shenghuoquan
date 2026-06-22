# G-③ Order Domain Acceptance Report v1.0

日期：2026-06-22

验收对象：外卖域 G 订单主链路领域核心

实现提交：`95bdabbe411428e06eae14e107c825083c1ebc80`

真实环境：MySQL 8.4.0

专项结论：**PASS**

## 1. 验收矩阵

| 验收项 | 证据 | 结果 |
| --- | --- | --- |
| 迁移隔离 | 仅新增 019 与 rollback；001～018 未改 | PASS |
| 真实迁移 | MySQL 8.4.0 apply → rollback → apply | PASS |
| 表结构 | 四表字段、类型、索引、CHECK 与默认值符合契约 | PASS |
| 种子配置 | `global,100,50,row_version=1` | PASS |
| 零外键 | 四表外键总数 0 | PASS |
| 金额纪律 | 全部整数分、服务端重算、乘加溢出保护 | PASS |
| Catalog 校验 | 仅 on_shelf 且属于订单商户的菜品可下单 | PASS |
| 多菜品快照 | 名称、单价、数量与行金额正确落库且后续不可变 | PASS |
| 订单事务 | order 与全部 items 同事务创建 | PASS |
| 完整状态图 | 九状态全部合法边已验证 | PASS |
| 非法/终态转移 | fail-closed | PASS |
| stale CAS | 返回 `ErrConcurrentUpdate` | PASS |
| 竞争 CAS | 同版本 Accept/Refund：1 成功、1 冲突 | PASS |
| 状态幂等 | 重复支付、重复完成不增加版本或副作用 | PASS |
| 完成落应结 | completed 与 earning 同事务 | PASS |
| 一单一条 | 重复 Complete 后 earning count=1 | PASS |
| 费率快照 | 完成时当前配置 `120/60` 正确写入 earning | PASS |
| 费率 CAS | 正常更新、stale 拒绝 | PASS |
| 结算 CAS | pending→settled，重复调用幂等 | PASS |
| 查询 | FindOrder、ListOrderItems、FindEarningByOrder | PASS |
| Repository 边界 | 生产数据库访问保持在 Repository | PASS |
| HTTP/跨域 | 无 HTTP、Route、A～F/Media import 或业务接线 | PASS |
| 数据残留 | orders/items/earnings/test dishes=0，fee seed 已恢复 | PASS |
| Go 回归 | `go test ./... -count=1` | PASS |
| 静态与构建 | `go vet ./...`、`go build ./...` | PASS |
| Workspace verify | boundary、双端 typecheck、backend tests | PASS |
| Production build | MiniApp、Admin | PASS |

## 2. 真实 MySQL 结果

```text
real_mysql_version=8.4.0
migration_019_apply_rollback_apply=PASS
tables=4
foreign_keys=0
fee_seed=global:100:50
multi_dish_snapshot=PASS
server_amount_recalculation=PASS
lifecycle_all_paths=PASS
stale_CAS=REJECTED
terminal_fail_closed=PASS
complete_earning_same_tx=PASS
repeated_complete_earning_count=1
fee_snapshot=120:60
settlement=PASS
competing_order_CAS=PASS success=1 conflict=1
order_residue orders=0 items=0 earnings=0 catalog_test_dishes=0 fee_config=global:100:50:rv1
```

真实测试使用现有 MySQL 8.4 开发库；DSN 仅从本机环境读取，未进入日志、报告或 Git。

## 3. 状态机验收

| From | To | 结果 |
| --- | --- | --- |
| pending_payment | paid | PASS |
| pending_payment | cancelled | PASS |
| paid | accepted | PASS |
| paid | refunded | PASS |
| accepted | preparing | PASS |
| accepted | refunded | PASS |
| preparing | ready_for_pickup | PASS |
| ready_for_pickup | delivering | PASS |
| delivering | completed | PASS |
| completed/cancelled/refunded | 任意状态 | REJECTED |

## 4. 边界验收

```text
production Order cross-domain imports: 0
production Order HTTP/route imports: 0
production Order float/decimal usage: 0
migration 019 SQL foreign keys: 0
migration 019 SQL float/decimal types: 0
backend/cmd/server db.AutoMigrate count: 0
apps/** changes: 0
001-018 migration changes: 0
```

## 5. 剩余风险

| 等级 | 风险 | 控制要求 |
| --- | --- | --- |
| 中 | Catalog 只读校验与订单事务间不持有同一行锁 | 当前 Catalog 无库存扣减语义；未来库存/限购必须单独设计，禁止隐式扩展 G-③ |
| 中 | 支付、退款、接单、配送边目前仅是进程内领域能力 | G-④/G-⑤ 获授权前不得暴露外部触发入口 |
| 中 | 完成时使用“当前”费率，可能不同于下单时 delivery fee | 这是本次冻结规则；运营修改费率需保留审计，后续不得无迁移改变结算口径 |
| 低 | ULID 极低概率唯一冲突 | 唯一索引 fail-closed；有限重试留给未来命令入口 |
| 低 | MiniApp 既有 vendors 体积提示 | 与 G-③ 无关，不阻断领域验收 |

无阻断 G-③ 验收的高风险问题。

## 6. 最终结论

**G-③ Order Domain：PASS。**

开发已停止在 G-③；分支不合并 master、不打基线标签、不进入 G-④，等待 owner 验收。

