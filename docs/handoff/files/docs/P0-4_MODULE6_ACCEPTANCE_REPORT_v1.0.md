# P0-4 Module ⑥ MediaGateway Acceptance Report v1.0

日期：2026-06-21  
验收对象：MediaGateway 只读 DTO、Repository 与 Service  
最终结论：**PASS**  
后续状态：停止开发；Module ⑦～⑩继续冻结。

## 1. 最终结论

Module ⑥已形成媒体域统一只读网关。Gateway 能从现有资产、审核投影和引用事实构建安全展示视图，并严格执行 final、approved、active、visible、无撤销 fence、未删除的可见性公式。

Gateway 不写数据库、不调用 COS/STS/审核或任何外部服务，不暴露内部存储身份。真实 MySQL 8.4、边界、零残留和全量工程回归均通过。

## 2. 验收矩阵

| 验收项 | 结果 |
| --- | --- |
| 统一可见性判断 | PASS |
| 已批准媒体元数据查询 | PASS |
| 审核状态投影读取 | PASS |
| 单引用展示视图 | PASS |
| 多图展示列表过滤 | PASS |
| 未批准媒体 fail-closed | PASS |
| hidden/revoked/fenced 引用 fail-closed | PASS |
| 删除中/非 final 媒体 fail-closed | PASS |
| DTO 不泄漏存储细节 | PASS |
| Repository 唯一数据库边界 | PASS |
| 数据库严格只读 | PASS |
| 真实 MySQL 8.4 | PASS |
| 零残留 | PASS |
| 冻结基线 | PASS |
| 全量 Go 回归 | PASS |
| npm verify | PASS |

## 3. 可见性验收

媒体展示要求同时满足：

1. 资产技术状态为 `final_ready`；
2. storage stage 为 `final`；
3. Final identity 已固定但不向 DTO 暴露；
4. 未进入删除流程；
5. 独立审核投影为 `approved`；
6. 引用状态为 `active`；
7. 父对象可见；
8. 不存在 revocation fence；
9. 引用 purpose 与资产 purpose 一致。

任一条件未知或不满足均不返回媒体元数据或展示视图。

## 4. Repository 与只读验收

Repository 使用显式字段白名单联合查询三张既有表。查询结果仅包含可见性所需事实，不读取 staging key、final key/version 内容、owner、ticket 或凭证。

静态扫描确认生产代码不存在数据库写方法、事务或锁。真实 MySQL 调用前后对资产、审核投影和引用记录逐字段比较，全部保持不变，包括 `row_version` 与 `updated_at`。

结论：**Strict Read Only PASS**。

## 5. DTO 安全验收

媒体展示 DTO 仅提供：

- 媒体公开 ID；
- purpose；
- MIME、字节数和尺寸；
- approved 审核状态；
- reference type、reference ID 和 slot。

未返回 Bucket、Region、COS URL、object key、version ID、STS、Secret、ticket、quota、owner 或审核任务 ID。

结论：**Storage Detail Isolation PASS**。

## 6. 真实 MySQL 8.4 验收

真实数据库版本：**MySQL 8.4.0**。

验收覆盖：

- Repository 三表聚合读取；
- 可见媒体完整读取链路；
- rejected、hidden、fenced 混合列表过滤；
- 投影状态读取；
- 调用前后数据不变；
- 测试清理与零残留。

结论：**PASS**。

## 7. 数据库与迁移验收

- Module ⑥没有新增迁移；
- 009～012 SHA-256 未变化；
- 没有新增表、列、索引、约束或跨域外键；
- 没有修改 Module ②～⑤ Repository 行为或状态机。

结论：**Frozen Contract PASS**。

## 8. 自动化测试

```text
go test ./internal/module/media -run 'MediaGateway' -count=1 -v  PASS
go test ./internal/module/media -count=1                         PASS
go test ./... -count=1                                           PASS
npm run verify                                                   PASS
```

详细脱敏证据见 `docs/evidence/P0-4_MODULE6_ACCEPTANCE_EVIDENCE_2026-06-21.md`。

## 9. 边界隔离

- 无 HTTP API、Gin Router 或 Controller 注册；
- 无管理端、小程序或业务域接线；
- 无 Consumer、Outbox 处理或 GC；
- 无上传、删除、Copy、STS 或审核调用；
- 无新增迁移；
- 无 Module ⑦～⑩实现。

## 10. 剩余风险

### 高风险

无 Module ⑥未关闭高风险。

### 中风险

无 Module ⑥未关闭中风险。

### 低风险

1. 当前展示视图只包含安全元数据，不提供实际媒体字节访问能力；后续若实现读取重定向，必须重新验收短时签名、固定 version 和权限边界。
2. 审核投影查询当前仅为内部 Service；未来暴露接口时必须在调用层增加用户/管理员授权，不能直接公开全部审核状态。

## 11. 最终判定

- Module ⑥ Development：**Complete**
- Module ⑥ Acceptance：**PASS**
- Strict Read Only：**PASS**
- Visibility Fail-Closed：**PASS**
- MySQL 8.4：**PASS**
- Boundary/Frozen Contract：**PASS**
- 是否进入 Module ⑦：**否**

Module ⑥在此停止并等待下一次专项授权。
