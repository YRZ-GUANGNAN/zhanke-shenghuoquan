# zhandui.top 管理后台域名迁移实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将管理后台的正式访问入口迁移为 `https://zhandui.top/`，保持小程序 API 继续使用 `https://zhanduiwang.top/api/v1`，不影响原项目、后端接口契约或当前生产站。

**Architecture:** 管理后台继续使用现有 Admin 静态构建和同机 Go API；Nginx 为 `zhandui.top` 增加独立的 HTTP ACME challenge、HTTPS 静态站和 `/api/` 反向代理，使用与现有站相同的安全头和会话策略。迁移采用“先准备、后切 DNS、再签证书、最后切流”的顺序，旧 `zhanduiwang.top` 配置和站点在新域名验收前保持不动。`www.zhandui.top` 只有在单独建立 DNS 记录后才加入证书和别名，不作为 apex 域名上线的前置条件。

**Tech Stack:** React/Vite Admin、Go API、Nginx、Let’s Encrypt/Certbot、Cloudflare DNS、PowerShell、Node.js contract checks。

---

### Task 1: 固化迁移边界和当前基线

**Files:**
- Create: `docs/superpowers/plans/2026-10-09-zhandui-admin-domain-migration.md`
- Inspect: `apps/admin/src/services/f-admin.ts`, `backend/internal/module/adminauth`, `deploy/native/nginx-zhanduiwang.top.conf`

- [x] **Step 1: 记录隔离工作区状态**

Run:

```powershell
git branch --show-current
git log -1 --oneline
git status --short
git worktree list
```

Expected: 当前分支为 `codex/campus-services-review`，原项目工作区仍是另一条 worktree，当前分支的未提交改动不被覆盖。

- [x] **Step 2: 确认 API 与管理站解耦**

Run:

```powershell
rg -n "DEFAULT_API_BASE|/api/v1|admin-auth/session|__Host-zhandui_admin_session" apps/admin apps/miniapp backend
```

Expected: Admin 使用相对 `/api/v1`，小程序仍显式指向 `https://zhanduiwang.top/api/v1`，管理会话继续由 HTTPS 安全 Cookie 承载。

### Task 2: 添加可审查的 zhandui.top Nginx 资产

**Files:**
- Create: `deploy/native/nginx-zhandui.top.conf`
- Create: `deploy/native/nginx-zhandui.top.acme.conf`
- Modify: `scripts/check-native-deployment.cjs`
- Modify: `deploy/native/README.md`

- [x] **Step 1: 创建独立新域名配置**

配置必须包含（首版只启用 apex 域名 `zhandui.top`）：

```nginx
server_name zhandui.top;
root /var/www/campus-admin-zhandui/current;
location ^~ /api/ { proxy_pass http://127.0.0.1:8080; }
location = /healthz { proxy_pass http://127.0.0.1:8080/healthz; }
location = /readyz { allow 127.0.0.1; allow ::1; deny all; proxy_pass http://127.0.0.1:8080/readyz; }
location = /version { proxy_pass http://127.0.0.1:8080/version; }
location / { try_files $uri $uri/ /index.html; }
```

`nginx-zhandui.top.acme.conf` 只提供 `/.well-known/acme-challenge/`，用于证书签发前的安全过渡；`nginx-zhandui.top.conf` 的 HTTPS server 仅在证书文件存在并通过 `nginx -t` 后启用。两份配置不能同时安装同一个 80 端口 server block。

- [x] **Step 2: 扩展部署契约检查**

在 `scripts/check-native-deployment.cjs` 中把两份新配置列入 `requiredFiles`，检查 `server_name zhandui.top;`、ACME challenge、`/api/`、健康检查、HSTS、静态回退和 `/readyz` 的 loopback 限制；保留对旧配置的检查，避免迁移期间误删旧站。

- [x] **Step 3: 更新部署说明**

在 `deploy/native/README.md` 说明：管理站使用 `https://zhandui.top/`，小程序 API 仍使用 `https://zhanduiwang.top/api/v1`；记录 DNS 切换、证书申请、原子发布和回滚命令；不写入任何密钥。

### Task 3: 构建并验证管理站产物

**Files:**
- Inspect: `apps/admin/package.json`, `apps/admin/vite.config.ts`, `apps/admin/dist/`

- [x] **Step 1: 运行静态检查和构建**

Run:

```powershell
npm run typecheck --workspace apps/admin
npm run build:admin
npm run check:native-deployment
```

