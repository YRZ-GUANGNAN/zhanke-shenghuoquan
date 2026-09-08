# 湛科生活圈生产发布 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将当前隔离分支已提交的后端、后台管理站点和新 AppID 小程序安全发布到现有生产环境，并保留可验证的回滚点。

**Architecture:** 使用临时 Git worktree 生成不含用户脏文件的发布源；后端和后台分别上传到带版本号的不可变目录，再通过软链接原子切换。服务器环境只更新真实微信登录所需三项，小程序通过微信开发者工具上传开发版本，平台审核与正式发布状态单独记录。

**Tech Stack:** Git worktree、Go、React/Vite、Taro、PowerShell、OpenSSH、systemd、Nginx、微信开发者工具 CLI

---

### Task 1: 固化发布源并执行发布前验证

**Files:**
- Verify: `package.json`
- Verify: `scripts/build-native-backend.ps1`
- Verify: `apps/miniapp/project.config.json`
- Preserve: `apps/miniapp/src/components/student-bottom-nav.scss`
- Preserve: `apps/miniapp/src/components/student-bottom-nav.tsx`
- Preserve: `apps/miniapp/src/pages/student-takeout/index.scss`
- Preserve: `apps/miniapp/src/pages/student-takeout/index.tsx`

- [ ] **Step 1: 记录源工作区状态和发布提交**

Run: `git status --short --branch; git rev-parse HEAD`

Expected: 分支为 `codex/campus-services-review`，只显示四个既有 MiniApp 修改和 `artifacts/`，发布提交可解析。

- [ ] **Step 2: 创建干净临时 worktree**

Run: `$releaseVersion='2026.09.08.1'; $releaseCommit=(git rev-parse --short=12 HEAD).Trim(); $releaseId=(Get-Date -Format 'yyyyMMdd_HHmmss')+'-'+$releaseCommit; $releaseSource=Join-Path ([System.IO.Path]::GetTempPath()) ('campus-release-'+[guid]::NewGuid().ToString('N')); git worktree add --detach $releaseSource HEAD`

Expected: 新 worktree 为 detached HEAD，`git -C $releaseSource status --porcelain` 无输出。

- [ ] **Step 3: 执行契约、类型和后端验证**

Run: `npm run check:real-wechat-secret-hygiene; npm run check:native-deployment; npm run check:miniapp-publish-contract; npm run check:admin-ui-system; npm run check:admin-password-session-contract; npm run typecheck; npm run test:backend`

Expected: 所有命令退出码为 0。

### Task 2: 构建可追踪生产产物

**Files:**
- Build output: `$releaseSource/artifacts/release/campus-api`
- Build output: `$releaseSource/apps/admin/dist/`
- Build output: `$releaseSource/apps/miniapp/dist/`

- [ ] **Step 1: 安装锁定依赖**

Run: `npm ci`

Expected: 退出码为 0，依赖版本来自 `package-lock.json`。

- [ ] **Step 2: 构建后端**

Run: `.\scripts\build-native-backend.ps1 -Version 2026.09.08.1 -OutputPath artifacts/release/campus-api`

Expected: 生成二进制和 `.sha256`，构建元数据包含当前提交。

- [ ] **Step 3: 构建后台和小程序**

Run: `npm run build:admin; $env:TARO_APP_API_BASE='https://zhanduiwang.top/api/v1'; npm run build:miniapp`

Expected: 两项构建退出码为 0，后台与小程序静态产物生成。

- [ ] **Step 4: 验证生产地址和 AppID**

Run: `rg -n 'localhost|127\.0\.0\.1' apps/admin/dist apps/miniapp/dist; (Get-Content apps/miniapp/dist/project.config.json -Raw | ConvertFrom-Json).appid`

Expected: 静态产物不包含本地 API 地址，输出目标 AppID。

### Task 3: 发布后端并验证回滚能力

**Files:**
- Deploy: `/opt/campus-circle/releases/$releaseId/bin/campus-api`
- Update: `/etc/campus-circle/campus-api.env`
- Update symlink: `/opt/campus-circle/current`

- [ ] **Step 1: 记录旧链接并创建新版本目录**

Run: `ssh admin@zhanduiwang.top "readlink -f /opt/campus-circle/current; sudo install -d -o campus -g campus -m 0750 /opt/campus-circle/releases/$releaseId/bin"`

Expected: 输出旧版本绝对路径，新目录创建成功。

- [ ] **Step 2: 上传并校验后端二进制**

