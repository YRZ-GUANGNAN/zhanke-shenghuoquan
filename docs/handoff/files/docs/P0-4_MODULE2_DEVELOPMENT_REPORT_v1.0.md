# P0-4 Module ② Development Report v1.0

日期：2026-06-21  
范围：Media Domain 数据模型、Repository、状态机与数据库迁移  
结论：Module ② 实现完成；未启动 Module ③～⑩。

## 1. 实现内容

建立独立 `backend/internal/module/media`：

- `model.go`：五张核心表的领域模型、枚举、校验规则和 JSON 值对象；
- `ulid.go`：基于 48-bit 毫秒时间戳与 80-bit CSPRNG 的 26 位 Crockford Base32 ULID；
- `state_machine.go`：媒体技术状态、存储阶段、引用、票据、配额与审核投影状态机；
- `repository.go`：唯一数据库访问边界，提供写入、查询、CAS 更新、引用计数重建和 Outbox 持久化；
- `service.go`：领域不变量校验与 Repository 编排，不包含外部基础设施；
- `controller.go`：仅作为进程内应用边界，不注册 HTTP 路由、不依赖 Gin。

媒体模块未导入用户、社区、交易或后台业务模块。边界检查已允许独立 Media Domain，并继续检查 Repository 之外的数据库访问。

## 2. 数据库变更

正向迁移：`backend/migrations/009_media_domain_v11.sql`  
回滚迁移：`backend/migrations/rollback/009_media_domain_v11.sql`

| 表 | 核心设计 |
| --- | --- |
| `media_assets` | ULID 公共 ID、owner、purpose、staging/final、技术状态、审核投影、可信摘要、尺寸、final key/version、TTL、软删除、`active_ref_count` 缓存和 `row_version` |
| `media_asset_references` | 引用事实源；双唯一约束、媒体域内 FK、引用状态、父对象可见性、聚合版本、撤销 fence 和 CAS |
| `media_upload_tickets` | 一资产一票据、声明大小/MD5/MIME、授权次数、过期时间、状态和 CAS；仅建立数据模型 |
| `media_quota_reservations` | 一票据一配额预留、字节/数量、终态结算和 CAS |
| `media_outbox` | event ID、aggregate/version/operation 幂等键、JSON payload、processed_at 和 CAS |

约束结果：

- 五张表均具备主键和 `row_version`；
- 10 组设计要求的命名唯一索引已建立；
- 3 个外键全部位于 Media Domain 内部；
- 对 users/posts/products 的跨域外键为 0；
- `active_ref_count` 没有通用写入口，只能由 Repository 统计 `media_asset_references.status=active` 后重建；GC 不得把缓存字段作为事实依据；
- 迁移发现旧记录缺失 ULID 时 fail-closed，要求先进行审计化 ULID 映射，禁止用截断旧 ID 伪造 ULID。

## 3. 状态机

技术状态严格实现冻结设计：

```text
pending_upload -> uploaded -> validating -> promoting -> final_ready
-> submitted -> reviewing -> approved/rejected/review_failed
-> delete_pending -> deleted/delete_deadletter -> deleted
```

同时保留设计中的失败分支，`deleted` 为终态；storage 只允许 `staging -> final`；票据 `consumed/expired/cancelled`、配额 `committed/released`、引用 `revoked` 均为终态。所有持久化状态变更使用 `row_version + expected status` CAS。

## 4. 本模块内部接口

本阶段没有新增 HTTP API。进程内接口包括：

- 资产记录创建、按 ULID 查询；
- 技术状态、存储阶段、审核投影 CAS；
- 引用记录创建、引用状态 CAS、活动引用计数重建；
- Repository 层的票据与配额持久化及状态 CAS 原语；
- Repository 层的 Outbox 追加、待处理查询和处理完成 CAS 原语。

上述票据、配额、审核投影和 Outbox 能力仅为数据层原语，没有暴露对应 Service/Controller 流程；没有实现 STS 签发、上传授权、审核编排或消费流程。

## 5. 测试结果

| 项目 | 结果 |
| --- | --- |
| MySQL 版本 | 8.4.0 |
| 009 正向迁移 | PASS |
| 009 回滚 | PASS |
| 回滚后 008 结构检查 | PASS；Module ② 子表/字段均为 0 |
| 带资产数据的回滚后再次正向迁移 | PASS；ULID、owner、purpose、row_version 保持正确 |
| 真实 MySQL Repository 集成测试 | PASS；资产/引用/票据/配额/Outbox/CAS 全通过 |
| 测试数据清理 | PASS；残留 0 |
| `go test ./...` | PASS |
| `npm run verify` | PASS |
| `git diff --check` | PASS |

## 6. 已知问题与未完成项

- 新模块按要求尚未接入 composition root，也未注册 HTTP；这是 Module ② 的范围约束。
- 旧 `internal/pkg/media` 保持未修改并仍由现有启动代码引用。Module ③ 接线前必须明确迁移所有权，避免旧 AutoMigrate 重新收紧兼容列；本阶段没有越权改动该链路。
- 当前开发库初始 `media_assets` 旧记录数为 0，因此无需 legacy ULID 回填。另以一条真实 v1.1 资产完成 rollback→reapply 数据保留测试。其他环境若存在非 ULID 旧媒体记录，009 会主动中止并要求审计化回填。
- STS、COS、上传、审核适配、MediaGateway、Inbox、GC、管理端与小程序均未实现。
- 先前 Gate-0 临时 STS 凭证已过期；本模块未读取或使用任何腾讯云凭证。

## 7. 修改文件

- `backend/internal/module/media/*.go`
- `backend/internal/module/media/*_test.go`
- `backend/migrations/009_media_domain_v11.sql`
- `backend/migrations/rollback/009_media_domain_v11.sql`
- `scripts/check-boundaries.cjs`
- 本报告及专项验收报告
