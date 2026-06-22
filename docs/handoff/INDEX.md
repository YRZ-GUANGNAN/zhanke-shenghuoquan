# Campus Circle Handoff Index

- 导出日期：2026-06-22（Asia/Shanghai）
- 任务性质：只读文档与源码交接导出
- 数据库快照：真实 MySQL 8.4，仅结构，`mysqldump --no-data --skip-comments`，无业务数据
- 安全结论：**已确认无凭证**；未包含 `.env`、`project.private.config.json`、AppSecret、STS/COS Secret、数据库口令、JWT/token、私钥或签名 URL
- 排除项：`node_modules`、构建产物、二进制、运行日志、临时测试文件
- 项目根目录 `MEMORY.md`：未找到，因此未导出；未使用或复制 Codex 私有记忆目录

## 内容统计

- 文档：51
- 迁移与回滚：20
- Media Domain Go 源码与测试：65
- 索引所列交接文件（不含 INDEX 与 ZIP）：145

## 文件索引

| 路径 | 用途 |
|---|---|
| `files/apps/admin/package.json` | Node workspace 依赖与脚本契约 |
| `files/apps/miniapp/package.json` | Node workspace 依赖与脚本契约 |
| `files/backend/go.mod` | Go 模块依赖契约 |
| `files/backend/go.sum` | Go 依赖完整性锁定 |
| `files/backend/internal/module/media/completion_cos_external_test.go` | Media Domain 单元、集成或边界测试模板 |
| `files/backend/internal/module/media/completion_cos_versioning_external_test.go` | Media Domain 单元、集成或边界测试模板 |
| `files/backend/internal/module/media/completion_mysql_test.go` | Media Domain 单元、集成或边界测试模板 |
| `files/backend/internal/module/media/completion_repository.go` | Media Domain Repository 边界模板 |
| `files/backend/internal/module/media/completion_service.go` | Media Domain Service 模板 |
| `files/backend/internal/module/media/completion_test.go` | Media Domain 单元、集成或边界测试模板 |
| `files/backend/internal/module/media/completion_types.go` | Media Domain 类型与模型模板 |
| `files/backend/internal/module/media/consumer_boundary_test.go` | Media Domain 单元、集成或边界测试模板 |
| `files/backend/internal/module/media/consumer_mysql_test.go` | Media Domain 单元、集成或边界测试模板 |
| `files/backend/internal/module/media/consumer_repository.go` | Media Domain Repository 边界模板 |
| `files/backend/internal/module/media/consumer_service.go` | Media Domain Service 模板 |
| `files/backend/internal/module/media/consumer_service_test.go` | Media Domain 单元、集成或边界测试模板 |
| `files/backend/internal/module/media/consumer_state_machine.go` | Media Domain 状态机模板 |
| `files/backend/internal/module/media/consumer_state_machine_test.go` | Media Domain 单元、集成或边界测试模板 |
| `files/backend/internal/module/media/consumer_types.go` | Media Domain 类型与模型模板 |
| `files/backend/internal/module/media/controller.go` | Media Domain 模板源码 |
| `files/backend/internal/module/media/gateway_boundary_test.go` | Media Domain 单元、集成或边界测试模板 |
| `files/backend/internal/module/media/gateway_repository.go` | Media Domain Repository 边界模板 |
| `files/backend/internal/module/media/gateway_repository_mysql_test.go` | Media Domain 单元、集成或边界测试模板 |
| `files/backend/internal/module/media/gateway_service.go` | Media Domain Service 模板 |
| `files/backend/internal/module/media/gateway_service_test.go` | Media Domain 单元、集成或边界测试模板 |
| `files/backend/internal/module/media/gateway_types.go` | Media Domain 类型与模型模板 |
| `files/backend/internal/module/media/gc_boundary_test.go` | Media Domain 单元、集成或边界测试模板 |
| `files/backend/internal/module/media/gc_cos.go` | Media Domain 云适配器模板 |
| `files/backend/internal/module/media/gc_cos_external_test.go` | Media Domain 单元、集成或边界测试模板 |
| `files/backend/internal/module/media/gc_mysql_test.go` | Media Domain 单元、集成或边界测试模板 |
| `files/backend/internal/module/media/gc_repository.go` | Media Domain Repository 边界模板 |
| `files/backend/internal/module/media/gc_service.go` | Media Domain Service 模板 |
| `files/backend/internal/module/media/gc_service_test.go` | Media Domain 单元、集成或边界测试模板 |
| `files/backend/internal/module/media/gc_state_machine.go` | Media Domain 状态机模板 |
| `files/backend/internal/module/media/gc_state_machine_test.go` | Media Domain 单元、集成或边界测试模板 |
| `files/backend/internal/module/media/gc_types.go` | Media Domain 类型与模型模板 |
| `files/backend/internal/module/media/miniappflow/boundary_test.go` | MiniApp 媒体流验收/边界测试模板 |
| `files/backend/internal/module/media/miniappflow/cos_e2e_external_test.go` | MiniApp 媒体流验收/边界测试模板 |
| `files/backend/internal/module/media/miniappflow/handler.go` | MiniApp 媒体流应用层模板源码 |
| `files/backend/internal/module/media/miniappflow/mysql_external_test.go` | MiniApp 媒体流验收/边界测试模板 |
| `files/backend/internal/module/media/miniappflow/service.go` | MiniApp 媒体流应用层模板源码 |
| `files/backend/internal/module/media/miniappflow/service_test.go` | MiniApp 媒体流验收/边界测试模板 |
| `files/backend/internal/module/media/miniappflow/status_repository.go` | MiniApp 媒体流应用层模板源码 |
| `files/backend/internal/module/media/miniappflow/types.go` | MiniApp 媒体流应用层模板源码 |
| `files/backend/internal/module/media/miniappflow/wechat_server_external_test.go` | MiniApp 媒体流验收/边界测试模板 |
| `files/backend/internal/module/media/model.go` | Media Domain 类型与模型模板 |
| `files/backend/internal/module/media/repository.go` | Media Domain Repository 边界模板 |
| `files/backend/internal/module/media/repository_external_test.go` | Media Domain 单元、集成或边界测试模板 |
| `files/backend/internal/module/media/review_projection_mysql_test.go` | 架构或设计审查报告 |
| `files/backend/internal/module/media/review_projection_repository.go` | 架构或设计审查报告 |
| `files/backend/internal/module/media/review_projection_service.go` | 架构或设计审查报告 |
| `files/backend/internal/module/media/review_projection_service_test.go` | 架构或设计审查报告 |
| `files/backend/internal/module/media/review_projection_state_machine.go` | 架构或设计审查报告 |
| `files/backend/internal/module/media/review_projection_state_machine_test.go` | 架构或设计审查报告 |
| `files/backend/internal/module/media/review_projection_types.go` | 架构或设计审查报告 |
| `files/backend/internal/module/media/service.go` | Media Domain Service 模板 |
| `files/backend/internal/module/media/service_test.go` | Media Domain 单元、集成或边界测试模板 |
| `files/backend/internal/module/media/state_machine.go` | Media Domain 状态机模板 |
| `files/backend/internal/module/media/state_machine_test.go` | Media Domain 单元、集成或边界测试模板 |
| `files/backend/internal/module/media/ulid.go` | Media Domain 类型与模型模板 |
| `files/backend/internal/module/media/ulid_test.go` | Media Domain 单元、集成或边界测试模板 |
| `files/backend/internal/module/media/upload_authorizer.go` | Media Domain 云适配器模板 |
| `files/backend/internal/module/media/upload_authorizer_external_test.go` | Media Domain 单元、集成或边界测试模板 |
| `files/backend/internal/module/media/upload_authorizer_test.go` | Media Domain 单元、集成或边界测试模板 |
| `files/backend/internal/module/media/upload_repository.go` | Media Domain Repository 边界模板 |
| `files/backend/internal/module/media/upload_repository_external_test.go` | Media Domain 单元、集成或边界测试模板 |
| `files/backend/internal/module/media/upload_service.go` | Media Domain Service 模板 |
| `files/backend/internal/module/media/upload_service_test.go` | Media Domain 单元、集成或边界测试模板 |
| `files/backend/internal/module/media/upload_types.go` | Media Domain 类型与模型模板 |
| `files/backend/migrations/001_init.sql` | 冻结数据库迁移原文 |
| `files/backend/migrations/002_seed_dev.sql` | 冻结数据库迁移原文 |
| `files/backend/migrations/003_domain_convergence.sql` | 冻结数据库迁移原文 |
| `files/backend/migrations/004_auth_hardening.sql` | 冻结数据库迁移原文 |
| `files/backend/migrations/005_community_loop.sql` | 冻结数据库迁移原文 |
| `files/backend/migrations/006_transaction_closure.sql` | 冻结数据库迁移原文 |
| `files/backend/migrations/007_media_pipeline.sql` | 冻结数据库迁移原文 |
| `files/backend/migrations/008_media_dimensions_and_audit.sql` | 冻结数据库迁移原文 |
| `files/backend/migrations/009_media_domain_v11.sql` | 冻结数据库迁移原文 |
| `files/backend/migrations/010_media_upload_ticket_service.sql` | 冻结数据库迁移原文 |
| `files/backend/migrations/011_media_upload_completion.sql` | 冻结数据库迁移原文 |
| `files/backend/migrations/012_media_review_projection.sql` | 架构或设计审查报告 |
| `files/backend/migrations/013_media_consumer.sql` | 冻结数据库迁移原文 |
| `files/backend/migrations/014_media_gc.sql` | 冻结数据库迁移原文 |
| `files/backend/migrations/rollback/009_media_domain_v11.sql` | 冻结迁移回滚原文 |
| `files/backend/migrations/rollback/010_media_upload_ticket_service.sql` | 冻结迁移回滚原文 |
| `files/backend/migrations/rollback/011_media_upload_completion.sql` | 冻结迁移回滚原文 |
| `files/backend/migrations/rollback/012_media_review_projection.sql` | 架构或设计审查报告 |
| `files/backend/migrations/rollback/013_media_consumer.sql` | 冻结迁移回滚原文 |
| `files/backend/migrations/rollback/014_media_gc.sql` | 冻结迁移回滚原文 |
| `files/docs/ARCHITECTURE.md` | 系统架构与领域边界基线 |
| `files/docs/DEVELOPMENT.md` | 开发约定与开发记录 |
| `files/docs/evidence/P0-3_IM_REAL_ENV_2026-06-19.md` | 脱敏专项验收证据 |
| `files/docs/evidence/P0-4_MODULE10_ACCEPTANCE_EVIDENCE_2026-06-21.md` | 脱敏专项验收证据 |
| `files/docs/evidence/P0-4_MODULE5_ACCEPTANCE_EVIDENCE_2026-06-21.md` | 脱敏专项验收证据 |
| `files/docs/evidence/P0-4_MODULE6_ACCEPTANCE_EVIDENCE_2026-06-21.md` | 脱敏专项验收证据 |
| `files/docs/evidence/P0-4_MODULE7_ACCEPTANCE_EVIDENCE_2026-06-21.md` | 脱敏专项验收证据 |
| `files/docs/evidence/P0-4_MODULE8_ACCEPTANCE_EVIDENCE_2026-06-21.md` | 脱敏专项验收证据 |
| `files/docs/evidence/P0-4_MODULE9_ACCEPTANCE_EVIDENCE_2026-06-21.md` | 脱敏专项验收证据 |
| `files/docs/P0-1_AUTHORIZATION_ACCEPTANCE_REPORT_v1.0.md` | 专项验收报告 |
| `files/docs/P0-2_COMMUNITY_LOOP_ACCEPTANCE_REPORT_v1.0.md` | 专项验收报告 |
| `files/docs/P0-3_IM_CLOUD_VALIDATION_TODO.md` | 已知待办与未关闭门禁 |
| `files/docs/P0-3_TRANSACTION_ACCEPTANCE_REPORT_v1.0.md` | 专项验收报告 |
| `files/docs/P0-3_TRANSACTION_ACCEPTANCE_REPORT_v1.1.md` | 专项验收报告 |
| `files/docs/P0-3_TRANSACTION_CLOSURE_DESIGN.md` | 专项设计与冻结架构 |
| `files/docs/P0-4_ARCHITECTURE_REVIEW_REPORT.md` | 架构或设计审查报告 |
| `files/docs/P0-4_ARCHITECTURE_REVIEW_REPORT_v1.1.md` | 架构或设计审查报告 |
| `files/docs/P0-4_COS_INFRASTRUCTURE_DIAGNOSTIC_REPORT_v1.0.md` | 基础设施诊断报告 |
| `files/docs/P0-4_DESIGN_REMEDIATION_REPORT_v1.0.md` | 专项设计与冻结架构 |
| `files/docs/P0-4_GATE0_ACCEPTANCE_REPORT_v1.1.md` | 专项验收报告 |
| `files/docs/P0-4_GATE0_ACCEPTANCE_REPORT_v1.2.md` | 专项验收报告 |
| `files/docs/P0-4_GATE0_DEVELOPMENT_REPORT_v1.0.md` | 专项开发报告 |
| `files/docs/P0-4_MEDIA_PIPELINE_ACCEPTANCE_REPORT_v1.0.md` | 专项验收报告 |
| `files/docs/P0-4_MEDIA_PIPELINE_DESIGN.md` | 专项设计与冻结架构 |
| `files/docs/P0-4_MEDIA_PIPELINE_DESIGN_v1.0.md` | 专项设计与冻结架构 |
| `files/docs/P0-4_MEDIA_PIPELINE_DESIGN_v1.1.md` | 专项设计与冻结架构 |
| `files/docs/P0-4_MEDIA_PIPELINE_DESIGN_v1.1_FINAL.md` | 专项设计与冻结架构 |
| `files/docs/P0-4_MODULE10_ACCEPTANCE_REPORT_v1.0.md` | 专项验收报告 |
| `files/docs/P0-4_MODULE10_DEVELOPMENT_REPORT_v1.0.md` | 专项开发报告 |
| `files/docs/P0-4_MODULE10_FINAL_ACCEPTANCE_REPORT_v1.0.md` | 专项验收报告 |
| `files/docs/P0-4_MODULE2_ACCEPTANCE_REPORT_v1.0.md` | 专项验收报告 |
| `files/docs/P0-4_MODULE2_DEVELOPMENT_REPORT_v1.0.md` | 专项开发报告 |
| `files/docs/P0-4_MODULE3_ACCEPTANCE_REPORT_v1.0.md` | 专项验收报告 |
| `files/docs/P0-4_MODULE3_DEVELOPMENT_REPORT_v1.0.md` | 专项开发报告 |
| `files/docs/P0-4_MODULE3_REAL_STS_ACCEPTANCE_REPORT_v1.0.md` | 专项验收报告 |
| `files/docs/P0-4_MODULE4_ACCEPTANCE_REPORT_v1.0.md` | 专项验收报告 |
| `files/docs/P0-4_MODULE4_ACCEPTANCE_REPORT_v1.1.md` | 专项验收报告 |
| `files/docs/P0-4_MODULE4_DESIGN_REVIEW_REPORT.md` | 专项设计与冻结架构 |
| `files/docs/P0-4_MODULE4_DEVELOPMENT_REPORT_v1.0.md` | 专项开发报告 |
| `files/docs/P0-4_MODULE5_ACCEPTANCE_REPORT_v1.0.md` | 专项验收报告 |
| `files/docs/P0-4_MODULE5_DEVELOPMENT_REPORT_v1.0.md` | 专项开发报告 |
| `files/docs/P0-4_MODULE6_ACCEPTANCE_REPORT_v1.0.md` | 专项验收报告 |
| `files/docs/P0-4_MODULE6_DEVELOPMENT_REPORT_v1.0.md` | 专项开发报告 |
| `files/docs/P0-4_MODULE7_ACCEPTANCE_REPORT_v1.0.md` | 专项验收报告 |
| `files/docs/P0-4_MODULE7_DEVELOPMENT_REPORT_v1.0.md` | 专项开发报告 |
| `files/docs/P0-4_MODULE8_ACCEPTANCE_REPORT_v1.0.md` | 专项验收报告 |
| `files/docs/P0-4_MODULE8_DEVELOPMENT_REPORT_v1.0.md` | 专项开发报告 |
| `files/docs/P0-4_MODULE9_ACCEPTANCE_REPORT_v1.0.md` | 专项验收报告 |
| `files/docs/P0-4_MODULE9_DEVELOPMENT_REPORT_v1.0.md` | 专项开发报告 |
| `files/docs/runbooks/SUPERADMIN_BOOTSTRAP.md` | 运维或权限操作手册（仅文档） |
| `files/docs/SYSTEM_ACCEPTANCE_REPORT_v1.0.md` | 专项验收报告 |
| `files/package.json` | Node workspace 依赖与脚本契约 |
| `files/README.md` | 项目总览与启动说明 |
| `manifests/migrations-sha256.md` | 迁移与回滚文件 SHA-256 清单 |
| `snapshots/backend-tree.txt` | Backend 六层目录结构快照 |
| `snapshots/database-schema.sql` | 真实 MySQL 8.4 仅结构导出，无数据 |
