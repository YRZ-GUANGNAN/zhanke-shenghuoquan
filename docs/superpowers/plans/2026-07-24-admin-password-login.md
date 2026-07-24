# 管理后台独立密码登录 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 `zhanduiwang.top` 管理后台增加与微信项目成员无关的永久密码登录，同时保持现有 Bearer JWT 管理接口兼容，并用真实超级管理员会话完成身份申请审核。

**Architecture:** 新建独立 `adminauth` 后端模块，凭据固定绑定现有 `superadmin` 用户，使用 Argon2id 保存密码摘要、随机不透明 Cookie 会话和审计日志。统一管理接口认证中间件同时接受原有 Bearer JWT 或新的 Web Session Cookie，但拒绝两种凭据同时出现；Cookie 写操作增加同源和 CSRF 校验。管理前端改为 Cookie 会话登录，不再依赖 `domainA.token`，并发布现有身份审核页。

**Tech Stack:** Go 1.25、Gin、GORM/MySQL 8、`golang.org/x/crypto/argon2`、`golang.org/x/term`、React 18、TypeScript、Zustand、Vite、Nginx/systemd。

---

## Task 1：新增管理登录数据库契约

**Files:**

- Create: `backend/migrations/035_admin_password_login.sql`
- Create: `backend/migrations/migration_035_test.go`
- Test: `backend/migrations/migration_035_test.go`

- [ ] 先写迁移源码契约测试，断言迁移包含：

  - `admin_credentials`：唯一用户名、`user_id`、Argon2id 摘要、失败次数、锁定时间、版本与时间戳。
  - `admin_web_sessions`：只保存 SHA-256 会话令牌摘要、CSRF 摘要、空闲/绝对过期时间、撤销时间、最后使用时间和客户端摘要。
  - `admin_auth_audit_logs`：登录、登出、密码变更、锁定和会话撤销审计事件。
  - 固定角色仍由 `users.role` 实时判定，不新增或提升业务用户。
  - 迁移锁 `campus_circle:migration:035`、基础表检查、幂等重放和 forward-only 约束。

- [ ] 运行红灯测试：

  ```powershell
  Set-Location backend
  go test ./migrations -run Migration035 -count=1
  ```

  预期：因迁移文件不存在而失败。

- [ ] 按 034 的存储过程与幂等保护风格实现 035；外键仅指向既有 `users(id)`，不修改用户角色和身份申请数据。

- [ ] 增加独立临时 MySQL 8 集成测试：首次执行、重复执行、错误基础表、错误已有表结构、索引/约束精确匹配；没有 `TEST_MYSQL_DSN` 时明确 `PENDING`/skip，禁止退回共享 `campus_circle`。

- [ ] 运行：

  ```powershell
  Set-Location backend
  go test ./migrations -run Migration035 -count=1
  ```

  预期：源码契约通过；配置独立 MySQL 时集成用例通过。

- [ ] 仅提交本任务文件：

  ```powershell
  git add backend/migrations/035_admin_password_login.sql backend/migrations/migration_035_test.go
  git commit -m "新增：管理后台密码登录数据契约"
  ```

## Task 2：实现密码、会话和锁定核心服务

**Files:**

- Create: `backend/internal/module/adminauth/model.go`
- Create: `backend/internal/module/adminauth/repository.go`
- Create: `backend/internal/module/adminauth/password.go`
- Create: `backend/internal/module/adminauth/service.go`
- Create: `backend/internal/module/adminauth/password_test.go`
- Create: `backend/internal/module/adminauth/service_test.go`
- Modify: `backend/go.mod`
- Modify: `backend/go.sum`

- [ ] 先写密码单元测试，覆盖 14–72 字符边界、Argon2id 参数 `64 MiB / 3 次 / parallelism 2`、随机盐、正确/错误密码验证和摘要解析失败。

- [ ] 先写服务测试，覆盖：

  - 固定用户名 `administrator`。
  - 不存在账号与错误密码均返回同一公共错误，并都执行 Argon2id 验证。
  - 账号/IP 组合连续 10 次失败锁定 30 分钟。
  - 仅 `status=active && role=superadmin` 的绑定用户可登录。
  - 创建 30 分钟空闲、8 小时绝对期限的随机会话。
  - 空闲或绝对过期、已撤销、用户失效/降权时会话拒绝。
  - 密码变更校验旧密码并撤销该凭据的全部现有会话。

- [ ] 运行红灯测试：

  ```powershell
  Set-Location backend
  go test ./internal/module/adminauth -count=1
  ```

- [ ] 实现模型、Repository、可注入时钟/随机源的 Service；密码摘要用 Argon2id，原始会话令牌和 CSRF 令牌仅返回一次，数据库只存 SHA-256 摘要；所有认证事件写审计日志。

