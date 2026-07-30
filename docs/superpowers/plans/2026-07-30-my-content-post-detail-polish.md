# “我的内容”与帖子详情页优化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复“我的内容”四类列表样式失效和帖子详情坏图大面积留白，在不改变业务接口的前提下交付可构建、可回归的双端页面。

**Architecture:** 继续使用现有 Taro React 页面和真实 API 数据，只统一页面内部的类名、列表结构和视觉层级。帖子详情增加媒体加载状态，坏图自动收起；既有审核状态、关注、点赞、收藏和评论流程保持不变。

**Tech Stack:** Taro 4、React 18、TypeScript、SCSS、Node.js 静态契约检查。

---

## 文件结构

- Modify: `scripts/check-miniapp-regression-contract.cjs`
  - 为“我的内容”类名契约、坏图收起和无图详情结构增加静态回归断言。
- Modify: `apps/miniapp/src/pages/my-content/index.tsx`
  - 保持四个数据源和路由不变，统一为紧凑媒体列表组件结构。
- Modify: `apps/miniapp/src/pages/my-content/index.scss`
  - 为实际 JSX 类名提供完整样式，覆盖列表、状态、空态和按压反馈。
- Modify: `apps/miniapp/src/pages/post-detail/index.tsx`
  - 增加首图加载失败状态，不再保留无效媒体区域。
- Modify: `apps/miniapp/src/pages/post-detail/index.scss`
  - 改为内容型详情布局，收紧媒体高度、字号、分区和底部安全区。

### Task 1: 建立页面回归契约

**Files:**
- Modify: `scripts/check-miniapp-regression-contract.cjs`

- [ ] **Step 1: 写入会失败的结构断言**

在现有 `postDetail` 断言后加入：

```js
const myContent = read('apps/miniapp/src/pages/my-content/index.tsx')
const myContentStyle = read('apps/miniapp/src/pages/my-content/index.scss')
const postDetailStyle = read('apps/miniapp/src/pages/post-detail/index.scss')

for (const className of [
  'my-content-row',
  'my-content-row__main',
  'my-content-row__title',
  'my-content-row__meta',
  'my-content-row__status'
]) {
  requireMatch(myContent, new RegExp(`className=['"\`][^'"\`]*${className}`), `我的内容 JSX 必须使用 ${className}`)
  requireMatch(myContentStyle, new RegExp(`\\.${className.replace(/__/g, '__')}(?:[\\s,{.:]|$)`), `我的内容样式必须覆盖 ${className}`)
}
requireMatch(myContent, /my-content-thumb--empty[\s\S]*无图/, '无图片内容必须显示紧凑的真实无图状态')
requireMatch(postDetail, /heroAvailable[\s\S]*onError=\{\(\) => setHeroAvailable\(false\)\}/, '帖子首图加载失败后必须收起媒体区')
requireMatch(postDetail, /showHero\s*\?[\s\S]*post-detail-hero/, '帖子详情必须只在首图有效时渲染媒体区')
forbidMatch(postDetailStyle, /post-detail-hero[\s\S]{0,240}background:\s*\$like-active/, '帖子媒体区不得恢复粉色占位背景')
```

- [ ] **Step 2: 运行断言并确认先失败**

Run:

```powershell
node scripts/check-miniapp-regression-contract.cjs
```

Expected: FAIL，首个错误为“我的内容 JSX 必须使用 my-content-row”或“帖子首图加载失败后必须收起媒体区”。

- [ ] **Step 3: 仅保存测试差异**

Run:

```powershell
git diff --check -- scripts/check-miniapp-regression-contract.cjs
```

Expected: 退出码 0，无空白错误。

### Task 2: 重构“我的内容”为紧凑媒体列表

**Files:**
- Modify: `apps/miniapp/src/pages/my-content/index.tsx`
- Modify: `apps/miniapp/src/pages/my-content/index.scss`
- Test: `scripts/check-miniapp-regression-contract.cjs`

- [ ] **Step 1: 统一列表缩略图与状态标签**

在 `index.tsx` 中保留 `reviewLabel`，新增商品状态和统一缩略图：

```tsx
function productStatusLabel(value: Product): string {
  if (value.status === 'sold') return '已售'
  if (value.status === 'reserved') return '已预订'
  return reviewLabel(value.reviewStatus)
}

