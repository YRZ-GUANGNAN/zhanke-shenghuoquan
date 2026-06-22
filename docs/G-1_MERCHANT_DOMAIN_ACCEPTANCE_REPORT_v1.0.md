# G-① Merchant Domain Acceptance Report v1.0

日期：2026-06-22

验收对象：外卖域 G 商户领域核心

验收环境：真实 MySQL 8.4.0

专项结论：**PASS**

## 1. 验收矩阵

| 验收项 | 证据 | 结果 |
| --- | --- | --- |
| 迁移编号与隔离 | 仅新增 016 与对应 rollback；001～015 未改 | PASS |
| 真实迁移 | MySQL 8.4.0 上 apply → rollback → apply | PASS |
| 表结构 | 12 列类型、NULL/default、AUTO_INCREMENT 与冻结契约一致 | PASS |
| 索引与约束 | public ID 唯一；owner/status 索引；状态 CHECK | PASS |
| 跨域外键 | `takeout_merchants` 外键数 0 | PASS |
| 创建 | 生成有效 ULID，初始状态 `pending_review`，版本 1 | PASS |
| 合法转移 | 审核、营业/休息、封禁/解封全路径 | PASS |
| 拒绝终态 | `rejected` 后非法转移 fail-closed | PASS |
| Owner 隔离 | owner 不匹配返回 not found，不泄漏/更新实体 | PASS |
| CAS stale-write | 旧 `row_version` 被 `ErrConcurrentUpdate` 拒绝 | PASS |
| CAS 并发 | 同版本竞争两写：1 成功、1 冲突 | PASS |
| 幂等 | 已达目标态重复调用不更新版本 | PASS |
| 查询 | 按 `merchant_public_id` 查询准确 | PASS |
| Repository 边界 | 生产 GORM 访问仅在 Repository | PASS |
| HTTP/业务接线 | 无 HTTP、路由、Controller、A～F/Media import | PASS |
| 数据残留 | `takeout_merchants` 测试数据 0 | PASS |
| Go 回归 | `go test ./... -count=1` | PASS |
| Workspace verify | boundary/admin typecheck/miniapp typecheck/backend tests | PASS |
| Build | Go、MiniApp、Admin production build | PASS |

## 2. 真实 MySQL 8.4 结果

```text
real_mysql_version=8.4.0
migration_apply_rollback_apply=PASS
schema_contract=PASS
cross_domain_fk=0
lifecycle=pending_review->active->resting->active->suspended->active PASS
rejected_terminal=PASS
idempotency=PASS
stale_CAS=REJECTED
competing_CAS=PASS success=1 conflict=1
takeout_merchants_residue=0
```

真实测试使用现有开发库，只读取本机已配置 DSN，报告与日志不包含数据库口令。

## 3. 权限与安全边界

- G-① 不负责认证，但 owner 写能力在 Repository 事务内再次匹配 `owner_user_id`；
- 审核与管理能力目前仅为进程内领域方法，没有外部入口；
- public ID 与 owner/logo ID 均先做格式或非零校验；
- 非法状态、无效版本、错误 owner、未知实体均 fail-closed；
- 没有跨域 JOIN、跨域外键或直接访问 users/media 表。

## 4. 剩余风险

| 等级 | 风险 | 控制要求 |
| --- | --- | --- |
| 中 | G-① 的 admin 方法尚无 F 权限接线 | 在后续获授权模块中通过 F 服务边界接入；当前不得暴露 HTTP |
| 中 | 逻辑 owner/logo 引用不会由数据库验证存在性 | 后续组合层通过 A/Media 公共服务校验，禁止新增跨域 FK |
| 低 | ULID 极小概率冲突会返回唯一约束错误 | 上层未来可在创建命令边界做有限重试，本模块不扩展范围 |
| 低 | MiniApp 既有 vendors 体积告警 | 与 G-① 无关，留待独立前端治理，不阻断领域核心 |

无阻断 G-① 验收的高风险问题。

## 5. 最终结论

**G-① Merchant Domain：PASS。**

实体、状态机、Repository、Service、迁移/回滚、CAS、幂等、真实 MySQL 8.4 与边界验收均满足授权要求。开发在 G-① 停止；不合并 master，不打基线标签，不进入 G-②，等待 owner 验收。