- [ ] 运行：

  ```powershell
  Set-Location backend
  go test ./internal/module/adminauth -count=1
  go test ./... -count=1
  ```

- [ ] 提交：

  ```powershell
  git add backend/internal/module/adminauth backend/go.mod backend/go.sum
  git commit -m "新增：管理后台密码与会话服务"
  ```

## Task 3：增加 TTY 安全初始化/重置命令

**Files:**

- Create: `backend/cmd/admin-credential/main.go`
- Create: `backend/cmd/admin-credential/main_test.go`
- Modify: `backend/internal/module/adminauth/service.go`

- [ ] 先写测试，覆盖用户名固定、用户 ID 必须存在且为 active superadmin、两次密码不一致、非 TTY 输入拒绝、重置时撤销旧会话。

- [ ] 实现独立命令：

  ```powershell
  go run ./cmd/admin-credential bootstrap --user-id 1
  go run ./cmd/admin-credential reset --user-id 1
  ```

  密码只通过 `term.ReadPassword` 从交互式 TTY 隐藏读取两次；禁止密码参数、环境变量和标准输入管道。

- [ ] 运行：

  ```powershell
  Set-Location backend
  go test ./cmd/admin-credential ./internal/module/adminauth -count=1
  ```

- [ ] 提交：

  ```powershell
  git add backend/cmd/admin-credential backend/internal/module/adminauth/service.go
  git commit -m "新增：管理员密码安全初始化命令"
  ```

## Task 4：接入 HTTP 登录、Cookie、CSRF 和双认证兼容

**Files:**

- Create: `backend/internal/module/adminauth/handler.go`
- Create: `backend/internal/module/adminauth/middleware.go`
- Create: `backend/internal/module/adminauth/handler_test.go`
- Create: `backend/internal/module/adminauth/middleware_test.go`
- Modify: `backend/internal/module/user/middleware.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/cmd/server/main_test.go`

- [ ] 先写 HTTP 测试：

  - `POST /api/v1/admin-auth/login` 统一错误响应；成功设置 `__Host-zhandui_admin_session`，属性为 `Secure; HttpOnly; SameSite=Strict; Path=/` 且没有 `Domain`。
  - `GET /api/v1/admin-auth/session` 返回当前管理员和 CSRF token。
  - `POST /api/v1/admin-auth/logout` 撤销会话并清空 Cookie。
  - `POST /api/v1/admin-auth/password/change` 校验旧/新密码。
  - Cookie 写请求缺少/错误 `Origin`、`Host` 或 `X-CSRF-Token` 均拒绝。
  - Bearer JWT 管理请求保持原行为且不要求 CSRF。
  - Bearer 与 Cookie 同时出现返回 400，避免身份歧义。

- [ ] 为 `user` 模块增加一个显式的“已认证用户写入上下文”入口，避免 `adminauth` 复制私有 context key；管理权限仍复用 `user.AdminRequired()` 与 `user.SuperAdminRequired()`。

- [ ] 在 composition root 创建 `adminauth.Repository/Service/Handler`，注册 `/api/v1/admin-auth`；将所有 `/api/v1/admin/**` 路由的认证中间件替换为兼容中间件，非管理端路由仍只接受原 Bearer JWT。

- [ ] 运行：

  ```powershell
  Set-Location backend
  go test ./internal/module/adminauth ./internal/module/user ./cmd/server -count=1
  go test ./... -count=1
  ```

- [ ] 提交：

  ```powershell
  git add backend/internal/module/adminauth backend/internal/module/user/middleware.go backend/cmd/server/main.go backend/cmd/server/main_test.go
  git commit -m "新增：管理后台 Cookie 登录接口"
  ```

## Task 5：管理前端改用永久 Web 会话

**Files:**

- Create: `apps/admin/src/services/http.ts`
- Create: `apps/admin/src/services/admin-auth.ts`
- Create: `apps/admin/src/Login.tsx`
- Create: `apps/admin/src/AccountPanel.tsx`
- Modify: `apps/admin/src/services/f-admin.ts`
- Modify: `apps/admin/src/services/delivery-admin.ts`
- Modify: `apps/admin/src/services/takeout-admin.ts`
- Modify: `apps/admin/src/store.ts`
- Modify: `apps/admin/src/types.ts`
- Modify: `apps/admin/src/App.tsx`
- Modify: `apps/admin/src/styles.css`
- Create: `scripts/check-admin-password-session-contract.cjs`
- Modify: `package.json`

- [ ] 先写静态契约检查，要求：

  - `apps/admin/src` 不再读取或写入 `domainA.token`。
  - 所有后台请求使用统一 `credentials:'same-origin'` fetch。
  - 非 GET/HEAD 请求自动带当前 `X-CSRF-Token`。
  - 存在登录、会话恢复、登出和修改密码调用。
  - `identities` 导航与 `IdentityApplications` 页面保留。

