# 帖子发布与学生发布入口 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复帖子发布请求 ID 和幂等头导致的 400 错误，并让学生发布入口只保留帖子与二手。

**Architecture:** 社区客户端继续使用现有 `createCommunityPost` 边界，但请求 ID 统一委托给共享 Crockford ULID 生成器，并在请求体与 `Idempotency-Key` 请求头中发送同一值。发布页用 `useRef` 保留一次逻辑提交的 ID，只在成功、4xx 契约错误或用户修改有效载荷后失效。学生发布选择页直接删除配送入口，不影响商家端合法配送页面。

**Tech Stack:** Taro 4、React 18、TypeScript、Node.js 静态契约检查、Go 社区接口现有幂等测试

---

## 文件结构

- Create: `scripts/check-miniapp-publish-contract.cjs` — 对社区请求 ID、幂等头、发布页重试状态和学生入口做静态契约检查。
- Modify: `package.json` — 注册 `check:miniapp-publish-contract` 并纳入 `verify`。
- Modify: `apps/miniapp/src/services/b-community.ts` — 统一 ULID 生成并发送幂等头。
- Modify: `apps/miniapp/src/pages/publish-post/index.tsx` — 复用重试 ID、细分错误反馈并在载荷变化时重置。
- Modify: `apps/miniapp/src/pages/publish-entry/index.tsx` — 删除学生配送发布入口与相关文案、联合类型。
- Verify only: `backend/internal/module/community/keyed_create_test.go` — 复用现有后端 ULID／幂等契约测试，不修改其已冻结行为。

### Task 1: 建立失败的发布契约检查

**Files:**
- Create: `scripts/check-miniapp-publish-contract.cjs`
- Modify: `package.json`

- [ ] **Step 1: 创建静态契约检查脚本**

```js
const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const read = (...parts) => fs.readFileSync(path.join(root, ...parts), 'utf8')

const community = read('apps', 'miniapp', 'src', 'services', 'b-community.ts')
const publishPost = read('apps', 'miniapp', 'src', 'pages', 'publish-post', 'index.tsx')
const publishEntry = read('apps', 'miniapp', 'src', 'pages', 'publish-entry', 'index.tsx')

if (!community.includes("import { createClientRequestId } from './client-request-id'")) {
  throw new Error('community service must use the shared Crockford ULID generator')
}
if (/community-\$\{Date\.now\(\)\}/.test(community)) {
  throw new Error('legacy non-ULID community request id is forbidden')
}
if (!/const clientRequestId = input\.clientRequestId \?\? createCommunityClientRequestId\(\)/.test(community)) {
  throw new Error('community create must resolve one request id before building the request')
}
if (!community.includes("header: { 'Idempotency-Key': clientRequestId }")) {
  throw new Error('community create must send the resolved id as Idempotency-Key')
}
if (!publishPost.includes('useRef') || !publishPost.includes('clientRequestIdRef')) {
  throw new Error('publish page must preserve a logical request id across retryable failures')
}
if (!publishPost.includes('isApiError(error)') || !publishPost.includes('error.status >= 400 && error.status < 500')) {
  throw new Error('publish page must discard a request id after a 4xx contract failure')
}
for (const forbidden of ["key: 'delivery'", '发布配送单', '/pages/merchant-publish/index']) {
  if (publishEntry.includes(forbidden)) {
    throw new Error(`student publish entry contains forbidden delivery target: ${forbidden}`)
  }
}
for (const required of ["key: 'post'", "key: 'secondhand'", '/pages/publish-post/index', '/pages/publish-secondhand/index']) {
  if (!publishEntry.includes(required)) {
    throw new Error(`student publish entry is missing: ${required}`)
  }
}

console.log('miniapp publish contract check PASS')
```

- [ ] **Step 2: 在根脚本中注册检查**

将 `package.json` 的 scripts 增加：

```json
"check:miniapp-publish-contract": "node scripts/check-miniapp-publish-contract.cjs"
```

并把 `verify` 改为：

```json
"verify": "npm run check:boundaries && npm run check:miniapp-publish-contract && npm run typecheck && npm run test:backend"
```

