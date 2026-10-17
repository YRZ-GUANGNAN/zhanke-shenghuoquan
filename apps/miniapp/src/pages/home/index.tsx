import ProductPrice from '../../components/product-price'
import { FeaturePageGate } from '../../components/feature-page-gate'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useDidShow } from '@tarojs/taro'
import { Image, Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import AppIcon from '../../components/app-icon'
import ChatAvatar from '../../components/chat-avatar'
import ChannelPager, { useChannelInteraction } from '../../components/channel-pager'
import { AppSkeleton, AppState } from '../../components/ui'
import RefreshScrollView from '../../components/refresh-scroll-view'
import ToolDirectory from '../../components/tool-directory'
import { isMiniProgramJumpCancelled } from '../../utils/mini-program-jump'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import StudentBottomNav from '../../components/student-bottom-nav'
import StudentSecondhandContent from '../student-secondhand/content'
import CampusInfoHome from '../../components/campus-info-home'
import { useIdentityGuard } from '../../hooks/use-identity-guard'
import type { CommunityPost, Product, ToolLink } from '../../types'
import { isApiError } from '../../services/api'
import { getCommunityPostsPage, type PageResult } from '../../services/b-community'
import { getProduct, getProductsPage, type IMConversationContext, type ProductPageResult } from '../../services/c-trade'
import { listIMConversations } from '../../services/e-im'
import { listNavigationItems } from '../../services/d-navigation'
import { requestWithSingleSessionRecovery } from '../../services/session-recovery'
import { useFeatureVisibility } from '../../hooks/use-feature-visibility'
import { usePageShare } from '../../hooks/use-page-share'
import Taro from '../../services/taro'
import { useSessionStore } from '../../stores/session'
import { postBodyWithoutTitle } from '../../utils/post-presentation'
import { navigateToPublicProfile } from '../../utils/navigation'
import { conversationPeer, uniquePeerConversations } from '../../utils/conversation-presentation'
import './index.scss'

type HomeTab = 'discover' | 'secondhand' | 'tools' | 'messages'

type DiscoverCard = {
  id: string
  target: 'post' | 'secondhand'
  category: string
  title: string
  summary: string
  author: string
  authorId: string
  avatar: string
  avatarUrl: string
  likes: number
  mediaCount: number
  price?: number
  imageUrl?: string
}

type SettledResult<T> =
  | { status: 'fulfilled'; value: T }
  | { status: 'rejected'; reason: unknown }

const tabs: Array<{ key: HomeTab; label: string }> = [
  { key: 'discover', label: '发现' },
  { key: 'secondhand', label: '二手' },
  { key: 'tools', label: '工具' }
]

function settle<T>(promise: Promise<T>): Promise<SettledResult<T>> {
  return promise.then(
    (value) => ({ status: 'fulfilled', value }),
    (reason: unknown) => ({ status: 'rejected', reason })
  )
}

function mergeUnique(current: DiscoverCard[], incoming: DiscoverCard[]) {
  const values = new Map(current.map((item) => [`${item.target}:${item.id}`, item]))
  incoming.forEach((item) => values.set(`${item.target}:${item.id}`, item))
  return Array.from(values.values())
}

function toPostCard(post: CommunityPost): DiscoverCard {
  return {
    id: post.id,
    target: 'post',
    category: post.topic || '校园帖子',
    imageUrl: post.images[0],
    mediaCount: post.images.length,
    title: post.title || post.content,
    summary: postBodyWithoutTitle(post.title, post.content),
    author: post.author,
    authorId: post.authorId,
    avatar: post.author.slice(0, 1) || '校',
    avatarUrl: post.avatar,
    likes: post.likedCount
  }
}

function toProductCard(product: Product): DiscoverCard {
  return {
    id: product.id,
    target: 'secondhand',
    category: `二手 · ${product.category || '闲置'}`,
    imageUrl: product.images[0],
    mediaCount: product.images.length,
    title: product.title,
    summary: product.description,
    author: product.sellerName,
    authorId: product.sellerId,
    avatar: product.sellerName.slice(0, 1) || '校',
    avatarUrl: product.sellerAvatar,
    price: product.price,
    likes: product.favoriteCount
  }
}

function interleaveCards(posts: DiscoverCard[], products: DiscoverCard[]) {
  const result: DiscoverCard[] = []
  const length = Math.max(posts.length, products.length)
  for (let index = 0; index < length; index += 1) {
    if (posts[index]) result.push(posts[index])
    if (products[index]) result.push(products[index])
  }
  return result
}

function splitCards(cards: DiscoverCard[]) {
  return {
    left: cards.filter((_, index) => index % 2 === 0),
    right: cards.filter((_, index) => index % 2 === 1)
  }
}

type ConversationItem = {
  context: IMConversationContext
  product: Product | null
}

type ConversationStatus = 'idle' | 'loading' | 'empty' | 'ready' | 'error'
type ToolStatus = 'idle' | 'loading' | 'ready' | 'empty' | 'error'

function conversationPeerLabel(context: IMConversationContext, currentUserId: string): string {
  if (context.referenceType === 'user') return '点击继续交流'
  if (String(context.sellerId) === currentUserId) {
    return '你是卖家 · 二手交易'
  }
  if (String(context.buyerId) === currentUserId) {
    return '你是买家 · 二手交易'
  }
  return '二手交易'
}

function HomeContent() {
  // Keep this page's entry mode when a pushed page becomes the current router.
  const [isMessagesPage] = useState(() => Taro.getCurrentInstance().router?.params.tab === 'messages')
  const [activeTab, setActiveTab] = useState<HomeTab>(isMessagesPage ? 'messages' : 'discover')
  const [marketVisited, setMarketVisited] = useState(false)
  const channelInteraction = useChannelInteraction()
  const [postCards, setPostCards] = useState<DiscoverCard[]>([])
  const [productCards, setProductCards] = useState<DiscoverCard[]>([])
  const [initialStatus, setInitialStatus] = useState<'loading' | 'ready' | 'empty' | 'error'>('loading')
  const [loadingMore, setLoadingMore] = useState(false)
  const [sourceError, setSourceError] = useState({ community: false, trade: false })
  const [failedImageKeys, setFailedImageKeys] = useState<Set<string>>(() => new Set())
  const [conversationItems, setConversationItems] = useState<ConversationItem[]>([])
  const [conversationStatus, setConversationStatus] = useState<ConversationStatus>('idle')
  const [tools, setTools] = useState<ToolLink[]>([])
  const [toolRefreshError, setToolRefreshError] = useState(false)
  const [conversationRefreshError, setConversationRefreshError] = useState(false)
  const [toolBusy, setToolBusy] = useState(false)
  const [conversationBusy, setConversationBusy] = useState(false)
  const [jumpingTool, setJumpingTool] = useState<string | null>(null)
  const toolJumpLock = useRef(false)
  const failedRefresh = useRef({ community: false, trade: false })
  const visibility = useFeatureVisibility()
  usePageShare({ kind: activeTab === 'secondhand' ? 'secondhand' : 'home', enabled: visibility.available && !isMessagesPage })
  const [toolStatus, setToolStatus] = useState<ToolStatus>('idle')
  const sourceRef = useRef({ communityPage: 0, tradePage: 0, communityTotal: 0, tradeTotal: 0 })
  const loadingRef = useRef(false)
  const mountedRef = useRef(true)
  const discoverRequestVersion = useRef(0)
  const conversationLoadingRef = useRef(false)
  const conversationRequestVersion = useRef(0)
  const conversationOwnerRef = useRef('')
  const toolLoadingRef = useRef(false)
  const toolRequestVersion = useRef(0)
  const tabScrollPositions = useRef<Record<HomeTab, number>>({ discover: 0, secondhand: 0, messages: 0, 'tools': 0 })
  const cards = useMemo(
    () => interleaveCards(visibility.forum ? postCards : [], visibility.secondhand ? productCards : []),
    [postCards, productCards, visibility.forum, visibility.secondhand]
  )
  const columns = useMemo(() => splitCards(cards), [cards])
  const identityAllowed = useIdentityGuard('student')
  const currentUserId = useSessionStore((state) => state.profile.id)
  const currentSessionKey = useSessionStore((state) => `${state.token}:${state.profile.id}`)
  const visibleTabs = useMemo(() => {
    if (!visibility) return []
    if (isMessagesPage) return visibility.messages
      ? [{ key: 'messages' as const, label: '私信' }] : []
    return tabs.filter((tab) => (
      tab.key === 'secondhand' ? visibility.secondhand :
      tab.key === 'discover' ? visibility.forum || visibility.secondhand :
      visibility.campus_tools
    ))
  }, [isMessagesPage, visibility])
  const channelKeys = useMemo(() => visibleTabs.map((tab) => tab.key), [visibleTabs])

  useEffect(() => {
    if (activeTab === 'secondhand' && visibility.secondhand) setMarketVisited(true)
  }, [activeTab, visibility.secondhand])

  const loadDiscover = useCallback(async (
    reset = false,
    sources = { community: visibility.forum, trade: visibility.secondhand }
  ) => {
    if (!mountedRef.current || loadingRef.current) return false
    loadingRef.current = true
    const version = discoverRequestVersion.current + 1
    discoverRequestVersion.current = version
    if (reset && sourceRef.current.communityPage === 0 && sourceRef.current.tradePage === 0) setInitialStatus('loading')
    else setLoadingMore(true)
    try {
      const state = sourceRef.current
      const resetCommunity = reset || failedRefresh.current.community
      const resetTrade = reset || failedRefresh.current.trade
      const communityPage = resetCommunity ? 1 : state.communityPage + 1
      const tradePage = resetTrade ? 1 : state.tradePage + 1
      const requestCommunity = sources.community && (
        resetCommunity || state.communityPage === 0 || state.communityPage * 10 < state.communityTotal
      )
      const requestTrade = sources.trade && (
        resetTrade || state.tradePage === 0 || state.tradePage * 10 < state.tradeTotal
      )

      if (!requestCommunity && !requestTrade) return true

      const communityRequest: Promise<PageResult<CommunityPost> | null> = requestCommunity
        ? getCommunityPostsPage({ page: communityPage, sort: 'recent', pageSize: 10 })
        : Promise.resolve(null)
      const tradeRequest: Promise<ProductPageResult | null> = requestTrade
        ? getProductsPage({ page: tradePage, pageSize: 10, status: 'selling' })
        : Promise.resolve(null)
      const results = await Promise.all([
        settle(communityRequest),
        settle(tradeRequest)
      ])
      if (!mountedRef.current || discoverRequestVersion.current !== version) return false

      const community = results[0].status === 'fulfilled' ? results[0].value : null
      const trade = results[1].status === 'fulfilled' ? results[1].value : null
      const communityFailed = requestCommunity && results[0].status === 'rejected'
      const tradeFailed = requestTrade && results[1].status === 'rejected'
      const nextPosts = (community?.items ?? []).map(toPostCard)
      const nextProducts = (trade?.items ?? []).map(toProductCard)

      if (community) {
        state.communityPage = community.page
        state.communityTotal = community.total
        setPostCards((current) => mergeUnique(resetCommunity ? [] : current, nextPosts))
        failedRefresh.current.community = false
      } else if (communityFailed && resetCommunity) {
        failedRefresh.current.community = true
      }
      if (trade) {
        state.tradePage = trade.page
        state.tradeTotal = trade.total
        setProductCards((current) => mergeUnique(resetTrade ? [] : current, nextProducts))
        failedRefresh.current.trade = false
      } else if (tradeFailed && resetTrade) {
        failedRefresh.current.trade = true
      }

      setSourceError(current => ({
        community: requestCommunity ? communityFailed : current.community,
        trade: requestTrade ? tradeFailed : current.trade
      }))
      if (reset && !communityFailed && !tradeFailed) setFailedImageKeys(new Set())
      const hasLoadedSource = state.communityPage > 0 || state.tradePage > 0
      const allRequestedSourcesFailed = (
        (!requestCommunity || communityFailed) &&
        (!requestTrade || tradeFailed)
      )
      if (!hasLoadedSource && allRequestedSourcesFailed) {
        setInitialStatus('error')
      } else if (
        !communityFailed &&
        !tradeFailed &&
        (!sources.community || (state.communityPage > 0 && state.communityTotal === 0)) &&
        (!sources.trade || (state.tradePage > 0 && state.tradeTotal === 0))
      ) {
        setInitialStatus('empty')
      } else {
        setInitialStatus('ready')
      }
      return !communityFailed && !tradeFailed
    } finally {
      if (discoverRequestVersion.current === version) {
        loadingRef.current = false
        if (mountedRef.current) setLoadingMore(false)
      }
    }
  }, [visibility.forum, visibility.secondhand])

  const loadConversations = useCallback(async (ownerKey: string) => {
    if (
      !mountedRef.current ||
      conversationLoadingRef.current ||
      conversationOwnerRef.current !== ownerKey
    ) return false
    conversationLoadingRef.current = true
    setConversationBusy(true)
    const version = conversationRequestVersion.current + 1
    conversationRequestVersion.current = version
    setConversationRefreshError(false)
    setConversationStatus((current) => current === 'ready' || current === 'empty' ? current : 'loading')
    try {
      const contexts = await requestWithSingleSessionRecovery(() => listIMConversations(1, 20))
      const items = await Promise.all(uniquePeerConversations(contexts).map(async (context): Promise<ConversationItem> => {
        if (context.referenceType === 'user') return { context, product: null }
        try {
          return { context, product: await getProduct(String(context.productId)) }
        } catch {
          return { context, product: null }
        }
      }))
      if (
        !mountedRef.current ||
        conversationRequestVersion.current !== version ||
        conversationOwnerRef.current !== ownerKey
      ) return false
      setConversationItems(items)
      setConversationStatus(items.length ? 'ready' : 'empty')
      return true
    } catch (error) {
      if (
        !mountedRef.current ||
        conversationRequestVersion.current !== version ||
        conversationOwnerRef.current !== ownerKey
      ) return false
      setConversationRefreshError(true)
      setConversationStatus(current => current === 'ready' || current === 'empty' ? current : 'error')
      if (isApiError(error) && error.status === 401) {
        void Taro.showToast({ title: '登录恢复失败，请点击重试', icon: 'none' })
      }
      return false
    } finally {
      if (
        conversationRequestVersion.current === version &&
        conversationOwnerRef.current === ownerKey
      ) {
        conversationLoadingRef.current = false
        if (mountedRef.current) setConversationBusy(false)
      }
    }
  }, [])

  const loadTools = useCallback(async () => {
    if (!mountedRef.current || toolLoadingRef.current) return false
    toolLoadingRef.current = true
    setToolBusy(true)
    const version = ++toolRequestVersion.current
    setToolRefreshError(false)
    setToolStatus(current => current === 'ready' || current === 'empty' ? current : 'loading')
    try {
      const values = await listNavigationItems()
      if (!mountedRef.current || toolRequestVersion.current !== version) return false
      setTools(values)
      setToolStatus(values.length > 0 ? 'ready' : 'empty')
      return true
    } catch {
      if (!mountedRef.current || toolRequestVersion.current !== version) return false
      setToolRefreshError(true)
      setToolStatus(current => current === 'ready' || current === 'empty' ? current : 'error')
      return false
    } finally {
      if (toolRequestVersion.current === version) {
        toolLoadingRef.current = false
        if (mountedRef.current) setToolBusy(false)
      }
    }
  }, [])

  useEffect(() => {
    mountedRef.current = true
    return () => {
      mountedRef.current = false
      discoverRequestVersion.current += 1
      loadingRef.current = false
      conversationRequestVersion.current += 1
      conversationLoadingRef.current = false
      toolRequestVersion.current += 1
      toolLoadingRef.current = false
    }
  }, [])

  useEffect(() => {
    discoverRequestVersion.current += 1
    loadingRef.current = false
    sourceRef.current = { communityPage: 0, tradePage: 0, communityTotal: 0, tradeTotal: 0 }
    tabScrollPositions.current.discover = 0
    setFailedImageKeys(new Set())
    setPostCards([])
    setProductCards([])
    setInitialStatus('loading')
    setLoadingMore(false)
    setSourceError({ community: false, trade: false })
    failedRefresh.current = { community: false, trade: false }
  }, [currentSessionKey, visibility.forum, visibility.secondhand])

  useEffect(() => {
    if (activeTab === 'discover' && initialStatus === 'loading' && (visibility.forum || visibility.secondhand)) {
      void loadDiscover(true, { community: visibility.forum, trade: visibility.secondhand })
    }
  }, [activeTab, currentSessionKey, initialStatus, loadDiscover, visibility.forum, visibility.secondhand])

  useEffect(() => {
    const nextTab = visibleTabs.some((tab) => tab.key === activeTab) ? activeTab : visibleTabs[0]?.key
    if (nextTab && nextTab !== activeTab) setActiveTab(nextTab)
  }, [activeTab, visibleTabs])

  useEffect(() => {
    conversationRequestVersion.current += 1
    conversationLoadingRef.current = false
    conversationOwnerRef.current = visibility.messages ? currentSessionKey : ''
    tabScrollPositions.current.messages = 0
    setConversationItems([])
    setConversationStatus('idle')
    setConversationRefreshError(false)
    setConversationBusy(false)
  }, [currentSessionKey, visibility.messages, visibility.secondhand_chat])

  useEffect(() => {
    toolRequestVersion.current += 1
    toolLoadingRef.current = false
    setTools([])
    setToolStatus('idle')
    setToolRefreshError(false)
    setToolBusy(false)
  }, [currentSessionKey, visibility.campus_tools])

  useEffect(() => {
    if (activeTab === 'messages' && conversationStatus === 'idle' && visibility.messages) {
      void loadConversations(currentSessionKey)
    }
  }, [activeTab, conversationStatus, currentSessionKey, loadConversations, visibility.messages, visibility.secondhand_chat])

  useEffect(() => {
    if (activeTab === 'tools' && toolStatus === 'idle' && visibility?.campus_tools) void loadTools()
  }, [activeTab, loadTools, toolStatus, visibility?.campus_tools])

  useDidShow(() => {
    if (activeTab === 'messages' && visibility.messages) void loadConversations(currentSessionKey)
  })

  const openSearch = () => {
    if (!channelInteraction.canActivate()) return
    void Taro.navigateTo({
      url: activeTab !== 'secondhand' && visibility?.forum
        ? '/pages/search/index?scope=community'
        : '/pages/search/index?scope=secondhand'
    })
  }

  const openCard = (card: DiscoverCard) => {
    if (!channelInteraction.canActivate()) return
    const url = card.target === 'secondhand'
      ? `/pages/secondhand-detail/index?id=${encodeURIComponent(card.id)}`
      : `/pages/post-detail/index?id=${encodeURIComponent(card.id)}`
    void Taro.navigateTo({ url })
  }

  const openTool = async (tool: ToolLink) => {
    if (!channelInteraction.canActivate() || !tool.appId || !tool.path || toolJumpLock.current) return
    toolJumpLock.current = true
    setJumpingTool(tool.id)
    try {
      await Taro.navigateToMiniProgram({ appId: tool.appId, path: tool.path })
    } catch (error) {
      if (isMiniProgramJumpCancelled(error)) return
      try {
        await Taro.showToast({ title: '工具暂时无法打开，请稍后再试', icon: 'none' })
      } catch {
        // Toast failures must not escape from the fire-and-forget click handler.
      }
    } finally {
      toolJumpLock.current = false
      if (mountedRef.current) setJumpingTool(null)
    }
  }

  const renderDiscoverCard = (card: DiscoverCard) => {
    const cardKey = `${card.target}:${card.id}`
    const imageKey = `${cardKey}:${card.imageUrl}`
    return (
      <View
        key={cardKey}
        className={`student-post-card${card.imageUrl ? '' : ' student-post-card--text'}`}
        role='button'
        aria-label={card.title}
        onClick={() => openCard(card)}
      >
        {card.imageUrl ? <View className={`student-post-card__media student-post-card__media--${card.target}`}>
          {!failedImageKeys.has(imageKey) ? (
            <Image
              className='student-post-card__image'
              src={card.imageUrl}
              mode='aspectFill'
              lazyLoad
              onError={() => setFailedImageKeys((current) => {
                const next = new Set(current)
                next.add(imageKey)
                return next
              })}
            />
          ) : (
            <Text className='student-post-card__media-error'>图片暂时无法显示</Text>
          )}
          {card.mediaCount > 1 && !failedImageKeys.has(imageKey) && <Text className='student-post-card__media-count'>{card.mediaCount} 图</Text>}
        </View> : null}
        <View className='student-post-card__body'>
          <Text className='student-post-card__category'>{card.category}</Text>
          <Text className='student-post-card__title'>{card.title}</Text>
          {!card.imageUrl && card.summary && card.summary !== card.title && <Text className='student-post-card__summary'>{card.summary}</Text>}
          {card.target === 'secondhand' && <ProductPrice className='student-post-card__price' value={card.price} />}
          <View className='student-post-card__meta'>
            <View className='student-post-card__profile' ariaRole={visibility.forum ? 'button' : undefined} ariaLabel={`查看${card.author}的主页`} onClick={(event) => {
              event.stopPropagation()
              if (visibility.forum && channelInteraction.canActivate()) void navigateToPublicProfile(card.authorId, card.target === 'post' ? 'posts' : 'secondhand')
            }}>
              <View className='student-post-card__avatar'>
                {card.avatarUrl ? <Image src={card.avatarUrl} mode='aspectFill' lazyLoad /> : <Text>{card.avatar}</Text>}
              </View>
              <Text className='student-post-card__author'>{card.author}</Text>
            </View>
            {(card.target === 'secondhand' ? visibility.favorites : visibility.likes) && <View className='student-post-card__like'>
              <AppIcon name={card.target === 'secondhand' ? 'star' : 'heart'} className='student-post-card__heart' />
              <Text>{card.likes}</Text>
            </View>}
          </View>
        </View>
      </View>
    )
  }

  const renderDiscoverState = () => {
    const retrying = initialStatus === 'error' && loadingMore
    const pending = initialStatus === 'loading' || retrying
    if (pending) return <AppSkeleton variant='feed' rows={4} />
    return (
      <AppState
        kind={pending ? 'loading' : initialStatus === 'empty' ? 'empty' : 'error'}
        title={pending ? '正在读取校园动态' : initialStatus === 'empty' ? '暂时还没有内容' : '内容加载失败'}
        description={pending ? '请稍候…' : initialStatus === 'empty' ? '有新内容时会显示在这里' : '请检查网络后重新试试'}
        actionText={initialStatus === 'error' && !retrying ? '重新加载' : undefined}
        onAction={() => { if (channelInteraction.canActivate()) void loadDiscover(false, sourceError) }}
      />
    )
  }

  if (!identityAllowed) return <View className='student-home-page' />

  return (
    <View className='page student-home-page'>
      <CapsulePageHeader
        variant={isMessagesPage ? 'standard' : 'tabs'}
        className='student-home-header'
        center={isMessagesPage ? <Text className='student-home-header__title'>私信</Text> :
          <View className='student-home-header__tabs'>
            {visibleTabs.map((tab) => (
              <View
                key={tab.key}
                className={`student-home-header__tab ${activeTab === tab.key ? 'student-home-header__tab--active' : ''}`}
                onClick={() => { if (channelInteraction.canActivate()) setActiveTab(tab.key) }}
              >
                <Text>{tab.label}</Text>
              </View>
            ))}
          </View>
        }
        right={!isMessagesPage && visibility.search && (visibility.forum || visibility.secondhand) ? (
          <View
            className='student-home-header__search'
            onClick={openSearch}
            hoverClass='student-home-header__search--pressed'
            ariaRole='button'
            ariaLabel='搜索'
          >
            <AppIcon name='search' className='student-home-header__search-icon' />
          </View>
        ) : undefined}
      />

      {channelKeys.length > 0 ? <ChannelPager
        keys={channelKeys}
        activeKey={activeTab}
        onChange={setActiveTab}
        interaction={channelInteraction}
        renderPage={(channel) => channel === 'secondhand' ? (
          marketVisited && visibility.secondhand ? <StudentSecondhandContent
            key={currentSessionKey}
            embedded
            active={activeTab === 'secondhand'}
            interaction={channelInteraction}
          /> : <AppState kind='loading' title='正在准备二手集市' />
        ) : (
      <RefreshScrollView
        key={channel}
        enabled={channel === activeTab}
        busy={channel === 'discover' ? loadingMore || initialStatus === 'loading' : channel === 'messages' ? conversationBusy : toolBusy}
        refreshKey={`${currentSessionKey}:${channel}:${visibility.forum}:${visibility.secondhand}`}
        onRefresh={() => channel === 'discover' ? loadDiscover(true) : channel === 'messages' ? loadConversations(currentSessionKey) : loadTools()}
        className='student-home-content'
        scrollY
        scrollTop={tabScrollPositions.current[channel]}
        onScroll={(event) => { tabScrollPositions.current[channel] = event.detail.scrollTop }}
        enhanced
        showScrollbar={false}
        lowerThreshold={120}
        onScrollToLower={() => {
          if (
            channel === activeTab && channel === 'discover' &&
            initialStatus === 'ready' &&
            !sourceError.community &&
            !sourceError.trade &&
            !loadingRef.current
          ) {
            void loadDiscover(false)
          }
        }}
      >
        {channel === 'messages' && (
          visibility.messages && (
          <View className='student-message-panel'>
            {conversationStatus === 'loading' || conversationStatus === 'idle' ? (
              <AppSkeleton rows={4} />
            ) : conversationStatus === 'error' ? (
              <AppState kind='error' title='私信加载失败' actionText='重新加载' onAction={() => { if (channelInteraction.canActivate()) void loadConversations(currentSessionKey) }} />
            ) : conversationStatus === 'empty' ? (
              <AppState kind='empty' title='还没有私信会话' description='从用户主页发私信，或从商品详情联系卖家后，可在这里继续交流。' />
            ) : (
              conversationItems.map((item) => {
                const peer = conversationPeer(item.context, item.product, currentUserId)
                return (
                <View
                  key={item.context.relationId}
                  className='student-message-row'
                  hoverClass='student-message-row--pressed'
                  role='button'
                  aria-label={`与${peer.name}的私信${item.product ? `，${item.product.title}` : ''}`}
                  onClick={() => {
                    if (!channelInteraction.canActivate()) return
                    void Taro.navigateTo({
                      url: `/pages/secondhand-chat/index?relationId=${encodeURIComponent(String(item.context.relationId))}`
                    })
                  }}
                >
                  <ChatAvatar className='student-message-row__avatar' src={peer.avatar} name={peer.name} />
                  <View className='student-message-row__main'>
                    <View className='student-message-row__top'>
                      <Text className='student-message-row__name'>{peer.name}</Text>
                      <AppIcon name='chevron-right' className='student-message-row__chevron' />
                    </View>
                    <Text className='student-message-row__product'>
                      {item.context.referenceType === 'user' ? '用户私信' : item.product?.title || '相关二手商品'}
                    </Text>
                    <Text className='student-message-row__preview'>
                      {conversationPeerLabel(item.context, currentUserId)}
                    </Text>
                  </View>
                  {item.context.referenceType !== 'user' && <View className='student-message-row__product-thumb'>
                    {item.product?.images[0] ? <Image src={item.product.images[0]} mode='aspectFill' /> : <AppIcon name='market' className='student-message-row__product-placeholder' />}
                  </View>}
                </View>
                )
              })
            )}
            {conversationStatus === 'ready' && <Text className='student-home-footnote'>仅展示最近 20 个会话</Text>}
            {conversationRefreshError && conversationStatus !== 'error' && <View className='student-discover-feedback student-discover-feedback--retry' onClick={() => void loadConversations(currentSessionKey)}><Text>更新失败，已保留当前会话 · 重试</Text></View>}
          </View>
          )
        )}

        {channel === 'discover' && (visibility?.forum || visibility?.secondhand) && (
          initialStatus !== 'ready' ? renderDiscoverState() : (
            <>
                <View className='student-waterfall'>
                <View className='student-waterfall__column'>
                  {columns.left.map(renderDiscoverCard)}
                </View>
                <View className='student-waterfall__column'>
                  {columns.right.map(renderDiscoverCard)}
                </View>
              </View>
              {(sourceError.community || sourceError.trade) ? (
                <View
                  className={`student-discover-feedback ${loadingMore ? '' : 'student-discover-feedback--retry'}`}
                  onClick={loadingMore ? undefined : () => { if (channelInteraction.canActivate()) void loadDiscover(false, sourceError) }}
                >
                  <Text>{loadingMore ? '正在重试加载...' : '部分内容加载失败，点击重试'}</Text>
                </View>
              ) : loadingMore ? (
                <View className='student-discover-feedback'>
                  <Text>正在加载更多...</Text>
                </View>
              ) : null}
            </>
          )
        )}

        {visibility.campus_tools && <>{channel === 'tools' && (
          <ToolDirectory tools={tools} status={toolStatus} jumpingId={jumpingTool} refreshError={toolRefreshError}
            onOpen={tool => void openTool(tool)} onRetry={() => { if (channelInteraction.canActivate()) void loadTools() }} />
        )}</>}

      </RefreshScrollView>
        )}
      /> : <View className='student-tool-state'><Text>{visibility.available ? '当前暂无开放的模块，可前往“我的”管理账号。' : '功能状态读取失败，请检查网络后重新进入。'}</Text></View>}

      <StudentBottomNav active={isMessagesPage ? 'messages' : 'home'} />
    </View>
  )
}

export default function HomePage() {
  const visibility = useFeatureVisibility()
  return (
    <RuntimeSessionGate preserveAuthenticatedChildrenDuringRetry>
      <FeaturePageGate>
        {visibility.presentationMode === 'campus_info' ? <CampusInfoHome /> : <HomeContent />}
      </FeaturePageGate>
    </RuntimeSessionGate>
  )
}