- [ ] 运行红灯：

  ```powershell
  node scripts/check-admin-password-session-contract.cjs
  ```

- [ ] 实现统一请求层和 auth store。应用启动先请求 `/admin-auth/session`：200 进入后台，401 显示用户名固定为 `administrator` 的密码登录页，403 显示权限错误；任何后续 401 清理内存态并回登录页。

- [ ] 顶栏增加当前管理员信息、修改密码和退出；不重做现有业务 UI，只修复登录相关中文提示和原来要求手工写 token 的错误文案。

- [ ] 运行：

  ```powershell
  node scripts/check-admin-password-session-contract.cjs
  npm --workspace apps/admin run typecheck
  npm --workspace apps/admin run build
  ```

- [ ] 提交：

  ```powershell
  git add apps/admin/src scripts/check-admin-password-session-contract.cjs package.json
  git commit -m "新增：管理后台密码登录页面"
  ```

## Task 6：本地全量验证与生产部署准备

**Files:**

- Create: `docs/ADMIN-PASSWORD-LOGIN-RUNBOOK.md`
- Modify: `deploy/native/README.md`

- [ ] 编写不含任何秘密的运行手册：迁移、构建、备份、部署、隐藏初始化密码、健康检查、回滚和审计查询；明确回滚不得删除用户、身份申请、授权或审计数据。

- [ ] 运行完整验证：

  ```powershell
  node scripts/check-admin-password-session-contract.cjs
  npm --workspace apps/admin run typecheck
  npm --workspace apps/admin run build
  Set-Location backend
  go test ./... -count=1
  ```

  没有独立 MySQL 权限的真实迁移集成测试保持 `PENDING`，不得在共享业务库中补跑。

- [ ] 检查仓库边界和差异，确保没有 `.env`、Cookie、密码、数据库凭据、私钥或无关脏文件进入提交。

- [ ] 提交文档：

  ```powershell
  git add docs/ADMIN-PASSWORD-LOGIN-RUNBOOK.md deploy/native/README.md
  git commit -m "文档：管理后台密码登录部署手册"
  ```

## Task 7：部署生产并完成真实审核

**Files/Targets:**

- Local build: `backend/cmd/server`, `backend/cmd/admin-credential`, `apps/admin/dist`
- Remote backup: `/opt/campus-circle/backups/<timestamp>/`
- Remote backend: `/opt/campus-circle/current/bin/campus-api`
- Remote credential command: `/opt/campus-circle/current/bin/admin-credential`
- Remote admin static: `/var/www/campus-admin/current`
- Remote env: `/etc/campus-circle/campus-api.env`

- [ ] 通过 SSH 只读确认当前 systemd、Nginx、磁盘和数据库版本，创建带时间戳备份；不打印 `/etc/mysql/debian.cnf`、环境变量或其他秘密。

- [ ] 上传新二进制、035 迁移和管理静态文件到临时路径；对迁移做 SQL 预检后，使用 `/etc/mysql/debian.cnf` 应用 035，再原子切换后台文件。

- [ ] 重启 `campus-api`，验证：

  ```text
  GET https://zhanduiwang.top/healthz -> 200
  GET https://zhanduiwang.top/api/v1/admin-auth/session -> 401（尚未登录）
  ```

- [ ] 在生产主机交互式 TTY 运行：

  ```bash
  sudo -u campus /opt/campus-circle/current/bin/admin-credential bootstrap --user-id 1
  ```

  由用户在本机终端隐藏输入两次初始密码；不得粘贴到聊天或保存到脚本。

- [ ] 在 `https://zhanduiwang.top/` 用 `administrator` 登录，验证刷新仍登录、主动退出后失效、重新登录成功、身份审核列表可见。

- [ ] 打开申请 `01KY9JWJ80C16W3T5G55TDYPT3`，核对：

  - 申请人为业务用户 ID 3。
  - 当前状态仍为 `pending`，`rowVersion=1`。
  - 证明图片已完成平台审核。
  - 审核操作人是当前真实 `superadmin` ID 1。

- [ ] 仅在以上事实一致时点击批准；若出现版本冲突或状态已变化则停止，不直接修改数据库。

- [ ] 通过 UI 和只读 API/数据库查询确认：

  - 申请状态为 `approved`。
  - 生成有效 merchant grant。
  - ID 3 的系统角色仍为普通 `user`，未被提升为 admin。
  - 审核事件与管理员认证审计均存在。

- [ ] 若失败，先回滚二进制和静态文件并保持 035 表与审计数据，不撤销/删除既有用户、身份申请或授权；记录未通过的真实运行边界。
