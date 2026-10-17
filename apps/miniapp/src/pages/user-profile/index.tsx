import ProductPrice from '../../components/product-price'
import { withFeaturePage } from '../../components/feature-page-gate'
import { useFeatureVisibility } from '../../hooks/use-feature-visibility'
import { usePageShare } from '../../hooks/use-page-share'
import React, { useEffect, useRef, useState } from 'react'
import { useDidShow } from '@tarojs/taro'
import { Button, Image, ScrollView, Text, View } from '@tarojs/components'
import { CapsuleSystemRow } from '../../components/capsule-page-header'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import { AppActionSheet, AppDialog } from '../../components/ui'
import { blockUser } from '../../services/a-user'
import {
  getPublicCommunityProfile,
  getPublicUserPosts,
  setUserFollowing,
  type PublicCommunityProfile
} from '../../services/b-community'
import { getPublicUserProducts } from '../../services/c-trade'
import Taro from '../../services/taro'
import { useSessionStore } from '../../stores/session'
import type { CommunityPost, Product } from '../../types'
import { useViewportRefresh } from '../../hooks/use-viewport-refresh'
import ChannelPager, { useChannelInteraction } from '../../components/channel-pager'
import { navigateBackOrHome } from '../../utils/navigation'
import AppIcon from '../../components/app-icon'
import { postBodyWithoutTitle } from '../../utils/post-presentation'
import { useEdgeBack } from '../../hooks/use-edge-back'
import { useProfilePagerLayout } from './use-profile-pager-layout'
import './index.scss'

type ProfileTab = 'posts' | 'secondhand'
type PageStatus = 'loading' | 'ready' | 'error'

function showToast(title: string) {
  void Taro.showToast({ title, icon: 'none', duration: 1600 })
}

