# UI-21a Taro Page Mount Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复微信开发者工具冷启动后 session 已认证但页面根节点仍为空的问题，并关闭 UI-21a 剩余五项真实运行 CONDITIONAL 验收。

**Architecture:** `App` 从首帧开始永久挂载 `props.children`，只保留 bootstrap 与身份首页恢复副作用。四个身份入口页采用“外层 `RuntimeSessionGate` + 内层 Content”结构，只有 session 已认证后才挂载页面 hooks 和主体；学生、骑手、商家 Content 再由 `useIdentityGuard` fail-closed 校验服务端身份。

**Tech Stack:** Taro 4.2、React 18、TypeScript、Zustand、微信开发者工具 CLI、`miniprogram-automator`、真实 Go/Gin 后端与 MySQL。

---

## 文件结构

- Create: `apps/miniapp/src/components/runtime-session-gate.tsx` — 唯一负责 session 启动、错误和重试呈现。
- Modify: `apps/miniapp/src/app.tsx` — 始终挂载页面，保留 bootstrap/身份路由恢复。
- Modify: `apps/miniapp/src/pages/home/index.tsx` — 学生首页外层 Gate 与 student guard。
- Modify: `apps/miniapp/src/pages/profile/index.tsx` — Profile 外层 Gate 与 student guard，避免 `useDidShow` 抢跑。
- Modify: `apps/miniapp/src/pages/merchant-home/index.tsx` — 商家页外层 Gate，既有 merchant guard 留在 Content。
- Modify: `apps/miniapp/src/pages/delivery-rider/index.tsx` — 骑手页外层 Gate，既有 rider guard 留在 Content。
- Modify: `docs/UI-21A-RUNTIME-IDENTITY-DEVELOPMENT.md` — 记录根因和修复。
- Modify: `docs/UI-21A-RUNTIME-IDENTITY-ACCEPTANCE.md` — 将五项 CONDITIONAL 替换为真实验收证据。
- Modify: `docs/UI-21A-RUNTIME-IDENTITY-RESIDUE.md` — 记录进程、临时目录和最终清理结果。
- Temporary only: `D:/codex-temp/ui21a-automator/assert-mounted.js` — 官方自动化挂载断言，不提交仓库。

### Task 1: 固化当前白屏为失败验收

**Files:**
- Create temporarily: `D:/codex-temp/ui21a-automator/assert-mounted.js`
- Read: `D:/codex-temp/ui21a-live/apps/miniapp/dist/app.json`

- [ ] **Step 1: 写入当前必失败的官方自动化断言**

在临时目录创建以下脚本，不加入 Git：

```js
const assert = require('node:assert/strict')
const automator = require('miniprogram-automator')

async function main() {
  const miniProgram = await automator.connect({ wsEndpoint: 'ws://127.0.0.1:9420' })
  const page = await miniProgram.reLaunch('/pages/home/index')
  await page.waitFor(5000)

  const root = await page.$('page')
  assert.ok(root, 'page root must exist')
  const rootWxml = (await root.outerWxml()).replace(/\s+/g, '')
  assert.notEqual(rootWxml, '<page></page>', 'page root must not stay empty after bootstrap')

  const home = await page.$('.student-home-page')
  assert.ok(home, 'authenticated student home must mount')
  console.log(JSON.stringify({ path: page.path, rootWxml: rootWxml.slice(0, 240) }, null, 2))
  miniProgram.disconnect()
}

main().catch((error) => {
  console.error(error)
  process.exitCode = 1
})
```

- [ ] **Step 2: 启动官方自动化端口并证明修复前失败**

先把验证 clone 固定到最后一个尚未包含挂载修复的代码提交 `6515cb82`，重新构建，且确认之后的设计提交没有改动小程序源码：

```powershell
git -C 'D:\codex-temp\ui21a-live' reset --hard 6515cb82
git -C 'C:\Users\33141\Documents\ui21a-runtime-identity' diff --exit-code 6515cb82..472894e0 -- apps/miniapp/src
Set-Location 'D:\codex-temp\ui21a-live'
npm run build:miniapp
& 'D:\3314\微信web开发者工具\cli.bat' auto --project 'D:\codex-temp\ui21a-live\apps\miniapp' --port 50628 --auto-port 9420 --trust-project --lang zh
Set-Location 'D:\codex-temp\ui21a-automator'
node .\assert-mounted.js
```

Expected: FAIL，错误包含 `page root must not stay empty after bootstrap`，并与原始 `<page></page>` 证据一致。

