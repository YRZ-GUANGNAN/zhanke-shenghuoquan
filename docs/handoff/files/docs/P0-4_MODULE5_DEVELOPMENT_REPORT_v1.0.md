# P0-4 Module ⑤ Development Report v1.0

日期：2026-06-21  
模块：Review Projection（审核投影）  
开发结论：**Development Complete**  
范围结论：仅完成 Module ⑤；未启动 Module ⑥～⑩。

## 1. 实现内容

本模块建立了与媒体技术状态完全解耦的审核投影：

```text
final_ready 资产（只读前置条件）
  -> 创建 pending 投影
  -> pending -> submitted（绑定 review_task_id）
  -> submitted -> reviewing
  -> reviewing -> approved / rejected / review_failed
  -> 或 submitted -> review_failed
```

实现包括：

- 独立审核投影模型 `media_review_projections`；
- 不可变审核结果模型 `media_review_decisions`；
- 六态审核状态机及终态约束；
- Repository 事务与行锁边界；
- `expected_status + row_version` CAS；
- 审核任务、决策版本和 payload hash 幂等/冲突检查；
- Service 输入校验与 fail-closed 行为；
- 单元、状态机、Repository、并发及真实 MySQL 8.4 测试。

审核状态只写入 `media_review_projections`；`media_assets` 在整个审核投影流程中只读。没有修改资产的 `review_status`、`row_version`、技术状态、存储阶段、Final Key/Version、上传票据、配额或 COS 对象。

## 2. 数据库变更

新增迁移：

- `backend/migrations/012_media_review_projection.sql`
- `backend/migrations/rollback/012_media_review_projection.sql`

### 2.1 media_review_projections

核心字段：

- `projection_id`：ULID 主键；
- `asset_id`：媒体域内部外键，一项资产至多一个投影；
- `review_task_id`：统一审核任务 ID，唯一且可空；
- `status`：仅允许 `pending/submitted/reviewing/approved/rejected/review_failed`；
- `applied_decision_version`、`last_decision_hash`：决策去重与防乱序；
- `submitted_at/reviewing_at/decided_at`：状态时间；
- `row_version`：CAS 版本。

### 2.2 media_review_decisions

该表以追加方式持久化审核结果：

- `(review_task_id, decision_version)` 唯一；
- `(projection_id, decision_version)` 唯一；
- 保存 decision、payload hash、result code 和接收时间；
- 只允许 `approved/rejected/review_failed` 三种最终决策。

外键仅指向 `media_assets` 和 `media_review_projections`，没有 users、posts、products 或其他跨域外键。

## 3. 代码清单

新增生产代码：

- `review_projection_types.go`
- `review_projection_state_machine.go`
- `review_projection_repository.go`
- `review_projection_service.go`

新增测试：

- `review_projection_state_machine_test.go`
- `review_projection_service_test.go`
- `review_projection_mysql_test.go`

未修改 Module ②、③、④核心文件和数据库契约。

## 4. 内部接口

本模块没有新增 HTTP API 或 Gin Route。新增内部应用服务：

```go
EnsureProjection(ctx, assetPublicID)
Submit(ctx, projectionID, reviewTaskID, expectedRowVersion)
StartReview(ctx, projectionID, expectedRowVersion)
RecordDecision(ctx, input)
Find(ctx, projectionID)
```

所有写入都在 Repository 内完成；Service 不持有数据库句柄。

## 5. 一致性与安全行为

- 投影创建前强制核验资产为 `technical_status=final_ready`、`storage_stage=final`，且 Final Key/Version 已固化；
- 状态推进以共享锁复核资产仍为固定版本 `final_ready`，但只对投影执行写入；
- 投影写入同时校验 expected status 与 row_version；
- 重复提交和重复受理在语义一致时幂等返回，即使调用方携带的是首次调用前的版本；
- 重复的同版本同内容决策幂等返回，不新增事实行；
- 同版本不同 payload/decision/result code 返回冲突；
- 未知任务、非法跳转、技术状态漂移、Final identity 缺失均 fail-closed；
- 审核结果不会触发 CopyObject、删除对象、配额释放或上传票据变更。

## 6. 测试结果

| 测试项 | 结果 |
| --- | --- |
| 六态正向状态机 | PASS |
| 全量反向/非法跳转矩阵 | PASS |
| Service CAS 参数边界 | PASS |
| 决策 SHA-256/结果码校验 | PASS |
| migration forward | PASS |
| rollback + reapply | PASS |
| Repository 生命周期 | PASS |
| 重复创建与重复决策幂等 | PASS |
| 同版本冲突决策拒绝 | PASS |
| 技术状态漂移 fail-closed | PASS |
| 两路相同事件并发幂等（2 成功、仅推进 1 次） | PASS |
| stale row_version CAS | PASS |
| 真实 MySQL 8.4 | PASS（8.4.0） |
| 测试残留 | PASS（0） |
| `go test ./... -count=1` | PASS |
| `npm run verify` | PASS |

## 7. 已知问题与未完成项

- 本阶段按边界不接入统一审核系统、微信内容安全、HTTP、业务域或管理端；真实审核事件如何送入 Service 属于后续获得明确授权后的接线工作。
- GORM 会把预期的“记录不存在”探测输出为红色查询日志；这不影响事务、测试结果或数据残留。
- 未实现 Module ⑥ MediaGateway、Module ⑦ Consumer、Module ⑧ GC、Module ⑨ Admin、Module ⑩ MiniApp。

以上均不是 Module ⑤ 数据模型、状态机和持久化验收阻断项。

## 8. 开发结论

Module ⑤ 开发内容已完成并通过本地与真实 MySQL 8.4 验证。代码停在审核投影服务边界，没有自行进入 Module ⑥。
