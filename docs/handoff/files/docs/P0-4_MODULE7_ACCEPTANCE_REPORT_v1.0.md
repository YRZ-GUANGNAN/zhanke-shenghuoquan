# P0-4 Module ⑦ Media Events Consumer Acceptance Report v1.0

日期：2026-06-21  
验收对象：Outbox、Inbox、Consumer、Review Projection 可靠投递闭环  
最终结论：**PASS**  
后续状态：停止开发；Module ⑧～⑩继续冻结。

## 1. 验收结论

Module ⑦已建立媒体域内部 `Outbox → Consumer → ReviewProjectionService` 闭环。Inbox、event lease、aggregate checkpoint、aggregate lease、CAS、重试和 deadletter 共同保证重复、乱序、并发和崩溃情况下的 fail-closed 收敛。

Consumer 没有直接更新资产或审核投影；所有审核状态推进都通过冻结的 Module ⑤ Service。

## 2. 验收矩阵

| 验收项 | 结果 |
| --- | --- |
| Consumer 数据模型 | PASS |
| Inbox event_id 去重 | PASS |
| Outbox 扫描与确认 | PASS |
| 同一事件重复投递效果一次 | PASS |
| Outbox 重复扫描不重复处理 | PASS |
| aggregate 顺序保护 | PASS |
| aggregate 多 worker 单飞 | PASS |
| payload hash 冲突保护 | PASS |
| stale row_version 拒绝 | PASS |
| 崩溃恢复 | PASS |
| retry_wait | PASS |
| deadletter 终态 | PASS |
| Migration/Rollback | PASS |
| MySQL 8.4 | PASS |
| 无跨域外键 | PASS |
| Boundary Check | PASS |
| 零残留 | PASS |
| 全量 Go 测试 | PASS |
| npm verify | PASS |

## 3. 数据库验收

新增两张媒体域表：

- `media_event_inbox`
- `media_consumer_checkpoints`

唯一外键从 Inbox 指向媒体域 `media_outbox`。没有 users、posts、products 或其他跨域外键。

真实 MySQL **8.4.0** 已完成 rollback、forward、Repository 和并发验证。009～012 未修改。

## 4. Inbox 与幂等验收

同一 event ID 只对应一个 Inbox 行。重复交付分为：

- processing lease 未过期：其他 worker 不可领取；
- lease 过期：允许重领，attempt count 增长；
- checkpoint 已确认同版本同 payload：作为 duplicate 原子确认；
- 同版本不同内容：event conflict，进入 deadletter。

崩溃恢复测试确认，投影 Service 成功但确认前中断后，事件会重新调用 Module ⑤；Module ⑤幂等保证投影 row_version 不会重复增长。

## 5. 乱序与并发验收

checkpoint 仅允许 `last_version + 1` 进入投影 Service。高版本提前到达时进入 retry_wait。

aggregate lease 通过以下 CAS 边界保证单飞：

```text
aggregate_id
active_event_id
lease_token
row_version
```

真实两 worker 并发测试最终只产生一个 processing 事件。MySQL 1205/1213 采用有界事务重试，不绕过 CAS。

## 6. Consumer 状态机验收

允许：

```text
pending -> processing
processing -> retry_wait / succeeded / deadletter
retry_wait -> processing
```

状态矩阵完整枚举通过；succeeded/deadletter 无出站边。

## 7. 失败与死信验收

- 可恢复错误进入 retry_wait；
- 永久契约错误直接 deadletter；
- 达到最大尝试次数后 deadletter；
- deadletter 与 Outbox processed 同事务确认；
- deadletter 不推进 checkpoint，避免后续乱序事件破坏投影。

## 8. 安全与边界

- Consumer Service 没有 GORM 访问；
- Repository 是 Inbox、checkpoint、Outbox 唯一数据库边界；
- Consumer 对审核投影只调用 `ReviewProjectionService`；
- aggregate ID 必须匹配 payload projection ID；
- 严格 JSON 字段和 operation 契约；
- 无 HTTP、Gin、业务域、外部审核、COS、STS、Gateway 或 GC。

## 9. 自动化结果

```text
go test ./internal/module/media -run 'MediaConsumer' -count=1 -v  PASS
go test ./internal/module/media -count=1                          PASS
go test ./... -count=1                                            PASS
npm run verify                                                    PASS
```

详细脱敏证据见 `docs/evidence/P0-4_MODULE7_ACCEPTANCE_EVIDENCE_2026-06-21.md`。

## 10. 剩余风险

### 高风险

无 Module ⑦未关闭高风险。

### 中风险

无 Module ⑦未关闭中风险。

### 低风险

1. 当前 Consumer 是内部应用服务，按范围未注册常驻 worker；未来启动调度必须另行验收优雅停止、批量大小和运行指标。
2. deadletter 当前保留数据库事实但没有管理页面或人工重放工具；本阶段禁止管理端扩展。
3. 尚未接入真实审核来源；013 只提供可靠消费基础设施。

## 11. 最终判定

- Module ⑦ Development：**Complete**
- Module ⑦ Acceptance：**PASS**
- Delivery/Idempotency：**PASS**
- Ordering/Aggregate Lease：**PASS**
- Retry/Deadletter：**PASS**
- MySQL 8.4：**PASS**
- Boundary/Frozen Contract：**PASS**
- 是否进入 Module ⑧：**否**

Module ⑦在此停止，等待下一次专项授权。
