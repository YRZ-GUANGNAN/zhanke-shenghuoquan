# 管理后台独立账号运行手册

## 安全边界

- 后台管理员账号**独立于小程序用户**：账号、密码与角色保存在 `admin_credentials` 表，不需要小程序注册，也不再读取 `users.role`/`users.status`。
- 后台账号管理在「账号与权限 → 管理员账号」面板完成：新建、编辑显示名/角色、重置密码、停用/启用；仅启用中的超级管理员可操作。
- 系统保护最后一名启用中的超级管理员，且不能停用或降级自己的账号。
- 停用账号会立即吊销其全部后台会话；管理后台 API 只接受后台会话 Cookie，不再接受小程序 JWT（Bearer）。
- 密码不得出现在聊天、命令参数、环境变量、脚本、日志或 shell 历史中。
- 初始化与重置只能在交互式 TTY 中隐藏输入两次。
- 管理 Cookie 为 `__Host-zhandui_admin_session`，必须同时具备 `Secure`、`HttpOnly`、`SameSite=Strict`、`Path=/`，且不能设置 `Domain`。
- 管理 Cookie 会话空闲 30 分钟失效，创建 8 小时后绝对失效。
- 数据迁移、部署和回滚均不得修改普通用户角色、直接批准身份申请，或删除既有用户、身份申请、身份授权和认证审计。

## 本地验证

```powershell
node scripts/check-admin-password-session-contract.cjs
node scripts/check-admin-ui-system.cjs
node scripts/check-migrations.cjs
npm --workspace apps/admin run typecheck
npm --workspace apps/admin run build

Set-Location backend
go test ./migrations ./internal/module/adminauth ./internal/module/user ./cmd/server -count=1
go test ./... -count=1
```

需要真实执行迁移集成测试时，只能设置指向可创建/删除独立临时数据库的 `TEST_MYSQL_DSN`。不得使用共享 `campus_circle` 作为测试回退。

## 生产备份

部署前记录当前版本，并在生产主机建立带时间戳的只读备份目录，至少包含：

- `/opt/campus-circle/current/bin/campus-api`
- `/var/www/campus-admin/current`
- 当前 systemd unit 与 Nginx 站点配置的副本
- 迁移前的数据库备份（`mysqldump --single-transaction`）

不得把 `/etc/campus-circle/campus-api.env`、`/etc/mysql/debian.cnf` 或其中任何值复制到日志和任务记录。

## 迁移与部署顺序

1. 备份数据库，然后使用生产 MySQL 管理凭据执行 `069_admin_accounts.sql`（可重放；存在后台账号但无启用中的超级管理员时会中止）。
2. 上传新 `campus-api`、`admin-credential` 与 `apps/admin/dist` 到临时目录，验证架构、SHA-256 与磁盘空间。
3. 原子替换两个二进制和管理后台静态目录。
4. 重启 `campus-api`，确认 `/healthz` 返回 200。
5. 在登录前确认 `/api/v1/admin-auth/session` 返回 401。
6. 迁移会把既有后台凭据原样升级为独立账号（保留用户名与密码哈希），并把历史操作人列按映射回填到新的 `*_admin_id` 列。

## 安全初始化或重置

生产主机必须以交互式终端执行。命令不会接受密码参数或管道输入。

首次创建超级管理员（`admin_credentials` 为空或需要新建时）：

```bash
sudo -u campus /opt/campus-circle/current/bin/admin-credential bootstrap --username administrator --role superadmin --display-name 超级管理员
```

重置已有账号密码：

```bash
sudo -u campus /opt/campus-circle/current/bin/admin-credential reset --username administrator
```

操作者在本机隐藏输入两次密码。密码不限制长度（不能为空，建议使用足够复杂的口令），用户名为 3–64 位字母、数字、点或下划线。重置密码会撤销该账号已有的管理网页会话。后续新增管理员一律在后台「管理员账号」面板完成。

## 上线验收

1. 打开管理后台，确认显示密码登录页（用户名不再预填）。
2. 使用原 `administrator` 账号与密码登录，确认历史账号已平滑迁移。
3. 在「账号与权限 → 管理员账号」中新建一个普通管理员（无需小程序注册），退出后用新账号登录验证。
4. 验证普通管理员无法修改「功能策略」等超级管理员功能；超级管理员可为其重置密码、停用账号。
5. 停用该测试账号后确认其会话立即失效、无法登录；重新启用后可登录。
6. 刷新页面确认会话仍有效；退出后确认受保护接口返回 401。

## 回滚

如果后端或静态页面失败：

1. 恢复备份的 `campus-api` 和管理后台静态目录。
2. 重启服务并检查 `/healthz`。
3. 数据库层可执行 `rollback/069_admin_accounts.sql`：它会删除迁移后新建的、没有绑定小程序用户的独立管理员账号（旧模型无法表示），旧账号与密码哈希保留。执行前确认没有只存在于新账号上的管理员。
4. 不得删除认证审计；不要撤销既有用户、身份申请、身份授权或审核事件。
