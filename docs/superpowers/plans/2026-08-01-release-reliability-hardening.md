# 湛科生活圈发布可靠性加固 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复已审计确认的弱网、分页、幂等、审核一致性、身份权限与发布可追踪性问题，产出可由用户在微信开发者工具中上传的本地发布候选版本。

**Architecture:** 保留现有 Taro/React、Gin/GORM 和 React Admin 边界，优先复用既有商品留言幂等、身份守卫、分页 `Page<T>` 与管理弹窗模式。客户端按请求生命周期加固，服务端把关联状态写入收口到事务，发布层增加 readiness、版本标识和严格配置验证。

**Tech Stack:** Taro 4、React 18、TypeScript、Gin、GORM、MySQL、Vite 8、Docker、Node.js。

---

### Task 1: 小程序登录和请求可靠性

**Files:**
- Modify: `apps/miniapp/src/services/api.ts`
- Modify: `apps/miniapp/src/services/a-user.ts`
- Modify: `apps/miniapp/src/stores/session.ts`
- Test: `scripts/check-miniapp-regression-contract.cjs`

- [ ] **Step 1: 扩充失败契约检查**

在 `check-miniapp-regression-contract.cjs` 断言登录与当前用户请求显式使用 `12000` 毫秒超时，并断言会话初始化存在一次受控网络重试而非无限重试。

- [ ] **Step 2: 运行检查并确认失败**

Run: `node scripts/check-miniapp-regression-contract.cjs`

Expected: FAIL，指出登录请求仍继承 3000 毫秒默认值。

- [ ] **Step 3: 实现可分类网络重试**

在 `api.ts` 导出只识别超时和网络错误的判断函数；`wechatLogin` 与 `currentUser` 显式传入 `timeout: 12000`。在 `session.ts` 用固定一次重试和 250–450 毫秒抖动包裹登录与用户初始化，保持现有 epoch 校验：

```ts
async function onceAfterNetworkFailure<T>(action: () => Promise<T>): Promise<T> {
  try {
    return await action()
  } catch (error) {
    if (!isRetryableNetworkError(error)) throw error
    await new Promise(resolve => setTimeout(resolve, 250 + Math.floor(Math.random() * 200)))
    return action()
  }
}
```

- [ ] **Step 4: 运行定向验证**

Run: `npm run check:miniapp-regression-fixes && npm --workspace apps/miniapp run typecheck`

Expected: PASS。

- [ ] **Step 5: 提交**

```powershell
git add -- apps/miniapp/src/services/api.ts apps/miniapp/src/services/a-user.ts apps/miniapp/src/stores/session.ts scripts/check-miniapp-regression-contract.cjs
git commit -m "修复：增强弱网登录与会话初始化"
```

### Task 2: 帖子评论幂等和详情分页

**Files:**
- Modify: `apps/miniapp/src/services/b-community.ts`
- Modify: `apps/miniapp/src/pages/post-detail/index.tsx`
- Modify: `apps/miniapp/src/pages/secondhand-detail/product-comments.tsx`
- Modify: `apps/miniapp/src/pages/post-detail/index.scss`
- Modify: `apps/miniapp/src/pages/secondhand-detail/index.scss`
- Test: `scripts/check-miniapp-regression-contract.cjs`
- Test: `scripts/check-secondhand-comments-contract.cjs`

- [ ] **Step 1: 写入幂等和分页失败检查**

断言帖子评论服务同时发送 `clientRequestId` 与 `Idempotency-Key`，并断言两个详情页具有 `page`、`total`、`hasMore` 与 `onScrollToLower`。

- [ ] **Step 2: 运行检查并确认失败**

Run: `npm run check:miniapp-regression-fixes && npm run check:secondhand-comments-contract`

Expected: FAIL，分别指出帖子评论缺少幂等键、详情仅加载第一页。

- [ ] **Step 3: 为帖子评论补齐幂等参数**

将服务签名扩展为：

```ts
export async function createCommunityComment(
  postId: string,
  content: string,
  clientRequestId = createCommunityClientRequestId(),
): Promise<CommunityComment> {
  return request(`/community/posts/${postId}/comments`, {
    url: '', method: 'POST', data: { content, clientRequestId },
    header: { 'Idempotency-Key': clientRequestId },
  })
}
```

页面用 `pendingSubmission` 复用同一请求 ID；成功或明确 4xx 后清除，网络失败保留。

- [ ] **Step 4: 为帖子和商品留言实现真实分页**

