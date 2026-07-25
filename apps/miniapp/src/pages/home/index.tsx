import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import TaroRuntime, { useDidShow } from '@tarojs/taro'
import { Image, Text, View } from '@tarojs/components'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import StudentBottomNav from '../../components/student-bottom-nav'
import CapsulePageHeader from '../../components/capsule-page-header'
import { useIdentityGuard } from '../../hooks/use-identity-guard'
import type { CommunityPost, Product, ToolLink } from '../../types'
import { isApiError } from '../../services/api'
import { getCommunityPostsPage, type PageResult } from '../../services/b-community'
import { getProduct, getProductsPage, type IMConversationContext, type ProductPageResult } from '../../services/c-trade'
import { listIMConversations } from '../../services/e-im'
import { listNavigationItems } from '../../services/d-navigation'
import { requestWithSingleSessionRecovery } from '../../services/session-recovery'
import Taro from '../../services/taro'
import './index.scss'

type HomeTab = 'messages' | 'discover' | 'tools'

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

type SettledResult<T> =
  | { status: 'fulfilled'; value: T }
  | { status: 'rejected'; reason: unknown }

type RuntimeIntersectionObserver = {
  relativeTo(selector: string): RuntimeIntersectionObserver
  observe(selector: string, callback: (entry: { intersectionRatio: number }) => void): void
  disconnect(): void
}

type RuntimeSelectorQuery = {
  in(component: object): RuntimeSelectorQuery
  select(selector: string): {
    boundingClientRect(callback: (rect: object | null) => void): RuntimeSelectorQuery
  }
  exec(): void
}

type ObserverRuntime = {
  getCurrentInstance(): { page?: object }
  createIntersectionObserver(page: object, options: { thresholds: number[] }): RuntimeIntersectionObserver
  createSelectorQuery(): RuntimeSelectorQuery
  nextTick(callback: () => void): void
}

const observerRuntime = TaroRuntime as unknown as ObserverRuntime

const tabs: Array<{ key: HomeTab; label: string }> = [
  { key: 'messages', label: '私信' },
  { key: 'discover', label: '发现' },
  { key: 'tools', label: '工具' }
]

const tones: DiscoverCard['tone'][] = ['peach', 'sky', 'mint', 'lilac', 'lemon', 'cyan']
const heights: DiscoverCard['height'][] = ['medium', 'tall', 'medium', 'medium', 'short']

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
  return { tone: tones[value % tones.length], height: heights[value % heights.length] }
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
    likes: product.favoriteCount,
    price: String(product.price)
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
  const toolLoadingRef = useRef(false)
  const cards = useMemo(
    () => interleaveCards(postCards, productCards),
    [postCards, productCards]
  )
  const columns = useMemo(() => splitCards(cards), [cards])
  const identityAllowed = useIdentityGuard('student')

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

  const loadConversations = useCallback(async () => {
    if (!mountedRef.current || conversationLoadingRef.current) return
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
      if (!mountedRef.current || conversationRequestVersion.current !== version) return
      setConversationItems(items)
      setConversationStatus(items.length ? 'ready' : 'empty')
    } catch (error) {
      if (!mountedRef.current || conversationRequestVersion.current !== version) return
      setConversationItems([])
      setConversationStatus('error')
      if (isApiError(error) && error.status === 401) {
        void Taro.showToast({ title: '登录恢复失败，请点击重试', icon: 'none' })
      }
    } finally {
      if (conversationRequestVersion.current === version) conversationLoadingRef.current = false
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
    if (activeTab === 'messages' && conversationStatus === 'idle') void loadConversations()
  }, [activeTab, conversationStatus, loadConversations])

  useEffect(() => {
    if (activeTab === 'tools' && toolStatus === 'idle') void loadTools()
  }, [activeTab, loadTools, toolStatus])

  useDidShow(() => {
    if (activeTab === 'messages') void loadConversations()
  })

  useEffect(() => {
    if (
      activeTab !== 'discover' ||
      initialStatus !== 'ready' ||
      sourceError.community ||
      sourceError.trade
    ) return undefined
    const page = observerRuntime.getCurrentInstance().page
    if (!page) return undefined
    let disposed = false
    let observer: RuntimeIntersectionObserver | null = null
    observerRuntime.nextTick(() => {
      if (disposed) return
      observerRuntime.createSelectorQuery()
        .in(page)
        .select('#home-discover-sentinel')
        .boundingClientRect((rect) => {
          if (!rect) return
          if (disposed) return
          observer = observerRuntime.createIntersectionObserver(page, { thresholds: [0] })
          observer.relativeTo('.student-home-content').observe('#home-discover-sentinel', (entry) => {
            if (entry.intersectionRatio > 0) void loadDiscover(false)
          })
        })
        .exec()
    })
    return () => {
      disposed = true
      observer?.disconnect()
    }
  }, [activeTab, initialStatus, loadDiscover, sourceError.community, sourceError.trade])

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
    const imageKey = `${card.target}:${card.id}`
    const hasMedia = Boolean(card.imageUrl)
    return (
      <View
        key={imageKey}
        className={`student-post-card ${hasMedia ? 'student-post-card--media' : 'student-post-card--text'}`}
        onClick={() => openCard(card)}
      >
        {hasMedia && (
          <View className={`student-post-card__media student-post-card__media--${card.tone} student-post-card__media--${card.height}`}>
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

  const renderDiscoverState = () => {
    const retrying = initialStatus === 'error' && loadingMore
    const copy = initialStatus === 'loading' || retrying
      ? { media: '正在加载', title: retrying ? '正在重新连接社区与二手集市...' : '正在加载校园新鲜事...' }
      : initialStatus === 'empty'
        ? { media: '暂无内容', title: '社区与二手集市暂时还没有内容' }
        : { media: '加载失败', title: '社区与二手集市加载失败，点击重试' }
    return (
      <View className='student-waterfall student-waterfall--state'>
        <View
          className={`student-post-card student-post-card--state ${initialStatus === 'error' && !retrying ? 'student-post-card--retry' : ''}`}
          onClick={initialStatus === 'error' && !retrying ? () => void loadDiscover(false, sourceError) : undefined}
        >
          <View className='student-post-card__media student-post-card__media--sky student-post-card__media--medium'>
            <Text>{copy.media}</Text>
          </View>
          <View className='student-post-card__body'>
            <Text className='student-post-card__title'>{copy.title}</Text>
          </View>
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
      />

      {activeTab === 'discover' && (
        <View className='student-home-search-row'>
          <View className='student-home-search' onClick={openSearch}>
            <View className='student-home-search__icon' />
            <Text className='student-home-search__placeholder'>搜帖子、话题和校园动态</Text>
          </View>
        </View>
      )}

      <View className='student-home-content'>
        {activeTab === 'messages' && (
          <View className='student-message-panel'>
            {conversationStatus === 'loading' || conversationStatus === 'idle' ? (
              <View className='student-message-state'><Text>正在读取真实私信...</Text></View>
            ) : conversationStatus === 'error' ? (
              <View className='student-message-state student-message-state--retry' onClick={() => void loadConversations()}>
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
                    <Text className='student-message-row__preview'>二手交易私信</Text>
                  </View>
                </View>
              ))
            )}
            {conversationStatus === 'ready' && <Text className='student-home-footnote'>仅展示最近 20 个会话</Text>}
          </View>
        )}

        {activeTab === 'discover' && (
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
              <View id='home-discover-sentinel' className='student-discover-sentinel' />
            </>
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

      </View>

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
