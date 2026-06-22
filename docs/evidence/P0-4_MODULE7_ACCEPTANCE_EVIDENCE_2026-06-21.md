# P0-4 Module ⑦ Acceptance Evidence

日期：2026-06-21  
范围：Media Events Consumer  
敏感信息：不包含 DSN、Secret、COS identity 或用户数据。

## 1. 真实 MySQL 8.4 与迁移

```text
real database version: 8.4.0
PASS rollback removed media_event_inbox and media_consumer_checkpoints
PASS forward migration recreated both tables
PASS only media_outbox foreign key exists
```

009～012 SHA-256 复核：

```text
PASS migrations 009-012 frozen hashes unchanged
```

## 2. 专项测试

执行：

```text
go test ./internal/module/media -run 'MediaConsumer' -count=1 -v
```

结果：

```text
PASS Test00MediaConsumerMigrationAndRollbackAgainstMySQL84
PASS TestMediaConsumerLifecycleAndRepeatedScanAgainstMySQL84
PASS TestMediaConsumerCrashRecoveryAndInboxDedupAgainstMySQL84
PASS TestMediaConsumerOutOfOrderEventuallyConvergesAgainstMySQL84
PASS TestMediaConsumerStaleCASAndDeadletterAgainstMySQL84
PASS TestMediaConsumerInboxCASRejectsStaleDeliveryAgainstMySQL84
PASS TestMediaConsumerAggregateLeaseSerializesWorkersAgainstMySQL84
PASS TestMediaConsumerMalformedEventDeadlettersAgainstMySQL84
PASS TestZZMediaConsumerDatabaseResidueAgainstMySQL84
PASS TestMediaConsumerBoundaryIsolation
PASS TestMediaConsumerRepositoryCASGuardsArePresent
PASS TestMediaConsumerStateMachineMatrix
PASS TestMediaConsumerTerminalStatesHaveNoOutboundTransition
```

## 3. 可靠投递证据

三条 Outbox 事件按 aggregate version 1、2、3 被处理，审核投影最终为：

```text
status=approved
row_version=4
checkpoint.last_aggregate_version=3
inbox.succeeded=3
outbox.processed=3
```

再次扫描结果 `claimed=0`，没有重复处理。

## 4. 崩溃恢复与幂等

测试在 Module ⑤ Submit 成功后故意不确认 Inbox/Outbox，模拟进程崩溃。lease 过期后：

```text
inbox.attempt_count=2
inbox rows for event=1
projection.status=submitted
projection.row_version=2
```

说明事件被重新投递，但 Module ⑤幂等使投影只推进一次；最终 checkpoint、Inbox 和 Outbox 正常确认。

## 5. 乱序保护

version 2 先到达时：

```text
inbox.status=retry_wait
projection.status=pending
checkpoint.last_version=0
```

version 1 到达并成功后，version 2 重试并收敛到 `reviewing/row_version=3`。乱序事件没有提前改变投影。

## 6. aggregate 并发单飞

两个真实 MySQL worker 并发领取同一 projection 的 version 1、2：

```text
claimed=1
processing inbox=1
active aggregate lease=1
unexpected errors=0
```

测试实际触发一次 MySQL 1213，Repository 安全重试后正确收敛，没有两个事件同时进入 dispatch。

## 7. CAS 与死信

- stale projection expected row_version：投影保持 pending/version 1，Inbox 进入 deadletter；
- stale Inbox row_version：返回 lease/CAS 拒绝；
- 非法 payload：直接 deadletter，error code=`invalid_payload`；
- deadletter 为终态，Outbox 不再重复扫描。

## 8. 边界证据

```text
PASS Module 7 forbidden dependency/direct-table scan: 0 matches
PASS npm run check:boundaries
```

生产代码没有 `media_assets`、`media_review_projections` 直接访问，没有 Community、Trade、User、HTTP、Gin、COS、STS、审核 SDK、MediaGateway 或 GC 依赖。

## 9. 零残留

验收 owner：`99_000_701`～`99_000_707`。

| 表 | 残留 |
| --- | ---: |
| media_assets | 0 |
| media_review_projections | 0 |
| media_review_decisions | 0 |
| media_outbox | 0 |
| media_event_inbox | 0 |
| media_consumer_checkpoints | 0 |

## 10. 全量回归

```text
go test ./internal/module/media -count=1  PASS
go test ./... -count=1                    PASS
npm run verify                            PASS
  check:boundaries                        PASS
  admin typecheck                         PASS
  miniapp typecheck                       PASS
  backend go test ./...                   PASS
```
