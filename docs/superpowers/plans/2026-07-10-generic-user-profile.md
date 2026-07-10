# Generic User Profile Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build one reusable mini-program personal homepage, reached from “我的”、帖子作者和二手卖家, with self/other actions plus post and second-hand tabs.

**Architecture:** Add a route-driven `pages/user-profile` page whose rendering consumes a `PublicProfileView` returned by a separate static data adapter. Keep self/other permissions in the view model, keep transient interactions local to the page, and leave backend contracts untouched until public profile and per-user content APIs exist.

**Tech Stack:** Taro 4.2, React 18, TypeScript 5.4, SCSS, existing `services/taro` facade.

---

## File Structure

- Create `apps/miniapp/src/pages/user-profile/data.ts`: public-profile view types, fixtures keyed by stable `userId`, and fallback resolver.
- Create `apps/miniapp/src/pages/user-profile/index.tsx`: route parsing, self/other action policy, tabs, waterfall rendering, navigation, and action sheet state.
- Create `apps/miniapp/src/pages/user-profile/index.scss`: full-screen responsive layout and stable card/action-sheet dimensions.
- Create `apps/miniapp/src/pages/user-profile/index.config.ts`: custom navigation configuration.
- Modify `apps/miniapp/src/app.config.ts`: register the page.
- Modify `apps/miniapp/src/pages/profile/index.tsx`: replace “编辑资料” with the self-profile entry.
- Modify `apps/miniapp/src/pages/profile/index.scss`: rename the entry selector and fit “主页 ›”.
- Modify `apps/miniapp/src/pages/post-detail/index.tsx`: make the author open `userId=milk-tea`.
- Modify `apps/miniapp/src/pages/secondhand-detail/index.tsx`: make the seller button open `userId=north` on the second-hand tab.

The miniapp has no frontend unit-test runner. Verification therefore uses TypeScript compilation, Taro production build, route/source assertions, generated page artifacts, and clean-diff checks.

### Task 1: Add the Generic Profile Data Contract

**Files:**
- Create: `apps/miniapp/src/pages/user-profile/data.ts`

- [ ] **Step 1: Confirm the contract does not already exist**

Run:

```powershell
rg -n "PublicProfileView|resolvePublicProfile" apps/miniapp/src
```

Expected: exit code 1 and no matches.

- [ ] **Step 2: Create the view types and route-keyed fixture adapter**

Create `apps/miniapp/src/pages/user-profile/data.ts` with:

