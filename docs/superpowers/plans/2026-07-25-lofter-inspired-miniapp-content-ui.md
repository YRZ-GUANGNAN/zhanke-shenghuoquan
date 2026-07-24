# 小程序内容优先 UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改变任何业务功能、接口、审核和权限的前提下，将小程序改造成图片优先、信息紧凑、搜索清晰的内容型视觉系统，并移除首页搜索下方的广告式占位。

**Architecture:** 新增一组学生内容端专用设计令牌和 SCSS mixin，通过现有 `CapsulePageHeader` 与 `StudentBottomNav` 保持微信胶囊和底部安全区一致。首页、搜索、帖子、二手、个人、发布和消息页使用内容型视觉；商家与骑手保留工作台结构，仅消费统一令牌并降低渐变、阴影和装饰。所有服务调用、路由、状态机和身份门禁保持不变。

**Tech Stack:** Taro 4、React 18、TypeScript、SCSS、微信小程序、Node.js 静态合同检查。

---

## 文件结构与责任

### 新增

- `apps/miniapp/src/styles/content-surfaces.scss`
  - 提供内容页、内容面、轻量标签、搜索框和底部固定栏 mixin。
  - 不包含页面业务类名。
- `scripts/check-miniapp-content-ui.cjs`
  - 静态验证首页整行搜索、无广告占位、分域搜索和学生端五入口。

### 修改：共享层

- `apps/miniapp/src/styles/design-tokens.scss`
  - 增加内容型颜色、间距、圆角、边框和阴影令牌。
- `apps/miniapp/src/app.scss`
  - 统一页面字体、基础背景、按钮和输入控件渲染。
- `apps/miniapp/src/components/capsule-page-header.scss`
  - 保留胶囊独占行，收敛业务行高度和分隔样式。
- `apps/miniapp/src/components/student-bottom-nav.scss`
  - 统一底部图标、文字和中央发布按钮。
- `package.json`
  - 注册内容 UI 静态合同检查。

### 修改：学生内容主路径

- `apps/miniapp/src/pages/home/index.tsx`
- `apps/miniapp/src/pages/home/index.scss`
- `apps/miniapp/src/pages/search/index.scss`
- `apps/miniapp/src/pages/post-detail/index.scss`
- `apps/miniapp/src/pages/student-secondhand/index.scss`
- `apps/miniapp/src/pages/secondhand-detail/index.scss`
- `apps/miniapp/src/pages/profile/index.scss`
- `apps/miniapp/src/pages/publish-entry/index.scss`
- `apps/miniapp/src/pages/publish-post/index.scss`
- `apps/miniapp/src/pages/publish-secondhand/index.scss`
- `apps/miniapp/src/pages/conversations/index.scss`
- `apps/miniapp/src/pages/chat/index.scss`

### 修改：学生交易与工具页面

- `apps/miniapp/src/pages/student-takeout/index.scss`
- `apps/miniapp/src/pages/takeout-cart/index.scss`
- `apps/miniapp/src/pages/takeout-orders/index.scss`
- `apps/miniapp/src/pages/takeout-order-detail/index.scss`
- `apps/miniapp/src/pages/product-detail/index.scss`
- `apps/miniapp/src/pages/my-content/index.scss`
- `apps/miniapp/src/pages/user-profile/index.scss`
- `apps/miniapp/src/pages/edit-profile/index.scss`
- `apps/miniapp/src/pages/settings/index.scss`
- `apps/miniapp/src/pages/notifications/index.scss`

### 修改：工作台视觉收敛

- `apps/miniapp/src/pages/merchant-home/index.scss`
- `apps/miniapp/src/pages/delivery-rider/index.scss`

### 明确不修改

- `apps/miniapp/src/services/**`
- `backend/**`
- 数据库迁移
- 身份审核、订单状态机、支付开关
- 当前已修改的 `apps/miniapp/src/pages/merchant-publish/index.tsx`
- `.env.example`、`.gitignore`、项目私有配置、部署文件和现有未提交文档

---

### Task 1: 建立内容 UI 静态合同和工作区保护

**Files:**
- Create: `scripts/check-miniapp-content-ui.cjs`
- Modify: `package.json`
- Test: `scripts/check-miniapp-content-ui.cjs`

- [ ] **Step 1: 记录实施基线和受保护文件**

Run:

```powershell
git rev-parse HEAD
git status --short
git diff -- apps/miniapp/src/pages/merchant-publish/index.tsx
```

Expected:

- HEAD 以设计提交 `949c154` 或其后继提交为基线。
- 输出当前所有用户未提交文件。
- `merchant-publish/index.tsx` 的现有改动只记录、不暂存、不覆盖。

- [ ] **Step 2: 写入会先失败的静态合同**

Create `scripts/check-miniapp-content-ui.cjs`:

```js
const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const read = (relativePath) => fs.readFileSync(path.join(root, relativePath), 'utf8')

const home = read('apps/miniapp/src/pages/home/index.tsx')
const search = read('apps/miniapp/src/pages/search/index.tsx')
const nav = read('apps/miniapp/src/components/student-bottom-nav.tsx')

function mustInclude(source, value, message) {
  if (!source.includes(value)) throw new Error(message)
}

function mustExclude(source, pattern, message) {
  if (pattern.test(source)) throw new Error(message)
}

mustInclude(home, "className='student-home-search-row'", '首页缺少整行搜索区域')
mustInclude(home, "className='student-home-search'", '首页缺少内容型搜索框')
mustInclude(home, "/pages/search/index?scope=community", '首页搜索必须进入帖子领域')
mustInclude(home, 'student-post-card--text', '首页纯文字帖子必须使用文字卡')
mustExclude(home, /student-home-(?:banner|advert|ad-slot|promo)/i, '首页不得包含广告式占位')

for (const scope of ['community', 'secondhand', 'takeout']) {
  mustInclude(search, `${scope}:`, `搜索页缺少 ${scope} 分域配置`)
}
mustInclude(search, "type SearchScope = 'community' | 'secondhand' | 'takeout'", '搜索领域不得合并')

for (const route of [
  '/pages/home/index',
  '/pages/student-takeout/index',
  '/pages/publish-entry/index',
  '/pages/student-secondhand/index',
  '/pages/profile/index',
]) {
  mustInclude(nav, route, `学生端导航缺少 ${route}`)
}

console.log('miniapp content UI contract ok')
```

- [ ] **Step 3: 注册检查命令**

Add to the root `package.json` scripts object immediately after `check:miniapp-capsule-layout`:

```json
"check:miniapp-content-ui": "node scripts/check-miniapp-content-ui.cjs",
```

Add it to `verify` immediately after `check:miniapp-capsule-layout`:

```json
"verify": "npm run check:external-takeout-directory && npm run check:miniapp-static-routes && npm run check:miniapp-business-closure && npm run check:boundaries && npm run check:miniapp-publish-contract && npm run check:miniapp-domain-search && npm run check:miniapp-settings-contract && npm run check:identity-application-contract && npm run check:miniapp-capsule-layout && npm run check:miniapp-content-ui && npm run check:home-real-tools-contract && npm run check:real-wechat-secret-hygiene && npm run check:merchant-home-clarity && npm run check:rider-workbench-clarity && npm run typecheck && npm run test:backend"
```

- [ ] **Step 4: 运行检查并确认它因新 UI 尚未实现而失败**

Run:

```powershell
npm run check:miniapp-content-ui
```

Expected: FAIL with `首页缺少整行搜索区域`.

- [ ] **Step 5: 检查本任务差异边界**

Run:

```powershell
git diff --check -- package.json scripts/check-miniapp-content-ui.cjs
git status --short
```

Expected: only the new check and the planned `package.json` edit are added by this task; protected dirty files remain unchanged.

---

### Task 2: 建立共享内容视觉令牌和表面 mixin

**Files:**
- Create: `apps/miniapp/src/styles/content-surfaces.scss`
- Modify: `apps/miniapp/src/styles/design-tokens.scss`
- Modify: `apps/miniapp/src/app.scss`
- Modify: `apps/miniapp/src/components/capsule-page-header.scss`
- Modify: `apps/miniapp/src/components/student-bottom-nav.scss`
- Test: miniapp TypeScript and Sass build

- [ ] **Step 1: 增加内容型设计令牌**

Append before the `page` CSS custom-property block in `design-tokens.scss`:

```scss
$content-canvas: #f5f6f7;
$content-surface: #ffffff;
$content-ink: #202327;
$content-ink-soft: #50565f;
$content-muted: #8d949d;
$content-faint: #b8bdc4;
$content-accent: #2f8f9d;
$content-accent-soft: #e8f4f4;
$content-border: #eceff1;
$content-input: #f3f4f5;
$content-like: #ec6a86;

$content-radius-small: 12rpx;
$content-radius-medium: 20rpx;
$content-radius-large: 28rpx;
$content-shadow: 0 2rpx 10rpx rgba(24, 32, 40, .035);
```

Add matching custom properties inside `page`:

```scss
--content-canvas: #f5f6f7;
--content-surface: #ffffff;
--content-ink: #202327;
--content-ink-soft: #50565f;
--content-muted: #8d949d;
--content-accent: #2f8f9d;
--content-border: #eceff1;
```

- [ ] **Step 2: 创建可复用表面 mixin**

Create `content-surfaces.scss`:

```scss
@use './design-tokens.scss' as *;

@mixin content-page {
  background: $content-canvas;
  color: $content-ink;
  font-family: -apple-system, BlinkMacSystemFont, "PingFang SC", "Microsoft YaHei", system-ui, sans-serif;
}

@mixin content-surface($radius: $content-radius-medium) {
  border: 1rpx solid $content-border;
  border-radius: $radius;
  background: $content-surface;
  box-shadow: $content-shadow;
}

@mixin content-search {
  display: flex;
  align-items: center;
  min-height: 72rpx;
  border-radius: 999rpx;
  background: $content-input;
  color: $content-muted;
}

@mixin content-pill {
  display: inline-flex;
  align-items: center;
  min-height: 42rpx;
  padding: 0 16rpx;
  border-radius: 999rpx;
  background: $content-input;
  color: $content-muted;
  font-size: 22rpx;
}

@mixin content-fixed-bottom {
  padding-bottom: env(safe-area-inset-bottom);
  border-top: 1rpx solid $content-border;
  background: rgba(255, 255, 255, .98);
}
```

- [ ] **Step 3: 收敛全局基础渲染**

Replace only the `page` block at the start of `app.scss` with:

```scss
page {
  min-height: 100%;
  background: #f5f6f7;
  color: #202327;
  font-family: -apple-system, BlinkMacSystemFont, "PingFang SC", "Microsoft YaHei", system-ui, sans-serif;
  -webkit-font-smoothing: antialiased;
  text-rendering: optimizeLegibility;
}
```

Add to the existing form-control selector:

```scss
button,
input,
textarea {
  font: inherit;
}
```

- [ ] **Step 4: 保持胶囊独占行并降低头部视觉噪声**

In `capsule-page-header.scss`, preserve all safe-area calculations and update the business row and divider to:

```scss
.capsule-page-header__business-row {
  min-height: 88rpx;
  padding-right: 30rpx;
  padding-left: 30rpx;
}

.capsule-page-header--divider {
  border-bottom: 1rpx solid $content-border;
  background: rgba(255, 255, 255, .98);
}
```

Do not change `.capsule-page-header__system-row` or `getCapsuleSafeStyle()`.

- [ ] **Step 5: 收敛学生底部导航**

Replace the visual values in `student-bottom-nav.scss` with:

```scss
.student-bottom-nav {
  min-height: 112rpx;
  padding: 12rpx 14rpx calc(24rpx + env(safe-area-inset-bottom));
  border-top: 1rpx solid $content-border;
  background: rgba(255, 255, 255, .98);
  box-shadow: 0 -4rpx 18rpx rgba(24, 32, 40, .035);
}

.student-bottom-nav__item {
  gap: 4rpx;
  color: $content-faint;
  font-size: 20rpx;
  font-weight: 500;
}

.student-bottom-nav__item--active {
  color: $content-ink;
  font-weight: 700;
}

.student-bottom-nav__fab {
  width: 88rpx;
  height: 88rpx;
  margin-top: -20rpx;
  border-radius: 24rpx;
  background: $content-ink;
  box-shadow: 0 10rpx 24rpx rgba(24, 32, 40, .18);
}
```

Keep the existing `AppIcon` assets and all click handlers.

- [ ] **Step 6: 验证共享层可编译**

Run:

```powershell
npm --workspace apps/miniapp run typecheck
npm run build:miniapp
```

Expected: both commands exit 0; the UI contract still fails only because the homepage task has not run.

---

### Task 3: 改造首页标签、整行搜索和瀑布流

**Files:**
- Modify: `apps/miniapp/src/pages/home/index.tsx`
- Modify: `apps/miniapp/src/pages/home/index.scss`
- Test: `scripts/check-miniapp-content-ui.cjs`

- [ ] **Step 1: 为二手卡补充已有价格展示字段**

Extend `DiscoverCard` in `home/index.tsx`:

```ts
type DiscoverCard = {
  id: string
  target: 'post' | 'secondhand'
  tone: 'peach' | 'sky' | 'mint' | 'lilac' | 'lemon' | 'cyan'
  mediaLabel: string
  title: string
  author: string
  avatar: string
  likes: number
  height: 'short' | 'medium' | 'tall'
  imageUrl?: string
  price?: string
}
```

Add to `toProductCard`:

```ts
price: String(product.price),
```

No service or API change is allowed.

- [ ] **Step 2: 将顶部搜索图标替换为整行搜索**

Remove the `right` search icon prop from `CapsulePageHeader` and insert immediately after the header:

```tsx
{activeTab === 'discover' && (
  <View className='student-home-search-row'>
    <View className='student-home-search' onClick={openSearch}>
      <View className='student-home-search__icon' />
      <Text className='student-home-search__placeholder'>搜帖子、话题和校园动态</Text>
    </View>
  </View>
)}
```

Keep `openSearch()` exactly routed to:

```ts
void Taro.navigateTo({ url: '/pages/search/index?scope=community' })
```

- [ ] **Step 3: 区分图片卡和纯文字卡**

