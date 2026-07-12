# UI-21b-1 Community and Trade Read-only Wiring Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改变 UI-20 冻结视觉、不修改后端的前提下，将学生端首页发现流、帖子详情与评论、二手列表和二手详情切换为真实 Community/Trade 只读数据。

**Architecture:** 页面只依赖 `services/b-community.ts` 与 `services/c-trade.ts`，Service 层负责响应信封、分页元数据、数字 ID、整数分金额和图片数组归一化；四个新版页面负责稳定 ViewModel、分页、四态呈现和业务层禁用写控件。所有请求失败都停留在诚实错误态，不回退静态样例；Community/Trade 图片直接使用后端 `images[].url` 的 legacy media 302 链路。

**Tech Stack:** Taro 4.2、React 18、TypeScript、微信开发者工具、Go/Gin、MySQL 8.4、Meilisearch、现有 Zustand session 与 `RuntimeSessionGate`。

---

## 文件结构

- Modify: `apps/miniapp/src/services/b-community.ts` — 增加保留分页元数据的社区帖子/评论读取函数，既有导出保持兼容。
- Modify: `apps/miniapp/src/services/c-trade.ts` — 增加支持 category/pageSize 的商品分页读取函数，既有导出保持兼容。
- Modify: `apps/miniapp/src/pages/home/index.tsx` — 首页发现双源真实数据、稳定瀑布流、部分失败和分页。
- Modify: `apps/miniapp/src/pages/home/index.scss` — 仅补原容器内骨架、空态、错误态和真实图片样式。
- Modify: `apps/miniapp/src/pages/post-detail/index.tsx` — 真实帖子/评论读取、分页、404、无图和写控件业务禁用。
- Modify: `apps/miniapp/src/pages/post-detail/index.scss` — 仅补原容器内状态与真实图片样式。
- Modify: `apps/miniapp/src/pages/student-secondhand/index.tsx` — 真实商品分页、服务端分类筛选、真实图片与四态。
- Modify: `apps/miniapp/src/pages/student-secondhand/index.scss` — 仅补原卡片容器内状态与图片样式。
- Modify: `apps/miniapp/src/pages/secondhand-detail/index.tsx` — 真实商品详情、404、卖家字段与写控件业务禁用。
- Modify: `apps/miniapp/src/pages/secondhand-detail/index.scss` — 仅补原 hero/info 容器内状态与图片样式。
- Create: `docs/UI-21B1-COMMUNITY-TRADE-READONLY-DEVELOPMENT.md` — 开发范围、数据映射、视觉不变和边界证据。
- Create: `docs/UI-21B1-COMMUNITY-TRADE-READONLY-ACCEPTANCE.md` — 自动门禁与真实 HTTP/MySQL/DevTools 证据。
- Create: `docs/UI-21B1-COMMUNITY-TRADE-READONLY-RESIDUE.md` — 测试夹具、进程、临时目录和最终清理。
- Temporary only: `D:/codex-temp/ui21b1/**` — 失败基线、自动化脚本、截图和临时日志，不提交。

### Task 1: 建立隔离工作树并固化失败基线

**Files:**
- Read: `docs/superpowers/specs/2026-07-12-ui21b1-community-trade-readonly-design.md`
- Read: four target pages
- Create temporary: `D:/codex-temp/ui21b1/baseline.txt`

- [ ] **Step 1: 核对基线门禁**

Run in main repository:

```powershell
Set-Location 'C:\Users\33141\Documents\New project'
git log --oneline -1 master
git tag --list ui21a-runtime-identity-baseline
git status --short
$autoMigrate = @(rg 'db\.AutoMigrate\(' backend/cmd/server 2>$null).Count
if ($autoMigrate -ne 0) { throw "AutoMigrate count=$autoMigrate" }
git worktree list
```

Also run:

```powershell
git merge-base --is-ancestor 60864d84 master
if ($LASTEXITCODE -ne 0) { throw 'approved UI-21b-1 design is not an ancestor' }
$headSubject = git log -1 --format='%s' master
if ($headSubject -ne 'docs(UI-21b-1): 编写社区二手只读接线实施计划') { throw "unexpected master subject: $headSubject" }
$planDelta = @(git diff --name-only 60864d84..master)
if ($planDelta.Count -ne 1 -or $planDelta[0] -ne 'docs/superpowers/plans/2026-07-12-ui21b1-community-trade-readonly.md') { throw 'unexpected files after approved design' }
git merge-base --is-ancestor 18dc0d28 master
if ($LASTEXITCODE -ne 0) { throw 'UI-21a acceptance commit is not an ancestor' }
git merge-base --is-ancestor 'ui21a-runtime-identity-baseline^{}' master
if ($LASTEXITCODE -ne 0) { throw 'UI-21a baseline tag is not an ancestor' }
```

Expected: `master` is the single plan-doc commit above `60864d84`; UI-21a acceptance and the peeled baseline tag are ancestors; status empty; AutoMigrate count 0; `feat/p0-identity-switch` is listed but untouched.

- [ ] **Step 2: 创建隔离 worktree**

```powershell
git worktree add '..\ui21b1-community-trade-readonly' -b 'feat/ui21b1-community-trade-readonly' master
git -C '..\ui21b1-community-trade-readonly' status --short
```

Expected: new worktree clean. Do not checkout, merge, delete or modify `feat/p0-identity-switch`.

- [ ] **Step 3: 安装隔离工作树依赖且确认 lockfile 不变**

```powershell
Set-Location 'C:\Users\33141\Documents\ui21b1-community-trade-readonly'
npm ci
git status --short
```

Expected: install succeeds; Git status remains empty.

- [ ] **Step 4: 运行真实性失败检查并保存基线**

```powershell
$target = @(
  'apps/miniapp/src/pages/home/index.tsx',
  'apps/miniapp/src/pages/post-detail/index.tsx',
  'apps/miniapp/src/pages/student-secondhand/index.tsx',
  'apps/miniapp/src/pages/secondhand-detail/index.tsx'
)
New-Item -ItemType Directory -Force 'D:\codex-temp\ui21b1' | Out-Null
rg -n 'discoverCards|postCopies|productCopies|const products|const comments|milk-tea|芋泥波波|九成新自行车' $target |
  Tee-Object 'D:\codex-temp\ui21b1\baseline.txt'
```

Expected: command returns matches, proving the four target pages still contain static sources. This is the fail-first acceptance baseline.

### Task 2: 增加兼容的分页读取契约

**Files:**
- Modify: `apps/miniapp/src/services/b-community.ts`
- Modify: `apps/miniapp/src/services/c-trade.ts`

- [ ] **Step 1: 在社区 Service 定义分页结果并保留旧导出**

Replace the private `Page<T>` and the three list functions with this compatible structure:

```ts
export interface PageResult<T> {
  items: T[]
  page: number
  pageSize: number
  total: number
}

interface BackendPage<T> {
  items: T[]
  page: number
  pageSize: number
  total: number
}

export async function getCommunityPostsPage(
  page = 1,
  sort: 'recent' | 'popular' = 'recent',
  pageSize = 20
): Promise<PageResult<CommunityPost>> {
  const result = await request<BackendPage<BackendPost>>(
    `/community/posts?page=${page}&pageSize=${pageSize}&sort=${sort}`
  )
  return { ...result, items: result.items.map(normalizePost) }
}

export async function listCommunityPosts(
  page = 1,
  sort: 'recent' | 'popular' = 'recent'
): Promise<CommunityPost[]> {
  return (await getCommunityPostsPage(page, sort)).items
}

export async function listFollowingPosts(page = 1): Promise<CommunityPost[]> {
  const result = await request<BackendPage<BackendPost>>(`/community/following-feed?page=${page}`)
  return result.items.map(normalizePost)
}

export async function getPostCommentsPage(
  id: string,
  page = 1,
  pageSize = 20
): Promise<PageResult<CommunityComment>> {
  const result = await request<BackendPage<BackendComment>>(
    `/community/posts/${encodeURIComponent(id)}/comments?page=${page}&pageSize=${pageSize}`
  )
  return {
    ...result,
    items: result.items.map((item) => ({
      id: String(item.id),
      author: item.author?.nickname || '校园用户',
      avatar: item.author?.avatar || '',
      content: item.content,
      createdAt: dateLabel(item.createdAt)
    }))
  }
}

export async function listPostComments(id: string, page = 1): Promise<CommunityComment[]> {
  return (await getPostCommentsPage(id, page)).items
}
```

Update `NotificationPage` to extend `BackendPage<BackendNotification>`. Keep `listFollowingPosts`, `listCommunityNotifications` and every write export's public signature and behavior unchanged.

- [ ] **Step 2: 在二手 Service 定义查询对象并正确处理“全部”**

