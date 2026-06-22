# P0-4 Module ⑨ Development Report v1.0

日期：2026-06-21  
模块：Admin Review Console  
开发状态：Development Complete

## 1. 实施边界

Module ⑨仅新增 Domain F 管理端对 Media Domain 既有事实的只读运营投影。未修改 Module ②～⑧生产代码、数据库契约、状态机、Repository、Service、COS 适配器、迁移 009～014或既有测试。

本阶段未实现：

- 人工重试；
- 人工写入审核结果；
- 直接修改 Asset、Projection、Decision、Checkpoint 或 GC Job；
- 新数据库迁移；
- 审核供应商联调；
- 常驻 Worker；
- 小程序上传流程；
- Module ⑩。

## 2. 后端实现

新增独立包 `backend/internal/module/admin/mediaconsole`：

- `types.go`：只读运营 DTO 与分页契约；
- `query_repository.go`：唯一 GORM 读取边界；
- `query_service.go`：过滤白名单、分页和时间窗校验；
- `handler.go`：只读 HTTP Handler。

Query Repository 只使用 `SELECT / COUNT / JOIN / UNION ALL`，不存在 Create、Save、Update、Delete 或 Exec。查询 DTO 不暴露：

- COS object key；
- versionId；
- Content-MD5；
- trusted SHA-256；
- lease token；
- Outbox payload；
- STS 或长期密钥。

## 3. HTTP API

所有路由均位于 `/api/v1/admin/media`，并统一经过：

```text
AuthRequired → AdminRequired → Read-only Handler
```

| Method | Path | 用途 |
|---|---|---|
| GET | `/summary` | 媒体、待审核、Consumer/GC 死信统计 |
| GET | `/assets` | Asset + Ticket + Completion + Projection 聚合列表 |
| GET | `/assets/:assetId` | 单资产完整运营事实详情 |
| GET | `/review-decisions` | 审核决策历史 |
| GET | `/events` | Outbox + Inbox 消费历史 |
| GET | `/gc-jobs` | Object/Auxiliary GC 执行历史 |
| GET | `/deadletters` | Consumer 与 GC 死信视图 |

未注册 POST、PUT、PATCH、DELETE。

## 4. 查询能力

支持的筛选维度：

- 精确 Asset Public ID；
- Technical/Projection/Consumer/GC 状态；
- Purpose；
- Review Task ID；
- Event Operation；
- GC Kind 与 object stage；
- RFC3339 开始/结束时间；
- 分页。

分页最大 100 条；时间窗同时指定时不得超过 366 天；状态、purpose、operation、kind 均使用上下文相关白名单。

资产详情聚合以下既有事实：

- `media_assets`；
- `media_asset_references`；
- `media_upload_tickets`；
- `media_upload_completions`；
- `media_review_projections`；
- `media_review_decisions`；
- `media_outbox`；
- `media_event_inbox`；
- `media_consumer_checkpoints`；
- `media_gc_jobs`；
- `media_gc_auxiliary_jobs`。

## 5. 管理端页面

新增“媒体运营”导航及只读控制台：

- 媒体资产总览；
- 待审核/死信指标；
- 资产列表和详情；
- 审核决策历史；
- 事件与消费状态；
- Consumer/GC 死信；
- GC 执行历史；
- 资产号、任务号、状态、时间范围筛选；
- 分页。

页面不存在审核、重试或 GC 操作按钮。

## 6. 权限模型

| 身份 | 结果 |
|---|---|
| 未认证 | 401 |
| 普通用户 | 403 |
| active 管理员 | 200，只读 |
| active 超级管理员 | 200，只读 |
| banned/muted 非 active 管理身份 | AdminRequired 拒绝 |

权限沿用 P0-1 实时用户状态、角色和 token 版本校验，不新增旁路权限模型。

## 7. 测试与验证

- Service 查询白名单与分页上限测试；
- HTTP 权限矩阵测试；
- GET-only route 测试；
- 内部错误脱敏测试；
- Repository 无写原语静态检查；
- DTO 敏感字段静态检查；
- 真实 MySQL 8.4 全事实聚合测试；
- 查询前后 row_version/status 不变测试；
- 数据库零残留测试；
- Admin TypeScript typecheck；
- Admin production build；
- 全量 Go Test；
- `npm run verify`。

## 8. 已知限制

- 本阶段按授权保持纯只读，因此死信只能查看，不能人工重试。
- 查询访问审计沿用现有 HTTP 日志；独立操作审计属于后续运维范围，本阶段未引入写库日志。
- 大数据量下 summary COUNT 仍依赖现有状态索引；未修改冻结索引契约。

## 9. 未完成项

无 Module ⑨授权范围内未完成项。开发在 Module ⑨结束后停止，未进入 Module ⑩。