Replace `renderDiscoverCard` with markup that keeps the existing click and image-error behavior, using these class rules:

```tsx
const renderDiscoverCard = (card: DiscoverCard) => {
  const imageKey = `${card.target}:${card.id}`
  const hasMedia = Boolean(card.imageUrl)
  return (
    <View
      key={imageKey}
      className={`student-post-card ${hasMedia ? 'student-post-card--media' : 'student-post-card--text'}`}
      onClick={() => openCard(card)}
    >
      {hasMedia && (
        <View className={`student-post-card__media student-post-card__media--${card.height}`}>
          {!failedImageKeys.has(imageKey) ? (
            <Image
              className='student-post-card__image'
              src={card.imageUrl!}
              mode='aspectFill'
              onError={() => setFailedImageKeys((current) => {
                const next = new Set(current)
                next.add(imageKey)
                return next
              })}
            />
          ) : (
            <Text className='student-post-card__media-fallback'>{card.mediaLabel}</Text>
          )}
        </View>
      )}
      <View className='student-post-card__body'>
        <View className='student-post-card__heading'>
          <Text className='student-post-card__title'>{card.title}</Text>
          <Text className='student-post-card__more'>•••</Text>
        </View>
        <View className='student-post-card__tags'>
          <Text>{card.target === 'secondhand' ? '#二手' : '#校园动态'}</Text>
          {card.price && <Text className='student-post-card__price'>¥{card.price}</Text>}
        </View>
        <View className='student-post-card__meta'>
          <View className={`student-post-card__avatar student-post-card__avatar--${card.tone}`}>
            <Text>{card.avatar}</Text>
          </View>
          <Text className='student-post-card__author'>{card.author}</Text>
          <View className='student-post-card__like'>
            <Text className='student-post-card__heart'>♡</Text>
            <Text>{card.likes}</Text>
          </View>
        </View>
      </View>
    </View>
  )
}
```

- [ ] **Step 4: 重写首页内容型布局**

At the top of `home/index.scss`, add:

```scss
@use '../../styles/content-surfaces.scss' as content;
```

Use these target values for the main layout:

```scss
.student-home-page {
  @include content.content-page;
}

.student-home-header {
  flex: 0 0 auto;
  box-shadow: none;
}

.student-home-search-row {
  flex: 0 0 auto;
  padding: 14rpx 28rpx 18rpx;
  background: $content-surface;
}

.student-home-search {
  @include content.content-search;
  gap: 18rpx;
  padding: 0 26rpx;
}

.student-home-search__placeholder {
  color: $content-muted;
  font-size: 26rpx;
}

.student-waterfall {
  gap: 16rpx;
  padding: 18rpx 20rpx 20rpx;
}

.student-waterfall__column {
  gap: 16rpx;
}

.student-post-card {
  overflow: hidden;
  border: 0;
  border-radius: $content-radius-medium;
  background: $content-surface;
  box-shadow: none;
}

.student-post-card--text {
  border: 1rpx solid $content-border;
  background: #f7f7f7;
}

.student-post-card__body {
  padding: 16rpx 16rpx 18rpx;
}

.student-post-card__title {
  color: $content-ink;
  font-size: 27rpx;
  font-weight: 650;
  line-height: 1.45;
}

.student-post-card__tags {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 12rpx;
  color: $content-muted;
  font-size: 22rpx;
}

.student-post-card__price {
  color: $content-accent;
  font-size: 25rpx;
  font-weight: 700;
}
```

Keep media heights, real image rendering, message loading, tool loading and the infinite-scroll sentinel.

- [ ] **Step 5: 运行首页静态合同**

Run:

```powershell
npm run check:miniapp-content-ui
npm run check:home-real-tools-contract
npm run check:miniapp-capsule-layout
```

Expected: all three commands exit 0.

---

### Task 4: 将搜索页统一为参考图式整行搜索

**Files:**
- Modify: `apps/miniapp/src/pages/search/index.scss`
- Test: `scripts/check-miniapp-domain-search.cjs`

- [ ] **Step 1: 保持分域行为原样**

Verify before editing:

```powershell
rg -n "type SearchScope|searchCommunityPosts|searchTradeProducts|searchExternalTakeoutMerchants" apps/miniapp/src/pages/search/index.tsx
```

Expected: `community`、`secondhand`、`takeout` each map to their existing service.

- [ ] **Step 2: 应用内容型搜索视觉**

Add the content mixin import and replace the main surface blocks:

```scss
@use '../../styles/content-surfaces.scss' as content;

.student-search-page {
  @include content.content-page;
  min-height: 100vh;
}

.student-search-header {
  background: $content-surface;
  box-shadow: none;
}

.student-search-box {
  @include content.content-search;
  width: 100%;
  padding: 0 24rpx;
}

.student-search-box__input {
  flex: 1 1 auto;
  min-width: 0;
  color: $content-ink;
  font-size: 27rpx;
}

.student-search-content {
  padding: 24rpx 28rpx 80rpx;
}

.student-search-chip {
  @include content.content-pill;
}

.student-search-result {
  padding: 24rpx 0;
  border-bottom: 1rpx solid $content-border;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
}

.student-search-result__title {
  color: $content-ink;
  font-size: 30rpx;
  font-weight: 650;
}

.student-search-result__summary {
  margin-top: 10rpx;
  color: $content-ink-soft;
  font-size: 25rpx;
  line-height: 1.55;
}
```

Do not add a banner, recommendation ad, image carousel or extra top spacer.

- [ ] **Step 3: 验证分域搜索合同**

Run:

```powershell
npm run check:miniapp-domain-search
npm run check:miniapp-content-ui
npm --workspace apps/miniapp run typecheck
```

Expected: all commands exit 0.

---

### Task 5: 统一帖子详情和二手主路径

**Files:**
- Modify: `apps/miniapp/src/pages/post-detail/index.scss`
- Modify: `apps/miniapp/src/pages/student-secondhand/index.scss`
- Modify: `apps/miniapp/src/pages/secondhand-detail/index.scss`
- Test: miniapp typecheck and publish contract

- [ ] **Step 1: 将帖子详情改为连续文章阅读面**

Add the content mixin import to `post-detail/index.scss` and use:

```scss
.post-detail-page {
  @include content.content-page;
  background: $content-surface;
}

.post-detail-scroll {
  background: $content-surface;
}

.post-detail-content,
.post-detail-comments {
  background: $content-surface;
}

.post-detail-content {
  padding: 32rpx 32rpx 20rpx;
}

.post-detail-content__title {
  color: $content-ink;
  font-size: 42rpx;
  font-weight: 700;
  line-height: 1.35;
}

.post-detail-content__body {
  margin-top: 22rpx;
  color: $content-ink-soft;
  font-size: 30rpx;
  line-height: 1.75;
}

.post-detail-comment {
  border-bottom: 1rpx solid $content-border;
}

.post-detail-actionbar {
  @include content.content-fixed-bottom;
  box-shadow: 0 -4rpx 18rpx rgba(24, 32, 40, .04);
}
```

Keep all existing comment, like, favorite and follow handlers unchanged.

- [ ] **Step 2: 收敛二手列表卡片**

In `student-secondhand/index.scss`, import the content mixin and update:

```scss
.student-secondhand-page {
  @include content.content-page;
}

.student-secondhand-search {
  @include content.content-search;
}

.student-secondhand-card {
  overflow: hidden;
  border: 0;
  border-radius: $content-radius-medium;
  background: $content-surface;
  box-shadow: none;
}

.student-secondhand-card__title {
  color: $content-ink;
  font-weight: 650;
}

.student-secondhand-card__price {
  color: $content-accent;
  font-weight: 750;
}
```

Keep category switching, pagination, status and detail routing unchanged.

- [ ] **Step 3: 收敛二手详情层级**

In `secondhand-detail/index.scss`, import the content mixin and update:

```scss
.secondhand-detail-page,
.secondhand-detail-scroll {
  @include content.content-page;
}

.secondhand-detail-info {
  padding: 30rpx 28rpx;
  border-bottom: 1rpx solid $content-border;
  background: $content-surface;
}

.secondhand-detail-info__price {
  color: $content-accent;
  font-size: 54rpx;
}

.secondhand-detail-seller {
  @include content.content-surface($content-radius-medium);
  box-shadow: none;
}

.secondhand-detail-warning {
  border: 1rpx solid rgba($badge-unread, .16);
  background: rgba($badge-unread, .055);
}

.secondhand-detail-actionbar {
  @include content.content-fixed-bottom;
}
```

Keep favorite, contact, seller profile and chat-sheet actions unchanged.

- [ ] **Step 4: 验证主内容功能合同**

Run:

```powershell
npm run check:miniapp-publish-contract
npm run check:miniapp-business-closure
npm --workspace apps/miniapp run typecheck
```

Expected: all commands exit 0.

---

### Task 6: 统一个人、发布和私信信息层级

**Files:**
- Modify: `apps/miniapp/src/pages/profile/index.scss`
- Modify: `apps/miniapp/src/pages/publish-entry/index.scss`
- Modify: `apps/miniapp/src/pages/publish-post/index.scss`
- Modify: `apps/miniapp/src/pages/publish-secondhand/index.scss`
- Modify: `apps/miniapp/src/pages/conversations/index.scss`
- Modify: `apps/miniapp/src/pages/chat/index.scss`
- Modify: `apps/miniapp/src/pages/edit-profile/index.scss`
- Test: identity and capsule checks

- [ ] **Step 1: 将个人页从大渐变头图收敛为资料头部**

