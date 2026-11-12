# 小程序主包分包瘦身 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不删除功能、不压缩代码、不改变现有页面 URL 和交互的前提下，把 12 个低频页面迁入分包，降低主包体积并保留足够的上线余量。

**Architecture:** 只修改 `apps/miniapp/src/app.config.ts` 的 `pages`、`subPackages` 和 `preloadRule`。每个现有页面目录继续作为独立分包根目录，因此 `/pages/<name>/index` 对外地址保持不变；主包只保留启动页、底部导航页和发布主流程。分包路由通过 `preloadRule` 在入口页面空闲时预下载。

**Tech Stack:** Taro 4、React、微信小程序分包、Node.js 契约测试、微信开发者工具 CLI。

---

### Task 1: 用测试锁住主包与分包边界

**Files:**
- Modify: `scripts/check-miniapp-static-routes.cjs`
- Test: `scripts/check-miniapp-static-routes.cjs`

- [ ] **Step 1: 添加目标分包路由清单**

在 `expectedRoutes` 后加入：

```js
const expectedLazyRoutes = [
  'pages/search/index',
  'pages/post-detail/index',
  'pages/secondhand-detail/index',
  'pages/user-profile/index',
  'pages/identity-application/index',
  'pages/my-content/index',
  'pages/edit-profile/index',
  'pages/settings/index',
  'pages/notifications/index',
  'pages/merchant-home/index',
  'pages/delivery-rider/index',
  'pages/delivery-conversation/index',
]
```

- [ ] **Step 2: 分开解析主包 pages 与 subPackages**

将当前的 `mainRoutes` 解析替换为：

```js
const mainPagesBlock = config.match(/pages:\s*\[([\s\S]*?)\],\s*subPackages:/)?.[1] || ''
const mainRoutes = [...mainPagesBlock.matchAll(/'([^']+)'/g)].map((match) => match[1])
const subpackageRoutes = [...config.matchAll(/root:\s*'([^']+)'[\s\S]*?pages:\s*\[([\s\S]*?)\]/g)]
  .flatMap(([, packageRoot, packagePages]) => [...packagePages.matchAll(/'([^']+)'/g)]
    .map((match) => `${packageRoot}/${match[1]}`))
```

- [ ] **Step 3: 添加分包边界断言**

在现有 `brokenTargets` 检查前加入：

```js
const mainSet = new Set(mainRoutes)
const subpackageSet = new Set(subpackageRoutes)
for (const route of expectedLazyRoutes) {
  if (mainSet.has(route)) throw new Error(`low-frequency route must not stay in main package: ${route}`)
  if (!subpackageSet.has(route)) throw new Error(`low-frequency route must be registered in a subpackage: ${route}`)
}
for (const route of expectedRoutes) {
  if (!expectedLazyRoutes.includes(route) && !mainSet.has(route)) {
    throw new Error(`primary route must stay in main package: ${route}`)
  }
}
```

- [ ] **Step 4: 运行测试并确认先失败**

Run:

```powershell
node scripts/check-miniapp-static-routes.cjs
```

Expected: FAIL，首个错误为 `low-frequency route must not stay in main package: pages/search/index`。

- [ ] **Step 5: Commit**

```powershell
git add scripts/check-miniapp-static-routes.cjs
git commit -m "测试：限制小程序主包与分包边界"
```

### Task 2: 迁移低频页面并增加预下载

**Files:**
- Modify: `apps/miniapp/src/app.config.ts`

- [ ] **Step 1: 从主包 pages 删除 12 个低频页面**

从 `pages` 数组删除：

```ts
'pages/search/index',
'pages/post-detail/index',
'pages/secondhand-detail/index',
'pages/user-profile/index',
'pages/identity-application/index',
'pages/my-content/index',
'pages/edit-profile/index',
'pages/settings/index',
'pages/notifications/index',
'pages/merchant-home/index',
'pages/delivery-rider/index',
'pages/delivery-conversation/index'
```

保留 `home`、`profile`、`student-takeout`、二手列表、发布主流程以及商家/骑手消息与资料等高频入口。

- [ ] **Step 2: 在原 URL 下注册独立分包**

在 `subPackages` 中追加：

```ts
{ root: 'pages/search', pages: ['index'] },
{ root: 'pages/post-detail', pages: ['index'] },
{ root: 'pages/secondhand-detail', pages: ['index'] },
{ root: 'pages/user-profile', pages: ['index'] },
{ root: 'pages/identity-application', pages: ['index'] },
{ root: 'pages/my-content', pages: ['index'] },
{ root: 'pages/edit-profile', pages: ['index'] },
{ root: 'pages/settings', pages: ['index'] },
{ root: 'pages/notifications', pages: ['index'] },
{ root: 'pages/merchant-home', pages: ['index'] },
{ root: 'pages/delivery-rider', pages: ['index'] },
{ root: 'pages/delivery-conversation', pages: ['index'] }
```

- [ ] **Step 3: 在入口页预下载分包**

增加 `preloadRule`：

