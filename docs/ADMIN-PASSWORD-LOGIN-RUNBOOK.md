# 管理后台独立密码登录运行手册

## 安全边界

- 管理网页固定用户名为 `administrator`，凭据必须绑定既有的 active `superadmin` 用户。
- 密码不得出现在聊天、命令参数、环境变量、脚本、日志或 shell 历史中。
- 初始化与重置只能在交互式 TTY 中隐藏输入两次。
- 管理 Cookie 为 `__Host-zhandui_admin_session`，必须同时具备 `Secure`、`HttpOnly`、`SameSite=Strict`、`Path=/`，且不能设置 `Domain`。
- 管理 Cookie 会话空闲 30 分钟失效，创建 8 小时后绝对失效。
- 数据迁移、部署和回滚均不得修改普通用户角色、直接批准身份申请，或删除既有用户、身份申请、身份授权和认证审计。

## 本地验证

为避免系统盘编译缓存占满，可将一次性 Go 缓存与临时目录放到空间充足的磁盘；不要将缓存加入仓库。

```powershell
node scripts/check-admin-password-session-contract.cjs
npm --workspace apps/admin run typecheck
npm --workspace apps/admin run build

Set-Location backend
go test ./migrations ./internal/module/adminauth ./internal/module/user ./cmd/server -count=1
go test ./... -count=1
```

需要真实执行 035 迁移集成测试时，只能设置指向可创建/删除独立临时数据库的 `TEST_MYSQL_DSN`。不得使用共享 `campus_circle` 作为测试回退。

## 生产备份

部署前记录当前版本，并在生产主机建立带时间戳的只读备份目录，至少包含：

- `/opt/campus-circle/current/bin/campus-api`
- `/var/www/campus-admin/current`
- 当前 systemd unit 与 Nginx 站点配置的副本
- 035 执行前的 `SHOW CREATE TABLE`/表存在性结果

不得把 `/etc/campus-circle/campus-api.env`、`/etc/mysql/debian.cnf` 或其中任何值复制到日志和任务记录。

## 迁移与部署顺序

1. 将新 `campus-api`、`admin-credential`、`035_admin_password_login.sql` 和 `apps/admin/dist` 上传到临时目录。
2. 验证二进制架构、SHA-256、磁盘空间和现有服务健康状态。
3. 使用生产 MySQL 管理凭据执行 035；该迁移有 `campus_circle:migration:035` 锁并可安全重放。
4. 原子替换两个二进制和管理后台静态目录。
5. 重启 `campus-api`，确认 `/healthz` 返回 200。
6. 在登录前确认 `/api/v1/admin-auth/session` 返回 401。

## 安全初始化或重置

生产主机必须以交互式终端执行。命令不会接受密码参数或管道输入。

首次初始化：

```bash
sudo -u campus /opt/campus-circle/current/bin/admin-credential bootstrap --user-id 1
```

后续重置：

```bash
sudo -u campus /opt/campus-circle/current/bin/admin-credential reset --user-id 1
```

操作者在本机隐藏输入两次密码。密码长度为 14–72 个字符。重置和网页内修改密码会撤销已有管理网页会话。

## 上线验收

1. 打开管理后台 `https://zhandui.top/`，确认显示密码登录页。小程序 API 仍使用 `https://zhanduiwang.top/api/v1`，不要把两个入口混用。
2. 使用固定用户名 `administrator` 和刚设置的密码登录。
3. 刷新页面，确认会话仍有效。
4. 退出后确认受保护接口返回 401。
5. 重新登录，打开“身份审核”，确认申请详情、证明图片和审核按钮可用。
6. 执行任何审核前，核对申请 ID、状态、`rowVersion`、申请用户和当前真实超级管理员。
7. 审核后通过 UI 与只读查询核对审核事件、身份授权和申请用户系统角色。

## 回滚

如果后端或静态页面失败：

1. 恢复备份的 `campus-api` 和管理后台静态目录。
2. 重启服务并检查 `/healthz` 与旧 Bearer 管理接口。
3. 保留 035 创建的凭据、会话和审计表，不执行破坏性回滚。
4. 可以撤销全部管理网页会话或重置管理员密码，但不得删除认证审计。
5. 不得撤销或删除既有用户、身份申请、身份授权或审核事件。