In `profile/index.scss`, import the content mixin and replace the hero visual values:

```scss
.student-profile-page {
  @include content.content-page;
}

.student-profile-hero {
  min-height: 300rpx;
  background: $content-surface;
  color: $content-ink;
}

.student-profile-hero__orb,
.student-profile-hero::after {
  display: none;
}

.student-profile-user__badge {
  background: $content-accent-soft;
  color: $content-accent;
}

.student-profile-stats,
.student-profile-card {
  @include content.content-surface($content-radius-medium);
  box-shadow: none;
}
```

Keep `CapsuleSystemRow`, profile editing, statistics, identity options and `IdentitySwitchModal`.

- [ ] **Step 2: 统一发布页面表单**

In `publish-entry/index.scss`, import `content-surfaces.scss` and apply:

```scss
.publish-entry-page {
  @include content.content-page;
}

.publish-entry-card {
  @include content.content-surface($content-radius-medium);
  box-shadow: none;
}
```

In `publish-post/index.scss`, import the same mixin and apply:

```scss
.publish-post-page {
  @include content.content-page;
}

.publish-post-body {
  background: $content-surface;
}

.publish-post-nav__publish--active {
  border-radius: 18rpx;
  background: $content-ink;
  color: $content-surface;
  box-shadow: none;
}
```

In `publish-secondhand/index.scss`, import the same mixin and apply:

```scss
.publish-secondhand-page {
  @include content.content-page;
}

.publish-secondhand-upload-card,
.publish-secondhand-form-card {
  @include content.content-surface($content-radius-medium);
  box-shadow: none;
}

.publish-secondhand-footer__button {
  border-radius: 18rpx;
  background: $content-ink;
  color: $content-surface;
  box-shadow: none;
}
```

Do not change TSX event handlers, upload services or form fields.

- [ ] **Step 3: 统一私信列表和聊天**

Apply:

```scss
.conversation-page,
.chat-page {
  @include content.content-page;
}

.conversation-row {
  border-bottom: 1rpx solid $content-border;
  background: $content-surface;
  box-shadow: none;
}

.chat-compose {
  @include content.content-fixed-bottom;
}

.chat-compose input {
  border-radius: 999rpx;
  background: $content-input;
}
```

Keep Tencent IM and image-send handlers unchanged.

- [ ] **Step 4: 统一资料编辑页并保护胶囊点击区**

In `edit-profile/index.scss`:

```scss
.edit-profile-page {
  @include content.content-page;
}

.edit-profile-card,
.edit-profile-signature-card {
  @include content.content-surface($content-radius-medium);
  box-shadow: none;
}

.edit-profile-nav__action,
.edit-profile-nav__save {
  min-width: 88rpx;
  min-height: 72rpx;
}
```

Do not move save/cancel into the capsule system row.

- [ ] **Step 5: 验证身份和胶囊边界**

Run:

```powershell
npm run check:identity-application-contract
npm run check:miniapp-capsule-layout
npm --workspace apps/miniapp run typecheck
```

Expected: all commands exit 0.

---

### Task 7: 统一学生交易、资料与设置页面

**Files:**
- Modify: `apps/miniapp/src/pages/student-takeout/index.scss`
- Modify: `apps/miniapp/src/pages/takeout-cart/index.scss`
- Modify: `apps/miniapp/src/pages/takeout-orders/index.scss`
- Modify: `apps/miniapp/src/pages/takeout-order-detail/index.scss`
- Modify: `apps/miniapp/src/pages/product-detail/index.scss`
- Modify: `apps/miniapp/src/pages/my-content/index.scss`
- Modify: `apps/miniapp/src/pages/user-profile/index.scss`
- Modify: `apps/miniapp/src/pages/settings/index.scss`
- Modify: `apps/miniapp/src/pages/notifications/index.scss`
- Test: static route and business closure checks

- [ ] **Step 1: 对交易列表应用内容面规则**

In `student-takeout/index.scss`, import the shared mixin and apply:

```scss
.student-takeout-page {
  @include content.content-page;
}

.student-takeout-merchant {
  @include content.content-surface($content-radius-medium);
  box-shadow: none;
}

.student-takeout-header__title,
.student-takeout-merchant__name {
  color: $content-ink;
  font-weight: 700;
}
```

In `takeout-cart/index.scss`, apply:

```scss
.takeout-cart-page {
  @include content.content-page;
}

.cart-card,
.cart-item {
  @include content.content-surface($content-radius-medium);
  box-shadow: none;
}

.cart-title,
.cart-item__name {
  color: $content-ink;
}
```

In `takeout-orders/index.scss`, apply:

```scss
.takeout-orders-page {
  @include content.content-page;
}

.order-card {
  @include content.content-surface($content-radius-medium);
  box-shadow: none;
}

.order-title {
  color: $content-ink;
}
```