```ts
preloadRule: {
  'pages/home/index': {
    network: 'all',
    packages: ['pages/search', 'pages/post-detail', 'pages/secondhand-detail', 'pages/user-profile', 'pages/notifications']
  },
  'pages/student-takeout/index': {
    network: 'all',
    packages: ['pages/search']
  },
  'pages/student-secondhand/index': {
    network: 'all',
    packages: ['pages/secondhand-detail']
  },
  'pages/profile/index': {
    network: 'all',
    packages: ['pages/identity-application', 'pages/my-content', 'pages/edit-profile', 'pages/settings']
  },
  'pages/merchant-messages/index': {
    network: 'all',
    packages: ['pages/merchant-home', 'pages/delivery-conversation']
  },
  'pages/rider-messages/index': {
    network: 'all',
    packages: ['pages/delivery-rider', 'pages/delivery-conversation']
  }
}
```

- [ ] **Step 4: 运行路由测试**

Run:

```powershell
node scripts/check-miniapp-static-routes.cjs
```

Expected: `miniapp static route check PASS (32 routes)`。

- [ ] **Step 5: Commit**

```powershell
git add apps/miniapp/src/app.config.ts
git commit -m "优化：将低频页面迁入小程序分包"
```

### Task 3: 验证功能和包体边界

**Files:**
- Verify: `apps/miniapp/src/app.config.ts`
- Verify: `apps/miniapp/dist/app.json`
- Verify: `apps/miniapp/dist/pages/**`

- [ ] **Step 1: 类型检查**

Run:

```powershell
npm --workspace apps/miniapp run typecheck
```

Expected: exit 0。

- [ ] **Step 2: 跑首页、导航、消息、支付和工作台契约**

Run:

```powershell
npm run check:miniapp-static-routes
npm run check:miniapp-back-navigation
npm run check:private-messaging
npm run check:delivery-stage-workbench
npm run check:release-regression
```

Expected: 全部 exit 0。

- [ ] **Step 3: 用支付开启参数构建**

Run:

```powershell
$env:TARO_APP_ENABLE_DELIVERY_PAYMENT='true'
$env:TARO_APP_ENABLE_DELIVERY_PAYMENT_MOCK='false'
npm run build:miniapp
```

Expected: exit 0，只有既有 CSS 顺序和大包警告。

- [ ] **Step 4: 核对主包下降和页面文件完整**

Run:

```powershell
node -e 'const fs=require("node:fs");const app=JSON.parse(fs.readFileSync("apps/miniapp/dist/app.json","utf8"));console.log(JSON.stringify({main:app.pages,subPackages:app.subPackages,preloadRule:app.preloadRule},null,2))'
```

Expected: 12 个目标页面不在 `app.pages`，并在 `app.subPackages`；32 个页面仍然全部注册。

- [ ] **Step 5: 在微信开发者工具逐个直达迁移页面**

Run each route with the WeChat IDE automation tool:

```text
pages/search/index?scope=community
pages/post-detail/index?id=1
pages/secondhand-detail/index?id=1
pages/user-profile/index?userId=1
pages/identity-application/index?role=rider
pages/my-content/index
pages/edit-profile/index
pages/settings/index
pages/notifications/index
pages/merchant-home/index
pages/delivery-rider/index
pages/delivery-conversation/index?orderId=1
```

Expected: 每次直达后 `automation_runtime_info --action currentPage` 返回对应路由；业务守卫可以显示登录/资格状态，但不得出现“页面不存在”或白屏。

- [ ] **Step 6: 对比包体**

先记录基线：

```text
main=1,985,503 bytes
total=2,147,083 bytes
```

迁移后要求：

```text
main < 1,750,000 bytes
每页 2 MiB 以下
总包 20 MiB 以下
```

- [ ] **Step 7: Commit**

```powershell
git add apps/miniapp/src/app.config.ts
git commit -m "验证：确认小程序分包后路由与包体正常"
```

### Task 4: 上传并留存回执

**Files:**
- Create: `.release/20261112-subpackage-slimming/manifest.json`
- Modify: `docs/operations/2026-11-12-production-release.md`

- [ ] **Step 1: 官方 CLI 上传体验版**

Run with a new version:

```powershell
& 'D:\3314\微信web开发者工具\cli.bat' upload `
  --project 'C:\Users\33141\.codex\worktrees\experience-ui-wallet-20261101\New project\apps\miniapp' `
  --version '2026.11.12.5' `
  --desc '主包分包瘦身与路由回归' `
  --info-output 'C:\Users\33141\.codex\worktrees\experience-ui-wallet-20261101\New project\.codex-tmp\upload-2026.11.12.5-subpackages.json'
```

Expected: `√ upload`，记录总包、主包和分包字节。

- [ ] **Step 2: 生成开发预览二维码**

Run:

```powershell
& 'D:\3314\微信web开发者工具\cli.bat' preview `
  --project 'C:\Users\33141\.codex\worktrees\experience-ui-wallet-20261101\New project\apps\miniapp' `
  --qr-format image `
  --qr-output 'C:\Users\33141\.codex\worktrees\experience-ui-wallet-20261101\New project\.codex-tmp\preview-2026.11.12.5-subpackages.png'
```

Expected: 生成可扫描二维码，页面包体与上传回执一致。

- [ ] **Step 3: 记录结果并推送**

更新生产发布记录，写入旧/新主包大小、迁移页面、验证命令和仍待真机验证的跳转页面，然后：

```powershell
git diff --check
git add docs/operations/2026-11-12-production-release.md
git commit -m "记录：小程序主包分包瘦身与体验版"
git push origin codex/experience-ui-wallet-20261101
```