```ts
export type ProfileTab = 'posts' | 'secondhand'
export type ProfileTone = 'peach' | 'sky' | 'mint' | 'lemon' | 'lilac'
export type CardHeight = 'short' | 'medium' | 'tall'

export interface PublicProfilePost {
  id: string
  title: string
  mediaLabel: string
  likes: number
  tone: ProfileTone
  height: CardHeight
}

export interface PublicProfileProduct {
  id: string
  title: string
  mediaLabel: string
  price: number
  status: 'selling' | 'sold'
  tone: ProfileTone
  height: CardHeight
}

export interface PublicProfileView {
  id: string
  nickname: string
  account: string
  avatarText: string
  school: string
  grade?: string
  bio: string
  followingCount: number
  followerCount: number
  likedCount: number
  isSelf: boolean
  isFollowing: boolean
  posts: PublicProfilePost[]
  products: PublicProfileProduct[]
}

const profiles: Record<string, PublicProfileView> = {
  'current-user': {
    id: 'current-user',
    nickname: '阿泽',
    account: 'zk_aze233',
    avatarText: '泽',
    school: '湛江科技学院',
    grade: '大二',
    bio: '记录校园日常，也分享认真淘来的好东西',
    followingCount: 26,
    followerCount: 118,
    likedCount: 486,
    isSelf: true,
    isFollowing: false,
    posts: [
      { id: 'aze-library', title: '期末周图书馆安静角落分享', mediaLabel: '图 · 图书馆', likes: 96, tone: 'sky', height: 'medium' },
      { id: 'aze-sunset', title: '下课路上遇到的蓝调晚霞', mediaLabel: '图 · 晚霞', likes: 73, tone: 'peach', height: 'short' },
      { id: 'aze-food', title: '北门新开的砂锅粉，学生价很实在', mediaLabel: '图 · 美食', likes: 52, tone: 'mint', height: 'medium' }
    ],
    products: [
      { id: 'aze-calculator', title: '科学计算器，考试课可用', mediaLabel: '图 · 计算器', price: 28, status: 'selling', tone: 'lemon', height: 'short' },
      { id: 'aze-books', title: '大二公共课教材和复习资料', mediaLabel: '图 · 教材', price: 35, status: 'selling', tone: 'lilac', height: 'medium' }
    ]
  },
  'milk-tea': {
    id: 'milk-tea',
    nickname: '奶茶脑袋',
    account: 'zk_naicha01',
    avatarText: '奶',
    school: '湛江科技学院',
    grade: '大三',
    bio: '万物皆可奶茶 🧋 分享校园好喝榜',
    followingCount: 88,
    followerCount: 326,
    likedCount: 1204,
    isSelf: false,
    isFollowing: false,
    posts: [
      { id: 'milk-tea-main', title: '三食堂新出的芋泥波波真的绝，一天一杯根本停不下来', mediaLabel: '图 · 奶茶', likes: 128, tone: 'peach', height: 'tall' },
      { id: 'milk-tea-review', title: '大学城奶茶测评｜十家喝下来这三家最能打', mediaLabel: '图 · 测评', likes: 256, tone: 'lemon', height: 'medium' },
      { id: 'milk-tea-menu', title: '隐藏喝法：这家的四季春加波波椰果绝配', mediaLabel: '图 · 隐藏菜单', likes: 89, tone: 'sky', height: 'medium' }
    ],
    products: []
  },
  north: {
    id: 'north',
    nickname: '向北',
    account: 'zk_xiangbei',
    avatarText: '向',
    school: '湛江科技学院',
    grade: '大四',
    bio: '毕业清物，校内可当面看货',
    followingCount: 42,
    followerCount: 79,
    likedCount: 216,
    isSelf: false,
    isFollowing: false,
    posts: [
      { id: 'north-graduation', title: '毕业季最后一次绕学校骑行', mediaLabel: '图 · 校园', likes: 64, tone: 'mint', height: 'medium' }
    ],
    products: [
      { id: 'bike', title: '九成新自行车，毕业急出，车况好无锈', mediaLabel: '图 · 自行车', price: 150, status: 'selling', tone: 'sky', height: 'tall' },
      { id: 'north-lamp', title: '宿舍护眼台灯，亮度三档可调', mediaLabel: '图 · 台灯', price: 35, status: 'selling', tone: 'lemon', height: 'short' }
    ]
  }
}

const fallbackProfile: PublicProfileView = {
  id: 'campus-user',
  nickname: '校园同学',
  account: 'campus_user',
  avatarText: '同',
  school: '湛江科技学院',
  bio: '这位同学还没有填写个人简介',
  followingCount: 0,
  followerCount: 0,
  likedCount: 0,
  isSelf: false,
  isFollowing: false,
  posts: [],
  products: []
}

export function resolvePublicProfile(userId?: string): { profile: PublicProfileView; usedFallback: boolean } {
  const key = userId?.trim() || ''
  const profile = profiles[key]
  if (profile) return { profile, usedFallback: false }
  return { profile: { ...fallbackProfile, id: key || fallbackProfile.id }, usedFallback: true }
}
```

- [ ] **Step 3: Run the miniapp type checker**

Run:

```powershell
npm --workspace apps/miniapp run typecheck
```

Expected: `tsc --noEmit` exits 0. If the worktree has no `node_modules`, run `npm install` once and rerun; do not commit lockfile-only formatting churn.

- [ ] **Step 4: Commit the data contract**

