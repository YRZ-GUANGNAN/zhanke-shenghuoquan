# P0-4 Module ⑤ Acceptance Evidence

日期：2026-06-21  
证据范围：Module ⑤ Review Projection  
敏感信息：本文件不包含 DSN、密码、Secret 或用户数据。

## 1. 真实 MySQL 8.4 与迁移证据

执行：

```text
go test ./internal/module/media \
  -run '^Test00ReviewProjectionMigrationAndRollbackAgainstMySQL84$' \
  -count=1 -v
```

脱敏结果：

```text
real database version: 8.4.0
schema contract: 5 required indexes, 2 media-only foreign keys
PASS rollback removed both Module ⑤ tables
PASS forward migration recreated both Module ⑤ tables
PASS
```

验证的索引：

- `uq_media_review_projection_asset`
- `uq_media_review_projection_task`
- `idx_media_review_projection_status`
- `uq_media_review_decision_task_version`
- `uq_media_review_decision_projection_version`

验证的外键目标仅为：

- `media_assets`
- `media_review_projections`

## 2. 状态机与 CAS 证据

允许的全部正向跳转：

| From | To | 结果 |
| --- | --- | --- |
| pending | submitted | PASS |
| submitted | reviewing | PASS |
| submitted | review_failed | PASS |
| reviewing | approved | PASS |
| reviewing | rejected | PASS |
| reviewing | review_failed | PASS |

其余 30 个状态组合均被拒绝；`approved/rejected/review_failed` 不允许再出站。

真实 MySQL 并发测试：两个完全相同的提交请求使用同一 `projection_id + review_task_id + expected_status=pending + row_version=1`。结果为：

```text
success=2
unexpected=0
final status=submitted
final row_version=2
```

另行把 projection `row_version` 从 1 并发推进到 2 后，使用旧版本提交返回 `ErrConcurrentUpdate`，状态保持 pending。

## 3. Repository 与事务证据

真实 MySQL 8.4 验证了：

- final_ready 资产创建 pending 投影；
- 重复 Ensure 返回同一 projection；
- submit、reviewing、approved 原子推进；
- 审核状态只写入 projection；
- 资产 `review_status/row_version/technical_status/storage_stage/final_key/final_version_id` 全程不变；
- 相同决策重复投递只保留一条 decision；
- 同 task/version 不同 payload hash 被拒绝；
- 非 final_ready 资产无法推进审核，失败后投影保持 pending/version=1。

## 4. 专项执行结果

```text
go test ./internal/module/media -run 'ReviewProjection' -count=1 -v

PASS Test00ReviewProjectionMigrationAndRollbackAgainstMySQL84
PASS TestReviewProjectionLifecycleAndRepositoryAgainstMySQL84
PASS TestReviewProjectionIllegalTransitionsAndAssetInvariantAgainstMySQL84
PASS TestReviewProjectionConcurrentIdempotencyAndCASAgainstMySQL84
PASS TestReviewProjectionDecisionConflictAndFailedPathAgainstMySQL84
PASS TestZZReviewProjectionDatabaseResidueAgainstMySQL84
PASS TestReviewProjectionServiceUsesExplicitCASBoundary
PASS TestReviewProjectionServiceValidatesDecisionFailClosed
PASS TestReviewProjectionStateMachineMatrix
PASS TestReviewProjectionRejectsUnknownStatus
```

全量回归：

```text
go test ./... -count=1       PASS
npm run verify               PASS
  check:boundaries           PASS
  admin typecheck            PASS
  miniapp typecheck          PASS
  backend go test ./...      PASS
```

## 5. 零残留证据

测试 owner 集合：`99_000_501`～`99_000_504`。测试结束后检查：

| 表 | 残留行数 |
| --- | ---: |
| media_assets | 0 |
| media_review_projections | 0 |
| media_review_decisions | 0 |

结论：**测试残留为 0**。

## 6. 边界隔离证据

- 新增生产文件仅位于 `backend/internal/module/media`；
- 新增迁移仅为 `012_media_review_projection.sql` 及其 rollback；
- 生产代码未导入 Gin、HTTP、业务域模块或 COS SDK；
- 新增代码未访问 Upload Ticket、Quota Reservation、Completion、Outbox、Inbox 或 GC；
- Repository 以外没有 GORM 数据访问；
- `npm run check:boundaries` 返回 `ok A-F boundaries plus isolated P0-4 media domain`；
- 未新增 HTTP Route、前端页面、业务接线或跨域外键。

## 7. 冻结基线证据

开发前后对 Gate-0、Module ②、③、④核心文件及 009～011 迁移执行 SHA-256 比对：

```text
PASS frozen baseline unchanged: 13 files
PASS Module 5 production forbidden-write/integration scan: 0 matches
```

锁定范围包括：

- Module ② `model.go/repository.go/state_machine.go`；
- Module ③ `upload_types.go/upload_repository.go/upload_service.go/upload_authorizer.go`；
- Module ④ `completion_types.go/completion_repository.go/completion_service.go`；
- `009_media_domain_v11.sql`、`010_media_upload_ticket_service.sql`、`011_media_upload_completion.sql`。

Module ⑤生产代码没有 `media_assets` UPDATE，也没有 Upload Ticket、Quota、COS、HTTP、Gateway、Consumer 或 GC 依赖。
