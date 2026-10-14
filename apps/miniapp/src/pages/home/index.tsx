import { FeaturePageGate } from '../../components/feature-page-gate'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useDidShow } from '@tarojs/taro'
import { Image, ScrollView, Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import AppIcon from '../../components/app-icon'
import { AppState } from '../../components/ui'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import StudentBottomNav from '../../components/student-bottom-nav'
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
import './index.scss'

type HomeTab = 'messages' | 'discover' | 'tools'

type DiscoverCard = {
  id: string
  target: 'post' | 'secondhand'
  category: string
  title: string
  summary: string
  author: string
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
  { key: 'messages', label: '私信' },
  { key: 'discover', label: '发现' },
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
  if (String(context.sellerId) === currentUserId) {
    return `你是卖家 · 对方买家 #${context.buyerId} · 会话 #${context.relationId}`
  }
  if (String(context.buyerId) === currentUserId) {
    return `你是买家 · 对方卖家 #${context.sellerId} · 会话 #${context.relationId}`
  }
  return `交易会话 #${context.relationId}`
}

function HomeContent() {
  const [activeTab, setActiveTab] = useState<HomeTab>('discover')
  const [postCards, setPostCards] = useState<DiscoverCard[]>([])
  const [productCards, setProductCards] = useState<DiscoverCard[]>([])
  const [initialStatus, setInitialStatus] = useState<'loading' | 'ready' | 'empty' | 'error'>('loading')
  const [loadingMore, setLoadingMore] = useState(false)
  const [sourceError, setSourceError] = useState({ community: false, trade: false })
  const [failedImageKeys, setFailedImageKeys] = useState<Set<string>>(() => new Set())
  const [conversationItems, setConversationItems] = useState<ConversationItem[]>([])
  const [conversationStatus, setConversationStatus] = useState<ConversationStatus>('idle')
  const [tools, setTools] = useState<ToolLink[]>([])
  const visibility = useFeatureVisibility()
  usePageShare({ kind: 'home', enabled: visibility.available && activeTab !== 'messages' })
  const [toolStatus, setToolStatus] = useState<ToolStatus>('idle')
  const sourceRef = useRef({ communityPage: 0, tradePage: 0, communityTotal: 0, tradeTotal: 0 })
  const loadingRef = useRef(false)
  const mountedRef = useRef(true)
  const discoverRequestVersion = useRef(0)
  const conversationLoadingRef = useRef(false)
  const conversationRequestVersion = useRef(0)
  const conversationOwnerRef = useRef('')
  const toolLoadingRef = useRef(false)
  const tabScrollPositions = useRef<Record<HomeTab, number>>({ discover: 0, messages: 0, 'tools': 0 })
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
    return tabs.filter((tab) => (
      tab.key === 'messages' ? visibility.messages && visibility.secondhand_chat :
      tab.key === 'discover' ? visibility.forum || visibility.secondhand :
      visibility.campus_tools
    ))
  }, [visibility])

  const loadDiscover = useCallback(async (
    reset = false,
    sources = { community: visibility.forum, trade: visibility.secondhand }
  ) => {
    if (!mountedRef.current || loadingRef.current) return
    loadingRef.current = true
    const version = discoverRequestVersion.current + 1
    discoverRequestVersion.current = version
    reset ? setInitialStatus('loading') : setLoadingMore(true)
    try {
      const state = sourceRef.current
      if (reset) {
        state.communityPage = 0
        state.tradePage = 0
        state.communityTotal = 0
        state.tradeTotal = 0
      }
      const communityPage = reset ? 1 : state.communityPage + 1
      const tradePage = reset ? 1 : state.tradePage + 1
      const requestCommunity = sources.community && (
        reset || state.communityPage === 0 || state.communityPage * 10 < state.communityTotal
      )
      const requestTrade = sources.trade && (
        reset || state.tradePage === 0 || state.tradePage * 10 < state.tradeTotal
      )

      if (!requestCommunity && !requestTrade) return

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
      if (!mountedRef.current || discoverRequestVersion.current !== version) return

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
    ) return
    conversationLoadingRef.current = true
    const version = conversationRequestVersion.current + 1
    conversationRequestVersion.current = version
    setConversationStatus('loading')
    try {
      const contexts = await requestWithSingleSessionRecovery(() => listIMConversations(1, 20))
      const items = await Promise.all(contexts.map(async (context): Promise<ConversationItem> => {
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
      ) return
      setConversationItems(items)
      setConversationStatus(items.length ? 'ready' : 'empty')
    } catch (error) {
      if (
        !mountedRef.current ||
        conversationRequestVersion.current !== version ||
        conversationOwnerRef.current !== ownerKey
      ) return
      setConversationItems([])
      setConversationStatus('error')
      if (isApiError(error) && error.status === 401) {
        void Taro.showToast({ title: '登录恢复失败，请点击重试', icon: 'none' })
      }
    } finally {
      if (
        conversationRequestVersion.current === version &&
        conversationOwnerRef.current === ownerKey
      ) conversationLoadingRef.current = false
    }
  }, [])

  const loadTools = useCallback(async () => {
    if (!mountedRef.current || toolLoadingRef.current) return
    toolLoadingRef.current = true
    setToolStatus('loading')
    try {
      const values = await listNavigationItems()
      if (!mountedRef.current) return
      setTools(values)
      setToolStatus(values.length > 0 ? 'ready' : 'empty')
    } catch {
      if (!mountedRef.current) return
      setTools([])
      setToolStatus('error')
    } finally {
      toolLoadingRef.current = false
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
      toolLoadingRef.current = false
    }
  }, [])

  useEffect(() => {
    discoverRequestVersion.current += 1
    loadingRef.current = false
    tabScrollPositions.current.discover = 0
    setFailedImageKeys(new Set())
    setPostCards([])
    setProductCards([])
    void loadDiscover(true, { community: visibility.forum, trade: visibility.secondhand })
  }, [currentSessionKey, loadDiscover, visibility.forum, visibility.secondhand])

  useEffect(() => {
    const nextTab = visibleTabs.some((tab) => tab.key === activeTab) ? activeTab : visibleTabs[0]?.key
    if (nextTab && nextTab !== activeTab) setActiveTab(nextTab)
  }, [activeTab, visibleTabs])

  useEffect(() => {
    conversationRequestVersion.current += 1
    conversationLoadingRef.current = false
    conversationOwnerRef.current = currentSessionKey
    setConversationItems([])
    setConversationStatus('idle')
    if (activeTab === 'messages' && visibility.messages && visibility.secondhand_chat) void loadConversations(currentSessionKey)
  }, [activeTab, currentSessionKey, loadConversations, visibility.messages, visibility.secondhand_chat])

  useEffect(() => {
    if (activeTab === 'tools' && toolStatus === 'idle' && visibility?.campus_tools) void loadTools()
  }, [activeTab, loadTools, toolStatus, visibility?.campus_tools])

  useDidShow(() => {
    if (activeTab === 'messages' && visibility.messages && visibility.secondhand_chat) void loadConversations(currentSessionKey)
  })

  const openSearch = () => {
    void Taro.navigateTo({
      url: visibility?.forum
        ? '/pages/search/index?scope=community'
        : '/pages/search/index?scope=secondhand'
    })
  }

  const openCard = (card: DiscoverCard) => {
    const url = card.target === 'secondhand'
      ? `/pages/secondhand-detail/index?id=${encodeURIComponent(card.id)}`
      : `/pages/post-detail/index?id=${encodeURIComponent(card.id)}`
    void Taro.navigateTo({ url })
  }

  const openTool = async (tool: ToolLink) => {
    if (!tool.appId || !tool.path) return
    try {
      await Taro.navigateToMiniProgram({ appId: tool.appId, path: tool.path })
    } catch (error) {
      try {
        await Taro.showToast({ title: error instanceof Error ? error.message : '工具打开失败', icon: 'none' })
      } catch {
        // Toast failures must not escape from the fire-and-forget click handler.
      }
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
          {card.target === 'secondhand' && <Text className='student-post-card__price'>¥{card.price}</Text>}
          <View className='student-post-card__meta'>
            <View className='student-post-card__avatar'>
              {card.avatarUrl ? <Image src={card.avatarUrl} mode='aspectFill' lazyLoad /> : <Text>{card.avatar}</Text>}
            </View>
            <Text className='student-post-card__author'>{card.author}</Text>
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
    return (
      <AppState
        kind={pending ? 'loading' : initialStatus === 'empty' ? 'empty' : 'error'}
        title={pending ? '正在读取校园动态' : initialStatus === 'empty' ? '暂时还没有内容' : '内容加载失败'}
        description={pending ? '请稍候…' : initialStatus === 'empty' ? '有新内容时会显示在这里' : '请检查网络后重新试试'}
        actionText={initialStatus === 'error' && !retrying ? '重新加载' : undefined}
        onAction={() => void loadDiscover(false, sourceError)}
      />
    )
  }

  if (!identityAllowed) return <View className='student-home-page' />

  return (
    <View className='page student-home-page'>
      <CapsulePageHeader
        variant='tabs'
        className='student-home-header'
        center={
          <View className='student-home-header__tabs'>
            {visibleTabs.map((tab) => (
              <View
                key={tab.key}
                className={`student-home-header__tab ${activeTab === tab.key ? 'student-home-header__tab--active' : ''}`}
                onClick={() => setActiveTab(tab.key)}
              >
                <Text>{tab.label}</Text>
              </View>
            ))}
          </View>
        }
        right={visibility.search && (visibility.forum || visibility.secondhand) ? (
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

      <ScrollView
        key={activeTab}
        className='student-home-content'
        scrollY
        scrollTop={tabScrollPositions.current[activeTab]}
        onScroll={(event) => { tabScrollPositions.current[activeTab] = event.detail.scrollTop }}
        enhanced
        showScrollbar={false}
        lowerThreshold={120}
        onScrollToLower={() => {
          if (
            activeTab === 'discover' &&
            initialStatus === 'ready' &&
            !sourceError.community &&
            !sourceError.trade &&
            !loadingRef.current
          ) {
            void loadDiscover(false)
          }
        }}
      >
        {activeTab === 'messages' && (
          visibility.messages && visibility.secondhand_chat && (
          <View className='student-message-panel'>
            {conversationStatus === 'loading' || conversationStatus === 'idle' ? (
              <View className='student-message-state'><Text>正在读取真实私信...</Text></View>
            ) : conversationStatus === 'error' ? (
              <View
                className='student-message-state student-message-state--retry'
                onClick={() => void loadConversations(currentSessionKey)}
              >
                <Text>私信加载失败，点击重试</Text>
              </View>
            ) : conversationStatus === 'empty' ? (
              <View className='student-message-state'><Text>还没有二手交易私信</Text></View>
            ) : (
              conversationItems.map((item) => (
                <View
                  key={item.context.relationId}
                  className='student-message-row'
                  onClick={() => void Taro.navigateTo({
                    url: `/pages/secondhand-chat/index?relationId=${encodeURIComponent(String(item.context.relationId))}`
                  })}
                >
                  <View className='student-message-row__avatar'>
                    {item.product?.images[0] ? (
                      <Image className='student-message-row__avatar-image' src={item.product.images[0]} mode='aspectFill' />
                    ) : <Text>聊</Text>}
                  </View>
                  <View className='student-message-row__main'>
                    <Text className='student-message-row__name'>{item.product?.title || `二手商品 #${item.context.productId}`}</Text>
                    <Text className='student-message-row__preview'>
                      {conversationPeerLabel(item.context, currentUserId)}
                    </Text>
                  </View>
                </View>
              ))
            )}
            {conversationStatus === 'ready' && <Text className='student-home-footnote'>仅展示最近 20 个会话</Text>}
          </View>
          )
        )}

        {activeTab === 'discover' && (visibility?.forum || visibility?.secondhand) && (
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
                  onClick={loadingMore ? undefined : () => void loadDiscover(false, sourceError)}
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

        {visibility.campus_tools && <>{activeTab === 'tools' && (
          toolStatus === 'ready' ? (
            <View className='student-tools-grid'>
              {tools.map((tool) => (
                <View key={tool.id} className='student-tool-card' onClick={() => void openTool(tool)}>
                  <View className='student-tool-card__icon' style={{ color: tool.color }}><Text>{tool.iconText}</Text></View>
                  <Text className='student-tool-card__label'>{tool.title}</Text>
                  <Text className='student-tool-card__sub'>{tool.subtitle}</Text>
                </View>
              ))}
            </View>
          ) : toolStatus === 'error' ? (
            <View className='student-tool-state student-tool-state--retry' onClick={() => void loadTools()}>
              <Text>工具加载失败，点击重试</Text>
            </View>
          ) : toolStatus === 'empty' ? (
            <View className='student-tool-state'><Text>暂无可用工具</Text></View>
          ) : (
            <View className='student-tool-state'><Text>正在加载真实工具...</Text></View>
          )
        )}</>}

        {visibleTabs.length === 0 && <View className='student-tool-state'><Text>{visibility.available ? '当前暂无开放的模块，可前往“我的”管理账号。' : '功能状态读取失败，请检查网络后重新进入。'}</Text></View>}
      </ScrollView>

      <StudentBottomNav active='home' />
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
