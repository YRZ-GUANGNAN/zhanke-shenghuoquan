# UI-21b-2b-1 Community Writes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不修改后端契约和现有静态视觉结构的前提下，把社区发帖、图片上传、点赞、收藏、关注与评论接入已冻结的真实写接口，并保证重复点击、超时重试和并发返回不会造成状态反转或重复创建。

**Architecture:** 继续沿用 `apps/miniapp/src/services/b-community.ts` 作为社区 HTTP 边界，在页面层维护稳定的 `clientRequestId`、目标状态和按动作隔离的 loading。媒体只通过现有 `services/media.ts` 上传与审核；写入成功后以服务端响应或重新读取 viewer-state 为准，页面不自行伪造最终状态。

**Tech Stack:** Taro 4.2、React 18、TypeScript 5.4、现有 `request`/JWT 信封、现有 Media 上传链路、Go/Gin 已冻结社区 API、真实 MySQL。

---

## 约束与完成定义

- 只改 `apps/miniapp/src/services/b-community.ts`、`apps/miniapp/src/pages/publish-post/**`、`apps/miniapp/src/pages/post-detail/**` 及本模块报告。
- 不改 `backend/**`、迁移、API 路径、响应信封、社区状态机、`package.json` 或依赖锁文件。
- 点赞、收藏、关注只调用目标状态端点，不调用历史 toggle 端点。
- 发帖和评论的 `Idempotency-Key` 请求头必须与 body 的 `clientRequestId` 完全一致；一次用户意图在失败重试时复用同一个值，成功或明确放弃后才生成新值。
- 图片使用 `purpose=post_image`，提交给社区接口的是真实 `assetId`，不是临时路径或空数组。
- 未确认结果时重新读取 viewer-state；不在失败后盲目回滚为本地猜测值。
- 无新增测试框架；以源代码契约断言、TypeScript、构建、边界检查和真实 HTTP/MySQL 验收形成证据链。

## Task 1：建立隔离 worktree 并冻结范围

**Files:**
- Read: `docs/superpowers/specs/2026-07-13-ui21b2b-community-trade-write-interactions-design.md`
- Read: `apps/miniapp/src/services/b-community.ts`
- Read: `apps/miniapp/src/services/media.ts`
- Read: `apps/miniapp/src/pages/publish-post/index.tsx`
- Read: `apps/miniapp/src/pages/post-detail/index.tsx`
- Create later: `docs/UI-21B-2B-1-COMMUNITY-WRITES-REPORT.md`

- [ ] Step 1: 核对主仓库基线与标签

```powershell
git log --oneline -1 master
git tag --list ui21b2a-write-contract-baseline
git status --short
Get-PSDrive C | Select-Object Used,Free
```

Expected: HEAD 含 UI-21b-2b 设计提交，标签存在，状态无输出，C 盘空间足以完成构建。

- [ ] Step 2: 建立实施 worktree

```powershell
git worktree add ..\ui21b2b1-community-writes -b feat/ui21b2b1-community-writes master
git -C ..\ui21b2b1-community-writes status --short
```

Expected: 第二条命令无输出。

- [ ] Step 3: 保存改动前边界证据

```powershell
git -C ..\ui21b2b1-community-writes diff -- backend apps/admin package.json package-lock.json
```

Expected: 无输出。

## Task 2：扩展社区 service 为目标状态与幂等写契约

**Files:**
- Modify: `apps/miniapp/src/services/b-community.ts`
- Reference: `apps/miniapp/src/services/delivery-merchant.ts`
- Reference: `backend/internal/module/community/handler.go`

- [ ] Step 1: 先做旧契约存在性检查

```powershell
rg -n "togglePostLike|togglePostFavorite|toggleFollowUser|createCommunityPost|addPostComment" apps/miniapp/src/services/b-community.ts
rg -n "Idempotency-Key|clientRequestId|viewer-state" apps/miniapp/src/services/b-community.ts
```

Expected: 第一条有命中；第二条缺少完整目标实现，作为改造前红灯证据。

- [ ] Step 2: 增加稳定请求 ID 生成器和写入 DTO

在 service 中增加 26 字符 Crockford 格式请求 ID；定义发帖、评论输入与 viewer-state 类型。核心形状：

