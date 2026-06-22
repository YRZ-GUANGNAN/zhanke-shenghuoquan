# P0-4 Module ⑧ Development Report v1.0

日期：2026-06-21  
模块：Media Garbage Collection  
开发结论：Development Complete

## 1. 实施边界

本次仅在 Media Domain 内实现 GC 领域能力，未新增 HTTP API、Gin Route、常驻调度进程、管理端、小程序页面、业务域接线、审核逻辑或 MediaGateway 扩展；未进入 Module ⑨、⑩。

迁移 009～013、Module ②～⑦ 的生产代码、状态机、Repository、Service 和既有测试均未修改。冻结文件 SHA-256 与开发前基线逐项一致。

## 2. 实现内容

### 2.1 GC Job 与状态机

新增 `media_gc_jobs`，记录固定对象身份、租约、重试、错误和删除回执；新增 `media_gc_auxiliary_jobs`，为数据库辅助记录回收提供独立 lease/CAS 事实。状态机为：

```text
pending -> processing -> succeeded
                     \-> retry_wait -> processing
                     \-> deadletter
```

`succeeded`、`deadletter` 均为终态。所有 job 状态变更使用 `expected_status + row_version + lease_token` CAS。

### 2.2 删除资格判定

- staging：仅允许删除资产中已固化的 `staging_key + staging_version_id`；支持 Promote 后过保留期的 staging 副本，以及已经进入 `delete_pending` 的 staging 资产。
- final：要求资产处于 `final + delete_pending`，存在固定 `final_key + final_version_id`，审核投影已进入 `approved/rejected/review_failed` 终态，且保留期已满。
- 引用事实：事务内锁定并检查 `media_asset_references`；`pending/active/revoking` 任一事实都会阻止 final 删除。
- `active_ref_count` 未参与任何 GC 资格或删除决策。
- 删除前在同一事务中重新锁定并校验 job、asset、reference facts、review projection、固定对象身份和保留窗口；任一条件变化立即 fail-closed。

### 2.3 COS 固定版本删除

新增窄接口 `GCObjectStore.DeleteVersion(key, versionId)`。腾讯云实现只调用：

```go
Object.Delete(ctx, key, &cos.ObjectDeleteOptions{VersionId: versionID})
```

接口不提供 latest、prefix、bucket 或批量删除能力；空 key 或空 versionId 在网络请求前被拒绝。

### 2.4 崩溃恢复与幂等

- `processing` job 带有 lease 和过期时间；崩溃后可由其他执行者重新认领。
- COS 删除成功但数据库提交前崩溃时，下一轮仍删除同一固定版本；腾讯云重复删除同一 versionId 返回幂等成功，job 随后收敛为 `succeeded`。
- 重试不会生成新对象身份，也不会删除 latest 或相同 key 的其他版本。

### 2.5 辅助记录回收

仅对已 `deleted` 且超过保留期的资产清理终态上传辅助记录，顺序为：quota reservation → upload completion → upload ticket。每个资产先以 `media_gc_auxiliary_jobs` 获取 lease，并以 status + row_version + lease_token CAS 完成；崩溃时事务回滚，过期 lease 可恢复。非终态记录、审核事实、引用事实及 GC tombstone 不删除；外键或状态不满足时事务整体回滚。

## 3. 数据库变更

- Forward：[014_media_gc.sql](../backend/migrations/014_media_gc.sql)
- Rollback：[rollback/014_media_gc.sql](../backend/migrations/rollback/014_media_gc.sql)

对象 job 唯一约束：`(asset_id, object_stage, object_key, object_version_id)`；辅助 job 对 asset 唯一。  
新增索引：object claim、asset/status、auxiliary claim。  
两张 GC 表的外键均只指向 `media_assets.id`，无跨域外键。

## 4. 生产代码清单

- `gc_types.go`：GC DTO、策略、对象存储窄接口
- `gc_state_machine.go`：状态机
- `gc_repository.go`：事务、lease、CAS、资格重验、辅助记录清理
- `gc_service.go`：Plan、批处理、重试与死信编排
- `gc_cos.go`：腾讯云 COS 固定版本删除适配器

## 5. 测试清单

- 状态机正反向测试
- 服务层固定版本参数测试
- fail-closed 与重试测试
- MySQL 8.4 migration/rollback/schema boundary 测试
- reference facts 与错误缓存值对照测试
- 删除前二次引用校验测试
- 双 worker lease 并发测试
- stale row_version CAS 拒绝测试
- 删除后崩溃恢复测试
- staging 清理与终态辅助记录回收测试
- 腾讯云 COS 双版本定向删除测试
- 边界与禁止项静态检查
- 数据库/COS 零残留检查

## 6. 已知限制

- 常驻调度与运行时接线按 Module ⑧范围明确禁止，本模块仅提供可被后续受控执行器调用的领域能力。
- 保留期数值由未来 composition root 注入；本模块拒绝零值或负值策略。
- COS 服务身份的控制台 IAM 最小权限仍属于部署配置责任；代码接口本身已限制为单 key、单 versionId 删除。

## 7. 未完成项

无 Module ⑧范围内未完成项。Module ⑨、⑩未启动。