Add these exported types and function, then make the old `listProducts` delegate to it:

```ts
export interface ProductListQuery {
  page?: number
  pageSize?: number
  keyword?: string
  category?: string
  status?: Product['status']
}

export interface ProductPageResult {
  items: Product[]
  page: number
  pageSize: number
  total: number
}

interface BackendPage<T> {
  items: T[]
  page: number
  pageSize: number
  total: number
}

export async function getProductsPage(query: ProductListQuery = {}): Promise<ProductPageResult> {
  const page = query.page ?? 1
  const pageSize = query.pageSize ?? 20
  const status = query.status ?? 'selling'
  const params = [`page=${page}`, `pageSize=${pageSize}`, `status=${status}`]
  if (query.keyword?.trim()) params.push(`keyword=${encodeURIComponent(query.keyword.trim())}`)
  if (query.category?.trim()) params.push(`category=${encodeURIComponent(query.category.trim())}`)

  const result = await request<BackendPage<BackendProduct>>(`/trade/products?${params.join('&')}`)
  return { ...result, items: result.items.map(normalizeProduct) }
}

export async function listProducts(
  page = 1,
  keyword = '',
  status: Product['status'] = 'selling'
): Promise<{ items: Product[]; total: number }> {
  const result = await getProductsPage({ page, keyword, status })
  return { items: result.items, total: result.total }
}

export async function listMyProducts(page = 1): Promise<Product[]> {
  const result = await request<BackendPage<BackendProduct>>(`/trade/my-products?page=${page}`)
  return result.items.map(normalizeProduct)
}
```

Define `BackendPage<T>` locally in this file; do not import the private type from Community. The UI label `全部` must be converted to `undefined`; this function must never receive the literal `全部`. Keep all other existing exports unchanged.

- [ ] **Step 3: Run typecheck and compatibility search**

```powershell
npm --workspace apps/miniapp run typecheck
rg -n 'listCommunityPosts\(|listPostComments\(|listProducts\(' apps/miniapp/src
git diff --check
```

Expected: typecheck PASS; existing call sites still compile; no whitespace errors.

- [ ] **Step 4: Commit the Service contract**

```powershell
git add apps/miniapp/src/services/b-community.ts apps/miniapp/src/services/c-trade.ts
git commit -m 'feat(UI-21b-1): 增加社区二手分页读取契约'
```

### Task 3: 接通首页发现双源瀑布流

**Files:**
- Modify: `apps/miniapp/src/pages/home/index.tsx`
- Modify: `apps/miniapp/src/pages/home/index.scss`

- [ ] **Step 1: 证明首页静态源仍然存在**

```powershell
rg -n 'const discoverCards|setLiked|toggleLike' apps/miniapp/src/pages/home/index.tsx
```

Expected: matches found before implementation.

- [ ] **Step 2: 定义稳定卡片映射与去重函数**

Import `CommunityPost` and `Product` types. Keep the existing `DiscoverCard` shape, add `imageUrl?: string`, and add these pure helpers:

```ts
const tones: DiscoverCard['tone'][] = ['peach', 'sky', 'mint', 'lilac', 'lemon', 'cyan']
const heights: DiscoverCard['height'][] = ['medium', 'tall', 'medium', 'medium', 'short']

function stableNumber(value: string) {
  return Array.from(value).reduce((hash, char) => ((hash * 31) + char.charCodeAt(0)) >>> 0, 7)
}

function visualFor(key: string) {
  const value = stableNumber(key)
  return { tone: tones[value % tones.length], height: heights[value % heights.length] }
}

function mergeUnique(current: DiscoverCard[], incoming: DiscoverCard[]) {
  const values = new Map(current.map((item) => [item.id, item]))
  incoming.forEach((item) => values.set(item.id, item))
  return Array.from(values.values())
}

function toPostCard(post: CommunityPost): DiscoverCard {
  const visual = visualFor(`post:${post.id}`)
  return {
    id: post.id,
    target: 'post',
    ...visual,
    mediaLabel: `图 · ${post.topic || '校园'}`,
    imageUrl: post.images[0],
    title: post.title || post.content,
    author: post.author,
    avatar: post.author.slice(0, 1) || '校',
    likes: post.likedCount
  }
}

function toProductCard(product: Product): DiscoverCard {
  const visual = visualFor(`product:${product.id}`)
  return {
    id: product.id,
    target: 'secondhand',
    ...visual,
    mediaLabel: `图 · ${product.category || '二手'}`,
    imageUrl: product.images[0],
    title: product.title,
    author: product.sellerName,
    avatar: product.sellerName.slice(0, 1) || '校',
    likes: product.favoriteCount
  }
}
```

The avatar cell remains text geometry: use the nickname's first character, not the avatar URL. Image URLs are used only in media `<Image>` nodes.

Use this stable interleave helper:

```ts
function interleaveCards(posts: DiscoverCard[], products: DiscoverCard[]) {
  const result: DiscoverCard[] = []
  const length = Math.max(posts.length, products.length)
  for (let index = 0; index < length; index += 1) {
    if (posts[index]) result.push(posts[index])
    if (products[index]) result.push(products[index])
  }
  return result
}
```

- [ ] **Step 3: 实现双源首屏、部分失败与分页状态**

Replace `discoverCards` and `liked` with state that distinguishes initial and incremental loading:

```ts
const [postCards, setPostCards] = useState<DiscoverCard[]>([])
const [productCards, setProductCards] = useState<DiscoverCard[]>([])
const [initialStatus, setInitialStatus] = useState<'loading' | 'ready' | 'empty' | 'error'>('loading')
const [loadingMore, setLoadingMore] = useState(false)
const [sourceError, setSourceError] = useState({ community: false, trade: false })
const sourceRef = useRef({ communityPage: 0, tradePage: 0, communityTotal: 0, tradeTotal: 0 })
const loadingRef = useRef(false)
const cards = useMemo(
  () => interleaveCards(postCards, productCards),
  [postCards, productCards]
)
```

Implement `loadDiscover(reset, sources)` with `Promise.allSettled`; skipped sources resolve to `null`, while requested failures remain distinguishable:

```ts
const loadDiscover = useCallback(async (
  reset = false,
  sources = { community: true, trade: true }
) => {
  if (loadingRef.current) return
  loadingRef.current = true
  reset ? setInitialStatus('loading') : setLoadingMore(true)
  try {
    const state = sourceRef.current
    const communityPage = reset ? 1 : state.communityPage + 1
    const tradePage = reset ? 1 : state.tradePage + 1
    const requestCommunity = sources.community && (
      reset || state.communityPage === 0 || state.communityPage * 10 < state.communityTotal
    )
    const requestTrade = sources.trade && (
      reset || state.tradePage === 0 || state.tradePage * 10 < state.tradeTotal
    )

    if (!requestCommunity && !requestTrade) return

    const results = await Promise.allSettled([
      requestCommunity
        ? getCommunityPostsPage(communityPage, 'recent', 10)
        : Promise.resolve(null),
      requestTrade
        ? getProductsPage({ page: tradePage, pageSize: 10, status: 'selling' })
        : Promise.resolve(null)
    ])

    const community = results[0].status === 'fulfilled' ? results[0].value : null
    const trade = results[1].status === 'fulfilled' ? results[1].value : null
    const communityFailed = requestCommunity && results[0].status === 'rejected'
    const tradeFailed = requestTrade && results[1].status === 'rejected'
    const nextPosts = (community?.items ?? []).map(toPostCard)
    const nextProducts = (trade?.items ?? []).map(toProductCard)

    if (community) {
      state.communityPage = community.page
      state.communityTotal = community.total
      setPostCards((current) => mergeUnique(reset ? [] : current, nextPosts))
    } else if (reset) {
      setPostCards([])
    }
    if (trade) {
      state.tradePage = trade.page
      state.tradeTotal = trade.total
      setProductCards((current) => mergeUnique(reset ? [] : current, nextProducts))
    } else if (reset) {
      setProductCards([])
    }

    setSourceError({ community: communityFailed, trade: tradeFailed })
    if (reset && (communityFailed || tradeFailed) && nextPosts.length + nextProducts.length === 0) {
      setInitialStatus('error')
    } else if (reset && !communityFailed && !tradeFailed && nextPosts.length + nextProducts.length === 0) {
      setInitialStatus('empty')
    } else if (
      !communityFailed &&
      !tradeFailed &&
      state.communityPage > 0 &&
      state.tradePage > 0 &&
      state.communityTotal === 0 &&
      state.tradeTotal === 0
    ) {
      setInitialStatus('empty')
    } else {
      setInitialStatus('ready')
    }
  } finally {
    loadingRef.current = false
    setLoadingMore(false)
  }
}, [])
```

