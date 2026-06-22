# G-② Catalog Domain Acceptance Report v1.0

日期：2026-06-22

验收对象：外卖域 G Catalog 后端领域核心

实现提交：`9a021c88`

真实环境：MySQL 8.4.0

专项结论：**PASS**

## 1. 验收矩阵

| 验收项 | 证据 | 结果 |
| --- | --- | --- |
| 迁移隔离 | 仅新增 017 与 rollback；001～016、018 未改 | PASS |
| 真实迁移 | MySQL 8.4.0 apply → rollback → apply | PASS |
| 表结构 | 两表字段、类型、空值、默认值、索引与 CHECK 符合契约 | PASS |
| 零外键 | 两张 Catalog 表外键总数 0 | PASS |
| 金额纪律 | `price_cents BIGINT UNSIGNED` / Go `uint64`，且必须大于 0 | PASS |
| 分类能力 | 创建、改名、排序、查询 | PASS |
| 菜品创建/更新 | draft 初始化、详情 CAS 更新、逻辑引用格式校验 | PASS |
| 完整状态图 | 所有冻结合法边均已验证 | PASS |
| 非法转移 | fail-closed | PASS |
| removed 终态 | 后续转移拒绝 | PASS |
| stale CAS | 返回 `ErrConcurrentUpdate` | PASS |
| 竞争 CAS | 同版本两写 1 成功、1 冲突 | PASS |
| 幂等 | 已达目标态/内容不重复写、不增加版本 | PASS |
| Merchant 隔离 | merchant 不匹配不泄漏、不修改实体 | PASS |
| Repository 边界 | 生产 GORM 访问保持在 Repository | PASS |
| Media 边界 | 无 media import、调用、绑定或展示 | PASS |
| HTTP/业务接线 | 无 HTTP、Route、Controller、Admin、MiniApp、F/E/支付接线 | PASS |
| 数据残留 | categories=0，dishes=0 | PASS |
| Go 回归 | `go test ./... -count=1` | PASS |
| 静态/构建 | `go vet ./...`、`go build ./...` | PASS |
| Workspace verify | boundary、双端 typecheck、backend tests | PASS |
| Production build | MiniApp、Admin | PASS |

## 2. 真实 MySQL 结果

```text
real_mysql_version=8.4.0
migration_017_apply_rollback_apply=PASS
tables=2
foreign_keys=0
media_018_unchanged=PASS
category_create_rename_reorder_find=PASS
dish_integer_cents=PASS
lifecycle_all_paths=PASS
stale_CAS=REJECTED
idempotency=PASS
competing_dish_CAS=PASS success=1 conflict=1 final_status=off_shelf
catalog_residue categories=0 dishes=0
```

真实测试读取本机已配置 DSN，但报告、日志和 Git 变更均不包含口令。

## 3. 状态机验收

| From | To | 结果 |
| --- | --- | --- |
| draft | on_shelf | PASS |
| on_shelf | sold_out | PASS |
| on_shelf | off_shelf | PASS |
| sold_out | on_shelf | PASS |
| sold_out | off_shelf | PASS |
| off_shelf | on_shelf | PASS |
| off_shelf | removed | PASS |
| removed | 任意状态 | REJECTED |

## 4. 边界验收

```text
production Catalog media imports: 0
production Catalog HTTP/route imports: 0
migration 017 FOREIGN KEY/REFERENCES: 0
backend/cmd/server db.AutoMigrate count: 0
apps/** changes: 0
pre-existing migration changes: 0
```

## 5. 剩余风险

| 等级 | 风险 | 控制要求 |
| --- | --- | --- |
| 中 | 逻辑引用不由数据库验证存在性 | 后续仅通过获授权组合层调用公开领域服务校验，禁止跨域 FK/JOIN |
| 中 | Catalog 尚无 HTTP 与权限入口 | 在 G 后续模块授权前保持不可达，不得临时暴露 |
| 低 | ULID 唯一冲突由唯一索引拒绝 | 未来可在创建命令边界增加有限重试，不在 G-② 扩围 |
| 低 | 前端既有 vendors 体积提示 | 与 G-② 无关，不阻断领域验收 |

无阻断本模块的高风险问题。

## 6. 最终结论

**G-② Catalog Domain：PASS。**

开发已停止在 G-②；分支不合并 master、不打基线标签、不进入 G-③，等待 owner 验收。