```powershell
git add apps/miniapp/src/pages/user-profile/data.ts
git commit -m "feat(ui): 添加通用个人主页数据模型"
```

### Task 2: Build the Route and Interactive Page

**Files:**
- Create: `apps/miniapp/src/pages/user-profile/index.tsx`
- Create: `apps/miniapp/src/pages/user-profile/index.config.ts`
- Modify: `apps/miniapp/src/app.config.ts`

- [ ] **Step 1: Register the route before creating the page and verify the build gate catches it**

Add `'pages/user-profile/index'` immediately after `'pages/profile/index'` in `apps/miniapp/src/app.config.ts`, then run:

```powershell
npm --workspace apps/miniapp run build:weapp
```

Expected: non-zero exit because `pages/user-profile/index` does not exist yet. This proves the route is part of the production build graph.

- [ ] **Step 2: Add the page config**

Create `apps/miniapp/src/pages/user-profile/index.config.ts`:

```ts
export default definePageConfig({
  navigationStyle: 'custom',
  backgroundColor: '#F2F5F9',
  navigationBarTextStyle: 'white'
})
```

- [ ] **Step 3: Implement route parsing, permissions, tabs, cards, and the action sheet**

Create `apps/miniapp/src/pages/user-profile/index.tsx` with these exact responsibilities and signatures:

```tsx
import React, { useEffect, useMemo, useState } from 'react'
import { ScrollView, Text, View } from '@tarojs/components'
import Taro from '../../services/taro'
import {
  type ProfileTab,
  type PublicProfilePost,
  type PublicProfileProduct,
  resolvePublicProfile
} from './data'
import './index.scss'

function showToast(title: string) {
  void Taro.showToast({ title, icon: 'none', duration: 1400 })
}

function splitColumns<T>(items: T[]) {
  return {
    left: items.filter((_, index) => index % 2 === 0),
    right: items.filter((_, index) => index % 2 === 1)
  }
}

export default function UserProfilePage() {
  const params = Taro.getCurrentInstance().router?.params || {}
  const initialTab: ProfileTab = params.tab === 'secondhand' ? 'secondhand' : 'posts'
  const resolved = useMemo(() => resolvePublicProfile(params.userId), [params.userId])
  const profile = resolved.profile
  const [activeTab, setActiveTab] = useState<ProfileTab>(initialTab)
  const [following, setFollowing] = useState(profile.isFollowing)
  const [followerCount, setFollowerCount] = useState(profile.followerCount)
  const [menuVisible, setMenuVisible] = useState(false)
  const postColumns = useMemo(() => splitColumns(profile.posts), [profile.posts])
  const productColumns = useMemo(() => splitColumns(profile.products), [profile.products])

  useEffect(() => {
    if (resolved.usedFallback) showToast('未找到该用户，已显示通用主页')
  }, [resolved.usedFallback])

  const goBack = () => {
    void Taro.navigateBack().catch(() => Taro.redirectTo({ url: '/pages/profile/index' }))
  }

  const toggleFollow = () => {
    setFollowing((value) => {
      setFollowerCount((count) => Math.max(0, count + (value ? -1 : 1)))
      return !value
    })
  }

  const openConversation = () => {
    const query = `scene=profile&peerName=${encodeURIComponent(profile.nickname)}&peerAvatar=${encodeURIComponent(profile.avatarText)}`
    void Taro.navigateTo({ url: `/pages/secondhand-chat/index?${query}` })
  }

  const handleSecondaryAction = () => {
    if (profile.isSelf) {
      showToast('主页分享静态预览')
      return
    }
    openConversation()
  }

  const handleMenuAction = (action: 'copy' | 'report' | 'block') => {
    setMenuVisible(false)
    const copy = {
      copy: '主页链接已复制（静态预览）',
      report: '举报入口已记录为静态预览',
      block: '拉黑入口已记录为静态预览'
    }
    showToast(copy[action])
  }

  const openPost = (item: PublicProfilePost) => {
    void Taro.navigateTo({ url: `/pages/post-detail/index?id=${encodeURIComponent(item.id)}` })
  }

  const openProduct = (item: PublicProfileProduct) => {
    void Taro.navigateTo({ url: `/pages/secondhand-detail/index?id=${encodeURIComponent(item.id)}` })
  }

  const renderPost = (item: PublicProfilePost) => (
    <View className='public-profile-card' key={item.id} onClick={() => openPost(item)}>
      <View className={`public-profile-card__media public-profile-card__media--${item.tone} public-profile-card__media--${item.height}`}>
        <Text>{item.mediaLabel}</Text>
      </View>
      <View className='public-profile-card__body'>
        <Text className='public-profile-card__title'>{item.title}</Text>
        <View className='public-profile-card__metric'>
          <Text className='public-profile-card__heart'>♥</Text>
          <Text>{item.likes}</Text>
        </View>
      </View>
    </View>
  )

  const renderProduct = (item: PublicProfileProduct) => (
    <View className='public-profile-card' key={item.id} onClick={() => openProduct(item)}>
      <View className={`public-profile-card__media public-profile-card__media--${item.tone} public-profile-card__media--${item.height}`}>
        <Text>{item.mediaLabel}</Text>
        {item.status === 'sold' && <Text className='public-profile-card__sold'>已售</Text>}
      </View>
      <View className='public-profile-card__body'>
        <Text className='public-profile-card__title'>{item.title}</Text>
        <View className='public-profile-card__product-meta'>
          <Text className='public-profile-card__price'>¥{item.price}</Text>
          <Text>{item.status === 'selling' ? '出售中' : '已售出'}</Text>
        </View>
      </View>
    </View>
  )

  const empty = activeTab === 'posts' ? profile.posts.length === 0 : profile.products.length === 0

  return (
    <View className='public-profile-page'>
      <View className='public-profile-hero'>
        <View className='public-profile-nav'>
          <View className='public-profile-nav__back' onClick={goBack} />
          <View className='public-profile-nav__more' onClick={() => setMenuVisible(true)}>
            <Text>•••</Text>
          </View>
        </View>

        <View className='public-profile-user'>
          <View className='public-profile-user__avatar'><Text>{profile.avatarText}</Text></View>
          <View className='public-profile-user__main'>
            <Text className='public-profile-user__name'>{profile.nickname}</Text>
            <Text className='public-profile-user__meta'>湛科号：{profile.account}</Text>
            <Text className='public-profile-user__meta'>{profile.school}{profile.grade ? ` · ${profile.grade}` : ''}</Text>
          </View>
        </View>

        <Text className='public-profile-bio'>{profile.bio}</Text>

        <View className='public-profile-stats'>
          <View><Text className='public-profile-stats__value'>{profile.followingCount}</Text><Text>关注</Text></View>
          <View><Text className='public-profile-stats__value'>{followerCount}</Text><Text>粉丝</Text></View>
          <View><Text className='public-profile-stats__value'>{profile.likedCount}</Text><Text>获赞</Text></View>
        </View>

        <View className='public-profile-actions'>
          <View className={`public-profile-action public-profile-action--primary ${following ? 'public-profile-action--active' : ''}`} onClick={profile.isSelf ? () => void Taro.navigateTo({ url: '/pages/edit-profile/index' }) : toggleFollow}>
            <Text>{profile.isSelf ? '编辑资料' : following ? '已关注' : '+ 关注'}</Text>
          </View>
          <View className='public-profile-action public-profile-action--secondary' onClick={handleSecondaryAction}>
            <Text>{profile.isSelf ? '分享主页' : '▱ 私信'}</Text>
          </View>
        </View>
      </View>

      <View className='public-profile-tabs'>
        <View className={`public-profile-tab ${activeTab === 'posts' ? 'public-profile-tab--active' : ''}`} onClick={() => setActiveTab('posts')}>
          <Text>帖子 {profile.posts.length}</Text>
        </View>
        <View className={`public-profile-tab ${activeTab === 'secondhand' ? 'public-profile-tab--active' : ''}`} onClick={() => setActiveTab('secondhand')}>
          <Text>二手 {profile.products.length}</Text>
        </View>
      </View>

      <View className='public-profile-content'>
        <ScrollView className='public-profile-scroll' scrollY enhanced showScrollbar={false}>
          {empty ? (
            <View className='public-profile-empty'>
              <Text className='public-profile-empty__icon'>{activeTab === 'posts' ? '文' : '物'}</Text>
              <Text className='public-profile-empty__title'>{activeTab === 'posts' ? '还没有发布帖子' : '还没有发布二手'}</Text>
              <Text className='public-profile-empty__desc'>发布后会展示在个人主页</Text>
            </View>
          ) : activeTab === 'posts' ? (
            <View className='public-profile-waterfall'>
              <View className='public-profile-waterfall__column'>{postColumns.left.map(renderPost)}</View>
              <View className='public-profile-waterfall__column'>{postColumns.right.map(renderPost)}</View>
            </View>
          ) : (
            <View className='public-profile-waterfall'>
              <View className='public-profile-waterfall__column'>{productColumns.left.map(renderProduct)}</View>
              <View className='public-profile-waterfall__column'>{productColumns.right.map(renderProduct)}</View>
            </View>
          )}
        </ScrollView>
      </View>

      {menuVisible && (
        <View className='public-profile-sheet-layer'>
          <View className='public-profile-sheet-layer__mask' onClick={() => setMenuVisible(false)} />
          <View className='public-profile-sheet'>
            <View className='public-profile-sheet__group'>
              <View className='public-profile-sheet__item' onClick={() => handleMenuAction('copy')}><Text>复制主页链接</Text></View>
              {!profile.isSelf && <View className='public-profile-sheet__item public-profile-sheet__item--warning' onClick={() => handleMenuAction('report')}><Text>举报该用户</Text></View>}
              {!profile.isSelf && <View className='public-profile-sheet__item public-profile-sheet__item--danger' onClick={() => handleMenuAction('block')}><Text>拉黑该用户</Text></View>}
            </View>
            <View className='public-profile-sheet__cancel' onClick={() => setMenuVisible(false)}><Text>取消</Text></View>
          </View>
        </View>
      )}
    </View>
  )
}
```