Retry with `loadDiscover(false, sourceError)` so only failed sources run. Keeping the two source arrays separate makes the final full-array interleave deterministic even when one source succeeds later. When both source totals are reached, the function returns without issuing a request and the `finally` block still releases the lock.

- [ ] **Step 4: 保持原滚动容器并增加观察哨兵**

Call `loadDiscover(true)` on mount. Import the real Taro runtime as `TaroRuntime` only for this API; do not expand `services/taro.ts`, which is outside the approved design scope. Add an existing-content sentinel with a stable ID at the end of the waterfall and observe it without replacing `student-home-content`:

```ts
useEffect(() => {
  if (activeTab !== 'discover' || initialStatus !== 'ready') return undefined
  const page = TaroRuntime.getCurrentInstance().page
  if (!page) return undefined
  const observer = TaroRuntime.createIntersectionObserver(page, { thresholds: [0] })
  observer.relativeTo('.student-home-content').observe('#home-discover-sentinel', (entry) => {
    if (entry.intersectionRatio > 0) void loadDiscover(false)
  })
  return () => observer.disconnect()
}, [activeTab, initialStatus, loadDiscover])
```

Add `id='home-discover-sentinel'` to a small View after the existing columns. Do not change any existing className or convert the container to `ScrollView`.

- [ ] **Step 5: Render real images and honest states inside existing cards**

Import `Image` and add `const [failedImageKeys, setFailedImageKeys] = useState<Set<string>>(() => new Set())`. Inside the existing media View, render without mutating Service objects:

```tsx
{card.imageUrl && !failedImageKeys.has(`${card.target}:${card.id}`) ? (
  <Image
    className='student-post-card__image'
    src={card.imageUrl}
    mode='aspectFill'
    onError={() => setFailedImageKeys((current) => {
      const next = new Set(current)
      next.add(`${card.target}:${card.id}`)
      return next
    })}
  />
) : (
  <Text>{card.mediaLabel}</Text>
)}
```

For loading/empty/error, render existing `student-post-card` geometry with state text. A source-partial error and pagination error stay after already-rendered cards and call `loadDiscover(false, sourceError)` on click. The real DevTools acceptance later blocks one source, retries it, and records that full-array interleave remains deterministic; no test framework or dependency is introduced.

Replace the old like handler with:

```ts
const showReadOnlyToast = () => {
  void Taro.showToast({ title: '互动功能下一阶段接入', icon: 'none', duration: 1400 })
}
```

Do not mutate counts or selected state.

- [ ] **Step 6: Verify and commit home**

```powershell
if (rg -n 'const discoverCards|setLiked|toggleLike|milk-tea|芋泥波波' apps/miniapp/src/pages/home/index.tsx) { throw 'static home data remains' }
npm --workspace apps/miniapp run typecheck
npm run build:miniapp
git diff --check
git add apps/miniapp/src/pages/home/index.tsx apps/miniapp/src/pages/home/index.scss
git commit -m 'feat(UI-21b-1): 接通首页社区二手发现流'
```

### Task 4: 接通帖子详情和评论读取

**Files:**
- Modify: `apps/miniapp/src/pages/post-detail/index.tsx`
- Modify: `apps/miniapp/src/pages/post-detail/index.scss`

- [ ] **Step 1: 证明详情静态回退仍存在**

```powershell
rg -n 'postCopies|const comments|milk-tea' apps/miniapp/src/pages/post-detail/index.tsx
```

Expected: matches found.

- [ ] **Step 2: 增加真实详情状态和路由校验**

Import `Image`, `useCallback`, `useEffect`, `useRef`, `CommunityPost`, `CommunityComment`, `getCommunityPost`, `getPostCommentsPage`, `isApiError`, and direct `TaroRuntime` from `@tarojs/taro` for the observer only. Replace static copies with:

```ts
const requestedId = Taro.getCurrentInstance().router?.params?.id ?? ''
const validId = /^[1-9]\d*$/.test(requestedId)
const [post, setPost] = useState<CommunityPost | null>(null)
const [comments, setComments] = useState<CommunityComment[]>([])
const [status, setStatus] = useState<'loading' | 'ready' | 'empty' | 'error'>(
  validId ? 'loading' : 'empty'
)
const [commentsPage, setCommentsPage] = useState(0)
const [commentsTotal, setCommentsTotal] = useState(0)
const [commentsError, setCommentsError] = useState(false)
const [failedImageKeys, setFailedImageKeys] = useState<Set<string>>(() => new Set())
const [reloadKey, setReloadKey] = useState(0)
const requestVersion = useRef(0)
const commentsLoading = useRef(false)
const retryPost = useCallback(() => setReloadKey((value) => value + 1), [])
```

Use this exact first-load lifecycle; it ignores stale completions and never substitutes a static post:

```ts
useEffect(() => {
  if (!validId) {
    setStatus('empty')
    return undefined
  }
  const version = requestVersion.current + 1
  requestVersion.current = version
  let active = true
  setStatus('loading')
  setCommentsError(false)

  void Promise.all([
    getCommunityPost(requestedId),
    getPostCommentsPage(requestedId, 1)
  ]).then(([nextPost, nextComments]) => {
    if (!active || requestVersion.current !== version) return
    setPost(nextPost)
    setComments(nextComments.items)
    setCommentsPage(nextComments.page)
    setCommentsTotal(nextComments.total)
    setStatus('ready')
  }).catch((error: unknown) => {
    if (!active || requestVersion.current !== version) return
    setPost(null)
    setComments([])
    setStatus(isApiError(error) && error.status === 404 ? 'empty' : 'error')
  })

  return () => {
    active = false
    requestVersion.current += 1
  }
}, [reloadKey, requestedId, validId])
```

- [ ] **Step 3: Add comment pagination without replacing the View**

Append comments by ID and retain already loaded comments if an incremental request fails:

```ts
function mergeCommentsById(current: CommunityComment[], incoming: CommunityComment[]) {
  const values = new Map(current.map((item) => [item.id, item]))
  incoming.forEach((item) => values.set(item.id, item))
  return Array.from(values.values())
}

const loadMoreComments = useCallback(async () => {
  if (!post || commentsLoading.current || comments.length >= commentsTotal) return
  commentsLoading.current = true
  setCommentsError(false)
  try {
    const result = await getPostCommentsPage(post.id, commentsPage + 1)
    setComments((current) => mergeCommentsById(current, result.items))
    setCommentsPage(result.page)
    setCommentsTotal(result.total)
  } catch {
    setCommentsError(true)
  } finally {
    commentsLoading.current = false
  }
}, [comments.length, commentsPage, commentsTotal, post])
```

Render `commentsError` as a non-overlay bottom retry row that calls `loadMoreComments`. Observe `#post-comments-sentinel` only while `status==='ready'` and more comments remain:

```ts
useEffect(() => {
  if (status !== 'ready' || comments.length >= commentsTotal) return undefined
  const page = TaroRuntime.getCurrentInstance().page
  if (!page) return undefined
  const observer = TaroRuntime.createIntersectionObserver(page, { thresholds: [0] })
  observer.relativeTo('.post-detail-scroll').observe('#post-comments-sentinel', (entry) => {
    if (entry.intersectionRatio > 0) void loadMoreComments()
  })
  return () => observer.disconnect()
}, [comments.length, commentsTotal, loadMoreComments, status])
```

Keep `.post-detail-scroll` as the existing CSS-scrolled View.

- [ ] **Step 4: Map truthful content and business-disable writes**

- First image -> existing hero `<Image mode='aspectFill'>`; no image -> existing hero placeholder.
- `topic` -> one real tag; title/content/counts/author/school/date -> existing positions.
- Comments show author/content/date. The backend has no comment like count, so render no numeric like value.
- 404 text: `帖子不存在或已下架`; network text: `加载失败，点击重试`, and that error container calls `retryPost`; empty comments: `暂时没有评论`.
- Follow/like/favorite buttons call `showReadOnlyToast` and never update local state.
- Comment input may update `draft`, but submit only shows the same Toast; it must not clear `draft` or append a comment.

Define `showReadOnlyToast` locally as `Taro.showToast({ title: '互动功能下一阶段接入', icon: 'none', duration: 1400 })`. Render the first image only when `!failedImageKeys.has(post.id)`; `onError` adds `post.id` to a cloned `Set`, preserving the existing hero placeholder geometry.

- [ ] **Step 5: Verify and commit post detail**

```powershell
if (rg -n 'postCopies|const comments|milk-tea|芋泥波波' apps/miniapp/src/pages/post-detail/index.tsx) { throw 'static post detail remains' }
npm --workspace apps/miniapp run typecheck
npm run build:miniapp
git diff --check
git add apps/miniapp/src/pages/post-detail/index.tsx apps/miniapp/src/pages/post-detail/index.scss
git commit -m 'feat(UI-21b-1): 接通帖子详情与评论读取'
```