首次加载重置列表；后续加载使用 `page + 1`，按 `commentId` 去重，并通过 `items.length >= total` 计算结束状态。后续页失败保留已有评论并显示“加载失败，点击重试”。

- [ ] **Step 5: 运行定向验证**

Run: `npm run check:miniapp-regression-fixes && npm run check:secondhand-comments-contract && npm --workspace apps/miniapp run typecheck`

Expected: PASS。

- [ ] **Step 6: 提交**

```powershell
git add -- apps/miniapp/src/services/b-community.ts apps/miniapp/src/pages/post-detail/index.tsx apps/miniapp/src/pages/post-detail/index.scss apps/miniapp/src/pages/secondhand-detail/product-comments.tsx apps/miniapp/src/pages/secondhand-detail/index.scss scripts/check-miniapp-regression-contract.cjs scripts/check-secondhand-comments-contract.cjs
git commit -m "修复：补齐评论幂等与详情分页"
```

### Task 3: 搜索和角色工作台分页刷新

**Files:**
- Modify: `apps/miniapp/src/pages/search/index.tsx`
- Modify: `apps/miniapp/src/pages/search/index.scss`
- Modify: `apps/miniapp/src/pages/merchant-home/index.tsx`
- Modify: `apps/miniapp/src/pages/delivery-rider/index.tsx`
- Modify: `apps/miniapp/src/pages/my-content/index.tsx`
- Modify: `apps/miniapp/src/pages/user-profile/index.tsx`
- Test: `scripts/check-miniapp-domain-search.cjs`
- Test: `scripts/check-role-workbench-ui.cjs`

- [ ] **Step 1: 扩展搜索与角色契约检查**

搜索必须读取服务端 `total`、向服务传入 `page`、使用滚动加载；商家工作台必须使用 `useDidShow`；角色与个人列表不得用固定首屏条数伪装完整总数。

- [ ] **Step 2: 运行检查并确认失败**

Run: `npm run check:miniapp-domain-search && npm run check:role-workbench-ui`

Expected: FAIL，指出搜索仅保留 `items`、商家页面仅在 `useEffect` 加载。

- [ ] **Step 3: 实现统一分页状态**

各页采用下列状态模型，追加时按 `id` 去重：

```ts
const [page, setPage] = useState(1)
const [total, setTotal] = useState(0)
const [loadingMore, setLoadingMore] = useState(false)
const hasMore = items.length < total
```

搜索标题显示 `找到 ${total} 条结果`，而不是 `results.length`。

- [ ] **Step 4: 修复商家返回刷新**

将商家首页加载函数改为 `useCallback`，在 `useDidShow` 触发；用递增请求代次忽略过期响应。保留正常态刷新入口并避免并发覆盖。

- [ ] **Step 5: 运行定向验证**

Run: `npm run check:miniapp-domain-search && npm run check:role-workbench-ui && npm --workspace apps/miniapp run typecheck`

Expected: PASS。

- [ ] **Step 6: 提交**

```powershell
git add -- apps/miniapp/src/pages/search apps/miniapp/src/pages/merchant-home/index.tsx apps/miniapp/src/pages/delivery-rider/index.tsx apps/miniapp/src/pages/my-content/index.tsx apps/miniapp/src/pages/user-profile/index.tsx scripts/check-miniapp-domain-search.cjs scripts/check-role-workbench-ui.cjs
git commit -m "修复：补齐列表分页与工作台返回刷新"
```

### Task 4: 多图发布审核等待收敛

**Files:**
- Modify: `apps/miniapp/src/pages/publish-post/index.tsx`
- Modify: `apps/miniapp/src/pages/publish-post/index.scss`
- Modify: `apps/miniapp/src/services/media.ts`
- Test: `scripts/check-miniapp-publish-contract.cjs`

- [ ] **Step 1: 增加发布等待契约检查**

断言发布页不再逐张 `await` 审核，存在最大并发数、整次提交总超时与“上传中/审核中/发布中”状态。

- [ ] **Step 2: 运行检查并确认失败**

Run: `npm run check:miniapp-publish-contract`

Expected: FAIL，指出多图仍串行等待。

- [ ] **Step 3: 实现受限并发**

最多同时审核 3 张图片；单张继续使用服务超时，整次提交最多等待 120 秒。任一图片不通过或总超时均保留草稿和资源，不创建帖子。

- [ ] **Step 4: 运行定向验证并提交**