- [ ] **Step 4: Run type checking**

Run:

```powershell
npm --workspace apps/miniapp run typecheck
```

Expected: exit 0 with no TypeScript errors.

- [ ] **Step 5: Commit the route and page behavior**

```powershell
git add apps/miniapp/src/app.config.ts apps/miniapp/src/pages/user-profile/index.tsx apps/miniapp/src/pages/user-profile/index.config.ts
git commit -m "feat(ui): 建立通用个人主页路由"
```

### Task 3: Add the Responsive Visual System

**Files:**
- Create: `apps/miniapp/src/pages/user-profile/index.scss`

- [ ] **Step 1: Add the complete page styling**

Create `apps/miniapp/src/pages/user-profile/index.scss`. Use `@use '../../styles/design-tokens.scss' as *;`, then implement these exact layout contracts:

```scss
@use '../../styles/design-tokens.scss' as *;

.public-profile-page { display: flex; flex-direction: column; height: 100vh; overflow: hidden; background: $bg-app; color: $text-strong; font-family: -apple-system, BlinkMacSystemFont, 'SF Pro Text', 'PingFang SC', 'Helvetica Neue', Arial, sans-serif; }
.public-profile-hero { flex: 0 0 auto; padding: 86rpx 30rpx 34rpx; background: $brand-primary; color: $text-on-color; box-sizing: border-box; }
.public-profile-nav { display: flex; align-items: center; justify-content: space-between; height: 54rpx; }
.public-profile-nav__back { position: relative; width: 54rpx; height: 54rpx; }
.public-profile-nav__back::before { content: ''; position: absolute; left: 20rpx; top: 15rpx; width: 20rpx; height: 20rpx; border-left: 4rpx solid $text-on-color; border-bottom: 4rpx solid $text-on-color; transform: rotate(45deg); }
.public-profile-nav__more { display: flex; align-items: center; justify-content: center; width: 64rpx; height: 54rpx; color: $text-on-color; font-size: 30rpx; }
.public-profile-user { display: flex; align-items: center; gap: 24rpx; margin-top: 24rpx; }
.public-profile-user__avatar { display: flex; align-items: center; justify-content: center; flex: 0 0 auto; width: 94rpx; height: 94rpx; border: 5rpx solid rgba(255,255,255,.72); border-radius: 50%; background: #F5A888; color: $text-on-color; font-size: 46rpx; font-weight: 700; }
.public-profile-user__main { display: flex; flex-direction: column; min-width: 0; }
.public-profile-user__name { overflow: hidden; color: $text-on-color; font-size: 36rpx; font-weight: 800; line-height: 1.25; white-space: nowrap; text-overflow: ellipsis; }
.public-profile-user__meta { margin-top: 5rpx; overflow: hidden; color: rgba(255,255,255,.86); font-size: 24rpx; line-height: 1.25; white-space: nowrap; text-overflow: ellipsis; }
.public-profile-bio { display: block; margin-top: 24rpx; overflow: hidden; color: rgba(255,255,255,.95); font-size: 26rpx; line-height: 1.4; white-space: nowrap; text-overflow: ellipsis; }
.public-profile-stats { display: flex; gap: 46rpx; margin-top: 26rpx; color: rgba(255,255,255,.84); font-size: 24rpx; }
.public-profile-stats > view { display: flex; align-items: baseline; gap: 9rpx; }
.public-profile-stats__value { color: $text-on-color; font-size: 34rpx; font-weight: 800; }
.public-profile-actions { display: grid; grid-template-columns: 1fr 1fr; gap: 16rpx; margin-top: 28rpx; }
.public-profile-action { display: flex; align-items: center; justify-content: center; height: 72rpx; border: 2rpx solid rgba(255,255,255,.48); border-radius: 8rpx; font-size: 27rpx; font-weight: 600; box-sizing: border-box; }
.public-profile-action--primary { border-color: $text-on-color; background: $text-on-color; color: $brand-primary-strong; }
.public-profile-action--primary.public-profile-action--active { border-color: rgba(255,255,255,.62); background: rgba(255,255,255,.16); color: $text-on-color; }
.public-profile-action--secondary { background: rgba(255,255,255,.12); color: $text-on-color; }
.public-profile-tabs { display: flex; flex: 0 0 74rpx; align-items: stretch; padding: 0 30rpx; border-bottom: 1rpx solid $divider-nav; background: $bg-surface; }
.public-profile-tab { position: relative; display: flex; align-items: center; justify-content: center; min-width: 136rpx; padding: 0 16rpx; color: $text-tertiary; font-size: 28rpx; box-sizing: border-box; }
.public-profile-tab--active { color: $text-strong; font-weight: 700; }
.public-profile-tab--active::after { content: ''; position: absolute; left: 50%; bottom: 0; width: 44rpx; height: 6rpx; border-radius: 3rpx; background: $brand-primary; transform: translateX(-50%); }
.public-profile-content { flex: 1 1 auto; min-height: 0; }
.public-profile-scroll { height: 100%; }
.public-profile-waterfall { display: grid; grid-template-columns: minmax(0,1fr) minmax(0,1fr); gap: 16rpx; padding: 18rpx 20rpx 44rpx; box-sizing: border-box; }
.public-profile-waterfall__column { display: flex; flex-direction: column; gap: 16rpx; min-width: 0; }
.public-profile-card { overflow: hidden; border-radius: 8rpx; background: $bg-surface; box-shadow: $shadow-card; }
.public-profile-card__media { position: relative; display: flex; align-items: center; justify-content: center; height: 220rpx; color: $placeholder-label; font-size: 23rpx; background-image: $placeholder-stripe; }
.public-profile-card__media--short { height: 178rpx; }
.public-profile-card__media--medium { height: 224rpx; }
.public-profile-card__media--tall { height: 278rpx; }
.public-profile-card__media--peach { background-color: #FFF0EA; }
.public-profile-card__media--sky { background-color: #EAF5FC; }
.public-profile-card__media--mint { background-color: #EAF7F1; }
.public-profile-card__media--lemon { background-color: #FFF8E6; }
.public-profile-card__media--lilac { background-color: #F3EEFA; }
.public-profile-card__sold { position: absolute; top: 14rpx; right: 14rpx; padding: 6rpx 12rpx; border-radius: 6rpx; background: rgba(26,43,60,.72); color: $text-on-color; font-size: 21rpx; }
.public-profile-card__body { padding: 18rpx 18rpx 20rpx; }
.public-profile-card__title { display: -webkit-box; overflow: hidden; color: $text-strong; font-size: 26rpx; line-height: 1.42; word-break: break-word; -webkit-box-orient: vertical; -webkit-line-clamp: 2; }
.public-profile-card__metric { display: flex; align-items: center; gap: 7rpx; margin-top: 14rpx; color: $like-idle-count; font-size: 22rpx; }
.public-profile-card__heart { color: $like-active; }
.public-profile-card__product-meta { display: flex; align-items: center; justify-content: space-between; gap: 12rpx; margin-top: 15rpx; color: $text-tertiary; font-size: 21rpx; }
.public-profile-card__price { color: #E64E55; font-size: 28rpx; font-weight: 800; }
.public-profile-empty { display: flex; flex-direction: column; align-items: center; justify-content: center; min-height: 390rpx; padding: 44rpx; color: $text-tertiary; box-sizing: border-box; }
.public-profile-empty__icon { display: flex; align-items: center; justify-content: center; width: 78rpx; height: 78rpx; border-radius: 50%; background: $brand-tint; color: $brand-primary; font-size: 28rpx; font-weight: 700; }
.public-profile-empty__title { margin-top: 24rpx; color: $text-primary; font-size: 28rpx; font-weight: 700; }
.public-profile-empty__desc { margin-top: 10rpx; font-size: 23rpx; }
.public-profile-sheet-layer { position: fixed; inset: 0; z-index: 100; }
.public-profile-sheet-layer__mask { position: absolute; inset: 0; background: rgba(17,31,48,.54); }
.public-profile-sheet { position: absolute; left: 0; right: 0; bottom: 0; padding: 20rpx 24rpx calc(28rpx + env(safe-area-inset-bottom)); background: $bg-app; box-sizing: border-box; }
.public-profile-sheet__group { overflow: hidden; border-radius: 8rpx; background: $bg-surface; }
.public-profile-sheet__item, .public-profile-sheet__cancel { display: flex; align-items: center; justify-content: center; height: 88rpx; color: $text-primary; font-size: 28rpx; }
.public-profile-sheet__item + .public-profile-sheet__item { border-top: 1rpx solid $divider-nav; }
.public-profile-sheet__item--warning { color: #F08A32; }
.public-profile-sheet__item--danger { color: #F0525F; }
.public-profile-sheet__cancel { margin-top: 16rpx; border-radius: 8rpx; background: $bg-surface; font-weight: 700; }
```