```ts
export interface CommunityViewerState {
  liked: boolean
  favorited: boolean
  followingAuthor: boolean
}

export interface CreateCommunityPostInput {
  title: string
  content: string
  topic: string
  imageAssetIds: string[]
  clientRequestId: string
}

export interface AddPostCommentInput {
  content: string
  clientRequestId: string
}
```

请求 ID 实现复用现有 delivery 端的 Crockford 约定，固定返回 26 个字符；不得引入第三方依赖。

- [ ] Step 3: 增加 viewer-state 与目标状态方法

```ts
export const getCommunityViewerState = (postId: number) =>
  request<CommunityViewerState>({
    url: `/api/v1/community/posts/${postId}/viewer-state`,
    method: 'GET',
  })

export const setPostLiked = (postId: number, desired: boolean) =>
  request<{ liked: boolean }>({
    url: `/api/v1/community/posts/${postId}/like`,
    method: desired ? 'PUT' : 'DELETE',
  })
```

按同一模式实现 `setPostFavorited` 和 `setAuthorFollowing`；关注路径使用 `/api/v1/community/users/:id/follow`。

- [ ] Step 4: 把创建类请求改为请求头与 body 双重幂等

```ts
export const createCommunityPost = (input: CreateCommunityPostInput) =>
  request<CommunityPost>({
    url: '/api/v1/community/posts',
    method: 'POST',
    data: input,
    header: { 'Idempotency-Key': input.clientRequestId },
  })
```

`addPostComment(postId, input)` 使用相同 header/body 一致规则并返回真实评论响应。

- [ ] Step 5: 检查 service 目标契约

```powershell
rg -n "method: 'PUT'|method: 'DELETE'|Idempotency-Key|clientRequestId|viewer-state" apps/miniapp/src/services/b-community.ts
npm --prefix apps/miniapp run typecheck
```

Expected: 目标契约均命中；typecheck 可能因调用页仍使用旧签名而失败，失败位置只能在本任务待改页面。

- [ ] Step 6: 提交 service 层

```powershell
git add apps/miniapp/src/services/b-community.ts
git commit -m "feat(UI-21b-2b-1): 接入社区目标状态写契约"
```

## Task 3：接通帖子详情点赞、收藏、关注与 viewer-state

**Files:**
- Modify: `apps/miniapp/src/pages/post-detail/index.tsx`
- Modify if needed: `apps/miniapp/src/pages/post-detail/index.scss`

- [ ] Step 1: 记录页面中的只读占位

```powershell
rg -n "showReadOnlyToast|只读|稍后|下一阶段" apps/miniapp/src/pages/post-detail/index.tsx
```

Expected: 关注、点赞、收藏或评论相关占位有命中。

- [ ] Step 2: 页面加载时并行读取帖子、评论与 viewer-state

仅在登录态请求 viewer-state；未登录时保留初始未激活状态并在写操作前提示登录。viewer-state 请求失败不得阻断公开帖子正文显示。

- [ ] Step 3: 为三类目标状态建立独立意图与 loading

```ts
const [viewerState, setViewerState] = useState<CommunityViewerState | null>(null)
const [pendingAction, setPendingAction] =
  useState<'like' | 'favorite' | 'follow' | null>(null)
const desiredRef = useRef({ like: false, favorite: false, follow: false })
```

每次点击把最新目标写入 `desiredRef`；同一动作进行中禁用重复发送，结束后如目标已变化，再按最新目标发送。服务端返回值覆盖本地值。

- [ ] Step 4: 未确认结果时重新读取 viewer-state

```ts
try {
  const result = await setPostLiked(post.id, desired)
  setViewerState((current) => current && { ...current, liked: result.liked })
} catch (error) {
  await refreshViewerState()
  showApiError(error)
}
```

收藏和关注使用相同收敛策略。计数只在服务端确认状态发生变化后增减，且不得小于 0。

- [ ] Step 5: 删除对应只读 toast，保留未授权交互的原有提示

`showReadOnlyToast` 不再承载点赞、收藏、关注和评论提交；不得顺带接入分享、举报或私信。

- [ ] Step 6: 运行类型与占位检查