function ContentThumb({ src, label = '无图' }: { src?: string; label?: string }) {
  return src
    ? <Image className='my-content-thumb' src={src} mode='aspectFill' />
    : <View className='my-content-thumb my-content-thumb--empty'><Text>{label}</Text></View>
}
```

- [ ] **Step 2: 将帖子与收藏改为统一行结构**

每条帖子使用如下结构，收藏只改变尾部文案，不改变数据或跳转：

```tsx
<View className='my-content-row' onClick={() => openPost(item.id)}>
  <ContentThumb src={item.images[0]} />
  <View className='my-content-row__main'>
    <Text className='my-content-row__title'>{item.title || item.content}</Text>
    <Text className='my-content-row__meta'>
      {favorite
        ? `${item.author || '校园用户'} · ${item.createdAt}`
        : `${reviewLabel(item.reviewStatus)} · ${item.likedCount} 赞 · ${item.commentCount} 评论`}
    </Text>
  </View>
  <View className='my-content-row__tail'>
    <Text className='my-content-row__status'>{favorite ? `赞 ${item.likedCount}` : reviewLabel(item.reviewStatus)}</Text>
    <Text className='my-content-row__chevron'>›</Text>
  </View>
</View>
```

- [ ] **Step 3: 将评论与宝贝改为统一层级**

评论保留原帖子锚点跳转，宝贝保留商品详情跳转：

```tsx
<View className='my-content-comment' onClick={() => openPost(item.postId, 'comments')}>
  <Text className='my-content-comment__content'>{item.content}</Text>
  <View className='my-content-comment__source'>
    <Text className='my-content-comment__source-title'>评论了《{item.postTitle}》</Text>
    <Text className='my-content-comment__meta'>{item.createdAt} · {reviewLabel(item.reviewStatus)}</Text>
  </View>
</View>
```

```tsx
<View className='my-content-row my-content-row--product' onClick={() => openProduct(item.id)}>
  <ContentThumb src={item.images[0]} />
  <View className='my-content-row__main'>
    <Text className='my-content-row__title'>{item.title}</Text>
    <Text className='my-content-row__price'>¥{item.price}</Text>
  </View>
  <View className='my-content-row__tail'>
    <Text className={`my-content-row__status${item.status === 'sold' ? ' my-content-row__status--muted' : ''}`}>
      {productStatusLabel(item)}
    </Text>
    <Text className='my-content-row__chevron'>›</Text>
  </View>
</View>
```

- [ ] **Step 4: 为加载、错误和空列表使用独立状态面板**

使用 `my-content-state`，错误状态保留点击重试：

```tsx
if (loading) {
  return <View className='my-content-state'><Text className='my-content-state__title'>正在加载</Text><Text className='my-content-state__desc'>正在读取你的内容…</Text></View>
}
if (error) {
  return <View className='my-content-state my-content-state--retry' onClick={() => void load()}><Text className='my-content-state__title'>内容加载失败</Text><Text className='my-content-state__desc'>{error}</Text><Text className='my-content-state__action'>点击重试</Text></View>
}
```

- [ ] **Step 5: 重写与实际 JSX 一致的 SCSS**

关键尺寸和层级必须落为以下值：

```scss
.my-content-body { padding: 20rpx 24rpx 72rpx; }
.my-content-list { display: flex; flex-direction: column; gap: 16rpx; }
.my-content-row {
  min-height: 156rpx;
  display: flex;
  align-items: center;
  gap: 20rpx;
  padding: 20rpx;
  border-radius: 24rpx;
  background: $bg-surface;
  box-shadow: $shadow-card;
}
.my-content-thumb { width: 116rpx; height: 116rpx; border-radius: 18rpx; }
.my-content-row__title {
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  font-size: 29rpx;
  line-height: 40rpx;
}
.my-content-row__meta { margin-top: 10rpx; color: $text-muted; font-size: 23rpx; line-height: 32rpx; }
.my-content-row__status { padding: 6rpx 12rpx; border-radius: 999rpx; background: $brand-tint; color: $brand-primary; font-size: 21rpx; }
.my-content-thumb--empty { background: $bg-screen; color: $text-faint; }
```

同时为 `.my-content-comment`、`.my-content-state`、`:active` 和安全区补齐样式，不再保留未被 JSX 使用的旧选择器。

- [ ] **Step 6: 运行聚焦回归和类型检查**

Run:

```powershell
node scripts/check-miniapp-regression-contract.cjs
npm.cmd --workspace apps/miniapp run typecheck
```

Expected: 两条命令退出码均为 0；回归脚本输出 `miniapp regression contract: PASS`。

### Task 3: 修复帖子详情坏图与阅读层级

**Files:**
- Modify: `apps/miniapp/src/pages/post-detail/index.tsx`
- Modify: `apps/miniapp/src/pages/post-detail/index.scss`
- Test: `scripts/check-miniapp-regression-contract.cjs`

- [ ] **Step 1: 增加首图可用状态**

在页面状态中加入并在帖子切换时重置：

```tsx
const [heroAvailable, setHeroAvailable] = useState(true)
const heroImage = post?.images[0] ?? ''
const showHero = Boolean(heroImage && heroAvailable)

