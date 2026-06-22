# G-① Merchant Domain Development Report v1.0

日期：2026-06-22

分支：`feat/g1-merchant`

基线：`master@bc309b6b`

范围：外卖域 G 第一模块，仅后端商户领域核心

结论：开发完成，未进入 G-②～G-⑦

## 1. 前置门禁

| 检查项 | 实际结果 | 结论 |
| --- | --- | --- |
| master 顶部提交 | `bc309b6b fix(infra): retire startup AutoMigrate...` | PASS |
| 基线标签 | `p0-4-media-baseline`、`infra-automigrate-retired` 均存在 | PASS |
| 当前最大迁移号 | `015`，开发前不存在 `016` | PASS |
| 启动期 DDL | `backend/cmd/server` 中 `db.AutoMigrate(` 为 0 | PASS |

## 2. 实现内容

在 `backend/internal/module/takeout` 建立独立领域核心：

- `model.go`：商户实体、领域错误、`MerchantStatus`、字段不变量和 `TableName()`；
- `ulid.go`：包内独立的 26 位 Crockford Base32 ULID 生成与校验；
- `state_machine.go`：冻结的商户状态转移图；
- `repository.go`：Repository 接口、GORM 实现、事务、行锁、CAS、幂等和 not-found 归一化；
- `service.go`：创建、审核结果、营业/休息、封禁/解封与查询领域能力；
- 单元测试、边界测试和真实 MySQL 8.4 集成测试。

没有新增 HTTP、Gin Route、Controller、Admin、小程序、支付、IM、F 审核流或任何业务接线。

## 3. 状态机

```text
pending_review -> active | rejected
active         -> resting | suspended
resting        -> active | suspended
suspended      -> active
rejected       -> terminal
```

所有状态写入均携带 `expected_status + expected_row_version`，SQL 更新条件同时匹配当前状态与版本，并执行 `row_version = row_version + 1`。目标状态已达时返回同一实体，不重复更新版本。

## 4. 数据库变更

- 正向迁移：`backend/migrations/016_takeout_merchant.sql`
- 回滚迁移：`backend/migrations/rollback/016_takeout_merchant.sql`
- 新表：`takeout_merchants`
- 主键：G 域内部 `BIGINT UNSIGNED AUTO_INCREMENT`
- 公共身份：`merchant_public_id CHAR(26)`，唯一索引
- 逻辑引用：`owner_user_id`、`logo_asset_public_id`，均无跨域外键
- 查询索引：owner、status
- 状态 CHECK：仅允许五个冻结状态

迁移 SHA-256：

```text
016 forward  a67e5aa2e890613192888f1e9a7dcf78b953e5d0e13755571e4b42e6b092e70f
016 rollback 6de68cc4d039d2929e74a4c53618df3791d17b5b5e081262de827b0731f0c5a8
```

迁移 `001～015` 未修改。

## 5. 内部接口

- `CreateMerchant`
- `Approve`
- `Reject`
- `OpenForBusiness`
- `Rest`
- `Suspend`
- `Unsuspend`
- `FindMerchant`

Owner 操作在 Repository 行锁查询和 CAS 更新中同时校验 `owner_user_id`。Admin/F 的身份认证和调用接线不属于本模块。

## 6. 边界控制

- G 的生产代码对 A～F、Media 的包导入为 0；
- G 的生产代码对 Gin/`net/http` 的依赖为 0；
- 迁移 016 的外键为 0；
- `main.go` 未修改，`AutoMigrate` 未恢复；
- `scripts/check-boundaries.cjs` 仅登记已获授权的 `takeout` 域并增加 G 核心禁止跨域/HTTP 导入检查，不包含业务实现。

## 7. 已知事项

- 本模块仅提供商户审核状态的领域能力，不接 F 统一审核流；接线留待后续专项授权。
- `owner_user_id` 和 `logo_asset_public_id` 只做逻辑引用；当前模块不读取 A 域或 Media 表。
- 前端构建保留既有 `vendors.js` 体积告警，不由 G-① 引入，也不影响本模块验收。