### Task 2: 实现首帧永久挂载与页面级 Gate

**Files:**
- Create: `apps/miniapp/src/components/runtime-session-gate.tsx`
- Modify: `apps/miniapp/src/app.tsx`
- Modify: `apps/miniapp/src/pages/home/index.tsx`
- Modify: `apps/miniapp/src/pages/profile/index.tsx`
- Modify: `apps/miniapp/src/pages/merchant-home/index.tsx`
- Modify: `apps/miniapp/src/pages/delivery-rider/index.tsx`

- [ ] **Step 1: 新增单一职责的 RuntimeSessionGate**

Create `apps/miniapp/src/components/runtime-session-gate.tsx`：

```tsx
import type { ReactNode } from 'react'
import { Text, View } from '@tarojs/components'
import { useSessionStore } from '../stores/session'

interface RuntimeSessionGateProps {
  children: ReactNode
}

export default function RuntimeSessionGate({ children }: RuntimeSessionGateProps) {
  const status = useSessionStore((state) => state.status)
  const error = useSessionStore((state) => state.error)
  const retry = useSessionStore((state) => state.retry)

  if (status === 'idle' || status === 'booting') {
    return (
      <View className='runtime-session-state'>
        <Text className='runtime-session-state__title'>正在登录...</Text>
      </View>
    )
  }

  if (status !== 'authenticated') {
    return (
      <View className='runtime-session-state'>
        <Text className='runtime-session-state__title'>暂时无法登录</Text>
        <Text className='runtime-session-state__message'>{error || '请重新尝试微信登录'}</Text>
        <View className='runtime-session-state__retry' onClick={() => void retry()}>
          <Text>重试</Text>
        </View>
      </View>
    )
  }

  return <>{children}</>
}
```

- [ ] **Step 2: 让 App 首帧始终返回页面 children**

将 `apps/miniapp/src/app.tsx` 改为以下完整结构；移除 `Text/View`、`error`、`retry` 和所有条件 return：

```tsx
import React, { useEffect, useRef } from 'react'
import { identityRoute } from './hooks/use-identity-guard'
import Taro from './services/taro'
import { useSessionStore } from './stores/session'
import './app.scss'

export default function App(props: { children: React.ReactNode }) {
  const status = useSessionStore((state) => state.status)
  const identity = useSessionStore((state) => state.identity)
  const bootstrap = useSessionStore((state) => state.bootstrap)
  const restoredIdentityRoute = useRef(false)

  useEffect(() => {
    void bootstrap()
  }, [bootstrap])

  useEffect(() => {
    if (status !== 'authenticated') {
      restoredIdentityRoute.current = false
      return
    }
    if (!identity || restoredIdentityRoute.current) return

    let cancelled = false
    let retryTimer: ReturnType<typeof setTimeout> | undefined

    const restoreIdentityRoute = (attempt = 0) => {
      if (cancelled || restoredIdentityRoute.current) return
      const path = Taro.getCurrentInstance().router?.path?.replace(/^\//, '')
      if (!path) {
        if (attempt < 20) retryTimer = setTimeout(() => restoreIdentityRoute(attempt + 1), 50)
        return
      }

      restoredIdentityRoute.current = true
      if (identity.currentIdentity === 'student') return
      if (path === 'pages/home/index' || path === 'pages/profile/index') {
        void Taro.reLaunch({ url: identityRoute(identity.currentIdentity) }).catch(() => {
          restoredIdentityRoute.current = false
        })
      }
    }

    restoreIdentityRoute()
    return () => {
      cancelled = true
      if (retryTimer) clearTimeout(retryTimer)
    }
  }, [identity, status])

  return props.children
}
```

- [ ] **Step 3: 改造学生首页为外层 Gate + 内层 Content**

在 `pages/home/index.tsx` 增加 imports：

```tsx
import RuntimeSessionGate from '../../components/runtime-session-gate'
import { useIdentityGuard } from '../../hooks/use-identity-guard'
```

把现有 `export default function HomePage()` 改名为 `function HomeContent()`，在所有 React hooks 之后增加：

```tsx
const identityAllowed = useIdentityGuard('student')
if (!identityAllowed) return <View className='student-home-page' />
```

文件末尾新增明确的外层页面：

```tsx
export default function HomePage() {
  return (
    <RuntimeSessionGate>
      <HomeContent />
    </RuntimeSessionGate>
  )
}
```

- [ ] **Step 4: 改造 Profile，确保 useDidShow 不在 bootstrap 前执行**

