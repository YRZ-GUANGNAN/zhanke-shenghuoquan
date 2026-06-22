# P0-4 Module ⑦ Media Events Consumer Development Report v1.0

日期：2026-06-21  
模块：Media Events Consumer  
开发状态：**Development Complete**  
范围结论：仅完成 Module ⑦；未进入 Module ⑧～⑩。

## 1. 实现目标

本模块建立媒体域内部可靠事件消费链路：

```text
media_outbox
  -> Inbox 去重与 event lease
  -> aggregate checkpoint / aggregate lease
  -> 顺序与冲突判断
  -> Module ⑤ ReviewProjectionService
  -> checkpoint + Inbox + Outbox 原子确认
```

Consumer 不生产业务事件，不连接 Community、Trade、User 或外部审核厂商。

## 2. 数据模型

### 2.1 media_event_inbox

保存事件投递事实和处理状态：

- `event_id`：与 `media_outbox.event_id` 一对一；
- aggregate ID/version、operation、payload SHA-256；
- `pending/processing/retry_wait/succeeded/deadletter`；
- event lease、attempt count、next attempt、error code；
- `row_version` CAS。

### 2.2 media_consumer_checkpoints

保存 aggregate 顺序事实与单飞锁：

- `last_aggregate_version`、last event/operation/payload hash；
- `active_event_id`、aggregate lease token/expiry；
- `row_version` CAS。

同一 projection aggregate 同时最多一个事件进入 dispatch，避免多 worker 并行推进审核投影。

## 3. 状态机

允许的状态变化：

```text
pending -> processing
processing -> retry_wait
processing -> succeeded
processing -> deadletter
retry_wait -> processing
```

`succeeded` 与 `deadletter` 均为终态。processing lease 过期后允许以相同 expected status、旧 row_version 已失效的新 CAS 重新领取。

## 4. 事件契约

Module ⑦仅接受三种媒体内部 operation：

- `ReviewProjectionSubmitted`
- `ReviewProjectionReviewing`
- `ReviewProjectionDecision`

强制校验：

- Outbox `aggregate_id` 必须等于 payload `projectionId`；
- JSON 禁止未知字段和尾随内容；
- 每种 operation 只允许对应字段组合；
- decision 必须满足版本、状态和 SHA-256 格式要求；
- payload 与首次 Inbox hash 不一致时进入冲突处理。

## 5. 顺序、幂等与崩溃恢复

- checkpoint 只允许 `last_version + 1` 进入 Module ⑤ Service；
- 高版本提前到达时进入 `retry_wait`，不修改投影；
- 已消费的同版本同 operation/hash 作为重复投递确认，不再次调用投影服务；
- 同版本冲突内容进入 deadletter；
- 投影 Service 成功但 Consumer 确认前崩溃时，lease 到期后重新领取；Module ⑤幂等保证投影只产生一次状态效果；
- checkpoint、Inbox succeeded 和 Outbox processed 在同一 MySQL 事务内确认；
- MySQL 1205/1213 在 claim 阶段最多进行三次安全事务重试。

## 6. CAS 设计

所有已有记录的状态变更都包含 expected state 与 row_version：

- Inbox：`event_id + expected status + row_version`；
- checkpoint：`aggregate_id + active_event_id + lease_token + row_version`；
- Outbox：`event_id + processed_at IS NULL + row_version`。

Consumer 不直接更新 `media_assets` 或 `media_review_projections`。审核投影变化仅通过冻结的 `ReviewProjectionService`。

## 7. 重试与死信

- 乱序、projection 暂不存在和 stale row version 等可重试错误进入 `retry_wait`；
- 非法 payload、非法状态、任务/决策冲突等永久错误直接进入 `deadletter`；
- 达到 `MaxAttempts` 后进入 deadletter；
- deadletter 同步标记 Outbox processed，防止无限重复扫描；
- deadletter 不推进 aggregate checkpoint，后续高版本继续 fail-closed。

## 8. 数据库变更

新增：

- `backend/migrations/013_media_consumer.sql`
- `backend/migrations/rollback/013_media_consumer.sql`

013 仅引用媒体域 `media_outbox`，没有跨域外键。009～012 未修改。

## 9. 代码清单

生产代码：

- `consumer_types.go`
- `consumer_state_machine.go`
- `consumer_repository.go`
- `consumer_service.go`

测试：

- `consumer_state_machine_test.go`
- `consumer_service_test.go`
- `consumer_mysql_test.go`
- `consumer_boundary_test.go`

## 10. 测试结果

| 项目 | 结果 |
| --- | --- |
| migration / rollback / reapply | PASS |
| 完整 submitted→reviewing→approved | PASS |
| Inbox 单 event 去重 | PASS |
| Outbox 重复扫描 | PASS |
| 乱序等待并最终收敛 | PASS |
| stale projection row_version | PASS |
| stale Inbox row_version | PASS |
| 崩溃后 lease 重领 | PASS |
| aggregate 多 worker 单飞 | PASS |
| MySQL deadlock 安全重试 | PASS |
| 重试与 deadletter | PASS |
| 非法 payload / aggregate mismatch | PASS |
| MySQL 8.4 | PASS（8.4.0） |
| Boundary Check | PASS |
| 测试残留 | PASS（0） |
| `go test ./... -count=1` | PASS |
| `npm run verify` | PASS |

## 11. 未实现项

- 未接入真实审核厂商或审核回调；
- 未接入业务域 Outbox；
- 未注册后台常驻 worker、HTTP、Gin、管理端或小程序；
- 未实现 deadletter 管理页面或人工重放；
- 未实现 GC 或 Module ⑧～⑩内容。

## 12. 开发结论

Module ⑦开发完成。可靠投递、幂等、顺序、CAS、恢复和死信基础设施已形成闭环，并严格停留在媒体域内部。