Expected: Admin typecheck、Vite production build 和 native deployment contract 均成功；构建产物只包含相对 `/api/v1` 请求，不写入小程序 API 域名。

- [x] **Step 2: 做本地静态产物检查**

Run:

```powershell
rg -n "zhanduiwang\.top|127\.0\.0\.1:8080|/api/v1" apps/admin/dist
```

Expected: 不出现硬编码生产 API 域名或本地后端地址；只允许 `/api/v1` 相对路径。

### Task 4: 准备线上 DNS、证书和原子发布步骤

**Files:**
- Create: `deploy/native/migrate-admin-domain.ps1`
- Inspect remotely: `/etc/nginx`, `/var/www/campus-admin-zhandui/current`, `/var/www/campus-admin/current`, `/etc/letsencrypt/live`

- [ ] **Step 1: 先验证 DNS 指向**

Run:

```powershell
Resolve-DnsName zhandui.top -Type A
Resolve-DnsName www.zhandui.top -Type A -ErrorAction SilentlyContinue
```

Observed on 2026-10-10: `zhandui.top` still resolves to `43.134.208.251`, so the script stops before any remote write. Change the Cloudflare A record to `8.138.227.107`, then rerun the script; `www.zhandui.top` can remain absent.

- [ ] **Step 2: 在 DNS 生效后申请证书**

远程只在 HTTP challenge 可达后执行：

```sh
sudo certbot certonly --webroot -w /var/www/letsencrypt \
  -d zhandui.top --non-interactive --keep-until-expiring
```

证书成功后检查 `/etc/letsencrypt/live/zhandui.top/fullchain.pem` 与 `privkey.pem`，再启用新域名 HTTPS 配置。若服务器没有可复用的 Certbot 账户，先由域名管理员提供运维邮箱并按 Certbot 提示完成注册，不把邮箱或账户凭据写入仓库。

- [ ] **Step 3: 原子发布 Admin**

迁移脚本已实现本地/远端 `sha256sum` 比对、独立 `/var/www/campus-admin-zhandui/current` 的 `ln -sfn` + `mv -Tf` 原子切换、旧 symlink 保存和分步回滚；旧 `/var/www/campus-admin/current` 不会被切换。待 DNS 生效后执行并记录输出。

### Task 5: 线上验收与回滚

**Files:**
- Create: `artifacts/admin-domain-migration-record.json`（仅记录状态，不含密钥）

- [ ] **Step 1: 验证新管理域名**

Run:

```powershell
Invoke-WebRequest https://zhandui.top/ -UseBasicParsing
Invoke-WebRequest https://zhandui.top/healthz -UseBasicParsing
Invoke-WebRequest https://zhandui.top/api/v1/admin-auth/session -SkipHttpErrorCheck -UseBasicParsing
```

Expected: 首页 `200`，`/healthz` 为 `200`，未登录会话返回 `401`；响应包含 HTTPS 安全头，登录后刷新仍保持会话。

- [ ] **Step 2: 回归旧域名和小程序 API**

Run:

```powershell
Invoke-WebRequest https://zhanduiwang.top/healthz -UseBasicParsing
Invoke-WebRequest https://zhanduiwang.top/api/v1/navigation/categories -UseBasicParsing
```

Expected: 两个旧入口继续可用，小程序 API 域名不变。

- [ ] **Step 3: 保留回滚点**

迁移失败时只回滚新域名 Nginx 配置和 Admin symlink，不删除旧证书、旧 release 或旧域名 server block；DNS 在验收完成前不改回旧值。

## 自我评审

- DNS、证书和服务器配置分别有独立步骤，任何一项未满足都不会中断现有 `zhanduiwang.top`。
- 迁移脚本要求单一 A 记录、无 AAAA 分流，且要求 DNS 目标与 SSH 目标一致；上传包先做本地/远端 SHA-256 比对。
- Admin `current` 与 Nginx site link 都保存旧目标并独立回滚，ACME webroot 在申请前用真实本机 Host 请求验证。
- 新域名只承载管理站，未把小程序 API 或功能策略实验自动带入生产。
- 新域名使用独立静态根目录，旧管理入口的静态文件和 symlink 不会因本次切换改变。
- 生产功能策略中心仍需单独决策；本计划不把 `ENABLE_FEATURE_POLICY_LAB` 在 `APP_ENV=prod` 中打开。
- 所有线上命令均不包含密码、AppSecret、私钥或 Cloudflare token。