在 `pages/profile/index.tsx` 增加 imports：

```tsx
import RuntimeSessionGate from '../../components/runtime-session-gate'
import { identityRoute, useIdentityGuard } from '../../hooks/use-identity-guard'
```

删除原来仅导入 `identityRoute` 的重复 import。把现有 `ProfilePage` 改名为 `ProfileContent`，在 hooks 区加入：

```tsx
const identityAllowed = useIdentityGuard('student')
```

在 JSX return 前加入：

```tsx
if (!identityAllowed) return <View className='student-profile-page' />
```

文件末尾加入：

```tsx
export default function ProfilePage() {
  return (
    <RuntimeSessionGate>
      <ProfileContent />
    </RuntimeSessionGate>
  )
}
```

- [ ] **Step 5: 改造商家页并保留既有 merchant guard**

在 `pages/merchant-home/index.tsx` 导入 `RuntimeSessionGate`，把现有 `MerchantHomePage` 改名为 `MerchantHomeContent`。保持 `useIdentityGuard('merchant')` 和空安全壳原位，文件末尾增加：

```tsx
export default function MerchantHomePage() {
  return (
    <RuntimeSessionGate>
      <MerchantHomeContent />
    </RuntimeSessionGate>
  )
}
```

- [ ] **Step 6: 改造骑手页并保留既有 rider guard**

在 `pages/delivery-rider/index.tsx` 导入 `RuntimeSessionGate`，把现有 `DeliveryRiderPage` 改名为 `DeliveryRiderContent`。保持 `useIdentityGuard('rider')` 和空安全壳原位，文件末尾增加：

```tsx
export default function DeliveryRiderPage() {
  return (
    <RuntimeSessionGate>
      <DeliveryRiderContent />
    </RuntimeSessionGate>
  )
}
```

- [ ] **Step 7: 把未提交实现同步到临时验证 clone**

不在 C 盘 worktree 安装第二份依赖。把六个实现文件复制到已有临时 clone；该 clone 只用于构建和 DevTools 验收，不提交：

```powershell
$source = 'C:\Users\33141\Documents\ui21a-runtime-identity'
$target = 'D:\codex-temp\ui21a-live'
$files = @(
  'apps/miniapp/src/app.tsx',
  'apps/miniapp/src/components/runtime-session-gate.tsx',
  'apps/miniapp/src/pages/home/index.tsx',
  'apps/miniapp/src/pages/profile/index.tsx',
  'apps/miniapp/src/pages/merchant-home/index.tsx',
  'apps/miniapp/src/pages/delivery-rider/index.tsx'
)
foreach ($file in $files) {
  $destination = Join-Path $target $file
  New-Item -ItemType Directory -Force -Path (Split-Path $destination) | Out-Null
  Copy-Item -LiteralPath (Join-Path $source $file) -Destination $destination -Force
}
```

- [ ] **Step 8: 运行静态验证并修正全部类型错误**

Run in `D:/codex-temp/ui21a-live`：

```powershell
npm --workspace apps/miniapp run typecheck
npm run check:miniapp-static-routes
npm run build:miniapp
```

Expected: three commands PASS; only the existing vendors-size warning may remain.

- [ ] **Step 9: 重新运行挂载断言**

Run:

```powershell
Set-Location 'D:\codex-temp\ui21a-automator'
node .\assert-mounted.js
```

Expected: PASS，输出 `path=pages/home/index`，WXML 非空且 `.student-home-page` 存在。

- [ ] **Step 10: 提交代码修复**

```powershell
git -C 'C:\Users\33141\Documents\ui21a-runtime-identity' add apps/miniapp/src/app.tsx apps/miniapp/src/components/runtime-session-gate.tsx apps/miniapp/src/pages/home/index.tsx apps/miniapp/src/pages/profile/index.tsx apps/miniapp/src/pages/merchant-home/index.tsx apps/miniapp/src/pages/delivery-rider/index.tsx
git -C 'C:\Users\33141\Documents\ui21a-runtime-identity' commit -m "fix(UI-21a): 修复Taro冷启动页面挂载"
```

- [ ] **Step 11: 让验证 clone 回到刚提交的精确版本并复跑断言**

```powershell
git -C 'D:\codex-temp\ui21a-live' fetch origin feat/ui21a-runtime-identity
git -C 'D:\codex-temp\ui21a-live' reset --hard origin/feat/ui21a-runtime-identity
Set-Location 'D:\codex-temp\ui21a-live'
npm --workspace apps/miniapp run typecheck
npm run build:miniapp
Set-Location 'D:\codex-temp\ui21a-automator'
node .\assert-mounted.js
```