### Task 5: 接通二手列表和服务端分类分页

**Files:**
- Modify: `apps/miniapp/src/pages/student-secondhand/index.tsx`
- Modify: `apps/miniapp/src/pages/student-secondhand/index.scss`

- [ ] **Step 1: 证明列表静态源仍存在**

```powershell
rg -n 'const products|desk-lamp|earphones|guitar' apps/miniapp/src/pages/student-secondhand/index.tsx
```

Expected: matches found.

- [ ] **Step 2: Replace current-page filtering with server query state**

Import `Image`, `useCallback`, `useEffect`, `useRef`, `Product` and `getProductsPage`. Keep the current category labels and add:

```ts
const [items, setItems] = useState<Product[]>([])
const [status, setStatus] = useState<'loading' | 'ready' | 'empty' | 'error'>('loading')
const [page, setPage] = useState(0)
const [total, setTotal] = useState(0)
const [loadingMore, setLoadingMore] = useState(false)
const [pagingError, setPagingError] = useState(false)
const [failedImageKeys, setFailedImageKeys] = useState<Set<string>>(() => new Set())
const queryVersion = useRef(0)
const categoryRef = useRef<CategoryKey>('全部')
const loadingKeys = useRef(new Set<string>())
```

Define `mergeProductsById`, then use a versioned fetch so a fast category switch cannot be blocked or overwritten by an older request:

```ts
function mergeProductsById(current: Product[], incoming: Product[]) {
  const values = new Map(current.map((item) => [item.id, item]))
  incoming.forEach((item) => values.set(item.id, item))
  return Array.from(values.values())
}

const fetchProductsPage = useCallback(async (
  category: CategoryKey,
  targetPage: number,
  version: number,
  reset: boolean
) => {
  const key = `${version}:${targetPage}`
  if (loadingKeys.current.has(key)) return
  loadingKeys.current.add(key)
  reset ? setStatus('loading') : setLoadingMore(true)
  setPagingError(false)
  try {
    const result = await getProductsPage({
      page: targetPage,
      pageSize: 20,
      category: category === '全部' ? undefined : category,
      status: 'selling'
    })
    if (queryVersion.current !== version || categoryRef.current !== category) return
    setItems((current) => mergeProductsById(reset ? [] : current, result.items))
    setPage(result.page)
    setTotal(result.total)
    setStatus(result.total === 0 ? 'empty' : 'ready')
  } catch {
    if (queryVersion.current !== version || categoryRef.current !== category) return
    if (reset) setStatus('error')
    else setPagingError(true)
  } finally {
    loadingKeys.current.delete(key)
    if (queryVersion.current === version) setLoadingMore(false)
  }
}, [])

useEffect(() => {
  categoryRef.current = activeCategory
  const version = queryVersion.current + 1
  queryVersion.current = version
  setItems([])
  setPage(0)
  setTotal(0)
  void fetchProductsPage(activeCategory, 1, version, true)
}, [activeCategory, fetchProductsPage])

const loadMoreProducts = useCallback(() => {
  if (status !== 'ready' || loadingMore || items.length >= total) return
  void fetchProductsPage(activeCategory, page + 1, queryVersion.current, false)
}, [activeCategory, fetchProductsPage, items.length, loadingMore, page, status, total])

const retryCurrentCategory = useCallback(() => {
  const category = categoryRef.current
  const version = queryVersion.current + 1
  queryVersion.current = version
  setItems([])
  setPage(0)
  setTotal(0)
  setPagingError(false)
  void fetchProductsPage(category, 1, version, true)
}, [fetchProductsPage])
```

Never send `category=全部`. A stale response is ignored before any state write. The per-version request key lets a new category's page 1 start even if the old category is still in flight.

- [ ] **Step 3: Add real card mapping and existing ScrollView pagination**

Change `ProductCard.category` from `CategoryKey` to `string` and add `imageUrl?: string`. Define the mapping explicitly:

```ts
function toCard(product: Product): ProductCard {
  const value = stableNumber(`product:${product.id}`)
  return {
    id: product.id,
    title: product.title,
    price: product.price,
    category: product.category,
    imageLabel: `图 · ${product.category || '二手'}`,
    imageUrl: product.images[0],
    status: product.status === 'sold' ? 'sold' : 'selling',
    tone: productTones[value % productTones.length],
    height: productHeights[value % productHeights.length]
  }
}
```

Reuse the exact `stableNumber` algorithm from Home, with page-local `productTones` and `productHeights` constants. Derive `cards = useMemo(() => items.map(toCard), [items])`. Add `onScrollToLower={loadMoreProducts}` and `lowerThreshold={120}` to the existing `student-secondhand-scroll` only.

Inside `secondhand-card__media`, render `<Image mode='aspectFill'>` only when `imageUrl` exists and the ID is not in `failedImageKeys`; on error add the ID to a cloned `Set` and fall back to `图 · ${category || '二手'}` in the same geometry.

- [ ] **Step 4: Render four states without moving header/navigation**

- First load: existing card skeletons inside `student-secondhand-waterfall`.
- Empty: one same-size card saying `该分类暂无在售宝贝`.
- Error: one same-size retry card wired to `retryCurrentCategory` (not `loadMoreProducts`, which deliberately rejects non-ready states).
- Loading more: a non-overlay footer inside the waterfall.
- Pagination error: preserve current cards and show a retry footer wired to `loadMoreProducts`.
- Header, category bar, notice, double columns and `StudentBottomNav` remain unchanged.

- [ ] **Step 5: Verify and commit secondhand list**

```powershell
if (rg -n 'const products|desk-lamp|earphones|guitar|九成新自行车' apps/miniapp/src/pages/student-secondhand/index.tsx) { throw 'static trade list remains' }
npm --workspace apps/miniapp run typecheck
npm run build:miniapp
git diff --check
git add apps/miniapp/src/pages/student-secondhand/index.tsx apps/miniapp/src/pages/student-secondhand/index.scss
git commit -m 'feat(UI-21b-1): 接通二手列表与分类分页'
```

### Task 6: 接通二手详情并移除伪造卖家字段

**Files:**
- Modify: `apps/miniapp/src/pages/secondhand-detail/index.tsx`
- Modify: `apps/miniapp/src/pages/secondhand-detail/index.scss`

- [ ] **Step 1: 证明详情静态回退仍存在**

```powershell
rg -n 'productCopies|missingProductCopy|bike|zk_xiangbei' apps/miniapp/src/pages/secondhand-detail/index.tsx
```

Expected: matches found.

- [ ] **Step 2: Load a validated real product**

Import `Image`, `useCallback`, `useEffect`, `useRef`, `Product`, `getProduct` and `isApiError`. Replace static copies with:

```ts
const requestedId = Taro.getCurrentInstance().router?.params?.id ?? ''
const validId = /^[1-9]\d*$/.test(requestedId)
const [product, setProduct] = useState<Product | null>(null)
const [status, setStatus] = useState<'loading' | 'ready' | 'empty' | 'error'>(
  validId ? 'loading' : 'empty'
)
const [reloadKey, setReloadKey] = useState(0)
const requestVersion = useRef(0)
const [imageFailed, setImageFailed] = useState(false)
const retryProduct = useCallback(() => setReloadKey((value) => value + 1), [])
```

Use this lifecycle; do not default to `bike`:

```ts
useEffect(() => {
  if (!validId) {
    setStatus('empty')
    return undefined
  }
  const version = requestVersion.current + 1
  requestVersion.current = version
  let active = true
  setStatus('loading')
  setImageFailed(false)

  void getProduct(requestedId).then((value) => {
    if (!active || requestVersion.current !== version) return
    setProduct(value)
    setStatus('ready')
  }).catch((error: unknown) => {
    if (!active || requestVersion.current !== version) return
    setProduct(null)
    setStatus(isApiError(error) && error.status === 404 ? 'empty' : 'error')
  })

  return () => {
    active = false
    requestVersion.current += 1
  }
}, [reloadKey, requestedId, validId])
```

- [ ] **Step 3: Map only fields that exist**

Use:

```ts
const statusLabel = product?.status === 'selling'
  ? '出售中'
  : product?.status === 'reserved'
    ? '已预订'
    : '已售'
const tags = product ? [product.category, product.condition].filter(Boolean) : []
const sellerMeta = product?.campus || '校内'
```

Render the first image in the existing hero only when `product.images[0] && !imageFailed`; its `onError` sets `imageFailed=true`. No image or image error uses `图 · 卖家未上传图片`. Render real title, description, price, time, seller nickname/campus, and use the nickname's first character in the existing text avatar geometry. Do not render static browser count, sold-count, account or WeChat ID.

