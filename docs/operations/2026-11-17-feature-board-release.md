# 2026-11-17 功能建议墙与作者的话上线记录

小程序「我的 → 功能意见与需求」新增独立的建议墙页面（提交建议、按支持数排行、作者的话），后端新增 `featureboard` 模块与管理端接口，后台新增「功能建议」分区。同批上线我的页整理与底部导航微调。

## 本次内容

- **迁移** `backend/migrations/065_feature_board.sql`：`feature_suggestions`（内容 500 字上限、状态 open/accepted/closed、支持数）、`feature_suggestion_supports`（唯一约束防重复支持）、`author_notes`；回滚 `rollback/065_feature_board.sql`；迁移测试 `migration_065_test.go`。
- **后端** `backend/internal/module/featureboard`：
  - 用户接口（登录；写操作另需未禁言）：`GET /api/v1/auth/feature-suggestions?sort=supports|latest&page=&pageSize=`、`POST /api/v1/auth/feature-suggestions`、`POST /api/v1/auth/feature-suggestions/:id/support`（`{supported:bool}`，事务内相对计数）、`GET /api/v1/auth/author-notes`。
  - 管理接口（管理员会话）：`GET /api/v1/admin/feature-board/suggestions`、`POST /api/v1/admin/feature-board/suggestions/:id/status`、`GET/POST /api/v1/admin/feature-board/notes`、`DELETE /api/v1/admin/feature-board/notes/:id`。
- **小程序**：新页面 `pages/feature-board/index`（建议墙 + 作者的话）；我的页「账户与支持」调整为 设置 / 众创申请 / 功能意见与需求 / 关于；移除用户卡「学生」徽章与用户号；众创行使用新灯泡线条图标；设置页去行间细线；底部导航胶囊下移（安全区系数 0.5）。
- **后台**：预览壳新增「功能建议」分区（懒加载 chunk `FeatureBoardView`）：状态筛选、采纳/关闭/重开、作者的话发布与删除。
- **契约**：新增 `check:feature-board` 接入 verify；路由/自定义导航清单更新为 37/34。

## 部署记录（2026-11-17）

- 代码：`04876e4` 已推送 origin/main。
- **生产库迁移 065 已执行**（debian-sys-maint 维护账号）：迁移前备份 `/var/backups/campus-circle/pre-migration-065-20261117T151427Z.sql.gz`（gzip 校验通过）；执行后存在 `feature_suggestions`、`feature_suggestion_supports`、`author_notes` 三表。
- **后端**：版本 `2026.11.17.candidate.1`（commit `04876e43b34b`），release `/opt/campus-circle/releases/20261117a-feature-board-04876e4`，二进制 SHA256 `9728986c58272fc3f84ea9979afca6adece83424430a2ceb2d30ccf0760f5110`；`campus-api` 已重启（active）。
- **后台静态**：release `/var/www/campus-admin/releases/admin-20261117-04876e4`（首页 `index-1DQCcN4X.js`，含 `FeatureBoardView-D62Pjbgu.js` chunk），已切换软链。
- **线上验证**：`/healthz` ok；`/version` 返回新版本与 commit；未登录访问 `/api/v1/auth/feature-suggestions`、`/api/v1/auth/author-notes`、`/api/v1/admin/feature-board/suggestions` 均 401；后台首页与 FeatureBoardView chunk 均 200。
- **回滚目标（上一版）**：后端 `20261116a-student-verify-997efc1`，后台 `admin-20261117-16b6659`；数据库回滚脚本 `backend/migrations/rollback/065_feature_board.sql`（纯新增表，回滚代码时也可保留数据）。

## 待办（人工）

- 小程序上传体验版并发版后，用户才能看到新的「功能意见与需求」页面与本次界面调整。
- 我的页「众创申请」弹窗中的微信号仍是占位值 `zhanke-zhongchuang`，需替换为真实微信号后随下次发版更新。
- 后台可在「功能建议」分区发布第一条「作者的话」，并在小程序上线后体验采纳/关闭流转。
