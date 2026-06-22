# P0-4 Module ③ Acceptance Report v1.0

日期：2026-06-21  
验收对象：Upload Ticket、Quota Reservation 与 STS 精确授权  
结论：**CONDITIONAL PASS**  
阶段状态：停留在 Module ③；禁止进入 Module ④～⑩。

## 1. 验收矩阵

| 验收项 | 结果 | 证据 |
| --- | --- | --- |
| 资产/票据一对一 | PASS | `uq_media_ticket_asset` |
| 票据/reservation 一对一 | PASS | `uq_media_quota_ticket` |
| 三记录同事务创建 | PASS | 真实 MySQL 故障回滚边界 |
| 原子配额预占 | PASS | owner 命名锁＋reservation 锁定/统计 |
| CAS | PASS | expected status＋row_version；stale write 拒绝 |
| credential_issue_count≤1 | PASS | DB CHECK＋CAS＋重复授权测试 |
| 幂等创建 | PASS | 相同声明返回同一 ticket/asset |
| 幂等冲突 | PASS | 不同声明返回 conflict |
| 过期处理 | PASS | ticket expired 与 quota released 同事务 |
| 新票据/新 staging Key | PASS | 过期后新幂等键产生新资产、票据、nonce Key |
| 用户权限边界 | PASS | 非 owner 操作拒绝 |
| 精确 staging Key | PASS | 服务端生成；authorizer 拒绝 final/路径逃逸 |
| 单 PUT 签名约束 | PASS（结构） | length/MD5/type/host/forbid-overwrite 全部进入签名 |
| 最小 STS 策略 | PASS（代码审计） | 仅 PutObject＋单 Key resource |
| 长期 Secret 不下发 | PASS | 返回 DTO 不含 SecretId/SecretKey；泄漏测试通过 |
| 真实 MySQL 8.4 | PASS | 生命周期、并发、迁移、回滚均真实执行 |
| 真实腾讯云 STS 本轮联调 | 未执行 | 环境无新测试配置；旧临时凭证已过期 |
| 无 HTTP/业务接线 | PASS | 无 Gin、无 Register、composition root 未变 |
| 无跨域外键 | PASS | 查询结果 0 |
| Module ④～⑩ 隔离 | PASS | 无 Copy/审核/Gateway/消费/GC/前端 |

## 2. 真实 MySQL 证据

```text
mysql_version                  8.4.0
module3_indexes                2
ticket_asset_unique            1
credential_limit_check         1
cross_domain_foreign_keys      0
acceptance_test_residue        0
```

并发验收：

```text
requests           20
quota limit         5
success             5
quota rejected     15
unexpected errors   0
persisted active    5
```

## 3. 生命周期验收

```text
create transaction       PASS
idempotent replay        PASS
declaration conflict     PASS
authorize once           PASS
second authorization     REJECTED
wrong owner              REJECTED
authorization_issued     row_version 1 -> 2
uploaded                 row_version 2 -> 3
consumed                 row_version 3 -> 4
reservation committed    row_version 1 -> 2
cancelled/expired         reservation released
```

## 4. 安全结论

- 返回对象中没有长期或临时 SecretKey；临时 token 只作为预签名 URL 的安全参数。
- 客户端不能更换 Key、长度、MD5、Content-Type 或覆盖头。
- STS policy 不包含读取、删除、列表、复制或 multipart 权限。
- 非 owner 无法读取或变更票据。
- 授权生成失败时票据 fail-closed 为 cancelled，并释放配额。
- 本模块没有把 `uploaded` 当作可信媒体或可展示媒体。

## 5. 自动化测试

```text
go test ./internal/module/media                         PASS
Module 3 real MySQL lifecycle tests                    PASS
Module 3 20-way concurrent quota test                  PASS
go test ./...                                          PASS
npm run verify                                         PASS
git diff --check                                       PASS
```

真实 STS 测试已保留为受环境变量控制的外部测试；本轮结果为 SKIP，而非 Mock PASS。

## 6. 剩余风险与关闭条件

| 等级 | 风险 | 关闭条件 |
| --- | --- | --- |
| 中 | 新 Module ③ authorizer 尚未使用新的真实腾讯云配置执行 STS 签发 | 配置临时测试环境所需的长期服务身份，运行 `TestTencentUploadAuthorizerAgainstRealSTS` 并保存脱敏证据 |
| 中 | `uploaded` 仅为声明，尚未进行 COS HEAD、MD5/格式和固定版本验证 | 只能在授权后的 Module ④ 实现，不得在本模块提前补做 |
| 低 | quota reservation 历史会增长 | 后续仅能由已批准 GC/保留策略处理；当前索引可支持 1～2 万用户规模 |
| 低 | 旧媒体写路径仍存在但未与新服务同时接线 | 后续接线时必须单写切换，禁止双写 |

## 7. 最终结论

**Module ③：CONDITIONAL PASS。**

数据库事务、票据状态机、CAS、幂等、原子配额、最小权限签名结构和范围隔离均通过；唯一未关闭条件是新适配器的真实腾讯云 STS 签发证据。由于该条件未完成，当前不得进入 Module ④，也不得注册 HTTP 或接入业务流程。