- [ ] **Step 4: Keep geometry and block writes honestly**

- 404: existing info container says `宝贝不存在或已下架`.
- Error: same container says `加载失败，点击重试` and calls `retryProduct`.
- Favorite: Toast only; no local selected state.
- Contact sheet may open as a pure display state. `发起私聊` says `私信功能下一阶段接入`; `加微信联系` says `卖家未提供微信联系方式`.
- Do not call `toggleProductFavorite` or `contactSeller`; do not navigate to static chat.
- Seller homepage button says `用户主页下一阶段接入` and does not navigate to a static fallback profile.

- [ ] **Step 5: Verify and commit secondhand detail**

```powershell
if (rg -n 'productCopies|missingProductCopy|zk_xiangbei|九成新自行车' apps/miniapp/src/pages/secondhand-detail/index.tsx) { throw 'static trade detail remains' }
npm --workspace apps/miniapp run typecheck
npm run build:miniapp
git diff --check
git add apps/miniapp/src/pages/secondhand-detail/index.tsx apps/miniapp/src/pages/secondhand-detail/index.scss
git commit -m 'feat(UI-21b-1): 接通二手详情只读数据'
```

### Task 7: 运行自动门禁和真实环境验收

**Files:**
- Create temporary: `D:/codex-temp/ui21b1/acceptance/**`
- Read: `backend/.env` without printing secrets
- Read: four built pages under `apps/miniapp/dist`

- [ ] **Step 1: Run the committed-scope guard**

```powershell
Set-Location 'C:\Users\33141\Documents\ui21b1-community-trade-readonly'
$forbidden = @(
  'discoverCards', 'postCopies', 'productCopies', 'missingProductCopy',
  'milk-tea', '芋泥波波', 'zk_xiangbei', '九成新自行车'
) -join '|'
$targets = @(
  'apps/miniapp/src/pages/home/index.tsx',
  'apps/miniapp/src/pages/post-detail/index.tsx',
  'apps/miniapp/src/pages/student-secondhand/index.tsx',
  'apps/miniapp/src/pages/secondhand-detail/index.tsx'
)
if (rg -n $forbidden $targets) { throw 'static fallback residue detected' }
if (rg -n 'Taro\.request' $targets) { throw 'page bypassed service layer' }
$allowed = '^(apps/miniapp/src/services/(b-community|c-trade)\.ts|apps/miniapp/src/pages/(home|post-detail|student-secondhand|secondhand-detail)/|docs/UI-21B1-COMMUNITY-TRADE-READONLY-(DEVELOPMENT|ACCEPTANCE|RESIDUE)\.md)$'
$changed = @(git diff --name-only master...HEAD)
$unexpected = @($changed | Where-Object { $_ -notmatch $allowed })
if ($unexpected.Count -gt 0) { throw "out-of-scope files: $($unexpected -join ', ')" }
if ($changed | Where-Object { $_ -match '(^|/)(package(-lock)?\.json|pnpm-lock\.yaml|yarn\.lock)$' }) {
  throw 'dependency manifest changed'
}
git diff --check master...HEAD
```

Expected: no static fallback; no direct request; forbidden scopes have zero diff.

- [ ] **Step 2: Run the complete automated gate**

```powershell
npm run check:boundaries
npm run check:miniapp-static-routes
npm --workspace apps/miniapp run typecheck
npm run build:miniapp
npm run verify
git diff --check
```

Expected: all PASS; route checker reports 25 routes; only pre-existing vendor-size/Sass warnings may remain.

- [ ] **Step 3: Start real dependencies and backend**

Use the existing local services; do not install or replace them. Build an exact temporary backend binary from the main repository (backend code is proven zero-diff by Step 1), load the real main-repository `backend/.env`, and use isolated port `18081` so no existing process is stopped.

The PowerShell fragments in Steps 3, 4, 4a, 4b, 5 and 6 are one contiguous temporary script: `D:/codex-temp/ui21b1/acceptance.ps1`. Assemble the complete file first with `apply_patch`, including the closing `finally` from Step 6, and only then execute it once. Never run the fragments piecemeal. Begin the script with this guard so every thrown assertion still reaches cleanup:

```powershell
$ErrorActionPreference = 'Stop'
$root = 'D:\codex-temp\ui21b1'
$acceptanceScript = Join-Path $root 'acceptance.ps1'
$previousHTTPAddr = [Environment]::GetEnvironmentVariable('HTTP_ADDR', 'Process')
$previousApiBase = [Environment]::GetEnvironmentVariable('TARO_APP_API_BASE', 'Process')
$fixtureCreated = $false
$devToolsOpened = $false
$ownedPid = $null
$cleanupErrors = [System.Collections.Generic.List[string]]::new()
try {
```

Append the following block immediately after that opening `try {`:

```powershell
$root = 'D:\codex-temp\ui21b1'
$acceptance = Join-Path $root 'acceptance'
New-Item -ItemType Directory -Force $acceptance | Out-Null
if (-not (Test-NetConnection 127.0.0.1 -Port 3306 -InformationLevel Quiet)) { throw 'MySQL 3306 unavailable' }
if (-not (Test-NetConnection 127.0.0.1 -Port 7700 -InformationLevel Quiet)) { throw 'Meilisearch 7700 unavailable' }
if (Test-NetConnection 127.0.0.1 -Port 18081 -InformationLevel Quiet) { throw 'isolated port 18081 already occupied' }
Remove-Item -Force -ErrorAction SilentlyContinue "$root\backend.pid", "$root\backend.owned.pid"

Set-Location 'C:\Users\33141\Documents\New project\backend'
go build -o "$root\backend-ui21b1.exe" ./cmd/server
if ($LASTEXITCODE -ne 0) { throw 'backend build failed' }
$env:HTTP_ADDR = ':18081'
$backend = Start-Process -FilePath "$root\backend-ui21b1.exe" -WorkingDirectory $PWD -RedirectStandardOutput "$acceptance\backend.stdout.log" -RedirectStandardError "$acceptance\backend.stderr.log" -PassThru
$ownedPid = $backend.Id
$backend.Id | Set-Content -Encoding ascii "$root\backend.pid"
$backend.Id | Set-Content -Encoding ascii "$root\backend.owned.pid"
$healthy = $false
for ($attempt = 0; $attempt -lt 40; $attempt += 1) {
  try {
    $health = Invoke-RestMethod 'http://127.0.0.1:18081/healthz' -TimeoutSec 1
    if ($health.code -eq 0) { $healthy = $true; break }
  } catch { Start-Sleep -Milliseconds 500 }
}
if (-not $healthy) { throw 'isolated backend did not become healthy; inspect backend.stderr.log' }
```

Expected: MySQL and Meili ports true; port 18081 was free; the directly owned binary becomes healthy. Never kill or reuse an unrelated 8080 process.

- [ ] **Step 4: Verify HTTP contracts before opening DevTools**

```powershell
$community = Invoke-RestMethod 'http://127.0.0.1:18081/api/v1/community/posts?page=1&pageSize=20&sort=recent'
$trade = Invoke-RestMethod 'http://127.0.0.1:18081/api/v1/trade/products?page=1&pageSize=20&status=selling'
if ($community.code -ne 0 -or $trade.code -ne 0) { throw 'read contract failed' }
if ($community.data.page -ne 1 -or $trade.data.page -ne 1) { throw 'page metadata mismatch' }
$community | ConvertTo-Json -Depth 8 | Set-Content -Encoding utf8 'D:\codex-temp\ui21b1\acceptance\community-page1.json'
$trade | ConvertTo-Json -Depth 8 | Set-Content -Encoding utf8 'D:\codex-temp\ui21b1\acceptance\trade-page1.json'
```

Always run the exact scoped fixture in Step 4a after saving this live baseline, then repeat the list requests in Step 4b. This guarantees a deterministic 25-comment target and page-2 coverage without depending on shared local data. Never print `MYSQL_DSN`.

- [ ] **Step 4a: Seed reproducible scoped fixtures when required**

Create temporary `D:/codex-temp/ui21b1/fixture.go` with `apply_patch`; it is outside Git and must be removed in cleanup:

```go
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const fixtureOpenID = "ui21b1_acceptance_user"
const fixturePrefix = "UI21B1_"

type User struct {
	ID uint64 `gorm:"primaryKey"`
	OpenID string `gorm:"column:open_id"`
	Nickname string
	School string
	Campus string
	Role string
	Status string
	TokenVersion uint64
	CreatedAt time.Time
	UpdatedAt time.Time
}
func (User) TableName() string { return "users" }

type Post struct {
	ID uint64 `gorm:"primaryKey"`
	UserID uint64
	Topic string
	Title string
	Content string
	ReviewStatus string
	Pinned bool
	LikedCount int64
	CommentCount int64
	FavoriteCount int64
	CreatedAt time.Time
	UpdatedAt time.Time
}
func (Post) TableName() string { return "posts" }

type Comment struct {
	ID uint64 `gorm:"primaryKey"`
	PostID uint64
	UserID uint64
	Content string
	ReviewStatus string
	CreatedAt time.Time
	UpdatedAt time.Time
}
func (Comment) TableName() string { return "comments" }

type Product struct {
	ID uint64 `gorm:"primaryKey"`
	SellerID uint64
	Title string
	Description string
	Category string
	Price int64
	OriginalPrice int64
	Condition string `gorm:"column:condition_label"`
	Campus string
	Status string
	ReviewStatus string
	FavoriteCount int64
	CreatedAt time.Time
	UpdatedAt time.Time
}
func (Product) TableName() string { return "products" }

type Manifest struct {
	UserID uint64 `json:"userId"`
	PostIDs []uint64 `json:"postIds"`
	ProductIDs []uint64 `json:"productIds"`
	CommentPostID uint64 `json:"commentPostId"`
	CommentIDs []uint64 `json:"commentIds"`
	OutboxCount int64 `json:"outboxCount"`
}

func cleanup(db *gorm.DB) error {
	var user User
	err := db.Where("open_id = ?", fixtureOpenID).First(&user).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) { return err }
	var postIDs, productIDs []uint64
	if err := db.Table("posts").Where("title LIKE ?", fixturePrefix+"%").Pluck("id", &postIDs).Error; err != nil { return err }
	if err := db.Table("products").Where("title LIKE ?", fixturePrefix+"%").Pluck("id", &productIDs).Error; err != nil { return err }
	return db.Transaction(func(tx *gorm.DB) error {
		if len(postIDs) > 0 {
			for _, table := range []string{"post_images", "post_likes", "post_favorites", "comments"} {
				if err := tx.Exec(fmt.Sprintf("DELETE FROM %s WHERE post_id IN ?", table), postIDs).Error; err != nil { return err }
			}
			if err := tx.Exec("DELETE FROM review_tasks WHERE object_type = ? AND object_id IN ?", "post", postIDs).Error; err != nil { return err }
			if err := tx.Exec("DELETE FROM posts WHERE id IN ?", postIDs).Error; err != nil { return err }
		}
		if len(productIDs) > 0 {
			for _, table := range []string{"product_images", "product_favorites", "trade_search_outbox"} {
				if err := tx.Exec(fmt.Sprintf("DELETE FROM %s WHERE product_id IN ?", table), productIDs).Error; err != nil { return err }
			}
			if err := tx.Exec("DELETE FROM im_conversations WHERE reference_type = ? AND reference_id IN ?", "product", productIDs).Error; err != nil { return err }
			if err := tx.Exec("DELETE FROM review_tasks WHERE object_type = ? AND object_id IN ?", "product", productIDs).Error; err != nil { return err }
			if err := tx.Exec("DELETE FROM products WHERE id IN ?", productIDs).Error; err != nil { return err }
		}
		if user.ID != 0 {
			if err := tx.Exec("DELETE FROM comments WHERE user_id = ?", user.ID).Error; err != nil { return err }
			if err := tx.Exec("DELETE FROM notifications WHERE recipient_id = ? OR actor_id = ?", user.ID, user.ID).Error; err != nil { return err }
			if err := tx.Exec("DELETE FROM user_follows WHERE follower_id = ? OR followee_id = ?", user.ID, user.ID).Error; err != nil { return err }
			if err := tx.Exec("DELETE FROM users WHERE id = ?", user.ID).Error; err != nil { return err }
		}
		return nil
	})
}

func residue(db *gorm.DB) (map[string]int64, error) {
	var users, posts, comments, products int64
	if err := db.Table("users").Where("open_id = ?", fixtureOpenID).Count(&users).Error; err != nil { return nil, err }
	if err := db.Table("posts").Where("title LIKE ?", fixturePrefix+"%").Count(&posts).Error; err != nil { return nil, err }
	if err := db.Table("comments").Where("content LIKE ?", fixturePrefix+"%").Count(&comments).Error; err != nil { return nil, err }
	if err := db.Table("products").Where("title LIKE ?", fixturePrefix+"%").Count(&products).Error; err != nil { return nil, err }
	return map[string]int64{"users": users, "posts": posts, "comments": comments, "products": products}, nil
}

func seed(db *gorm.DB) (Manifest, error) {
	if err := cleanup(db); err != nil { return Manifest{}, err }
	manifest := Manifest{}
	categories := []string{"数码", "书籍资料", "生活用品"}
	err := db.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		user := User{OpenID: fixtureOpenID, Nickname: "UI21B1验收用户", School: "湛江科技学院", Campus: "校内", Role: "user", Status: "active", TokenVersion: 1, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&user).Error; err != nil { return err }
		manifest.UserID = user.ID
		for index := 0; index < 25; index++ {
			stamp := now.Add(time.Duration(index) * time.Millisecond)
			post := Post{UserID: user.ID, Topic: fmt.Sprintf("UI21B1话题%d", index%3), Title: fmt.Sprintf("%s社区帖子%02d", fixturePrefix, index+1), Content: fmt.Sprintf("%s真实分页验收内容%02d", fixturePrefix, index+1), ReviewStatus: "approved", LikedCount: int64(index), CommentCount: 0, FavoriteCount: int64(index / 2), CreatedAt: stamp, UpdatedAt: stamp}
			if index == 0 { post.CommentCount = 25 }
			if err := tx.Create(&post).Error; err != nil { return err }
			manifest.PostIDs = append(manifest.PostIDs, post.ID)
			if index == 0 {
				manifest.CommentPostID = post.ID
				for commentIndex := 0; commentIndex < 25; commentIndex++ {
					comment := Comment{PostID: post.ID, UserID: user.ID, Content: fmt.Sprintf("%s评论%02d", fixturePrefix, commentIndex+1), ReviewStatus: "approved", CreatedAt: stamp.Add(time.Duration(commentIndex)*time.Millisecond), UpdatedAt: stamp}
					if err := tx.Create(&comment).Error; err != nil { return err }
					manifest.CommentIDs = append(manifest.CommentIDs, comment.ID)
				}
			}
			product := Product{SellerID: user.ID, Title: fmt.Sprintf("%s二手商品%02d", fixturePrefix, index+1), Description: fmt.Sprintf("%s真实商品详情%02d", fixturePrefix, index+1), Category: categories[index%len(categories)], Price: int64(1500 + index), OriginalPrice: int64(3000 + index), Condition: "验收成色", Campus: "校内", Status: "selling", ReviewStatus: "approved", FavoriteCount: int64(index), CreatedAt: stamp, UpdatedAt: stamp}
			if err := tx.Create(&product).Error; err != nil { return err }
			manifest.ProductIDs = append(manifest.ProductIDs, product.ID)
		}
		return nil
	})
	if err == nil {
		err = db.Table("trade_search_outbox").Where("product_id IN ?", manifest.ProductIDs).Count(&manifest.OutboxCount).Error
	}
	return manifest, err
}

func main() {
	if err := godotenv.Load(".env"); err != nil { panic("backend/.env unavailable") }
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" { panic("MYSQL_DSN is empty") }
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil { panic(err) }
	command := "seed"
	if len(os.Args) > 1 { command = os.Args[1] }
	if command == "cleanup" {
		if err := cleanup(db); err != nil { panic(err) }
		remaining, err := residue(db)
		if err != nil { panic(err) }
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"cleaned": true, "residue": remaining})
		return
	}
	manifest, err := seed(db)
	if err != nil { panic(err) }
	if err := json.NewEncoder(os.Stdout).Encode(manifest); err != nil { panic(err) }
}
```

Run from the main backend directory so `.env` is loaded without copying or printing secrets:

```powershell
Set-Location 'C:\Users\33141\Documents\New project\backend'
go run 'D:\codex-temp\ui21b1\fixture.go' seed | Set-Content -Encoding utf8 'D:\codex-temp\ui21b1\acceptance\fixture.json'
if ($LASTEXITCODE -ne 0) { throw 'fixture seed failed' }
$fixture = Get-Content -Raw -Encoding utf8 'D:\codex-temp\ui21b1\acceptance\fixture.json' | ConvertFrom-Json
if ($fixture.postIds.Count -ne 25 -or $fixture.productIds.Count -ne 25 -or $fixture.commentIds.Count -ne 25) { throw 'fixture count mismatch' }
if ($fixture.outboxCount -ne 0) { throw 'direct fixture unexpectedly created trade outbox rows' }
$fixtureCreated = $true
```

The fixture uses only approved rows, has no images/interactions, creates no Meili document or outbox event, and is isolated by a unique `open_id` plus recorded IDs.