function UserProfilePage() {
  const capsuleStyle = useViewportRefresh()
  const visibility = useFeatureVisibility()
  const [params] = useState(() => Taro.getCurrentInstance().router?.params || {})
  const userId = String(params.userId || '').trim()
  const currentUserId = useSessionStore((state) => state.profile.id)
  const initialTab: ProfileTab = params.tab === 'secondhand' ? 'secondhand' : 'posts'
  const [status, setStatus] = useState<PageStatus>('loading')
  const [reloadKey, setReloadKey] = useState(0)
  const [profile, setProfile] = useState<PublicCommunityProfile | null>(null)
  const [posts, setPosts] = useState<CommunityPost[]>([])
  const [products, setProducts] = useState<Product[]>([])
  const [postTotal, setPostTotal] = useState(0)
  const [productTotal, setProductTotal] = useState(0)
  const [postPage, setPostPage] = useState(1)
  const [productPage, setProductPage] = useState(1)
  const [loadingMore, setLoadingMore] = useState(false)
  const [loadMoreErrors, setLoadMoreErrors] = useState<Record<ProfileTab, string>>({ posts: '', secondhand: '' })
  const [loadingTab, setLoadingTab] = useState<ProfileTab | null>(null)
  const [activeTab, setActiveTab] = useState<ProfileTab>(initialTab)
  const [following, setFollowing] = useState(false)
  const [followerCount, setFollowerCount] = useState(0)
  const [menuVisible, setMenuVisible] = useState(false)
  const [busy, setBusy] = useState(false)
  const [contacting, setContacting] = useState(false)
  const contactPending = useRef(false)
  const [blockConfirmVisible, setBlockConfirmVisible] = useState(false)
  const [blockPending, setBlockPending] = useState(false)
  const [error, setError] = useState('')
  const [failedCover, setFailedCover] = useState('')
  const [failedAvatar, setFailedAvatar] = useState('')
  const [failedMedia, setFailedMedia] = useState<Set<string>>(() => new Set())
  const [headerScrolled, setHeaderScrolled] = useState(false)
  const requestVersion = useRef(0)
  const loadingMoreRef = useRef(false)
  const followPending = useRef(false)
  const didShowOnce = useRef(false)
  const interaction = useChannelInteraction()
  const edgeBack = useEdgeBack(!menuVisible && !blockConfirmVisible)
  const pagerLayout = useProfilePagerLayout(status === 'ready', `${userId}:${posts.length}:${products.length}:${visibility.secondhand}:${visibility.likes}:${visibility.following}:${loadingMore}:${loadMoreErrors.posts}:${loadMoreErrors.secondhand}`)
  const switchTab = (tab: ProfileTab) => {
    if (tab === activeTab) return
    setHeaderScrolled(pagerLayout.switchScroll(tab) > 48)
    setActiveTab(tab)
  }
  const visibleTabs: ProfileTab[] = visibility.secondhand ? ['posts', 'secondhand'] : ['posts']
  const shareReady = usePageShare({ kind: 'profile', enabled: visibility.forum && status === 'ready', id: userId, loadedId: String(profile?.profile.id || ''), nickname: profile?.profile.nickname || '' })
  useEffect(() => { if (!visibility.secondhand && activeTab === 'secondhand') setActiveTab('posts') }, [visibility.secondhand, activeTab])

  useEffect(() => {
    let active = true
    const version = requestVersion.current + 1
    requestVersion.current = version
    loadingMoreRef.current = false
    setLoadingMore(false)
    setLoadingTab(null)
    setLoadMoreErrors({ posts: '', secondhand: '' })
    setPostPage(1)
    setProductPage(1)
    setStatus('loading')
    setError('')
    if (!/^[1-9]\d*$/.test(userId)) {
      setStatus('error')
      setError('用户主页入口无效，请返回后重新进入')
      return () => { active = false }
    }
    void Promise.all([
      getPublicCommunityProfile(userId),
      getPublicUserPosts(userId, 1, 20),
      visibility.secondhand ? getPublicUserProducts(userId, 1, 20) : Promise.resolve({ items: [] as Product[], total: 0, page: 1 })
    ]).then(([profileValue, postPage, productPage]) => {
      if (!active || requestVersion.current !== version) return
      setProfile(profileValue)
      setFollowing(profileValue.isFollowing)
      setFollowerCount(profileValue.followerCount)
      setPosts(postPage.items)
      setProducts(productPage.items)
      setPostTotal(postPage.total)
      setProductTotal(productPage.total)
      setPostPage(postPage.page || 1)
      setProductPage(productPage.page || 1)
      setStatus('ready')
    }).catch((value: unknown) => {
      if (!active || requestVersion.current !== version) return
      setStatus('error')
      setError(value instanceof Error ? value.message : '用户主页加载失败')
    })
    return () => { active = false; requestVersion.current += 1 }
  }, [reloadKey, userId, visibility.secondhand])

  // Reconcile a changed cover/follow state after returning without replacing
  // either content list or its saved scroll position.
  useDidShow(() => {
    if (!didShowOnce.current) { didShowOnce.current = true; return }
    if (status !== 'ready' || followPending.current) return
    const version = requestVersion.current
    void getPublicCommunityProfile(userId).then((value) => {
      if (requestVersion.current !== version || followPending.current) return
      setProfile(value)
      setFollowing(value.isFollowing)
      setFollowerCount(value.followerCount)
    }).catch(() => showToast('资料刷新失败，已保留当前主页'))
  })

  const loadMore = async (tab: ProfileTab = activeTab) => {
    if (loadingMoreRef.current || tab !== activeTab || (tab === 'secondhand' && !visibility.secondhand)) return
    const isPosts = tab === 'posts'
    const currentItems = isPosts ? posts : products
    const total = isPosts ? postTotal : productTotal
    if (currentItems.length >= total) return
    const nextPage = (isPosts ? postPage : productPage) + 1
    const version = requestVersion.current + 1
    requestVersion.current = version
    loadingMoreRef.current = true
    setLoadingMore(true)
    setLoadingTab(tab)
    setLoadMoreErrors((current) => ({ ...current, [tab]: '' }))
    try {
      if (isPosts) {
        const result = await getPublicUserPosts(userId, nextPage, 20)
        if (requestVersion.current !== version) return
        setPosts((current) => {
          const byId = new Map(current.map((item) => [item.id, item]))
          result.items.forEach((item) => byId.set(item.id, item))
          return Array.from(byId.values())
        })
        setPostPage(result.page || nextPage)
        setPostTotal(result.total)
      } else {
        const result = await getPublicUserProducts(userId, nextPage, 20)
        if (requestVersion.current !== version) return
        setProducts((current) => {
          const byId = new Map(current.map((item) => [item.id, item]))
          result.items.forEach((item) => byId.set(item.id, item))
          return Array.from(byId.values())
        })
        setProductPage(result.page || nextPage)
        setProductTotal(result.total)
      }
    } catch (value) {
      if (requestVersion.current === version) {
        const message = value instanceof Error ? `${value.message}，点击重试` : '更多内容加载失败，点击重试'
        setLoadMoreErrors((current) => ({ ...current, [tab]: message }))
      }
    } finally {
      if (requestVersion.current === version) {
        loadingMoreRef.current = false
        setLoadingMore(false)
        setLoadingTab(null)
      }
    }
  }

  const isSelf = currentUserId === userId
  const hasCover = Boolean(profile?.profile.cover && profile.profile.cover !== failedCover)
  const hasAvatar = Boolean(profile?.profile.avatar && profile.profile.avatar !== failedAvatar)

  const toggleFollow = async () => {
    if (followPending.current || isSelf || !visibility.following) return
    followPending.current = true
    const next = !following
    setBusy(true)
    try {
      await setUserFollowing(userId, next)
      setFollowing(next)
      setFollowerCount((count) => Math.max(0, count + (next ? 1 : -1)))
    } catch (value) {
      showToast(value instanceof Error ? value.message : '关注操作失败')
    } finally {
      followPending.current = false
      setBusy(false)
    }
  }

  const openDirectMessage = async () => {
    if (isSelf || contactPending.current || !visibility.messages || !interaction.canActivate()) return
    contactPending.current = true
    setContacting(true)
    try {
      await Taro.navigateTo({ url: `/pages/secondhand-chat/index?userId=${encodeURIComponent(userId)}` })
    } catch {
      showToast('私信页面打开失败，请稍后重试')
    } finally {
      contactPending.current = false
      setContacting(false)
    }
  }

  const copyProfilePath = async () => {
    setMenuVisible(false)
    await Taro.setClipboardData({ data: `pages/user-profile/index?userId=${userId}` })
    showToast('主页路径已复制')
  }

  const reportUser = () => {
    setMenuVisible(false)
    void Taro.navigateTo({
      url: `/pages/feedback/index?category=report&targetUserId=${encodeURIComponent(userId)}`
    })
  }

  const requestBlock = () => {
    setMenuVisible(false)
    setBlockConfirmVisible(true)
  }

  const confirmBlock = async () => {
    if (blockPending) return
    setBlockPending(true)
    try {
      await blockUser(Number(userId))
      setBlockConfirmVisible(false)
      await Taro.showToast({ title: '已加入黑名单', icon: 'success' })
      await Taro.navigateBack()
    } catch (value) {
      showToast(value instanceof Error ? value.message : '拉黑失败')
    } finally {
      setBlockPending(false)
    }
  }

  const openContent = (url: string) => {
    if (!interaction.canActivate() || menuVisible || blockConfirmVisible) return
    void Taro.navigateTo({ url })
  }

  const markMediaFailed = (src: string) => setFailedMedia((current) => new Set(current).add(src))

  const renderPost = (item: CommunityPost) => {
    const image = item.images.find((src) => src.trim())
    const summary = postBodyWithoutTitle(item.title, item.content)
    return (
    <View className={`public-profile-card public-profile-card--post${image ? '' : ' public-profile-card--text'}`} key={item.id}
      ariaRole='button' ariaLabel={item.title || item.content} hoverClass='public-profile-card--pressed'
      onClick={() => openContent(`/pages/post-detail/index?id=${encodeURIComponent(item.id)}`)}>
      {image ? <View className='public-profile-card__media public-profile-card__media--post'>
        {!failedMedia.has(image) ? <Image className='public-profile-card__media-image' src={image} mode='aspectFill' lazyLoad onError={() => markMediaFailed(image)} />
          : <View className='public-profile-card__no-media'><AppIcon name='image' /><Text>图片暂不可用</Text></View>}
      </View> : <View className='public-profile-card__text'>
        <Text className='public-profile-card__topic'>{item.topic || '校园帖子'}</Text>
        <Text className='public-profile-card__text-title'>{item.title || item.content}</Text>
        {summary && summary !== item.title && <Text className='public-profile-card__excerpt'>{summary}</Text>}
      </View>}
      <View className='public-profile-card__body'>
        {image && <Text className='public-profile-card__title'>{item.title || item.content}</Text>}
        {visibility.likes && <View className='public-profile-card__metric'><AppIcon name='heart' className='public-profile-card__heart' /><Text>{item.likedCount}</Text></View>}
      </View>
    </View>
    )
  }

  const renderProduct = (item: Product) => {
    const image = item.images.find((src) => src.trim())
    const status = item.status === 'selling' ? '出售中' : item.status === 'reserved' ? '已预留' : '已售出'
    return (
    <View className='public-profile-card public-profile-card--product' key={item.id}
      ariaRole='button' ariaLabel={`${item.title}，${item.price}元`} hoverClass='public-profile-card--pressed'
      onClick={() => openContent(`/pages/secondhand-detail/index?id=${encodeURIComponent(item.id)}`)}>
      <View className='public-profile-card__media public-profile-card__media--product'>
        {image && !failedMedia.has(image) ? <Image className='public-profile-card__media-image' src={image} mode='aspectFill' lazyLoad onError={() => markMediaFailed(image)} />
          : <View className='public-profile-card__no-media'><AppIcon name='image' /><Text>{image ? '图片暂不可用' : '卖家未上传图片'}</Text></View>}
      </View>
      <View className='public-profile-card__body'>
        <Text className='public-profile-card__title'>{item.title}</Text>
        <View className='public-profile-card__product-meta'>
          <ProductPrice className='public-profile-card__price' value={item.price} />
        </View>
        <Text className='public-profile-card__product-facts'>{[status, item.condition || item.category].filter(Boolean).join(' · ')}</Text>
      </View>
    </View>
    )
  }

  const navigation = (
    <View className={`public-profile-navigation${status !== 'ready' || headerScrolled ? ' public-profile-navigation--scrolled' : ''}${status === 'ready' && hasCover && !headerScrolled ? ' public-profile-navigation--on-cover' : ''}`}>
      <View className='public-profile-nav'>
        <View className={`public-profile-nav__back${edgeBack.tracking ? ' public-profile-nav__back--tracking' : ''}`}
          style={{ '--profile-back-offset': `${edgeBack.offset}px` } as React.CSSProperties}
          {...(edgeBack.available ? edgeBack.handlers : {})}
          ariaRole='button' ariaLabel='返回' hoverClass='public-profile-control--pressed'
          onClick={() => { if (edgeBack.canActivate()) void navigateBackOrHome() }} />
      </View>
    </View>
  )

  const edgeBackLayer = edgeBack.available ? <View className='public-profile-edge-back' {...edgeBack.handlers} catchMove aria-hidden='true' /> : null

  if (status !== 'ready' || !profile) {
    return (
      <View className='public-profile-page' style={capsuleStyle}>
        {edgeBackLayer}
        {navigation}
        <View className='public-profile-state public-profile-empty'>
          <Text className='public-profile-empty__title'>{status === 'loading' ? '正在加载主页…' : error || '用户主页加载失败'}</Text>
          {status !== 'loading' && <View className='public-profile-retry' ariaRole='button' ariaLabel='重新加载主页' onClick={() => setReloadKey((value) => value + 1)}><Text>重新加载</Text></View>}
        </View>
      </View>
    )
  }

  const value = profile.profile
  const hero = (
      <View className={`public-profile-hero${hasCover ? ' public-profile-hero--with-cover' : ''}`}>
        <View className='public-profile-cover'>
          {hasCover && <Image className='public-profile-cover__image' src={value.cover || ''} mode='aspectFill' ariaLabel='个人封面' onError={() => setFailedCover(value.cover || '')} />}
        </View>
        <CapsuleSystemRow />
        <View className='public-profile-hero__toolbar' aria-hidden />
        <View className='public-profile-user'>
          <View className='public-profile-user__top'>
            <View className='public-profile-user__avatar'>
              {hasAvatar ? <Image className='public-profile-user__avatar-image' src={value.avatar} mode='aspectFill' onError={() => setFailedAvatar(value.avatar)} /> : <Text>{value.nickname.slice(0, 1) || '校'}</Text>}
            </View>
            <View className='public-profile-actions'>
              {(isSelf || visibility.following) && <View className={`public-profile-action public-profile-action--primary ${following && !isSelf ? 'public-profile-action--active' : ''}`} ariaRole='button' onClick={isSelf ? () => openContent('/pages/edit-profile/index') : () => { if (interaction.canActivate()) void toggleFollow() }}>
                <Text>{isSelf ? '编辑资料' : busy ? '处理中…' : following ? '已关注' : '+ 关注'}</Text>
              </View>}
              {isSelf && <Button className='public-profile-action public-profile-action--secondary' openType='share' disabled={!shareReady}>分享主页</Button>}
              {!isSelf && visibility.messages && <Button className='public-profile-action public-profile-action--secondary public-profile-action--message' ariaLabel='发私信' disabled={contacting} onClick={() => void openDirectMessage()}>{contacting ? '打开中…' : '私信'}</Button>}
            </View>
          </View>
          <View className='public-profile-user__main'>
            <Text className='public-profile-user__name'>{value.nickname}</Text>
            <Text className='public-profile-user__meta'>用户号 {value.id}</Text>
            <View className='public-profile-user__options' ariaRole='button' ariaLabel='主页资料操作' hoverClass='public-profile-control--pressed' onClick={() => { if (interaction.canActivate()) setMenuVisible(true) }}><AppIcon name='more' /></View>
          </View>
        </View>
        <View className='public-profile-stats'>
          {visibility.following && <View><Text className='public-profile-stats__value'>{profile.followingCount}</Text><Text>关注</Text></View>}
          {visibility.following && <View><Text className='public-profile-stats__value'>{followerCount}</Text><Text>粉丝</Text></View>}
          {visibility.likes && <View><Text className='public-profile-stats__value'>{profile.likedCount}</Text><Text>获赞</Text></View>}
        </View>
      </View>
  )

  const tabs = (
      <View className='public-profile-tabs'>
        <View className={`public-profile-tab ${activeTab === 'posts' ? 'public-profile-tab--active' : ''}`} onClick={() => { if (interaction.canActivate()) switchTab('posts') }}><Text>帖子 {postTotal}</Text></View>
        {visibility.secondhand && <View className={`public-profile-tab ${activeTab === 'secondhand' ? 'public-profile-tab--active' : ''}`} onClick={() => { if (interaction.canActivate()) switchTab('secondhand') }}><Text>二手 {productTotal}</Text></View>}
      </View>
  )
  return (
    <View className='public-profile-page' style={capsuleStyle}>
      {edgeBackLayer}
      {navigation}
      <ScrollView
        className='public-profile-scroll'
        scrollY enhanced showScrollbar={false} lowerThreshold={120}
        scrollTop={pagerLayout.scrollTarget}
        onScroll={(event) => {
          const top = event.detail.scrollTop
          pagerLayout.rememberScroll(activeTab, top)
          if (Number.isFinite(top)) setHeaderScrolled(top > 48)
        }}
        onScrollToLower={() => void loadMore(activeTab)}
      >
        {hero}
        {tabs}
        <View className='public-profile-content' style={pagerLayout.layout.heights[activeTab] > 0 ? { height: `${Math.max(pagerLayout.layout.heights[activeTab], pagerLayout.layout.minimumBody)}px` } : undefined}>
          <ChannelPager
            keys={visibleTabs}
            activeKey={activeTab}
            onChange={switchTab}
            interaction={interaction}
            renderPage={(tab) => (
              <View className='public-profile-panel' id={`public-profile-panel-${tab}`}>
                {(tab === 'posts' ? posts.length === 0 : products.length === 0) ? (
                  <View className='public-profile-empty'><Text className='public-profile-empty__title'>{tab === 'posts' ? '暂无公开帖子' : '暂无公开二手商品'}</Text></View>
                ) : tab === 'posts' ? (
                  <View className='public-profile-grid public-profile-grid--posts'>{posts.map(renderPost)}</View>
                ) : (
                  <View className='public-profile-grid public-profile-grid--products'>{products.map(renderProduct)}</View>
                )}
                {loadMoreErrors[tab] ? <Text className='public-profile-load-more public-profile-load-more--retry' onClick={() => { if (interaction.canActivate()) void loadMore(tab) }}>{loadMoreErrors[tab]}</Text> : null}
                {loadingMore && loadingTab === tab ? <Text className='public-profile-load-more'>正在加载更多内容…</Text> : null}
              </View>
            )}
          />
        </View>
      </ScrollView>
      <AppActionSheet visible={menuVisible} title='更多操作' onClose={() => setMenuVisible(false)}>
        <View className='public-profile-sheet__content'>
            <View className='public-profile-sheet__group'>
              <View className='public-profile-sheet__item' onClick={() => void copyProfilePath()}><Text>复制主页路径</Text></View>
              {!isSelf ? <View className='public-profile-sheet__item public-profile-sheet__item--warning' onClick={reportUser}><Text>举报该用户</Text></View> : null}
              {!isSelf ? <View className='public-profile-sheet__item public-profile-sheet__item--danger' onClick={requestBlock}><Text>拉黑该用户</Text></View> : null}
            </View>
        </View>
      </AppActionSheet>
      <AppDialog
        visible={blockConfirmVisible}
        title='确认拉黑该用户'
        description='拉黑后将不再展示对方的新内容，记录会进入设置中的黑名单。'
        confirmText='确认拉黑'
        tone='danger'
        loading={blockPending}
        onCancel={() => setBlockConfirmVisible(false)}
        onConfirm={() => void confirmBlock()}
      />
    </View>
  )
}

export default withFeaturePage(UserProfilePage, RuntimeSessionGate)