Run: `npm run check:miniapp-publish-contract && npm --workspace apps/miniapp run typecheck`

```powershell
git add -- apps/miniapp/src/pages/publish-post/index.tsx apps/miniapp/src/pages/publish-post/index.scss apps/miniapp/src/services/media.ts scripts/check-miniapp-publish-contract.cjs
git commit -m "修复：收敛多图发布审核等待"
```

### Task 5: 骑手身份守卫和严格审核请求

**Files:**
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/cmd/server/delivery_http_mysql_test.go`
- Modify: `backend/internal/module/admin/handler.go`
- Modify: `backend/internal/module/admin/review/service.go`
- Modify: `backend/internal/module/admin/review/service_test.go`
- Test: `backend/internal/module/admin/handler_test.go`

- [ ] **Step 1: 写骑手越权失败测试**

在已有配送 HTTP 测试中创建“有激活骑手记录但当前身份不是 rider”的用户，访问 `/delivery/rider/orders/claimable` 预期 403。

- [ ] **Step 2: 写严格审核请求失败测试**

覆盖畸形 JSON、未知字段、尾随 JSON、空白拒绝原因和超长原因，预期 400；通过请求只接受空原因。

- [ ] **Step 3: 运行测试并确认失败**

Run: `cd backend; go test ./cmd/server ./internal/module/admin/...`

Expected: FAIL，当前骑手路由只要求登录且审核解析错误被吞掉。

- [ ] **Step 4: 加入身份守卫与严格解码**

路由改为：

```go
riderRoutes := api.Group(
    "/delivery/rider",
    userService.AuthRequired(),
    userService.IdentityRequired(user.IdentityRider),
)
```

审核 handler 使用 `json.Decoder.DisallowUnknownFields()` 并验证单个 JSON 值；`Decide` 对 reason 执行 `strings.TrimSpace`、拒绝为空和最大长度限制。

- [ ] **Step 5: 运行测试并提交**

Run: `cd backend; go test ./cmd/server ./internal/module/admin/...`

```powershell
git add -- backend/cmd/server/main.go backend/cmd/server/delivery_http_mysql_test.go backend/internal/module/admin/handler.go backend/internal/module/admin/handler_test.go backend/internal/module/admin/review/service.go backend/internal/module/admin/review/service_test.go
git commit -m "修复：收紧骑手身份与审核请求校验"
```

### Task 6: 审核和举报事务一致性

**Files:**
- Modify: `backend/internal/module/admin/review/repository.go`
- Modify: `backend/internal/module/admin/review/service.go`
- Modify: `backend/internal/module/admin/review/service_test.go`
- Modify: `backend/internal/module/admin/repository.go`
- Modify: `backend/internal/module/admin/service.go`
- Modify: `backend/internal/module/admin/service_test.go`

- [ ] **Step 1: 写事务回滚与并发测试**

覆盖目标状态更新成功但任务保存失败必须回滚、同一任务并发通过/拒绝只能有一个最终决定、举报建单失败不留下孤立举报、相同幂等键只创建一条举报。

- [ ] **Step 2: 运行测试并确认失败**

Run: `cd backend; go test ./internal/module/admin/...`

Expected: FAIL，当前关联写入分两次提交。

- [ ] **Step 3: 实现事务执行器**

Repository 暴露 `WithinTransaction`，事务内 Repository 继续使用同一 `*gorm.DB`；审核任务读取使用 `clause.Locking{Strength: "UPDATE"}`。目标投影和任务保存必须共享事务。

- [ ] **Step 4: 为举报增加幂等键**

服务接收裁剪后的 `clientRequestId`，事务内查重或依靠唯一索引；创建举报与 `EnsureSubmitted` 同时成功或同时失败。

- [ ] **Step 5: 运行测试并提交**

Run: `cd backend; go test ./internal/module/admin/...`

```powershell
git add -- backend/internal/module/admin
git commit -m "修复：保证审核与举报写入原子一致"
```

### Task 7: 管理后台审核分页

**Files:**
- Modify: `apps/admin/src/services/f-admin.ts`
- Modify: `apps/admin/src/App.tsx`
- Modify: `apps/admin/src/styles.css`
- Modify: `scripts/check-admin-ui-system.cjs`

- [ ] **Step 1: 写第 21 条可达契约检查**

断言 `reviews` 接受 `status/page/pageSize`，审核页保存 `Page<ReviewTask>`、展示总数和上一页/下一页，刷新当前页而非固定第一页。

- [ ] **Step 2: 运行检查并确认失败**

Run: `npm run check:admin-ui-system`

Expected: FAIL，当前 `adminAPI.reviews()` 无分页参数。

- [ ] **Step 3: 实现分页和筛选**

默认筛选待审核，支持待审核、审核中、已通过、已拒绝和全部；请求使用服务端 `total`。处理成功后保留当前筛选与页码；若当前页已空且页码大于 1，则回到上一页。

- [ ] **Step 4: 运行定向验证并提交**

Run: `npm run check:admin-ui-system && npm --workspace apps/admin run typecheck && npm run build:admin`

```powershell
git add -- apps/admin/src/services/f-admin.ts apps/admin/src/App.tsx apps/admin/src/styles.css scripts/check-admin-ui-system.cjs
git commit -m "修复：补齐审核后台分页与筛选"
```

### Task 8: 生产发布硬化

**Files:**
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/config/config_test.go`
- Modify: `backend/internal/middleware/cors.go`
- Modify: `backend/internal/middleware/cors_test.go`
- Modify: `backend/cmd/server/main.go`
- Test: `backend/cmd/server/main_test.go`
- Modify: `package.json`
- Modify: `apps/admin/Dockerfile`
- Modify: `deploy/docker-compose.yml`
- Create: `.dockerignore`
- Modify: `scripts/check-native-deployment.cjs`