Run: `scp "$releaseSource/artifacts/release/campus-api" "admin@zhanduiwang.top:/tmp/campus-api.$releaseId"; ssh admin@zhanduiwang.top "sudo install -o campus -g campus -m 0750 /tmp/campus-api.$releaseId /opt/campus-circle/releases/$releaseId/bin/campus-api && rm /tmp/campus-api.$releaseId && sha256sum /opt/campus-circle/releases/$releaseId/bin/campus-api"`

Expected: 远端 SHA-256 与本地产物一致。

- [ ] **Step 3: 安全更新微信环境**

Run: 从 Git 忽略的 `backend/.env` 读取 `WECHAT_APP_ID` 和 `WECHAT_APP_SECRET`，通过 SSH 标准输入交给服务器端脚本；脚本先把原环境文件复制为时间戳备份，再精确替换两项并写入 `ALLOW_MOCK_WECHAT_LOGIN=false`。

Expected: 服务器只回显更新后的变量名和 AppID，绝不回显 Secret；环境文件权限保持 `0600`。

- [ ] **Step 4: 原子切换并重启**

Run: `ssh admin@zhanduiwang.top "sudo ln -sfn /opt/campus-circle/releases/$releaseId /opt/campus-circle/current.next && sudo mv -Tf /opt/campus-circle/current.next /opt/campus-circle/current && sudo systemctl restart campus-api"`

Expected: `campus-api` 处于 active。

- [ ] **Step 5: 验证健康与版本**

Run: `ssh admin@zhanduiwang.top 'curl -fsS http://127.0.0.1:8080/healthz && curl -fsS http://127.0.0.1:8080/readyz && curl -fsS http://127.0.0.1:8080/version'`

Expected: health 为 ok、ready 为 ready、version 为 `2026.09.08.1` 且提交匹配发布源。

### Task 4: 发布后台管理站点

**Files:**
- Deploy: `/var/www/campus-admin/releases/$releaseId/`
- Update symlink: `/var/www/campus-admin/current`

- [ ] **Step 1: 上传后台静态包到新目录**

Run: 将 `apps/admin/dist` 打包上传，在服务器以 `root:root` 和目录 `0755`、文件 `0644` 解压到新的版本目录。

Expected: 新目录包含 `index.html` 与 `assets/`，且没有覆盖当前目录。

- [ ] **Step 2: 原子切换站点链接并检查 Nginx**

Run: `ssh admin@zhanduiwang.top "sudo ln -sfn /var/www/campus-admin/releases/$releaseId /var/www/campus-admin/current.next && sudo mv -Tf /var/www/campus-admin/current.next /var/www/campus-admin/current && sudo nginx -t && sudo systemctl reload nginx"`

Expected: Nginx 配置检查通过并成功 reload。

- [ ] **Step 3: 执行公网冒烟检查**

Run: `Invoke-WebRequest https://zhanduiwang.top/; Invoke-WebRequest https://zhanduiwang.top/healthz; Invoke-WebRequest https://zhanduiwang.top/version`

Expected: 三个请求均成功，版本接口返回本次版本。

### Task 5: 上传微信小程序开发版本

**Files:**
- Upload source: `$releaseSource/apps/miniapp/dist/`

- [ ] **Step 1: 检查微信开发者工具登录状态**

Run: `& 'C:\Program Files (x86)\Tencent\微信web开发者工具\cli.bat' islogin --lang zh`

Expected: CLI 确认已登录；若 IDE CLI 端口未初始化，则先用该工具打开项目并完成本机登录初始化。

- [ ] **Step 2: 上传开发版本**

Run: `& 'C:\Program Files (x86)\Tencent\微信web开发者工具\cli.bat' upload --project (Join-Path $releaseSource 'apps\miniapp\dist') --version $releaseVersion --desc '后台与真实微信登录生产发布' --lang zh`

Expected: CLI 返回上传成功和版本信息。若平台要求管理员扫码、审核或发布确认，记录为平台待办，不绕过权限。

### Task 6: 收尾验证与保留证据

**Files:**
- Verify: source worktree status
- Remove: temporary release worktree only

- [ ] **Step 1: 复核生产状态**

Run: 再次检查 systemd、服务器本机 ready、公开 health/version、后台首页和 MiniApp 上传结果。

Expected: 生产服务稳定，版本一致。

- [ ] **Step 2: 删除临时发布 worktree**

Run: `git worktree remove $releaseSource`

Expected: 只删除本次临时干净副本，不影响隔离工作区和原工作区。

- [ ] **Step 3: 复核用户文件未受影响**

Run: `git status --short --branch`

Expected: 原有四个未提交 MiniApp 文件和 `artifacts/` 仍在，除此之外没有发布过程产生的意外修改。