- [ ] **Step 4b: Assert list/detail/comment pagination contracts**

```powershell
$base = 'http://127.0.0.1:18081/api/v1'
$community1 = Invoke-RestMethod "$base/community/posts?page=1&pageSize=20&sort=recent"
$community2 = Invoke-RestMethod "$base/community/posts?page=2&pageSize=20&sort=recent"
$trade1 = Invoke-RestMethod "$base/trade/products?page=1&pageSize=20&status=selling"
$trade2 = Invoke-RestMethod "$base/trade/products?page=2&pageSize=20&status=selling"
$postID = [string]$community1.data.items[0].id
$productID = [string]$trade1.data.items[0].id
$post = Invoke-RestMethod "$base/community/posts/$postID"
$product = Invoke-RestMethod "$base/trade/products/$productID"
if ([string]$post.data.id -ne $postID -or [string]$product.data.id -ne $productID) { throw 'list/detail ID mismatch' }

$commentID = [string]$fixture.commentPostId
$comments1 = Invoke-RestMethod "$base/community/posts/$commentID/comments?page=1&pageSize=20"
$comments2 = Invoke-RestMethod "$base/community/posts/$commentID/comments?page=2&pageSize=20"
if ($comments1.data.total -ne 25 -or $comments2.data.items.Count -ne 5) { throw 'comment pagination mismatch' }

foreach ($pair in @(@($community1,$community2), @($trade1,$trade2), @($comments1,$comments2))) {
  $ids = @($pair[0].data.items.id) + @($pair[1].data.items.id)
  if (($ids | Select-Object -Unique).Count -ne $ids.Count) { throw 'duplicate ID across pages' }
}

$missingPostStatus = try { Invoke-WebRequest "$base/community/posts/999999999999999" -UseBasicParsing -ErrorAction Stop | Select-Object -ExpandProperty StatusCode } catch { [int]$_.Exception.Response.StatusCode }
$missingProductStatus = try { Invoke-WebRequest "$base/trade/products/999999999999999" -UseBasicParsing -ErrorAction Stop | Select-Object -ExpandProperty StatusCode } catch { [int]$_.Exception.Response.StatusCode }
if ($missingPostStatus -ne 404 -or $missingProductStatus -ne 404) { throw 'missing resource contract mismatch' }

[ordered]@{
  postListID = $postID
  postDetailID = [string]$post.data.id
  productListID = $productID
  productDetailID = [string]$product.data.id
  communityTotal = $community1.data.total
  productTotal = $trade1.data.total
  commentTotal = $comments1.data.total
  pageDuplicates = 0
  missingPostStatus = $missingPostStatus
  missingProductStatus = $missingProductStatus
} | ConvertTo-Json | Set-Content -Encoding utf8 'D:\codex-temp\ui21b1\acceptance\http-assertions.json'
```

- [ ] **Step 5: Run DevTools acceptance scenarios**

Build and open the exact worktree path:

```powershell
Set-Location 'C:\Users\33141\Documents\ui21b1-community-trade-readonly'
$env:TARO_APP_API_BASE = 'http://127.0.0.1:18081/api/v1'
npm run build:miniapp
if ($LASTEXITCODE -ne 0) { throw 'miniapp acceptance build failed' }
& 'D:\3314\微信web开发者工具\cli.bat' open --project 'C:\Users\33141\Documents\ui21b1-community-trade-readonly\apps\miniapp'
$devToolsOpened = $true
if ($LASTEXITCODE -ne 0) { throw 'DevTools exact-project open failed' }
```

Capture screenshots and Network evidence under `D:/codex-temp/ui21b1/acceptance` for:

1. Home discover shows real Community/Trade IDs and content.
2. List card -> post detail preserves ID; comments match `/comments` response.
3. Secondhand list -> detail preserves ID and integer-cent price display.
4. Page 2 appends without duplicate IDs and stops at `total`.
5. Unknown numeric ID shows 404 state, not a static post/bike.
6. Stop only the current run's in-memory `$ownedPid` after verifying its executable path (the same PID is recorded in `backend.owned.pid` only as evidence); confirm port 18081 closes, page shows retry state and never shows demo data. Restart the same binary from the main `backend` directory, replace both PID files, wait for health, then retry successfully.
7. No-image items keep the frozen media geometry.
8. Like/favorite/follow/comment/contact controls send no write request and show only the next-stage message.
9. In DevTools Network request blocking, block only `*/trade/products*`: Community cards stay visible, a Trade retry row appears, and unblocking/retrying restores a deterministic full interleave. Repeat with Community blocked.
10. Switch categories `数码 → 书籍资料 → 生活用品` quickly; only the last category's response may populate the list.
11. Block page-2 comments once; page-1 comments remain, retry succeeds, and no unhandled rejection appears.
12. If a real approved row has `images[0].url`, record its list response, the legacy content request's 302 `Location`, and the rendered `<Image>`. Block that one image request and capture same-geometry fallback. If the real database has no approved image row, record this single image-chain item as `CONDITIONAL`; never manufacture a public image URL.

Keep the guarded script alive while the operator captures scenarios 1-5:

```powershell
Read-Host 'Complete DevTools scenarios 1-5, save evidence, then press Enter for owned-backend restart scenario'
```

Use this owned-process sequence for scenario 6:

```powershell
$root = 'D:\codex-temp\ui21b1'
if (-not $ownedPid) { throw 'owned backend PID missing from current acceptance run' }
$ownedProcess = Get-Process -Id $ownedPid -ErrorAction SilentlyContinue
if (-not $ownedProcess) { throw 'owned backend exited before restart scenario' }
$expectedExe = [IO.Path]::GetFullPath("$root\backend-ui21b1.exe")
$actualExe = [IO.Path]::GetFullPath($ownedProcess.Path)
if (-not [StringComparer]::OrdinalIgnoreCase.Equals($expectedExe, $actualExe)) {
  throw "refusing to stop reused PID $ownedPid at $actualExe"
}
Stop-Process -Id $ownedPid -Force
for ($attempt = 0; $attempt -lt 20 -and (Test-NetConnection 127.0.0.1 -Port 18081 -InformationLevel Quiet); $attempt += 1) { Start-Sleep -Milliseconds 250 }
if (Test-NetConnection 127.0.0.1 -Port 18081 -InformationLevel Quiet) { throw 'backend did not stop' }
$ownedPid = $null
# Capture the retry-state screenshot here.
Set-Location 'C:\Users\33141\Documents\New project\backend'
$env:HTTP_ADDR = ':18081'
$backend = Start-Process -FilePath "$root\backend-ui21b1.exe" -WorkingDirectory $PWD -RedirectStandardOutput "$acceptance\backend-restart.stdout.log" -RedirectStandardError "$acceptance\backend-restart.stderr.log" -PassThru
$ownedPid = $backend.Id
$backend.Id | Set-Content -Encoding ascii "$root\backend.pid"
$backend.Id | Set-Content -Encoding ascii "$root\backend.owned.pid"
$healthy = $false
for ($attempt = 0; $attempt -lt 40; $attempt += 1) {
  try {
    if ((Invoke-RestMethod 'http://127.0.0.1:18081/healthz' -TimeoutSec 1).code -eq 0) {
      $healthy = $true
      break
    }
  } catch { Start-Sleep -Milliseconds 500 }
}
if (-not $healthy) { throw 'backend restart health check failed' }
```

Then complete the remaining scenarios before allowing the script to enter `finally`:

```powershell
Read-Host 'Complete DevTools scenarios 6-12, save evidence, then press Enter to clean up'
```

Do not claim an empty-state PASS unless it is observed against a real empty response. If local shared data prevents a clean empty category, create a scoped category fixture/query environment or report that single item as CONDITIONAL rather than hiding the limitation.

- [ ] **Step 6: Restore and clean real-environment state**

Close the contiguous script with this real `finally`. Each cleanup action catches and records its own failure, so a fixture cleanup error cannot skip process shutdown, DevTools closure, config restoration or environment restoration:

```powershell
} finally {
  try {
    if (Test-Path 'D:\codex-temp\ui21b1\fixture.go') {
      Set-Location 'C:\Users\33141\Documents\New project\backend'
      go run 'D:\codex-temp\ui21b1\fixture.go' cleanup |
        Set-Content -Encoding utf8 'D:\codex-temp\ui21b1\acceptance\fixture-cleanup.json'
      if ($LASTEXITCODE -ne 0) { throw 'fixture cleanup command failed' }
      $cleanup = Get-Content -Raw -Encoding utf8 'D:\codex-temp\ui21b1\acceptance\fixture-cleanup.json' | ConvertFrom-Json
      if (
        $cleanup.residue.users -ne 0 -or
        $cleanup.residue.posts -ne 0 -or
        $cleanup.residue.comments -ne 0 -or
        $cleanup.residue.products -ne 0
      ) { throw 'fixture residue remains' }
    }
  } catch { $cleanupErrors.Add("fixture: $($_.Exception.Message)") }

  try {
    if ($ownedPid) {
      $ownedProcess = Get-Process -Id $ownedPid -ErrorAction SilentlyContinue
      if ($ownedProcess) {
        $expectedExe = [IO.Path]::GetFullPath('D:\codex-temp\ui21b1\backend-ui21b1.exe')
        $actualExe = [IO.Path]::GetFullPath($ownedProcess.Path)
        if (-not [StringComparer]::OrdinalIgnoreCase.Equals($expectedExe, $actualExe)) {
          throw "refusing to stop reused PID $ownedPid at $actualExe"
        }
        Stop-Process -Id $ownedPid -Force
      }
    }
    for (
      $attempt = 0;
      $attempt -lt 20 -and (Test-NetConnection 127.0.0.1 -Port 18081 -InformationLevel Quiet);
      $attempt += 1
    ) { Start-Sleep -Milliseconds 250 }
    if (Test-NetConnection 127.0.0.1 -Port 18081 -InformationLevel Quiet) {
      throw 'owned backend port residue'
    }
  } catch { $cleanupErrors.Add("backend: $($_.Exception.Message)") }

  try {
    if ($devToolsOpened) {
      & 'D:\3314\微信web开发者工具\cli.bat' close --project 'C:\Users\33141\Documents\ui21b1-community-trade-readonly\apps\miniapp'
      if ($LASTEXITCODE -ne 0) { throw 'DevTools exact-project close failed' }
    }
  } catch { $cleanupErrors.Add("devtools: $($_.Exception.Message)") }

  try {
    $worktree = 'C:\Users\33141\Documents\ui21b1-community-trade-readonly'
    $configRelative = 'apps/miniapp/project.config.json'
    git -C $worktree diff --quiet -- $configRelative
    $configExit = $LASTEXITCODE
    if ($configExit -gt 1) { throw 'project.config.json diff inspection failed' }
    if ($configExit -eq 1) {
      git -C $worktree diff -- $configRelative |
        Set-Content -Encoding utf8 'D:\codex-temp\ui21b1\acceptance\project-config-devtools-rewrite.diff'
      git -C $worktree restore -- $configRelative
      if ($LASTEXITCODE -ne 0) { throw 'project.config.json restore failed' }
    }
  } catch { $cleanupErrors.Add("project-config: $($_.Exception.Message)") }

  try {
    $runtimePaths = @(
      'D:\codex-temp\ui21b1\backend-ui21b1.exe',
      'D:\codex-temp\ui21b1\backend.pid',
      'D:\codex-temp\ui21b1\backend.owned.pid',
      'D:\codex-temp\ui21b1\fixture.go'
    )
    Remove-Item -LiteralPath $runtimePaths -Force -ErrorAction SilentlyContinue
    $survivors = @($runtimePaths | Where-Object { Test-Path -LiteralPath $_ })
    if ($survivors.Count -gt 0) { throw "runtime files remain: $($survivors -join ', ')" }
  } catch { $cleanupErrors.Add("temporary-runtime: $($_.Exception.Message)") }

  if ($null -eq $previousHTTPAddr) { Remove-Item Env:HTTP_ADDR -ErrorAction SilentlyContinue }
  else { $env:HTTP_ADDR = $previousHTTPAddr }
  if ($null -eq $previousApiBase) { Remove-Item Env:TARO_APP_API_BASE -ErrorAction SilentlyContinue }
  else { $env:TARO_APP_API_BASE = $previousApiBase }

  if ($cleanupErrors.Count -gt 0) {
    throw "acceptance cleanup failed: $($cleanupErrors -join '; ')"
  }
}
```

After all fragments and the `finally` above are present, run the guarded script once from the invoking shell and remove a surviving script file after it returns:

```powershell
$tokens = $null
$parseErrors = $null
[void][System.Management.Automation.Language.Parser]::ParseFile(
  'D:\codex-temp\ui21b1\acceptance.ps1',
  [ref]$tokens,
  [ref]$parseErrors
)
if ($parseErrors.Count -gt 0) {
  $parseErrors | Format-List | Out-String | Write-Error
  throw 'acceptance.ps1 syntax validation failed'
}
powershell -NoProfile -ExecutionPolicy Bypass -File 'D:\codex-temp\ui21b1\acceptance.ps1'
$acceptanceExit = $LASTEXITCODE
Remove-Item -LiteralPath 'D:\codex-temp\ui21b1\acceptance.ps1' -Force -ErrorAction SilentlyContinue
if (Test-Path -LiteralPath 'D:\codex-temp\ui21b1\acceptance.ps1') { throw 'acceptance.ps1 residue remains' }
if ($acceptanceExit -ne 0) { throw "acceptance.ps1 failed with exit code $acceptanceExit" }
```

- The public GET acceptance must not modify `domainA.token`; authentication storage is outside this task.
- Direct fixtures deliberately create no `trade_search_outbox` row and no Meili document; `fixture.outboxCount === 0` is a required assertion before DevTools opens.
- Close only the owned backend PID and the exact DevTools project; never kill all DevTools or an unrelated 8080 service.
- Revert only a DevTools rewrite to this worktree's `apps/miniapp/project.config.json` after saving its exact diff; do not use broad checkout/reset.
- Keep `D:/codex-temp/ui21b1/acceptance/**` JSON, screenshots and logs as external evidence. The guarded script removes and asserts absence of the executable, PID files and fixture; the invoking shell removes and asserts absence of `acceptance.ps1` after it returns.
- Re-run `git status --short` and the allowed-path whitelist.

### Task 8: Write reports, run final checks and stop for owner acceptance

**Files:**
- Create: `docs/UI-21B1-COMMUNITY-TRADE-READONLY-DEVELOPMENT.md`
- Create: `docs/UI-21B1-COMMUNITY-TRADE-READONLY-ACCEPTANCE.md`
- Create: `docs/UI-21B1-COMMUNITY-TRADE-READONLY-RESIDUE.md`

- [ ] **Step 1: Write Development report**

Record exact modified files, page-to-endpoint mapping, stable waterfall mapping, pagination concurrency, integer-cent conversion, legacy image URL handling, write-control business disablement and zero backend/API changes.

- [ ] **Step 2: Write Acceptance report**

For every automatic and real-environment scenario, record command, timestamp, HTTP route, list/detail IDs, observed state, screenshot/JSON path and PASS/CONDITIONAL result. Never upgrade a missing screenshot, untested empty state or unavailable image chain to PASS.

- [ ] **Step 3: Write Residue report**

Record inserted/deleted fixture IDs, PIDs started/stopped, ports after cleanup, token restoration, Meili cleanup, temporary evidence directory and Git cleanliness. Secrets and `.env` values must not appear.

- [ ] **Step 4: Commit reports**

```powershell
git add docs/UI-21B1-COMMUNITY-TRADE-READONLY-DEVELOPMENT.md docs/UI-21B1-COMMUNITY-TRADE-READONLY-ACCEPTANCE.md docs/UI-21B1-COMMUNITY-TRADE-READONLY-RESIDUE.md
git commit -m 'docs(UI-21b-1): 记录社区二手只读接线验收'
```

- [ ] **Step 5: Run final boundary and FF checks**

```powershell
npm run check:boundaries
npm run check:miniapp-static-routes
npm --workspace apps/miniapp run typecheck
npm run build:miniapp
npm run verify
$allowed = '^(apps/miniapp/src/services/(b-community|c-trade)\.ts|apps/miniapp/src/pages/(home|post-detail|student-secondhand|secondhand-detail)/|docs/UI-21B1-COMMUNITY-TRADE-READONLY-(DEVELOPMENT|ACCEPTANCE|RESIDUE)\.md)$'
$changed = @(git diff --name-only master...HEAD)
$unexpected = @($changed | Where-Object { $_ -notmatch $allowed })
if ($unexpected.Count -gt 0) { throw "out-of-scope files: $($unexpected -join ', ')" }
git diff --check master...HEAD
git status --short
git -C 'C:\Users\33141\Documents\New project' merge-base --is-ancestor master feat/ui21b1-community-trade-readonly
```

Expected: all gates PASS; forbidden scopes zero diff; worktree clean; branch is fast-forwardable.

- [ ] **Step 6: Stop for owner acceptance**

Report final commit hash, Development/Acceptance/Residue paths, real list-to-detail IDs, pagination, 404/error/retry evidence, Boundary, Residue and FF-OK. Do not merge, tag, remove the worktree or enter UI-21b-2; owner performs serial `--ff-only` acceptance and baseline tag.