- [ ] **Step 2: Run the production miniapp build**

Run:

```powershell
npm --workspace apps/miniapp run build:weapp
```

Expected: Taro reports a successful WeChat mini-program build and emits `dist/pages/user-profile/index.*`.

- [ ] **Step 3: Commit the styles**

```powershell
git add apps/miniapp/src/pages/user-profile/index.scss
git commit -m "style(ui): 完善通用个人主页布局"
```

### Task 4: Connect Self, Post Author, and Seller Entry Points

**Files:**
- Modify: `apps/miniapp/src/pages/profile/index.tsx`
- Modify: `apps/miniapp/src/pages/profile/index.scss`
- Modify: `apps/miniapp/src/pages/post-detail/index.tsx`
- Modify: `apps/miniapp/src/pages/secondhand-detail/index.tsx`

- [ ] **Step 1: Replace the “我的” header action**

Replace:

```tsx
<View className='student-profile-user__edit' onClick={() => void Taro.navigateTo({ url: '/pages/edit-profile/index' })}>
  <Text>编辑资料</Text>
</View>
```

with:

```tsx
<View className='student-profile-user__home' onClick={() => void Taro.navigateTo({ url: '/pages/user-profile/index?userId=current-user&source=profile' })}>
  <Text>主页</Text>
  <Text className='student-profile-user__home-arrow'>›</Text>
</View>
```