In `my-content/index.scss`, apply:

```scss
.my-content-page {
  @include content.content-page;
}

.my-content-card {
  @include content.content-surface($content-radius-medium);
  box-shadow: none;
}

.my-content-favorite__title,
.my-content-post__title,
.my-content-market__title {
  color: $content-ink;
}
```

Only SCSS changes are allowed.

- [ ] **Step 2: 对交易详情应用连续信息层级**

In `takeout-order-detail/index.scss`, import the shared mixin and apply:

```scss
.takeout-order-detail-page {
  @include content.content-page;
}

.detail-card {
  border-bottom: 1rpx solid $content-border;
  border-radius: $content-radius-medium;
  background: $content-surface;
  box-shadow: none;
}
```

In `product-detail/index.scss`, apply:

```scss
.product-detail-page {
  @include content.content-page;
}

.product-info,
.seller-card {
  border: 1rpx solid $content-border;
  border-radius: $content-radius-medium;
  background: $content-surface;
  box-shadow: none;
}

.product-actions .cc-button--primary {
  border-radius: 18rpx;
  background: $content-ink;
  color: $content-surface;
  box-shadow: none;
}
```

Do not change order or product data access.

- [ ] **Step 3: 统一用户资料、设置和通知**

In `user-profile/index.scss`, import the shared mixin and apply:

```scss
.public-profile-page {
  @include content.content-page;
}

.public-profile-card {
  border: 1rpx solid $content-border;
  border-radius: $content-radius-medium;
  background: $content-surface;
  box-shadow: none;
}
```

In `settings/index.scss`, apply:

```scss
.student-settings-page {
  @include content.content-page;
}

.student-settings-card {
  border: 1rpx solid $content-border;
  border-radius: $content-radius-medium;
  background: $content-surface;
  box-shadow: none;
}
```

In `notifications/index.scss`, apply:

```scss
.notifications-page {
  @include content.content-page;
}

.notification {
  border-bottom: 1rpx solid $content-border;
  background: $content-surface;
  box-shadow: none;
}
```

Preserve logout, blacklist, legal, feedback and notification routing.

- [ ] **Step 4: 验证静态路由和学生业务闭环**

Run:

```powershell
npm run check:miniapp-static-routes
npm run check:miniapp-business-closure
npm --workspace apps/miniapp run typecheck
```

Expected: all commands exit 0.

---

### Task 8: 收敛商家和骑手工作台视觉但不改结构

**Files:**
- Modify: `apps/miniapp/src/pages/merchant-home/index.scss`
- Modify: `apps/miniapp/src/pages/delivery-rider/index.scss`
- Test: merchant and rider clarity checks

- [ ] **Step 1: 降低商家头部和卡片装饰**

Use these values in `merchant-home/index.scss`:

```scss
.merchant-home-hero {
  background: linear-gradient(155deg, #286fae 0%, #347fb9 100%);
}

.merchant-home-shortcut,
.merchant-order-card {
  border: 1rpx solid $content-border;
  box-shadow: $content-shadow;
}

.merchant-home-shortcut__icon {
  background: rgba($content-accent, .08);
}
```

Keep shop identity, wallet, four shortcuts, three order tabs and all routes.

- [ ] **Step 2: 降低骑手头部和订单卡装饰**

Use these values in `delivery-rider/index.scss`:

```scss
.rider-grab-page,
.rider-grab-content {
  background: $content-canvas;
}

.rider-grab-hero {
  background: linear-gradient(155deg, #286fae 0%, #347fb9 100%);
}

.rider-grab-tabs,
.rider-order-card,
.rider-empty-state {
  border: 1rpx solid $content-border;
  box-shadow: $content-shadow;
}
```

Keep rider identity, refresh, three task tabs, all order actions, phone, message and proof upload.

- [ ] **Step 3: 运行工作台合同**

Run:

```powershell
npm run check:merchant-home-clarity
npm run check:rider-workbench-clarity
npm run check:identity-application-contract
```

Expected: all commands exit 0.

---

### Task 9: 完成自动验证和微信开发者工具验收

**Files:**
- Modify only files already listed if verification exposes a scoped UI defect
- Test: full miniapp verification ladder

- [ ] **Step 1: 扫描广告占位和字符图标回归**

Run:

```powershell
rg -n "banner|advert|ad-slot|promo" apps/miniapp/src/pages/home apps/miniapp/src/pages/search
rg -n ">[+¥账聊骑C]<" apps/miniapp/src/pages apps/miniapp/src/components
```

Expected:

- First command returns no homepage/search advertising placeholder.
- Second command returns no newly introduced text-character icons; any pre-existing match is inspected and excluded from this implementation commit unless it lies in a touched UI file.

- [ ] **Step 2: 运行完整静态和类型验证**

Run:

