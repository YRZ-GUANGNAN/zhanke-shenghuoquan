# P0-4 Module ⑤ Acceptance Report v1.0

日期：2026-06-21  
验收对象：Review Projection 数据模型、状态机、持久化、Repository 与 Service  
最终结论：**PASS**  
后续状态：Module ⑤完成后停止；Module ⑥～⑩继续冻结。

## 1. 验收结论

Module ⑤已形成独立审核投影闭环：已固化的 final_ready 媒体资产可以创建审核投影，关联统一审核任务，按严格状态机推进，并持久化不可变审核结果。媒体技术状态与审核状态已解耦。

所有状态更新使用 `expected status + row_version` CAS，且只写入独立审核投影。`media_assets` 仅用于复核 final_ready 与固定 Final identity，不接收任何审核结果写入。任何状态、任务、版本或资产前置条件不一致都会 fail-closed。

## 2. 验收矩阵

| 验收项 | 结果 | 证据 |
| --- | --- | --- |
| 审核投影领域模型 | PASS | projection 一资产一行 |
| 审核结果持久化 | PASS | append-only decision，双唯一约束 |
| 六态约束 | PASS | DB CHECK + Go enum |
| 正向状态机 | PASS | 6 条允许边完整覆盖 |
| 反向/非法状态机 | PASS | 其余 30 个组合全部拒绝 |
| 终态保护 | PASS | approved/rejected/review_failed 无出站边 |
| expected status CAS | PASS | projection UPDATE 条件包含状态 |
| row_version CAS | PASS | stale projection version 被拒绝 |
| 并发幂等 | PASS | 2 路相同事件均成功，仅推进一次 |
| 决策幂等 | PASS | 相同 task/version/hash 仅一条记录 |
| 决策冲突 | PASS | 同版本不同内容拒绝且不新增记录 |
| fail-closed | PASS | 技术状态漂移后无任何审核状态写入 |
| migration | PASS | MySQL 8.4 执行成功 |
| rollback | PASS | 两表删除成功且可重建 |
| Repository | PASS | 生命周期与事务集成测试通过 |
| 数据残留 | PASS | 0 |
| 边界隔离 | PASS | 无跨域外键/路由/业务接线 |
| 全量 Go 回归 | PASS | `go test ./... -count=1` |
| 工程 verify | PASS | boundaries + typecheck + backend tests |

## 3. 数据库验收

真实数据库：**MySQL 8.4.0**。

执行顺序：

1. 检查 Module ⑤表为空；
2. 执行 rollback，确认 projection/decision 两表均不存在；
3. 执行 forward migration；
4. 确认两表重建；
5. 检查 5 个必需索引；
6. 检查外键只指向媒体域内部表；
7. 执行 Repository 生命周期与并发测试；
8. 清理并确认残留为 0。

迁移和回滚均为 PASS。没有创建 users、posts、products 等跨域外键。

## 4. 审核状态机验收

```text
pending -> submitted
submitted -> reviewing
submitted -> review_failed
reviewing -> approved
reviewing -> rejected
reviewing -> review_failed
```

`approved`、`rejected`、`review_failed` 为终态。状态机矩阵枚举了六个状态的 36 种组合，仅上述 6 种返回允许，其余全部拒绝。

投影创建还要求资产同时满足：

- `technical_status=final_ready`；
- `storage_stage=final`；
- Final Key 与 Final Version 非空；
- `review_status=pending`。

上述条件任一不满足，投影流程 fail-closed。

## 5. CAS 与并发验收

### 5.1 投影 CAS

所有投影推进 SQL 均包含：

```text
projection_id
expected status
expected row_version
```

### 5.2 资产 CAS

审核投影不会对 `media_assets` 执行 UPDATE。资产只以共享锁读取并验证 `technical_status=final_ready`、`storage_stage=final` 及固定 Final identity。

### 5.3 并发结果

真实 MySQL 两路相同 submit 使用同一 projection、review task 与旧 row_version：两个调用均幂等成功，但数据库只推进一次，投影版本从 1 精确增长至 2。独立 stale-version 用例通过模拟并发版本推进，确认旧 row_version 返回 `ErrConcurrentUpdate` 且状态不变。

## 6. 审核结果与幂等验收

- `review_task_id + decision_version` 唯一；
- `projection_id + decision_version` 唯一；
- 相同决策重复到达时返回当前投影，不重复写入；
- 同版本不同 decision、payload hash 或 result code 返回 `ErrDecisionConflict`；
- 低版本、未知任务和不匹配状态不会改变投影；
- payload hash 必须是 64 位十六进制 SHA-256；
- `review_failed` 必须携带失败码，approved/rejected 禁止携带失败码。

## 7. 技术状态解耦验收

审核全过程只允许更新：

- `media_review_projections` 自身状态与决策元数据；
- projection 自身的 `row_version/updated_at` CAS 元数据；
- 追加 `media_review_decisions` 事实行。

真实数据库测试确认以下资产字段从审核开始到 approved 均保持不变：

- `media_assets.technical_status=final_ready`；
- `media_assets.review_status=pending`；
- `media_assets.row_version`；
- `media_assets.storage_stage=final`；
- `media_assets.final_key`；
- `media_assets.final_version_id`。

没有写入 Upload Ticket、Quota Reservation、Completion，也没有 COS HEAD/GET/Copy/Delete 调用。

## 8. 自动化测试结果

```text
go test ./internal/module/media -run 'ReviewProjection' -count=1 -v  PASS
go test ./... -count=1                                             PASS
npm run verify                                                     PASS
```

专项共 10 项测试全部通过，包括 migration/rollback、Repository 生命周期、非法跳转、资产不变量、并发 CAS、决策冲突、Service 校验、完整状态矩阵和零残留。

详细脱敏证据见 `docs/evidence/P0-4_MODULE5_ACCEPTANCE_EVIDENCE_2026-06-21.md`。

## 9. 边界隔离证明

- 未修改 P0-1、P0-2、P0-3；
- 未修改 Module ②、③、④核心行为和数据库契约；
- 未修改 STS、Upload Ticket、Completion、CopyObject 或 Versioning；
- 未新增 HTTP API、Gin Router、前端、管理端或业务接线；
- 未启动 MediaGateway、Outbox/Inbox Consumer、GC；
- 新表外键仅指向媒体域内部表；
- boundary checker 与全量回归均通过。

## 10. 剩余风险

### 高风险

无 Module ⑤未关闭高风险。

### 中风险

无 Module ⑤未关闭中风险。

### 低风险

1. 本阶段禁止接入真实统一审核事件，因此当前验收覆盖投影接收边界，不代表微信内容安全真实环境联调已完成。
2. 预期的 GORM not-found 探测会输出红色日志，后续可在不改变行为的前提下调整测试日志级别；本阶段不做无关重构。

## 11. 最终判定

- Module ⑤ Development：**Complete**
- Module ⑤ Acceptance：**PASS**
- MySQL 8.4：**PASS**
- Migration/Rollback：**PASS**
- State Machine/CAS：**PASS**
- Repository/事务一致性：**PASS**
- Boundary Isolation：**PASS**
- 是否进入 Module ⑥：**否**

Module ⑤在此停止开发并等待专项验收确认。未经新授权，不进入 Module ⑥ MediaGateway 或任何后续模块。