- [ ] **Step 3: 运行检查并确认当前实现失败**

Run: `npm run check:miniapp-publish-contract`

Expected: FAIL，首个错误为社区服务未使用共享 ULID 生成器。

- [ ] **Step 4: 提交契约检查**

```powershell
git add -- package.json scripts/check-miniapp-publish-contract.cjs
git commit -m "测试：约束帖子发布与学生入口"
```

### Task 2: 修复社区请求 ID 与幂等头

**Files:**
- Modify: `apps/miniapp/src/services/b-community.ts`

- [ ] **Step 1: 导入共享请求 ID 生成器**

在现有导入区加入：

```ts
import { createClientRequestId } from './client-request-id'
```

- [ ] **Step 2: 替换旧社区 ID 生成函数**

```ts
export function createCommunityClientRequestId(): string {
  return createClientRequestId()
}
```

- [ ] **Step 3: 一次解析请求 ID 并同时写入 body/header**

将 `createCommunityPost` 替换为：

```ts
export async function createCommunityPost(input: CreateCommunityPostInput): Promise<CommunityPost> {
  const clientRequestId = input.clientRequestId ?? createCommunityClientRequestId()
  const data = await request<unknown>('/community/posts', {
    url: '',
    method: 'POST',
    data: {
      title: input.title,
      content: input.content,
      topic: input.topic || '湛科日常',
      imageAssetIds: input.imageAssetIds ?? [],
      clientRequestId,
    },
    header: { 'Idempotency-Key': clientRequestId },
  })
  return unwrapPost(data)
}
```

- [ ] **Step 4: 运行类型检查与后端幂等测试**

Run: `npm --prefix apps/miniapp run typecheck`

Expected: PASS。

Run: `Push-Location backend; go test ./internal/module/community -run 'Test.*Keyed|TestResolveCommunityIdempotencyKey' -count=1; Pop-Location`

Expected: PASS；合法 ULID 创建、重放和 header/body 冲突测试保持通过。

- [ ] **Step 5: 提交服务修复**

```powershell
git add -- apps/miniapp/src/services/b-community.ts
git commit -m "修复：统一帖子发布幂等请求标识"
```

### Task 3: 保留可重试发布 ID 并提供准确错误反馈

**Files:**
- Modify: `apps/miniapp/src/pages/publish-post/index.tsx`

- [ ] **Step 1: 增加 `useRef` 与 API 错误识别导入**

```ts
import { useMemo, useRef, useState } from 'react'
import { isApiError } from '../../services/api'
```

- [ ] **Step 2: 增加一次逻辑提交的引用与重置函数**

放在页面状态声明附近：

```ts
const clientRequestIdRef = useRef<string | null>(null)

const resetClientRequestId = () => {
  clientRequestIdRef.current = null
}
```

- [ ] **Step 3: 在用户改变有效载荷时重置旧 ID**

内容输入改为：

```tsx
onInput={(event) => {
  resetClientRequestId()
  setContent(event.detail.value)
}}
```

`toggleTopic` 的开头调用：

```ts
resetClientRequestId()
```

成功加入新图片前调用：

```ts
resetClientRequestId()
setImages((current) => [...current, ...uploaded].slice(0, MAX_IMAGES))
```

- [ ] **Step 4: 在发布时复用或生成 ID**

在 `setSubmitting(true)` 之前加入：

```ts
const clientRequestId = clientRequestIdRef.current ?? createCommunityClientRequestId()
clientRequestIdRef.current = clientRequestId
```

请求体改为：

```ts
clientRequestId,
```

成功后、跳转前加入：

```ts
clientRequestIdRef.current = null
```

- [ ] **Step 5: 只在 4xx 时丢弃 ID，并区分提示**

将 catch 中的通用提示替换为：

```ts
} catch (error) {
  console.error('publish community post failed', error)
  if (isApiError(error) && error.status >= 400 && error.status < 500) {
    clientRequestIdRef.current = null
  }
  const title = !isApiError(error)
    ? '发布失败，请稍后再试'
    : error.status === 0
      ? '网络异常，内容已保留'
      : error.status === 409
        ? '发布内容已变化，请重新提交'
        : error.status >= 500
          ? '服务暂不可用，稍后可重试'
          : error.message || '发布内容不符合要求'
  toast(title, 'none')
}
```