useEffect(() => {
  setHeroAvailable(true)
}, [postId, heroImage])
```

- [ ] **Step 2: 仅渲染有效媒体区**

将原 `heroImage ?` 判断替换为：

```tsx
{showHero ? (
  <View className='post-detail-hero'>
    <Image
      className='post-detail-hero__image'
      src={heroImage}
      mode='aspectFill'
      onError={() => setHeroAvailable(false)}
    />
  </View>
) : null}
```

正文容器增加无媒体修饰类：

```tsx
<View className={`post-detail-content${showHero ? '' : ' post-detail-content--without-media'}`}>
```

- [ ] **Step 3: 将详情样式改为受限媒体和连续内容**

媒体、正文和评论的关键尺寸：

```scss
.post-detail-hero {
  width: 100%;
  height: 500rpx;
  max-height: 38vh;
  overflow: hidden;
  background: $bg-screen;
}
.post-detail-content { padding: 34rpx 32rpx 36rpx; background: $bg-surface; }
.post-detail-content--without-media { padding-top: 30rpx; }
.post-detail-content__title { font-size: 40rpx; line-height: 54rpx; }
.post-detail-content__body { margin-top: 22rpx; font-size: 30rpx; line-height: 48rpx; }
.post-detail-comments { margin-top: 16rpx; padding: 30rpx 32rpx; background: $bg-surface; }
.post-detail-actionbar {
  min-height: 104rpx;
  padding: 14rpx 24rpx calc(14rpx + env(safe-area-inset-bottom));
}
```

删除媒体区粉色背景、过大的固定像素字号和 116px 底栏，保留所有现有类名及审核状态配色。

- [ ] **Step 4: 运行聚焦回归**

Run:

```powershell
node scripts/check-miniapp-regression-contract.cjs
npm.cmd --workspace apps/miniapp run typecheck
```

Expected: 两条命令退出码均为 0。

### Task 4: 完整构建与边界验收

**Files:**
- Verify only: `apps/miniapp/**`
- Verify only: `scripts/check-miniapp-regression-contract.cjs`

- [ ] **Step 1: 运行小程序生产构建**

Run:

```powershell
npm.cmd --workspace apps/miniapp run build:weapp
```

Expected: Taro 构建退出码 0，生成 `apps/miniapp/dist`。

- [ ] **Step 2: 运行相关静态契约**

Run:

```powershell
npm.cmd run check:miniapp-regression-fixes
npm.cmd run check:miniapp-capsule-layout
npm.cmd run check:miniapp-static-routes
npm.cmd run check:miniapp-business-closure
```

Expected: 四条命令全部 PASS。

- [ ] **Step 3: 检查差异边界**

Run:

```powershell
git diff --check
git status --short
git diff -- apps/miniapp/src/pages/my-content/index.tsx apps/miniapp/src/pages/my-content/index.scss apps/miniapp/src/pages/post-detail/index.tsx apps/miniapp/src/pages/post-detail/index.scss scripts/check-miniapp-regression-contract.cjs
```

Expected: `git diff --check` 退出码 0；功能差异只涉及计划列出的五个文件，其他既有脏文件保持未暂存、未覆盖。

- [ ] **Step 4: 真机验收边界**

在 Android 与 iOS 体验版分别验证：

1. “我的内容”的收藏、帖子、评论、宝贝四个标签均可打开详情。
2. 有图列表显示清晰缩略图，无图列表显示小型“无图”状态。
3. 无图帖子进入详情后首屏直接看到标题与正文。
4. 坏图帖子不会留下大面积粉色或空白媒体区。
5. 帖子详情可滚动，键盘弹起时底部输入栏仍可用。
6. 点赞、星标收藏、关注、评论和审核状态保持原行为。

Expected: 代码与构建验收在本会话完成；Android/iOS 真机视觉及手势验收在未执行前明确标记为 `PENDING`，不得误报为通过。
