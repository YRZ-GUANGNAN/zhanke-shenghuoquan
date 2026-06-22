# Media Purpose Takeout Acceptance Report v1.0

日期：2026-06-22

验收环境：真实 MySQL 8.4.0

专项结论：**PASS**

## 1. 验收矩阵

| 验收项 | 证据 | 结果 |
| --- | --- | --- |
| 实际迁移号 | 最大既有号 016；017 保留；采用 018 | PASS |
| 正向迁移 | 三值 CHECK 扩展为五值 | PASS |
| rollback | 无新引用时恢复三值 | PASS |
| rollback fail-closed | 存在 dish 引用时 Error 3819，扩展约束仍在 | PASS |
| 可逆性 | apply → rollback → apply | PASS |
| Purpose 枚举 | dish、merchant_logo 均通过 `Valid()` | PASS |
| 长度 | merchant_logo=13，满足 size:16 | PASS |
| dish 资产创建 | 既有 `Service.CreateAssetRecord` | PASS |
| dish 引用绑定 | 既有 `Service.CreateReferenceRecord` | PASS |
| dish 可见性 | final + approved 经 MediaGateway 返回 | PASS |
| merchant_logo 资产创建 | 既有 `Service.CreateAssetRecord` | PASS |
| merchant_logo 引用绑定 | 既有 `Service.CreateReferenceRecord` | PASS |
| merchant_logo 可见性 | final + approved 经 MediaGateway 返回 | PASS |
| purpose 列宽 | 迁移前后均为真实库既有 varchar(32) | PASS，未触碰 |
| 其它 schema | reference_type 列、索引、其它 CHECK 不变 | PASS |
| 临时约束残留 | takeout/rollback guard 均为 0 | PASS |
| 测试数据残留 | assets/references/projections/decisions 均为 0 | PASS |
| 全仓 Go Test | `go test ./... -count=1` | PASS |
| Workspace Verify | boundaries/typecheck/backend tests | PASS |

## 2. 真实 MySQL 结果

```text
real_mysql_version=8.4.0
rollback_with_new_reference=FAIL_CLOSED Error 3819
migration_apply_rollback_apply=PASS
purpose_column_unchanged=varchar(32)
reference_type_column=varchar(16)
temporary_guard_constraints=0
purpose=dish asset_create=PASS reference_bind=PASS projection=approved gateway_visible=PASS
purpose=merchant_logo asset_create=PASS reference_bind=PASS projection=approved gateway_visible=PASS
purpose_acceptance_residue assets=0 references=0 projections=0 decisions=0
```

## 3. 权限与边界

- 新值只进入 Media Domain 的既有 Purpose 类型；
- 没有修改 Gateway 可见性规则：仍要求 fixed final identity、`final_ready`、approved projection、active/visible reference；
- 没有绕过 Repository/Service 创建资产或绑定引用；
- 验收中的 final/review 状态仅用于构造既有 Gateway 验收事实，不形成新生产逻辑；
- 没有触碰 takeout 或任何 A～F 代码。

## 4. 剩余风险

| 等级 | 风险 | 处理 |
| --- | --- | --- |
| 中 | `media_assets.purpose varchar(32)` 与 model `size:16` 漂移仍存在 | 已登记至 68 项对齐专项，本任务禁止处理 |
| 低 | rollback 必须先清退新类型引用 | 已通过 fail-closed guard 防止静默丢数据或失去约束 |

无阻断本专项验收的高风险问题。

## 5. 最终结论

**Media Purpose Takeout Extension：PASS。**

迁移 018、应用枚举、两类真实 MySQL 资产/引用/Gateway 链路、可逆性和 fail-closed 均通过。提交后停止，等待 owner 验收；不自行合并或进入其它模块。
