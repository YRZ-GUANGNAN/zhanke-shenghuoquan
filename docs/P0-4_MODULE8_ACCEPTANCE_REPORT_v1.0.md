# P0-4 Module ⑧ Acceptance Report v1.0

日期：2026-06-21  
验收对象：Media Garbage Collection  
最终结论：**PASS**

## 1. 验收矩阵

| 验收项 | 结果 | 证据摘要 |
|---|---|---|
| Migration / Rollback | PASS | 真实 MySQL 8.4.0 执行 rollback、forward，表/索引/约束验证通过 |
| 固定对象身份 | PASS | 空 key/versionId 拒绝；job 唯一绑定 key + versionId |
| COS 固定版本删除 | PASS | 真实 COS 同 key 两版本，仅目标版本消失，非目标版本仍可 HEAD |
| 引用事实源 | PASS | `active_ref_count=0` 但存在 active row 时拒绝；缓存为 999 且无引用事实时可删除 |
| 删除前重验 | PASS | Plan 后新增 pending reference，执行期 fail-closed，COS 调用次数为 0 |
| 审核状态与保留期 | PASS | final 删除要求终态审核投影、decided_at、delete_requested_at 和对应保留窗口 |
| Lease / CAS | PASS | 双 worker 仅一方 claim；stale row_version 在存储调用前拒绝 |
| 幂等重试 | PASS | 同一固定 versionId 重复删除不产生额外对象身份或误删其他版本 |
| 崩溃恢复 | PASS | 模拟 COS 已删、DB 未确认，lease 到期后重新认领并收敛 succeeded |
| staging 回收 | PASS | Promote 后仅删除固定 staging version，final asset 状态保持不变 |
| 辅助记录回收 | PASS | 独立 auxiliary job 取得 lease，以 CAS 清理已删除且过期的 terminal records |
| 边界隔离 | PASS | 无 HTTP/Gin/业务域引用；唯一外键指向 media_assets |
| 零残留 | PASS | Module ⑧测试 owner 数据为 0；真实 COS测试 key 的 version/delete marker 为 0 |
| 全量 Go Test | PASS | `go test ./... -count=1` |
| Type Check / Boundary / Build Gate | PASS | `npm run verify` |

## 2. 真实 MySQL 8.4 验收

- 数据库版本：MySQL 8.4.0。
- `014_media_gc.sql` 可执行。
- `rollback/014_media_gc.sql` 可执行，随后重新应用 forward migration 成功。
- 唯一约束可防止同一资产对象版本重复建 job，并防止同一资产重复建 auxiliary job。
- claim 使用 `FOR UPDATE SKIP LOCKED`；所有状态推进验证 status、row_version 和 lease_token。
- 引用判断读取并锁定 `media_asset_references` 实际行，不读取 `active_ref_count`。
- 验收后测试资产、引用、投影、票据、完成记录、配额和 GC job 残留为 0。

## 3. 真实腾讯云 COS 验收

真实环境检查结果：

- Bucket Versioning：Enabled。
- 同一测试 key 成功创建两个不同 versionId。
- `DeleteObject(key, targetVersionId)` 成功。
- targetVersionId 的固定版本 HEAD 返回 not found。
- survivorVersionId 的固定版本 HEAD 成功。
- 重复删除 targetVersionId 幂等成功。
- 清理后该测试 key 的 Version 与 DeleteMarker 残留均为 0。

报告及日志只保留 versionId、requestId 的脱敏指纹，未输出 SecretId、SecretKey 或完整对象身份。

## 4. 并发与崩溃恢复

- 两个并发 worker 竞争同一 pending job：仅一个取得 processing lease。
- job row_version 被外部推进后：旧 delivery 返回 `ErrGCLease`，未调用 COS。
- lease 过期后 processing job 可重新 claim。
- 模拟对象已删除但数据库未确认：再次执行同一个 key/versionId 后成功落库，不创建第二 job。

## 5. 安全与一致性结论

- final 删除要求资产 `delete_pending`、审核投影终态、保留窗口到期、无任何非 revoked 引用事实。
- staging 删除要求固定 staging version，并区分 Promote 后副本与 staging-only delete_pending 资产。
- COS 接口无法表达 latest/prefix/bucket 删除。
- 审核事实、引用事实和 GC tombstone 均保留；辅助记录回收同样受 lease + status + row_version + lease_token CAS 保护。
- 网络异常、状态漂移、引用变化、身份缺失均 fail-closed。

## 6. 冻结边界证明

迁移 009～013、Module ⑥/⑦核心文件 SHA-256 与开发前基线完全一致；Module ②～⑦既有生产逻辑和测试未修改。Boundary Check 输出：`ok A-F boundaries plus isolated P0-4 media domain`。

## 7. 剩余风险

### 中风险

1. 常驻调度进程不在本模块授权范围内。当前仅具备领域执行能力，尚不会自动周期运行。
2. COS 服务身份的云端 IAM 权限需在部署阶段持续审计；代码已强制固定版本，但云账号权限本身可能宽于本接口。

### 低风险

1. 腾讯云对重复固定版本删除返回成功而非统一的 NotFound，代码按幂等成功处理，并以固定版本 HEAD/List 验证实际结果。
2. 大规模积压时需由未来受控执行器配置批次和频率；单批上限已限制为 500。

无 P0/P1 阻断风险。

## 8. 最终结论

Module ⑧在授权范围内达到正式 **PASS**：数据库契约、固定版本删除、引用事实判断、lease/CAS、并发控制、崩溃恢复、幂等重试、辅助记录回收、真实 MySQL 8.4、真实腾讯云 COS、全量回归和零残留均通过。

开发在此停止。**未进入 Module ⑨或 Module ⑩。**
