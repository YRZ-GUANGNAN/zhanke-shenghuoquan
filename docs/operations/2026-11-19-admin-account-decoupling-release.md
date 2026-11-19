# 2026-11-19 后台管理员账号与小程序用户解耦（独立账号体系）

## 本次内容

后台管理员账号不再绑定小程序用户：账号、密码、角色、状态全部保存在 `admin_credentials`，后台登录与会话校验不再读取 `users.role`/`users.status`。

- **迁移 069**：`admin_credentials` 增加 `role/status/display_name/legacy_user_id`，`user_id` 改为可空；`admin_web_sessions.user_id` 可空；`admin_auth_audit_logs` 增加 `actor_credential_id` 并扩展事件集合；各后台操作人列新增 `*_admin_id` 并按 `legacy_user_id` 回填历史值。迁移带回滚（`rollback/069_admin_accounts.sql`），已在本地 MySQL 8.4 验证正向、回滚与重放。
- **adminauth 重写**：Login/Authenticate 只看管理员账号自身状态与角色；新增账号 CRUD（新建/编辑显示名与角色/重置密码/停用启用）、最后一名启用超管保护、禁止停用或降级自己；审计记录真实用户名与操作者；`EnsureSchema` 在启动时校验 069 已应用。
- **CLI**：`admin-credential bootstrap --username <name> --role superadmin [--display-name <name>]`、`reset --username <name>`；不再要求先有小程序用户。
- **后台 API 只接受管理员会话 Cookie**：移除小程序 JWT（Bearer）访问后台的通道。
- **后台前端**：新增「管理员账号」面板（独立于用户列表）；登录页不再预填用户名；用户列表的“角色”改为社区角色（仅影响小程序站内权限，如「仅管理员可发帖」）。
- 操作人切换：审核任务、用户反馈、功能策略、分享设置、通知渠道、配送改派/取消、骑手状态、打款与提现、身份申请、学生认证、建议墙作者回复等写新的 `*_admin_id` 列。

## 部署记录（2026-11-19）

- 代码：commit `934e9bc8bed4` 已推送 origin/main。
- **迁移 069 已执行**：迁移前备份 `/var/backups/campus-circle/pre-migration-069-20261118T235103Z.sql.gz`（gzip 校验通过，115 张表）；回填后唯一后台账号 `administrator` 为 `superadmin/active`，`legacy_user_id=1`，显示名“校园用户”。
- **后端**：版本 `2026.11.19.candidate.1`，release `/opt/campus-circle/releases/20261119a-admin-accounts-934e9bc`，二进制 SHA256 `d73bd070ca7da3a84fe81ea6c40157ee8b35cdb1670f030919d2be50ecb50a5f`；`campus-api` 重启 active。
- **后台静态**：release `/var/www/campus-admin/releases/admin-20261119a-934e9bc`，首页 `index-CrvlWLnq.js`，`PeopleView-LLCDvn4-.js` 200 且包含「新建管理员账号」。
- **线上验证**：`/healthz` ok、`/version` 返回 candidate.1；`/api/v1/admin-auth/session` 与 `/accounts` 未登录均 401；旧 `/admin-auth/credentials` 已 404；`admin_credentials` 四个新列与 `review_tasks.operator_admin_id` 等列存在。
- **回滚目标（上一版）**：后端 `20261118e-wecom-reassign-876ef05`；后台静态 `admin-20261118e-470eecb`；数据库可执行 `rollback/069_admin_accounts.sql`（会删除迁移后新建、无 user_id 的独立管理员账号；旧账号与密码哈希保留）。

## 使用与注意

- 现有 `administrator` 账号密码不变，可直接登录；已登录的后台会话继续有效。
- 新增管理员：后台「账号与权限 → 管理员账号 → 新建管理员账号」，填写用户名、显示名、角色与初始密码即可，无需小程序注册。
- `users.role` 仅作为小程序社区角色保留（用于「仅管理员可发帖」等站内权限），不再授予后台访问。
- 首个超级管理员由运维 CLI 创建，详见 `docs/runbooks/SUPERADMIN_BOOTSTRAP.md` 与 `docs/ADMIN-PASSWORD-LOGIN-RUNBOOK.md`。

## 后续调整（2026-11-19，candidate.2）

- **密码长度不再限制**：管理员密码只要求非空（去掉了 14–72 位限制），登录请求体保留 16KB 大小保护；后台登录页、修改密码、管理员账号面板与运行手册同步更新。
- 版本 `2026.11.19.candidate.2`（commit `1c21451d750a`），release `/opt/campus-circle/releases/20261119b-password-policy-1c21451`，二进制 SHA256 `4342e50954813e4fab9f89602e879cc02681fcd56139d873dbcd03faed9ff964`；后台静态 `/var/www/campus-admin/releases/admin-20261119b-1c21451`（首页 `index-DVRUYX_h.js`）。线上验证：`/version` 返回 candidate.2、新 PeopleView 分包 200 且含新提示。
- 同时补齐运维 CLI `/opt/campus-circle/current/bin/admin-credential`（构建脚本已改为随发布一并产出）；回滚目标同前。