Expected: clone clean且 HEAD 等于实现提交；typecheck/build/挂载断言全部 PASS。

### Task 3: 完成五项真实运行安全验收

**Files:**
- Read: `backend/.env`
- Read: `D:/codex-temp/ui21a-runtime/backend.stdout.log`
- Create temporary evidence: `D:/codex-temp/ui21a-runtime/ui21a-*.json`
- Create temporary screenshots: `D:/codex-temp/ui21a-runtime/ui21a-*.png`

所有场景写入同一个 `try/finally` 验收脚本：进入场景前读取原 `current_identity`、`domainA.token` 和当前用户的 `takeout_riders` 行；`finally` 必须恢复原身份、原 token、原骑手行和后端进程。脚本异常退出也要执行 scoped cleanup，禁止留下 active 骑手资格或错误身份。

- [ ] **Step 1: 验证无 token 冷启动只有一次登录**

清理 `domainA.token`，把 `backend.stdout.log` 当前字节长度写入 `ui21a-login-log-offset.json`，关闭后重新执行官方 `auto`，等待首页出现，并把断言结果写入 `ui21a-cold-start.json`、截图写入 `ui21a-cold-start.png`。断言：

```text
页面根 WXML 非空
最终路由 pages/home/index
新增日志中 POST /api/v1/auth/wechat-login 恰好 1 次
storage 中 domainA.token 非空
```

- [ ] **Step 2: 验证 merchant 冷启动恢复**

从运行中的官方 automator 读取当前 JWT，再调用真实身份 API：

```powershell
Set-Location 'D:\codex-temp\ui21a-automator'
$token = node -e "const a=require('miniprogram-automator');(async()=>{const m=await a.connect({wsEndpoint:'ws://127.0.0.1:9420'});process.stdout.write(String(await m.callWxMethod('getStorageSync','domainA.token')));m.disconnect()})().catch(e=>{console.error(e);process.exit(1)})"
$headers = @{ Authorization = "Bearer $token" }
$body = @{ identity = 'merchant' } | ConvertTo-Json -Compress
Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:8080/api/v1/auth/identity' -Headers $headers -ContentType 'application/json' -Body $body
```

关闭并重开官方 automator。把最终路由、selector 结果与截图写入 `ui21a-merchant-restore.json/png`；断言最终路由为 `pages/merchant-home/index`，`.merchant-home-page` 存在，学生首页主体不出现。

- [ ] **Step 3: 验证 rider 冷启动恢复**

先保存当前用户原骑手行；若不存在，用同一真实 MySQL 的 scoped fixture 插入 `status=active,row_version=1`，若存在则临时更新为 active 并保存完整原值。再调用身份切换 API：

```json
{"identity":"rider"}
```

关闭并重开 automator。把结果写入 `ui21a-rider-restore.json/png`；断言最终路由为 `pages/delivery-rider/index`，`.rider-grab-page` 存在，商家和学生主体不出现。`finally` 恢复验收前身份与骑手行，而不是固定写成 student。

- [ ] **Step 4: 验证快速重复身份切换只写一次**

在 Profile 身份弹窗确认按钮上连续触发两次点击；点击前记录日志字节偏移，点击后只统计该偏移之后的 `POST /api/v1/auth/identity`。把请求次数、按钮状态和截图写入 `ui21a-switch-dedup.json/png`；断言同一轮请求数为 1，Promise 未完成前按钮保持 disabled/loading。

- [ ] **Step 5: 验证网络/5xx 保留 token 并可重试**

保留有效 token 并记录值，停止 Residue 中记录的本次验收后端 PID，使 `/auth/me` 产生确定的网络错误；冷启动后断言显示 `.runtime-session-state__retry`，并用 `getStorageSync('domainA.token')` 证明 token 与停止前完全相同。按原命令重新启动后端并确认 `/healthz`，点击重试，断言进入正常页面且新增日志中没有 `/auth/wechat-login`。结果写入 `ui21a-network-retry.json/png`；`finally` 保证后端恢复。

- [ ] **Step 6: 验证非法深链不泄露受限内容**

把当前身份切回 `student` 后分别执行：

```js
await miniProgram.reLaunch('/pages/merchant-home/index')
await miniProgram.reLaunch('/pages/delivery-rider/index')
```