- [ ] **Step 6: 运行类型检查**

Run: `npm --prefix apps/miniapp run typecheck`

Expected: PASS；不出现 hook、Taro 事件或 `ApiError` 类型错误。

- [ ] **Step 7: 提交发布页重试行为**

```powershell
git add -- apps/miniapp/src/pages/publish-post/index.tsx
git commit -m "修复：保留帖子发布失败后的重试状态"
```

### Task 4: 删除学生端配送发布入口

**Files:**
- Modify: `apps/miniapp/src/pages/publish-entry/index.tsx`

- [ ] **Step 1: 收紧发布入口类型**

```ts
type PublishEntry = {
  key: 'post' | 'secondhand'
  icon: string
  title: string
  subtitle: string
  tag: string
  tone: 'blue' | 'orange'
  url: string
}
```

- [ ] **Step 2: 删除 `delivery` 数据项**

`entries` 只保留以下两个对象：

```ts
const entries: PublishEntry[] = [
  {
    key: 'post',
    icon: '帖',
    title: '发帖子',
    subtitle: '分享校园动态、求助、攻略和日常',
    tag: '学生',
    tone: 'blue',
    url: '/pages/publish-post/index'
  },
  {
    key: 'secondhand',
    icon: '二',
    title: '发布二手',
    subtitle: '转让闲置、书籍资料、数码和生活用品',
    tag: '二手',
    tone: 'orange',
    url: '/pages/publish-secondhand/index'
  }
]
```

- [ ] **Step 3: 修正文案，不再暗示学生可发布配送单**

```tsx
<Text className='publish-entry-hero__desc'>帖子与二手内容从这里进入，选择后填写对应信息。</Text>
```

```tsx
<Text>选择内容类型后进入对应发布表单。</Text>
```

- [ ] **Step 4: 运行发布契约检查**

Run: `npm run check:miniapp-publish-contract`

Expected: PASS，输出 `miniapp publish contract check PASS`。

- [ ] **Step 5: 确认商家合法入口未被删除**

Run: `rg -n "发布配送单" apps/miniapp/src/pages/merchant-home apps/miniapp/src/pages/merchant-publish`

Expected: 商家首页和商家发布页仍有合法配送文案；学生 `publish-entry` 无匹配。

- [ ] **Step 6: 提交学生入口修复**

```powershell
git add -- apps/miniapp/src/pages/publish-entry/index.tsx
git commit -m "修复：移除学生端配送发布入口"
```

### Task 5: 完成阶段验证

**Files:**
- Verify: `scripts/check-miniapp-publish-contract.cjs`
- Verify: `apps/miniapp/src/services/b-community.ts`
- Verify: `apps/miniapp/src/pages/publish-post/index.tsx`
- Verify: `apps/miniapp/src/pages/publish-entry/index.tsx`

- [ ] **Step 1: 运行静态路由与发布契约检查**

Run: `npm run check:miniapp-static-routes`

Expected: PASS。

Run: `npm run check:miniapp-publish-contract`

Expected: PASS。

- [ ] **Step 2: 运行前端类型检查与构建**

Run: `npm --prefix apps/miniapp run typecheck`

Expected: PASS。

Run: `npm run build:miniapp`

Expected: PASS，生成微信小程序产物且没有 TypeScript/Taro 构建错误。

- [ ] **Step 3: 运行社区后端测试**

Run: `Push-Location backend; go test ./internal/module/community -count=1; Pop-Location`

Expected: PASS。

- [ ] **Step 4: 运行仓库验证并检查差异**

Run: `npm run verify`

Expected: PASS；若独立 MySQL、微信真机或外部服务不可用，只把对应运行时项目标记为 `PENDING`，不把它误报为源代码失败。

Run: `git diff --check`

Expected: 无输出，退出码 0。

- [ ] **Step 5: 记录真机待验项**

在阶段交接中明确列出：带图帖子真实发布、弱网重试不重复创建、学生“＋”仅显示两个入口。未在真机执行前状态为 `PENDING`。