```powershell
npm --prefix apps/miniapp run typecheck
rg -n "点赞功能稍后|收藏功能稍后|关注功能稍后" apps/miniapp/src/pages/post-detail/index.tsx
```

Expected: typecheck PASS；第二条无输出。

- [ ] Step 7: 提交详情目标状态

```powershell
git add apps/miniapp/src/pages/post-detail
git commit -m "feat(UI-21b-2b-1): 接通帖子详情目标状态交互"
```

## Task 4：接通评论提交与稳定幂等重试

**Files:**
- Modify: `apps/miniapp/src/pages/post-detail/index.tsx`
- Modify if needed: `apps/miniapp/src/pages/post-detail/index.scss`

- [ ] Step 1: 为当前评论草稿保存稳定请求 ID

```ts
const commentRequestIdRef = useRef(createCommunityClientRequestId())
```

输入变化、网络失败或超时都不更换该值；服务端确认成功后清空输入并生成下一次请求 ID。

- [ ] Step 2: 接入评论提交

校验 trim 后非空；提交期间禁用按钮；调用：

```ts
await addPostComment(postId, {
  content: commentText.trim(),
  clientRequestId: commentRequestIdRef.current,
})
```

- [ ] Step 3: 成功后重新读取评论页和帖子统计

不得只向本地数组追加虚构记录。成功提示只出现一次；重复响应仍以刷新后的服务端列表为准。

- [ ] Step 4: 失败后保留草稿与请求 ID

401 提示重新登录；409、5xx 或网络错误展示明确错误。用户再次点击时复用原 ID。

- [ ] Step 5: 类型检查并提交

```powershell
npm --prefix apps/miniapp run typecheck
git diff --check
git add apps/miniapp/src/pages/post-detail
git commit -m "feat(UI-21b-2b-1): 接通社区评论幂等提交"
```

## Task 5：接通发帖图片上传、审核和创建

**Files:**
- Modify: `apps/miniapp/src/pages/publish-post/index.tsx`
- Modify if needed: `apps/miniapp/src/pages/publish-post/index.scss`
- Reference: `apps/miniapp/src/services/media.ts`

- [ ] Step 1: 记录发帖占位行为

```powershell
rg -n "发布功能稍后开放|图片功能稍后开放" apps/miniapp/src/pages/publish-post/index.tsx
```

Expected: 改造前有命中。

- [ ] Step 2: 用现有媒体方法替换图片占位

调用 `chooseAndUploadImages('post_image', remainingCount, onProgress)`；保存每张图片的 `assetId`、`previewPath`、上传/审核状态和错误信息。最多 9 张。

- [ ] Step 3: 对上传完成的 asset 等待审核结果

使用 `waitForMediaApproval(assetId)`。审核失败图片不得进入 `imageAssetIds`；允许删除失败项和重新选择上传。不得直接把 `previewPath` 提交给后端。

- [ ] Step 4: 为当前发帖草稿保存稳定请求 ID

页面首次进入生成一次；字段编辑、图片重试、网络失败都复用；服务端确认创建成功或用户取消离开后生命周期结束。

- [ ] Step 5: 组装真实发帖请求

```ts
await createCommunityPost({
  title: derivedTitle,
  content: content.trim(),
  topic: selectedTopics[0] ?? '',
  imageAssetIds: uploadedImages.map((item) => item.assetId),
  clientRequestId: requestIdRef.current,
})
```

`derivedTitle` 必须由明确页面规则生成，例如正文首行截断到后端允许长度；不能提交未定义值。若真实后端允许空标题，直接传空字符串并在报告中记录契约依据。

- [ ] Step 6: 处理一次性成功和失败保留

成功后只 toast 一次并 `navigateBack`；提交按钮在进行中禁用。网络错误保留正文、话题、已上传 asset 和请求 ID。

- [ ] Step 7: 清除已接通占位并验证

```powershell
rg -n "发布功能稍后开放|图片功能稍后开放|imageAssetIds:\s*\[\]" apps/miniapp/src/pages/publish-post apps/miniapp/src/services/b-community.ts
npm --prefix apps/miniapp run typecheck
npm run check:miniapp-static-routes
```

Expected: 第一条无输出；其余 PASS。

- [ ] Step 8: 提交发帖闭环