```powershell
npm run check:miniapp-content-ui
npm run check:miniapp-domain-search
npm run check:miniapp-capsule-layout
npm run check:home-real-tools-contract
npm run check:miniapp-publish-contract
npm run check:miniapp-business-closure
npm run check:identity-application-contract
npm run check:merchant-home-clarity
npm run check:rider-workbench-clarity
npm --workspace apps/miniapp run typecheck
```

Expected: every command exits 0.

- [ ] **Step 3: 构建微信小程序**

Run:

```powershell
$env:TARO_APP_API_BASE='https://zhanduiwang.top/api/v1'
$env:TARO_APP_ALLOW_MOCK_DELIVERY_FEE_CONFIRMATION='false'
$env:TARO_APP_ALLOW_MOCK_DELIVERY_PAYMENT='false'
npm run build:miniapp
```

Expected: exit 0 and update `apps/miniapp/dist` without TypeScript or Sass errors.

- [ ] **Step 4: 在微信开发者工具逐页验收**

Check in this exact order:

1. 首页胶囊安全行、标签、整行搜索和无广告间隙。
2. 首页图片卡、纯文字卡、二手价格卡、加载更多和部分失败。
3. 帖子搜索、二手搜索、外卖搜索各自只显示本领域结果。
4. 帖子详情、评论输入、点赞、收藏。
5. 二手列表、详情、收藏、联系卖家和私聊。
6. 发布入口、发帖、发布二手、上传图片。
7. 私信列表、聊天文字和图片按钮。
8. 个人资料、编辑资料、身份切换和未审核门禁。
9. 审核通过的商家和骑手工作台。

Expected: no overlap, no white screen, no missing action, no route change, no capsule conflict.

- [ ] **Step 5: 检查窄屏与安全区**

Use iPhone 15 Pro and one width at or below 360 CSS pixels.

Expected:

- Waterfall columns do not overlap.
- Search placeholder does not collide with buttons.
- Bottom actions do not cover the last content row.
- Capsule row has no custom click target.

---

### Task 10: 生成单一可回退 UI 提交

**Files:**
- Stage only the files listed by this plan
- Never stage the protected dirty files

- [ ] **Step 1: 审计最终差异**

Run:

```powershell
git status --short
git diff --check
git diff --name-only
```

Expected: UI files are distinguishable from pre-existing user changes.

- [ ] **Step 2: 明确排除受保护文件**

Verify these are not staged:

```powershell
git diff --cached --name-only | Select-String -Pattern '^\\.env|^\\.gitignore|project\\.config|project\\.private|backend/|deploy/|merchant-publish/index\\.tsx|G-7-QUICKSTART'
```

Expected: no output.

- [ ] **Step 3: 只暂存本计划 UI 文件**

Run `git add --` with the exact verified UI file list from `git diff --name-only`. Do not use `git add .` or `git add -A`.

Expected: `git diff --cached --name-only` contains only:

- shared content UI files,
- planned student page SCSS/TSX,
- merchant/rider SCSS,
- `scripts/check-miniapp-content-ui.cjs`,
- root `package.json`.

- [ ] **Step 4: 检查暂存内容并提交**

Run:

```powershell
git diff --cached --check
git diff --cached --stat
git commit -m "界面：统一小程序内容优先信息流"
```

Expected: one implementation commit is created.

- [ ] **Step 5: 记录回退命令**

Run:

```powershell
git rev-parse HEAD
```

Record the output as `<UI_COMMIT>`. Rollback command:

```powershell
git revert <UI_COMMIT>
```

This rollback must leave design commit `949c154` and all pre-existing user changes intact.

---

## 计划自审

### 规格覆盖

- 首页整行搜索、无广告占位：Task 1、Task 3、Task 9。
- 分域搜索：Task 1、Task 4。
- 双列图片优先和纯文字卡：Task 3。
- 帖子和评论详情：Task 5。
- 二手列表、价格、卖家和联系：Task 5。
- 发布、私信、个人资料和身份切换：Task 6。
- 学生交易、资料、设置与通知：Task 7。
- 商家和骑手保留工作台结构：Task 8。
- 胶囊、安全区、空状态和错误状态：Task 2、Task 3、Task 5、Task 6、Task 9。
- 自动验证、真机边界和一键回退：Task 9、Task 10。

### 类型与接口一致性

- `SearchScope` 继续是 `community | secondhand | takeout`。
- `DiscoverCard.price` 是可选字符串，只用于展示现有 `Product.price`。
- `openSearch()`、`openCard()`、服务调用和身份门禁签名不变。
- 新增内容令牌只供 SCSS 使用，不进入运行时数据。

### 提交边界

- 设计规格已独立提交为 `949c154`。
- 实施只产生一个可整体回退的 UI 提交。
- 现有 `merchant-publish/index.tsx`、环境、部署、项目配置和快速启动文档改动不进入 UI 提交。
