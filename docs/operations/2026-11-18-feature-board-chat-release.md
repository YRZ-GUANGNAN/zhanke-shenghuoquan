# 2026-11-18 建议对话与联系设置部署记录

本次部署覆盖 11-17 晚至 11-18 凌晨的全部提交（`cbd2d8e..e19422a`），新增两块后端能力：功能建议对话（用户与作者交谈）与众创申请弹窗内容后台可配；同时带上此前的小程序界面批次（头图白字方案、众创图标、背景效果选项、学生认证开始入口等，均需小程序发版后对用户生效）。

## 本次内容

- **迁移 066** `contact_settings`：单行平台配置（`maker_wechat` 众创联系微信号、`maker_intro` 弹窗文案、`row_version` 乐观锁），幂等建表与默认行；回滚 `rollback/066_contact_settings.sql`。
- **迁移 067** `feature_suggestion_comments`：建议对话表（`suggestion_id`+`id` 索引、`is_maintainer` 标记、内容 300 字）；回滚 `rollback/067_feature_suggestion_comments.sql`。
- **后端 contactsettings 模块**：公开读 `GET /api/v1/contact-settings`（小程序弹窗拉取），管理员 `PUT /api/v1/admin/contact-settings`（乐观锁）。
- **后端 featureboard 扩展**：对话列表 `GET /api/v1/auth/feature-suggestions/:id/comments`、用户发言 `POST …/comments`（登录且未禁言，1–300 字，已关闭建议不可发言）；管理端 `GET/POST /api/v1/admin/feature-board/suggestions/:id/comments`（作者回复带 `isMaintainer`）；建议列表返回 `commentCount`。
- **小程序**：建议墙每条可展开「对话 N」（用户参与、作者回复带标签）；众创申请弹窗改为接口驱动并重做样式（文案 + 微信号卡片 + 复制）。
- **后台**：「服务入口 → 众创申请弹窗」面板（改微信号与文案，保存即生效）；「功能建议」支持查看对话与作者回复。

## 部署记录（2026-11-18）

- 代码：`e19422a` 已推送 origin/main。
- **生产库迁移 066、067 已执行**（debian-sys-maint，`--default-character-set=utf8mb4`）：迁移前备份 `/var/backups/campus-circle/pre-migration-066-067-20261117T162425Z.sql.gz`；`contact_settings` 默认行与 `feature_suggestion_comments` 表存在且文案编码正确。
- **后端**：版本 `2026.11.18.candidate.1`（commit `e19422a47e88`），release `/opt/campus-circle/releases/20261118a-feature-board-chat-e19422a`，二进制 SHA256 `b2cd8b110bd359649e012cfd56d6eec54e3f86e9ae17b5309ae7f349181a1b05`；`campus-api` 已重启（active）。
- **后台静态**：release `/var/www/campus-admin/releases/admin-20261118-e19422a`（首页 `index-BUWsco6g.js`，含新 `FeatureBoardView-C2fKuM8r.js` chunk），软链已切换。
- **线上验证**：`/healthz` ok；`/version` 返回新版本与 commit；`GET /api/v1/contact-settings` 公开 200 且返回默认微信号与文案；未登录 POST 建议对话与 PUT 管理端联系设置均 401；后台首页与 FeatureBoardView chunk 200。
- **回滚目标（上一版）**：后端 `20261117a-feature-board-04876e4`，后台 `admin-20261117-04876e4`；数据库回滚脚本 `rollback/066_contact_settings.sql`、`rollback/067_feature_suggestion_comments.sql`（纯新增表，回滚代码时可保留数据）。

## 待办（人工）

- 小程序发版后：建议墙对话、弹窗新样式、头图白字与背景效果选项等一起对用户生效。
- 后台「服务入口 → 众创申请弹窗」把占位微信号 `zhanke-zhongchuang` 修改为真实联系方式。
- 建议墙现有的「支持」即路人的点赞，已随列表展示；作者回复可从后台「功能建议」进入。