```powershell
git add apps/miniapp/src/pages/publish-post
git commit -m "feat(UI-21b-2b-1): 接通社区图片发帖闭环"
```

## Task 6：真实环境验收

**Files:**
- Create: `docs/UI-21B-2B-1-COMMUNITY-WRITES-REPORT.md`

- [ ] Step 1: 启动真实依赖与后端

使用仓库既有本地环境，连接真实 MySQL、真实 Gin handler、真实 JWT；媒体走已配置 COS 与审核链路。不得使用 mock HTTP server 替代后端。

- [ ] Step 2: 验证目标状态写入

在微信开发者工具中依次执行点赞→取消点赞、收藏→取消收藏、关注→取消关注；Network 中确认只出现 `PUT/DELETE` 目标状态请求。刷新页面后状态与计数保持一致。

- [ ] Step 3: 验证快速重复点击和异常收敛

快速点击同一按钮，最终状态等于最后一次用户意图；制造一次断网或超时后恢复，页面通过 viewer-state 收敛，不出现永久反转。

- [ ] Step 4: 验证评论幂等

提交一条评论，复用同一 `clientRequestId` 重放请求；数据库仅一条评论，页面成功反馈一次，刷新后评论和统计一致。

- [ ] Step 5: 验证图片发帖闭环

选择真实图片→上传 COS→审核通过→创建帖子；请求 body 中 `imageAssetIds` 为真实 asset public id，`Idempotency-Key` 与 `clientRequestId` 相同。重复重放不产生第二帖，帖子详情可显示图片。

- [ ] Step 6: 验证失败保留

在提交前断网，确认正文、话题、图片和请求 ID 未丢失；恢复后重试只创建一次。

- [ ] Step 7: 清理验收数据和进程

仅清理本次创建的 scoped 用户、帖子、评论、互动及媒体引用；关闭本次启动的后端和构建进程。

## Task 7：全量门禁、报告和收尾

**Files:**
- Create: `docs/UI-21B-2B-1-COMMUNITY-WRITES-REPORT.md`

- [ ] Step 1: 运行全量前端与仓库门禁

```powershell
npm --prefix apps/miniapp run typecheck
npm run check:miniapp-static-routes
npm run check:boundaries
npm run build:miniapp
npm run verify
npm run build
```

Expected: 全部 PASS；仅允许既有 vendors 体积告警。

- [ ] Step 2: 证明边界无漂移

```powershell
git diff master...HEAD -- backend apps/admin package.json package-lock.json
git diff master...HEAD -- apps/miniapp/package.json
rg -n "togglePostLike|togglePostFavorite|toggleFollowUser" apps/miniapp/src/pages/post-detail apps/miniapp/src/pages/publish-post
rg -n "发布功能稍后开放|图片功能稍后开放|点赞功能稍后|收藏功能稍后|关注功能稍后" apps/miniapp/src/pages/post-detail apps/miniapp/src/pages/publish-post
```

Expected: 四条均无输出。

- [ ] Step 3: 写 Development/Acceptance/Residue 报告

报告必须包含：基线、改动文件、目标状态端点证据、幂等 header/body 证据、图片 asset 绑定证据、快速重复点击、失败重试、真实 HTTP/MySQL/COS 结果、Boundary、构建、数据与进程清理。

- [ ] Step 4: 自检代码质量

```powershell
git diff --check
rg -n "as any|@ts-ignore|@ts-expect-error" apps/miniapp/src/services/b-community.ts apps/miniapp/src/pages/post-detail apps/miniapp/src/pages/publish-post
git status --short
```

Expected: 前两条无输出；status 只显示待提交报告。

- [ ] Step 5: 提交报告并确认干净

```powershell
git add docs/UI-21B-2B-1-COMMUNITY-WRITES-REPORT.md
git commit -m "docs(UI-21b-2b-1): 记录社区真实写交互验收"
git status --short
git log --oneline -5
```

Expected: status 无输出；日志包含 service、详情状态、评论、发帖和报告的可辨识提交。

- [ ] Step 6: 停止等待 owner 验收

不得在该 worktree 继续二手写交互。owner 在主仓库串行 `--ff-only` 合并后打 `ui21b2b1-community-writes-baseline`，再开始 UI-21b-2b-2。
