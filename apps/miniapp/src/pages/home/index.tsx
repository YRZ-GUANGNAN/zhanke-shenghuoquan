import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useDidShow } from '@tarojs/taro'
import { Image, ScrollView, Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import AppIcon from '../../components/app-icon'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import StudentBottomNav from '../../components/student-bottom-nav'
import { useIdentityGuard } from '../../hooks/use-identity-guard'
import type { CommunityPost, Product, ToolLink } from '../../types'
import { isApiError } from '../../services/api'
import { getCommunityPostsPage, type PageResult } from '../../services/b-community'
import { getProduct, getProductsPage, type IMConversationContext, type ProductPageResult } from '../../services/c-trade'
import { listIMConversations } from '../../services/e-im'
import { listNavigationItems } from '../../services/d-navigation'
import { requestWithSingleSessionRecovery } from '../../services/session-recovery'
import Taro from '../../services/taro'
import { useSessionStore } from '../../stores/session'
import './index.scss'

type HomeTab = 'messages' | 'discover' | 'tools'

type DiscoverCard = {
  id: string
  kind: 'post' | 'product'
  target: 'post' | 'secondhand'
  tone: 'sky' | 'mint' | 'warm'
  mediaLabel: string
  title: string
  author: string
  avatar: string
  likes: number
  imageUrl?: string
  priceLabel?: string
  statusLabel?: string
}

type SettledResult<T> =
  | { status: 'fulfilled'; value: T }
  | { status: 'rejected'; reason: unknown }

const tabs: Array<{ key: HomeTab; label: string }> = [
  { key: 'messages', label: '私信' },
  { key: 'discover', label: '发现' },
  { key: 'tools', label: '工具' }
]

const tones: DiscoverCard['tone'][] = ['sky', 'mint', 'warm']

function settle<T>(promise: Promise<T>): Promise<SettledResult<T>> {
  return promise.then(
    (value) => ({ status: 'fulfilled', value }),
    (reason: unknown) => ({ status: 'rejected', reason })
  )
}

function stableNumber(value: string) {
  return Array.from(value).reduce((hash, char) => ((hash * 31) + char.charCodeAt(0)) >>> 0, 7)
}

function visualFor(key: string) {
  const value = stableNumber(key)
  return { tone: tones[value % tones.length] }
}

function mergeUnique(current: DiscoverCard[], incoming: DiscoverCard[]) {
  const values = new Map(current.map((item) => [`${item.target}:${item.id}`, item]))
  incoming.forEach((item) => values.set(`${item.target}:${item.id}`, item))
  return Array.from(values.values())
}

function toPostCard(post: CommunityPost): DiscoverCard {
  const visual = visualFor(`post:${post.id}`)
  return {
    id: post.id,
    kind: 'post',
    target: 'post',
    ...visual,
    mediaLabel: post.topic || '校园动态',
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
    kind: 'product',
    target: 'secondhand',
    ...visual,
    mediaLabel: product.category || '二手好物',
    imageUrl: product.images[0],
    title: product.title,
    author: product.sellerName,
    avatar: product.sellerName.slice(0, 1) || '校',
    likes: product.favoriteCount,
    priceLabel: `¥${product.price}`,
    statusLabel: product.status === 'reserved' ? '已预订' : product.status === 'sold' ? '已售' : '在售'
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
  const left: DiscoverCard[] = []
  const right: DiscoverCard[] = []

  cards.forEach((card, index) => {
    const rowIndex = Math.floor(index / 2)
    const columnIndex = index % 2
    const placeOnLeft = rowIndex % 2 === 0 ? columnIndex === 0 : columnIndex === 1
    if (placeOnLeft) {
      left.push(card)
    } else {
      right.push(card)
    }
  })

  return {
    left,
    right
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
  const [toolStatus, setToolStatus] = useState<ToolStatus>('idle')
  const sourceRef = useRef({ communityPage: 0, tradePage: 0, communityTotal: 0, tradeTotal: 0 })
  const loadingRef = useRef(false)
  const mountedRef = useRef(true)
  const discoverRequestVersion = useRef(0)
  const conversationLoadingRef = useRef(false)
  const conversationRequestVersion = useRef(0)
  const conversationOwnerRef = useRef('')
  const toolLoadingRef = useRef(false)
  const cards = useMemo(
    () => interleaveCards(postCards, productCards),
    [postCards, productCards]
  )
  const cardColumns = useMemo(() => splitCards(cards), [cards])
  const identityAllowed = useIdentityGuard('student')
  const currentUserId = useSessionStore((state) => state.profile.id)
  const currentSessionKey = useSessionStore((state) => `${state.token}:${state.profile.id}`)

  const loadDiscover = useCallback(async (
    reset = false,
    sources = { community: true, trade: true }
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
      if (discoverRequestVersion.current === version) {
        loadingRef.current = false
        if (mountedRef.current) setLoadingMore(false)
      }
    }
  }, [])

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
    void loadDiscover(true)
  }, [loadDiscover])

  useEffect(() => {
    conversationRequestVersion.current += 1
    conversationLoadingRef.current = false
    conversationOwnerRef.current = currentSessionKey
    setConversationItems([])
    setConversationStatus('idle')
    if (activeTab === 'messages') void loadConversations(currentSessionKey)
  }, [activeTab, currentSessionKey, loadConversations])

  useEffect(() => {
    if (activeTab === 'tools' && toolStatus === 'idle') void loadTools()
  }, [activeTab, loadTools, toolStatus])

  useDidShow(() => {
    if (activeTab === 'messages') void loadConversations(currentSessionKey)
  })

  const openSearch = () => {
    void Taro.navigateTo({ url: '/pages/search/index?scope=community' })
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
    const imageKey = `${cardKey}:${card.imageUrl || 'no-image'}`
    const hasUsableImage = Boolean(card.imageUrl) && !failedImageKeys.has(imageKey)
    return (
      <View
        key={cardKey}
        className={`student-media-card student-media-card--${card.kind} ${hasUsableImage ? 'student-media-card--with-image' : 'student-media-card--no-image'}`}
        onClick={() => openCard(card)}
      >
        <View className='student-media-card__visual'>
          {hasUsableImage ? (
            <Image
              className='student-media-card__image'
              src={card.imageUrl!}
              mode='aspectFill'
              onError={() => setFailedImageKeys((current) => {
                const next = new Set(current)
                next.add(imageKey)
                return next
              })}
            />
          ) : (
            <View className={`student-media-card__empty student-media-card__empty--${card.tone}`}>
              <Text className='student-media-card__empty-title'>
                {card.kind === 'product' ? '无图商品' : '文字内容'}
              </Text>
              <Text className='student-media-card__empty-sub'>#{card.mediaLabel}</Text>
            </View>
          )}
          <Text className='student-media-card__badge'>
            {card.kind === 'product' ? `二手 · ${card.statusLabel}` : '帖子'}
          </Text>
        </View>
        <View className='student-media-card__body'>
          <Text className='student-media-card__title'>{card.title}</Text>
          <Text className={`student-media-card__support student-media-card__support--${card.kind}`}>
            {card.kind === 'product' ? card.priceLabel : `#${card.mediaLabel}`}
          </Text>
          <View className='student-card-meta'>
            <View className={`student-card-meta__avatar student-card-meta__avatar--${card.tone}`}>
              <Text>{card.avatar}</Text>
            </View>
            <Text className='student-card-meta__author'>{card.author}</Text>
            <View className='student-card-meta__like'>
              <AppIcon
                name={card.kind === 'product' ? 'star' : 'heart'}
                className='student-card-meta__heart'
              />
              <Text>{card.likes}</Text>
            </View>
          </View>
        </View>
      </View>
    )
  }

  const renderDiscoverState = () => {
    const retrying = initialStatus === 'error' && loadingMore
    const copy = initialStatus === 'loading' || retrying
      ? { media: '正在加载', title: retrying ? '正在重新连接社区与二手集市...' : '正在加载校园新鲜事...' }
      : initialStatus === 'empty'
        ? { media: '暂无内容', title: '社区与二手集市暂时还没有内容' }
        : { media: '加载失败', title: '社区与二手集市加载失败，点击重试' }
    return (
      <View className='student-discover-state-wrap'>
        <View
          className={`student-discover-state ${initialStatus === 'error' && !retrying ? 'student-discover-state--retry' : ''}`}
          onClick={initialStatus === 'error' && !retrying ? () => void loadDiscover(false, sourceError) : undefined}
        >
          <View className='student-discover-state__icon'><Text>{copy.media.slice(0, 1)}</Text></View>
          <Text className='student-discover-state__title'>{copy.media}</Text>
          <Text className='student-discover-state__desc'>{copy.title}</Text>
        </View>
      </View>
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
            {tabs.map((tab) => (
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
        right={<View className='student-home-header__search' onClick={openSearch} />}
      />

      <ScrollView
        key={activeTab}
        className='student-home-content'
        scrollY
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
        )}

        {activeTab === 'discover' && (
          initialStatus !== 'ready' ? renderDiscoverState() : (
            <View className='student-discover-content'>
              <View className='student-discover-feed'>
                {cards.length > 0 ? (
                  <View className='student-media-grid'>
                    <View className='student-media-grid__column'>
                      {cardColumns.left.map(renderDiscoverCard)}
                    </View>
                    <View className='student-media-grid__column'>
                      {cardColumns.right.map(renderDiscoverCard)}
                    </View>
                  </View>
                ) : null}
                {cards.length === 0 ? (
                  <View className='student-discover-state'>
                    <View className='student-discover-state__icon'><Text>空</Text></View>
                    <Text className='student-discover-state__title'>暂时没有可展示的内容</Text>
                    <Text className='student-discover-state__desc'>新帖子和二手商品会在发布后出现在这里。</Text>
                  </View>
                ) : null}
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
            </View>
          )
        )}

        {activeTab === 'tools' && (
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
        )}

      </ScrollView>

      <StudentBottomNav active='home' />
    </View>
  )
}

export default function HomePage() {
  return (
    <RuntimeSessionGate preserveAuthenticatedChildrenDuringRetry>
      <HomeContent />
    </RuntimeSessionGate>
  )
}