Rename `.student-profile-user__edit` to `.student-profile-user__home`, set `gap: 8rpx`, reduce `min-width` to `126rpx`, and add:

```scss
.student-profile-user__home-arrow {
  margin-top: -1rpx;
  font-size: 30rpx;
  line-height: 1;
}
```

- [ ] **Step 2: Connect the post author**

Replace the author click handler in `pages/post-detail/index.tsx` with:

```tsx
<View className='post-detail-author' onClick={() => void Taro.navigateTo({ url: '/pages/user-profile/index?userId=milk-tea&source=post' })}>
```

- [ ] **Step 3: Connect the second-hand seller and preserve the tab context**

Replace the seller button click handler in `pages/secondhand-detail/index.tsx` with:

```tsx
<View className='secondhand-detail-seller__button' onClick={() => void Taro.navigateTo({ url: '/pages/user-profile/index?userId=north&source=secondhand&tab=secondhand' })}>
```

- [ ] **Step 4: Verify every entry uses the one generic route**

Run:

```powershell
rg -n "pages/user-profile/index\?userId=" apps/miniapp/src/pages/profile apps/miniapp/src/pages/post-detail apps/miniapp/src/pages/secondhand-detail
```

Expected: exactly three route references with `current-user`, `milk-tea`, and `north`.

