# P0-4 Module ② Acceptance Report v1.0

日期：2026-06-21  
验收对象：Media Domain 数据模型与迁移  
专项结论：**PASS**  
项目阶段：停留在 P0-4 Module ②，等待专项确认；未授权 Module ③。

## 1. 验收矩阵

| 验收项 | 证据 | 结果 |
| --- | --- | --- |
| 独立 `module/media` | controller/service/repository/model/state_machine 独立包 | PASS |
| 五张核心表 | 当前开发库确认 5/5 存在 | PASS |
| 主键与唯一约束 | 5 个主键；10 组要求的命名唯一索引 | PASS |
| Media 内部外键 | reference→asset、ticket→asset、quota→ticket，共 3 个 | PASS |
| 禁止跨域外键 | Media→users/posts/products 数量为 0 | PASS |
| CAS | 五张表均有 `row_version`；状态更新匹配 expected version/status | PASS |
| ULID | 26 位大写 Crockford Base32；时间排序与非法字符测试通过 | PASS |
| 状态机 | 冻结技术状态图及非法跳转测试通过 | PASS |
| 引用事实源 | 引用表为唯一事实源；计数仅由活动引用重建 | PASS |
| Repository 边界 | 生产代码仅 `repository.go` 持有/使用 GORM DB | PASS |
| 正向迁移 | MySQL 8.4.0 真实执行成功 | PASS |
| 回滚迁移 | 真实执行成功，Module ② 子表及新增字段清零 | PASS |
| 再迁移 | 带一条 v1.1 资产回滚后重新执行 009，ULID/owner/purpose/version 保留；最终保持 009 已应用 | PASS |
| Repository 集成 | 五类记录、FK、CAS stale-write 拒绝、计数重建通过 | PASS |
| 全仓 Go Test | `go test ./...` 全通过 | PASS |
| 前后端 Verify | 边界检查、Admin/Miniapp TypeScript、后端测试全通过 | PASS |
| 范围隔离 | 无 HTTP、STS、上传、审核调用、COS Copy、Gateway | PASS |

## 2. 数据库验收证据

真实环境：本机 MySQL 8.4.0，项目开发库。

```text
module2_tables                 5
cas_columns                    5
media_foreign_keys             3
cross_domain_foreign_keys      0
required_unique_indexes       10
acceptance_test_residue        0
rollback_child_tables          0
rollback_v11_columns           0
```

执行序列：

```text
008 baseline -> 009 PASS -> schema assertions PASS
-> rollback 009 PASS -> rollback assertions PASS
-> persisted asset round-trip -> 009 PASS
-> repository integration PASS -> test residue 0
```

## 3. 自动化测试结果

```text
go test ./...
PASS: all backend packages

npm run verify
PASS: module boundaries
PASS: admin typecheck
PASS: miniapp typecheck
PASS: backend tests
```

专项测试覆盖：ULID 规范与排序、全部允许/禁止状态跳转、非法跳转不访问 Repository、初始状态不变量、JSON payload 校验、真实 MySQL FK、唯一约束、CAS stale-write 拒绝、活动引用计数重建、Outbox 处理 CAS。

## 4. 权限与边界结论

- Media Repository 不读取或写入 A/B/C 表；owner/reference ID 均为不带跨域外键的标识值。
- `media_asset_references` 是引用事实源；`active_ref_count` 仅是可重建缓存。
- 没有新增路由、外部 SDK、网络调用、后台页面或小程序逻辑。
- 没有使用已过期的 Gate-0 STS 临时凭证。

## 5. 剩余风险

| 等级 | 风险 | 控制要求 |
| --- | --- | --- |
| 中 | 旧 `internal/pkg/media` 与新模块暂时并存，现有 composition root 仍引用旧模型 AutoMigrate | Module ③ 接线前先完成迁移所有权检查；不得让旧模型覆盖 009 的可空兼容列，不得同时启用两套写路径 |
| 中 | 其他环境若有缺失 ULID 的旧媒体记录，009 将 fail-closed | 先导出记录、生成真实 ULID 映射并审计，再重跑迁移；禁止截断旧 ID 冒充 ULID |
| 低 | Module ② 只验证单库 Repository，尚无 Outbox 消费者 | 属于后续已冻结模块范围，本阶段不得提前实现 |
| 低 | STS 临时凭证已过期 | Module ③ 真实联调前重新签发；不得持久化临时密钥 |

不存在阻断 Module ② 验收的高风险问题。

## 6. 最终结论

**P0-4 Module ②：PASS。**

数据模型、迁移/回滚、Repository、状态机、CAS、引用事实源与模块边界均满足本阶段要求。开发在此停止；未获得新的专项授权前，不进入 Module ③ Upload Ticket 与 STS 签发服务。