- [ ] **Step 1: 写严格配置、CORS、readiness 与版本测试**

生产环境默认 DSN/默认监听地址、非法布尔值必须失败；CORS 不得同时返回 `*` 与 credentials；`/readyz` 在数据库不可用时返回 503；`/version` 只返回非秘密构建元数据。

- [ ] **Step 2: 运行测试并确认失败**

Run: `cd backend; go test ./internal/config ./internal/middleware ./cmd/server`

Expected: FAIL，当前配置会静默回退且只有 `/healthz`。

- [ ] **Step 3: 实现严格生产配置与探针**

`envBool` 改为返回 `(bool, error)` 或在加载阶段累积错误，只接受 `true/false/1/0`；生产必须显式提供 `HTTP_ADDR`、`MYSQL_DSN`。CORS 从配置读取显式 allowlist，未配置时同源。新增 readiness 和版本结构体，不输出环境变量值。

- [ ] **Step 4: 固化 Node 与 Docker 构建**

根 `engines.node` 改为 `^20.19.0 || >=22.12.0`。Admin 构建上下文改为仓库根，复制根 `package.json`、`package-lock.json` 与 `apps/admin/package.json`，使用 `npm ci --workspace apps/admin`。Compose 文件顶部和服务命名明确标记开发用途，不再把固定密码清单描述为生产部署。

- [ ] **Step 5: 运行定向验证并提交**

Run: `npm run check:native-deployment && cd backend; go test ./internal/config ./internal/middleware ./cmd/server`

```powershell
git add -- backend/internal/config backend/internal/middleware backend/cmd/server/main.go backend/cmd/server/main_test.go package.json apps/admin/Dockerfile deploy/docker-compose.yml .dockerignore scripts/check-native-deployment.cjs
git commit -m "运维：加固生产配置与可追踪发布"
```

### Task 9: 全量回归与本地交付

**Files:**
- Verify only

- [ ] **Step 1: 检查磁盘与工作区边界**

Run: `Get-PSDrive C; git status --short`

Expected: C 盘保留至少 5GB；只出现本计划内改动和用户原有三个未跟踪计划文件。

- [ ] **Step 2: 运行完整验证**

Run: `npm run verify`

Expected: PASS。

- [ ] **Step 3: 运行生产构建**

Run: `$env:TARO_APP_API_BASE='https://zhanduiwang.top/api/v1'; npm run build`

Expected: miniapp 与 admin 构建 PASS。

- [ ] **Step 4: 扫描构建产物**

Run: `rg -l "127\\.0\\.0\\.1:8080|localhost:8080|http://127\\.0\\.0\\.1" apps/miniapp/dist apps/admin/dist`

Expected: 无输出。

- [ ] **Step 5: 检查格式和提交边界**

Run: `git diff --check; git status --short`

Expected: 无空白错误；用户原有未跟踪文件保持不动。

- [ ] **Step 6: 交付说明**

记录本地 commit、构建时间、后端测试边界、真机仍需用户上传后验证的项目，以及微信开发者工具上传步骤；不执行上传、体验版提交或生产数据写入。