- [ ] **Step 5: Run type checking and commit**

```powershell
npm --workspace apps/miniapp run typecheck
git add apps/miniapp/src/pages/profile/index.tsx apps/miniapp/src/pages/profile/index.scss apps/miniapp/src/pages/post-detail/index.tsx apps/miniapp/src/pages/secondhand-detail/index.tsx
git commit -m "feat(ui): 接通通用个人主页入口"
```

Expected: type checking exits 0 and the commit contains only the four entry-point files.

### Task 5: Final Verification and Clean Handoff

**Files:**
- Verify all files changed in Tasks 1-4.

- [ ] **Step 1: Run repository boundary checks**

```powershell
npm run check:boundaries
```

Expected: `ok A-G boundaries plus independent delivery and isolated P0-4 media domains`.

- [ ] **Step 2: Run miniapp type checking and production build**

```powershell
npm --workspace apps/miniapp run typecheck
npm --workspace apps/miniapp run build:weapp
```

Expected: both commands exit 0. Existing Sass deprecation or bundle-size warnings are acceptable; compilation errors are not.

- [ ] **Step 3: Verify generated page artifacts**

Run:

```powershell
Get-ChildItem apps/miniapp/dist/pages/user-profile | Select-Object Name,Length
```

Expected: non-empty `index.js`, `index.json`, `index.wxml`, and `index.wxss` artifacts.

- [ ] **Step 4: Verify scope and whitespace**

```powershell
git diff master...HEAD --stat
git diff master...HEAD -- backend
git diff --check
git status --short --branch
```

Expected: the diff contains the design/plan docs and miniapp files only; backend diff is empty; `git diff --check` exits 0; status is clean on `codex/ui-user-profile`.

- [ ] **Step 5: Report the exact branch, worktree, commits, and verification results**

Use:

```powershell
git log --oneline master..HEAD
git worktree list
```

Expected: all implementation commits are on `codex/ui-user-profile`, `master` remains at the original baseline, and the handoff names any warning that did not block build output.