在每次 `reLaunch` 后立即以 50ms 间隔连续采样 20 次：`.merchant-home-hero`、`.merchant-home-orders`、`.rider-grab-hero`、`.rider-order-card` 均不得出现；最终路由必须回到 `pages/profile/index`。采样数组与截图写入 `ui21a-deep-link-fail-closed.json/png`。

### Task 4: 回归、证据与报告

**Files:**
- Modify: `docs/UI-21A-RUNTIME-IDENTITY-DEVELOPMENT.md`
- Modify: `docs/UI-21A-RUNTIME-IDENTITY-ACCEPTANCE.md`

- [ ] **Step 1: 运行完整回归**

Run:

```powershell
Set-Location 'C:\Users\33141\Documents\ui21a-runtime-identity'
npm run check:miniapp-static-routes
npm run check:boundaries
npm run typecheck
npm run verify
npm run build
$autoMigrateCount = @(rg "db\.AutoMigrate\(" backend/cmd/server 2>$null).Count
if ($autoMigrateCount -ne 0) { throw "AutoMigrate residue count=$autoMigrateCount" }
git diff master...HEAD -- backend/internal/module/takeout backend/internal/module/media backend/migrations apps/admin
```

Expected: 所有 npm 命令 PASS；AutoMigrate 命中 0；列出的后端旧外卖/media/迁移/admin 范围无本补丁 diff。

- [ ] **Step 2: 更新 Development 报告**

写清：真实白屏根因、App 永久挂载契约、Runtime Gate、外层/内层组件拆分、Profile 副作用边界，以及没有修改后端/API/冻结视觉。

- [ ] **Step 3: 更新 Acceptance 报告**

用实际命令、HTTP/Network 次数、路由、selector、storage 和截图路径替换五项 CONDITIONAL。任何一项未通过必须继续标记 CONDITIONAL，不得写 PASS。

- [ ] **Step 4: 提交 Development 与 Acceptance 证据**

```powershell
git -C 'C:\Users\33141\Documents\ui21a-runtime-identity' add docs/UI-21A-RUNTIME-IDENTITY-DEVELOPMENT.md docs/UI-21A-RUNTIME-IDENTITY-ACCEPTANCE.md
git -C 'C:\Users\33141\Documents\ui21a-runtime-identity' commit -m "docs(UI-21a): 关闭运行时身份条件验收"
```

### Task 5: 清理并交付 owner 验收

**Files:**
- Remove temporary only: `D:/codex-temp/ui21a-automator/assert-mounted.js`
- Remove temporary only after evidence is captured: `D:/codex-temp/ui21a-live`
- Modify: `docs/UI-21A-RUNTIME-IDENTITY-RESIDUE.md`
- Inspect: both Git worktrees

- [ ] **Step 1: 关闭本次启动的验收进程**

只关闭记录在 Residue 报告中的 DevTools automation、后端和 Meilisearch PID；不得误杀用户其它程序。

- [ ] **Step 2: 清理临时验收文件**

删除临时 automator 断言与验证 clone；保留报告中引用的必要截图/JSON到明确证据目录，或把关键文本摘录入报告后再删除。

- [ ] **Step 3: 更新并提交最终 Residue 报告**

在进程和临时目录完成清理后，记录实际关闭的 PID、已删除路径、保留证据路径、数据库 scoped fixture 回滚结果和端口核查结果，然后提交：

```powershell
git -C 'C:\Users\33141\Documents\ui21a-runtime-identity' add docs/UI-21A-RUNTIME-IDENTITY-RESIDUE.md
git -C 'C:\Users\33141\Documents\ui21a-runtime-identity' commit -m "docs(UI-21a): 记录最终验收残留清理"
```

- [ ] **Step 4: 核对最终 Git 状态**

Run:

```powershell
git -C 'C:\Users\33141\Documents\ui21a-runtime-identity' status
git -C 'C:\Users\33141\Documents\New project' status
git -C 'C:\Users\33141\Documents\New project' merge-base --is-ancestor master feat/ui21a-runtime-identity
git -C 'C:\Users\33141\Documents\New project' diff --stat master...feat/ui21a-runtime-identity
```

Expected: 两个工作区 clean；分支仍可 fast-forward；`feat/p0-identity-switch` 未改、未删；未执行 merge 或 tag。

- [ ] **Step 5: 停止等待 owner 串行验收**

报告最终 commit hash、五项真实验收结果、Boundary、Residue 和 `FF-OK`。由 owner 在主仓库目录执行 `--ff-only` 与 baseline tag；本计划不得自行进入后续模块。
