# WeChat Test Release wx0dacef976daa8e55 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build and upload a WeChat mini-program test version for AppID `wx0dacef976daa8e55` that calls `https://zhanduiwang.top/api/v1`.

**Architecture:** Keep the public API origin injected only at production build time, while the repository project metadata identifies the new mini-program account. Verify the public HTTPS service and compiled bundle before using the WeChat Developer Tools CLI; treat account login, legal-domain registration, and the matching server-side AppSecret as external release gates.

**Tech Stack:** Taro 4, React 18, TypeScript, WeChat Developer Tools CLI, PowerShell, Nginx/Go API.

---

### Task 1: Preserve the current workspace and verify the public target

**Files:**
- Inspect: repository worktree and deployment configuration
- Inspect: `deploy/native/nginx-zhanduiwang.top.conf`

- [x] **Step 1: Record the dirty worktree without modifying unrelated files**

Run:

```powershell
git status --short
git worktree list --porcelain
```

Expected: existing deployment changes remain visible and are not reset or overwritten.

- [x] **Step 2: Verify DNS, TLS, health, and one public API path**

Run:

```powershell
Resolve-DnsName zhanduiwang.top -Type A
curl.exe -fsS --max-time 20 https://zhanduiwang.top/healthz
curl.exe -fsS --max-time 20 "https://zhanduiwang.top/api/v1/trade/products?page=1&pageSize=1"
```

Expected: DNS resolves, HTTPS succeeds, and both HTTP requests return JSON with `"code":0`.

### Task 2: Switch repository metadata to the new mini-program account

**Files:**
- Modify: `apps/miniapp/project.config.json`
- Modify: `apps/miniapp/project.private.config.example.json`
- Modify: `.env.example`
- Modify: `backend/.env.example`
- Modify: `docs/G-7-QUICKSTART-LOCAL-VERIFY.md`

- [x] **Step 1: Replace only the previous AppID values**

Set each public example/config value to:

```text
wx0dacef976daa8e55
```

Do not add `WECHAT_APP_SECRET` to Git-tracked files.

- [x] **Step 2: Confirm no previous AppID remains in active configuration**

Run:

```powershell
rg -n "wx961bc6699396f25b|wx0dacef976daa8e55" .env.example backend/.env.example apps/miniapp/project.config.json apps/miniapp/project.private.config.example.json docs/G-7-QUICKSTART-LOCAL-VERIFY.md
```

Expected: only `wx0dacef976daa8e55` is reported in those files.

### Task 3: Build and inspect the production mini-program bundle

**Files:**
- Generate: `apps/miniapp/dist/**`

- [x] **Step 1: Run static/type checks**

Run:

```powershell
npm --prefix apps/miniapp run typecheck
npm run check:miniapp-static-routes
```

Expected: both commands exit with code 0.

- [x] **Step 2: Build with the public HTTPS API base**

Run:

```powershell
$env:NODE_ENV='production'
$env:TARO_APP_API_BASE='https://zhanduiwang.top/api/v1'
npm run build:miniapp
```

Expected: Taro build exits with code 0 and produces `apps/miniapp/dist/app.json`.

- [x] **Step 3: Reject local API leakage and verify package metadata**

Run:

```powershell
if (rg -n "127\.0\.0\.1|localhost" apps/miniapp/dist) { throw 'Local API address leaked into production bundle' }
rg -n "https://zhanduiwang\.top/api/v1" apps/miniapp/dist
Get-Content -Raw apps/miniapp/project.config.json | ConvertFrom-Json | Select-Object appid,miniprogramRoot
```

Expected: no local address is found, the public API base is present, and AppID/root match the release target.

### Task 4: Verify account gates and upload the test version

**Files:**
- Generate: `.release/wechat-upload-wx0dacef976daa8e55.json`

- [ ] **Step 1: Check Developer Tools login for the target project**

Current blocker: the logged-in WeChat account shown by Developer Tools is `广柚`, and Developer Tools reports that this account is not a developer of AppID `wx0dacef976daa8e55`.

Run:

```powershell
& 'C:\Program Files (x86)\Tencent\微信web开发者工具\cli.bat' islogin --project 'C:\Users\33141\Documents\New project\apps\miniapp' --lang zh
```

Expected: the IDE reports a logged-in developer who has permission for AppID `wx0dacef976daa8e55`; otherwise the user scans the displayed QR code and the check is repeated.

- [ ] **Step 2: Confirm external release settings**

In the WeChat mini-program administration console, add `https://zhanduiwang.top` as the request/download/upload legal domain as required by the used APIs. On the production host, set `WECHAT_APP_ID=wx0dacef976daa8e55` together with the matching secret in `/etc/campus-circle/campus-api.env`, then restart `campus-api`; never place the secret in chat, command output, or Git.

- [ ] **Step 3: Upload version 0.1.0 as a test build**

Run:

```powershell
& 'C:\Program Files (x86)\Tencent\微信web开发者工具\cli.bat' upload --project 'C:\Users\33141\Documents\New project\apps\miniapp' --version '0.1.0' --desc '测试版：接入 zhanduiwang.top' --info-output 'C:\Users\33141\Documents\New project\.release\wechat-upload-wx0dacef976daa8e55.json' --lang zh
```

Expected: CLI exits with code 0 and the WeChat console lists version `0.1.0`. Mark it as the experience/test version and assign testers in the console.

### Task 5: Report evidence without overstating acceptance

**Files:**
- Inspect: `.release/wechat-upload-wx0dacef976daa8e55.json`

- [ ] **Step 1: Recheck workspace preservation**

Run:

```powershell
git status --short
```

Expected: pre-existing edits remain; only the intentional AppID/plan changes and ignored release artifacts are added.

- [ ] **Step 2: Record the release outcome**

Report build/typecheck/public-health evidence separately from external gates. If login, legal-domain registration, AppSecret rotation, or WeChat upload is unavailable, label that item `PENDING` or `BLOCKED` rather than claiming the test version is usable.
