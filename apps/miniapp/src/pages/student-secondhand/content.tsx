import { useCallback, useEffect, useRef, useState } from 'react'
import { useDidShow } from '@tarojs/taro'
import { Image, ScrollView, Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import AppIcon from '../../components/app-icon'
import { AppState } from '../../components/ui'
import StudentBottomNav from '../../components/student-bottom-nav'
import type { ChannelInteraction } from '../../components/channel-pager'
import { getProductsPage } from '../../services/c-trade'
import Taro from '../../services/taro'
import type { Product } from '../../types'
import { useFeatureVisibility } from '../../hooks/use-feature-visibility'
import { useViewportRefresh } from '../../hooks/use-viewport-refresh'
import { useSessionStore } from '../../stores/session'
import './index.scss'

type CategoryKey = '全部' | '数码' | '书籍资料' | '生活用品' | '代步'
const categories: CategoryKey[] = ['全部', '数码', '书籍资料', '生活用品', '代步']
interface CategorySnapshot { items: Product[]; page: number; total: number; scrollTop: number }

function mergeProductsById(current: Product[], incoming: Product[]) {
  const values = new Map(current.map((item) => [item.id, item]))
  incoming.forEach((item) => values.set(item.id, item))
  return Array.from(values.values())
}

interface StudentSecondhandContentProps {
  embedded?: boolean
  active?: boolean
  interaction?: ChannelInteraction
}

export default function StudentSecondhandContent({ embedded = false, active = true, interaction }: StudentSecondhandContentProps) {
  const capsuleStyle = useViewportRefresh()
  const ownerKey = useSessionStore(state => `${state.profile.id}:${state.token}`)
  const [activeCategory, setActiveCategory] = useState<CategoryKey>('全部')
  const [items, setItems] = useState<Product[]>([])
  const [status, setStatus] = useState<'loading' | 'ready' | 'empty' | 'error'>('loading')
  const [page, setPage] = useState(0)
  const [total, setTotal] = useState(0)
  const [loadingMore, setLoadingMore] = useState(false)
  const [pagingError, setPagingError] = useState(false)
  const [refreshing, setRefreshing] = useState(false)
  const [refreshError, setRefreshError] = useState(false)
  const [failedImages, setFailedImages] = useState<Set<string>>(() => new Set())
  const visibility = useFeatureVisibility()
  const queryVersion = useRef(0)
  const categoryRef = useRef<CategoryKey>('全部')
  const loadedPageRef = useRef(0)
  const loadingKeys = useRef(new Set<string>())
  const refreshingRef = useRef(false)
  const navigatingRef = useRef(false)
  const didShowOnce = useRef(false)
  const refreshOnActivate = useRef(false)
  const snapshots = useRef<Partial<Record<CategoryKey, CategorySnapshot>>>({})
  const snapshotOwner = useRef(ownerKey)
  const scrollPosition = useRef(0)
  const [restoreScrollTop, setRestoreScrollTop] = useState(0)
  const refreshCurrent = useRef<() => void>(() => {})
  if (snapshotOwner.current === ownerKey && page > 0) {
    snapshots.current[categoryRef.current] = { items, page, total, scrollTop: scrollPosition.current }
  }

  const fetchProductsPage = useCallback(async (category: CategoryKey, targetPage: number, version: number, reset: boolean) => {
    const key = version + ':' + targetPage
    if (loadingKeys.current.has(key)) return
    loadingKeys.current.add(key)
    if (reset) setStatus('loading')
    else setLoadingMore(true)
    setPagingError(false)
    try {
      const result = await getProductsPage({
        page: targetPage, pageSize: 20,
        category: category === '全部' ? undefined : category, status: 'selling'
      })
      if (queryVersion.current !== version || categoryRef.current !== category) return
      setItems((current) => mergeProductsById(reset ? [] : current, result.items))
      loadedPageRef.current = result.page
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

  const retryCurrentCategory = useCallback(() => {
    if (!visibility.secondhand) return
    const version = ++queryVersion.current
    loadedPageRef.current = 0
    refreshingRef.current = false
    setItems([])
    setPage(0)
    setTotal(0)
    setRefreshing(false)
    setLoadingMore(false)
    setRefreshError(false)
    void fetchProductsPage(categoryRef.current, 1, version, true)
  }, [fetchProductsPage, visibility.secondhand])

  useEffect(() => {
    if (snapshotOwner.current !== ownerKey || !visibility.secondhand) {
      snapshots.current = {}
      snapshotOwner.current = ownerKey
    }
    categoryRef.current = activeCategory
    const saved = snapshots.current[activeCategory]
    scrollPosition.current = saved?.scrollTop ?? 0
    setRestoreScrollTop(scrollPosition.current)
    refreshingRef.current = false
    setRefreshing(false)
    setLoadingMore(false)
    setPagingError(false)
    setRefreshError(false)
    if (visibility.secondhand && saved) {
      queryVersion.current += 1
      setItems(saved.items)
      setPage(saved.page)
      loadedPageRef.current = saved.page
      setTotal(saved.total)
      setStatus(saved.items.length ? 'ready' : 'empty')
      refreshCurrent.current()
    } else if (visibility.secondhand) retryCurrentCategory()
    else {
      queryVersion.current += 1
      setItems([])
      setPage(0)
      loadedPageRef.current = 0
      setStatus('empty')
    }
    return () => { queryVersion.current += 1 }
  }, [activeCategory, retryCurrentCategory, visibility.secondhand, ownerKey])

  // Keep the mounted list and its scroll position while reconciling real server changes.
  const refreshProducts = useCallback(async () => {
    if (!visibility.secondhand || refreshingRef.current) return
    if (loadedPageRef.current === 0) {
      if (status === 'error') retryCurrentCategory()
      return
    }
    refreshingRef.current = true
    const lastPage = loadedPageRef.current
    const category = categoryRef.current
    const version = ++queryVersion.current
    setRefreshing(true)
    setLoadingMore(false)
    setRefreshError(false)
    setPagingError(false)
    try {
      let refreshed: Product[] = []
      let refreshedTotal = 0
      let refreshedPage = 1
      for (let targetPage = 1; targetPage <= lastPage; targetPage += 1) {
        const result = await getProductsPage({
          page: targetPage, pageSize: 20,
          category: category === '全部' ? undefined : category, status: 'selling'
        })
        if (queryVersion.current !== version || categoryRef.current !== category) return
        refreshed = mergeProductsById(refreshed, result.items)
        refreshedTotal = result.total
        refreshedPage = result.page
        if (targetPage * result.pageSize >= result.total || result.items.length === 0) break
      }
      setItems(refreshed)
      setTotal(refreshedTotal)
      setPage(refreshedPage)
      loadedPageRef.current = refreshedPage
      setStatus(refreshed.length ? 'ready' : 'empty')
    } catch {
      if (queryVersion.current === version) setRefreshError(true)
    } finally {
      if (queryVersion.current === version) {
        refreshingRef.current = false
        setRefreshing(false)
      }
    }
  }, [retryCurrentCategory, status, visibility.secondhand])
  refreshCurrent.current = () => { void refreshProducts() }

  useDidShow(() => {
    navigatingRef.current = false
    if (!didShowOnce.current) {
      didShowOnce.current = true
      // A lazily mounted channel may first receive didShow when returning
      // from a product, after the parent's initial show has already happened.
      if (!embedded) return
    }
    if (active) void refreshProducts()
    else refreshOnActivate.current = true
  })

  useEffect(() => {
    if (active && refreshOnActivate.current) {
      refreshOnActivate.current = false
      void refreshProducts()
    }
  }, [active, refreshProducts])

  const loadMoreProducts = useCallback(() => {
    if (!active || !visibility.secondhand || status !== 'ready' || loadingMore || refreshingRef.current || items.length >= total) return
    void fetchProductsPage(activeCategory, page + 1, queryVersion.current, false)
  }, [active, activeCategory, fetchProductsPage, items.length, loadingMore, page, status, total, visibility.secondhand])

  const navigate = async (url: string) => {
    if (!active || navigatingRef.current || (interaction && !interaction.canActivate())) return
    navigatingRef.current = true
    try { await Taro.navigateTo({ url }) }
    catch { void Taro.showToast({ title: '页面打开失败，请重试', icon: 'none' }) }
    finally { navigatingRef.current = false }
  }

  const markImageFailed = (key: string) => {
    setFailedImages((current) => new Set(current).add(key))
  }

  const renderCard = (item: Product) => {
    const imageUrl = item.images.find((src) => src.trim())
    const imageKey = item.id + ':' + imageUrl
    const avatarKey = 'avatar:' + item.id + ':' + item.sellerAvatar
    return (
      <View
        key={item.id}
        className={'secondhand-card' + (item.status === 'sold' ? ' secondhand-card--sold' : '')}
        onClick={() => void navigate('/pages/secondhand-detail/index?id=' + encodeURIComponent(item.id))}
        hoverClass='secondhand-card--pressed'
        ariaRole='button'
        ariaLabel={item.title + '，' + item.price + '元'}
      >
        <View className='secondhand-card__media'>
          {imageUrl && !failedImages.has(imageKey) ? (
            <Image className='secondhand-card__image' src={imageUrl} mode='aspectFill' lazyLoad onError={() => markImageFailed(imageKey)} />
          ) : <View className='secondhand-card__no-image'><AppIcon name='image' /><Text>{imageUrl ? '图片暂不可用' : '卖家未上传图片'}</Text></View>}
          {item.images.length > 1 && <Text className='secondhand-card__image-count'>{item.images.length} 图</Text>}
          {item.status !== 'selling' && <Text className='secondhand-card__status'>{item.status === 'sold' ? '已售出' : '已预订'}</Text>}
        </View>
        <View className='secondhand-card__body'>
          <Text className='secondhand-card__title'>{item.title}</Text>
          <Text className='secondhand-card__price'><Text className='secondhand-card__currency'>¥</Text>{item.price}</Text>
          <Text className='secondhand-card__meta'>{[item.condition, item.category].filter(Boolean).join(' · ')}</Text>
          <View className='secondhand-card__seller'>
            {item.sellerAvatar && !failedImages.has(avatarKey)
              ? <Image className='secondhand-card__avatar' src={item.sellerAvatar} mode='aspectFill' onError={() => markImageFailed(avatarKey)} />
              : <Text className='secondhand-card__avatar secondhand-card__avatar--text'>{item.sellerName.trim().charAt(0) || '校'}</Text>}
            <Text className='secondhand-card__seller-name'>{item.sellerName}</Text>
          </View>
        </View>
      </View>
    )
  }

  return (
    <View className={`student-secondhand-page${embedded ? ' student-secondhand-page--embedded' : ''}`} style={capsuleStyle}>
      <View className='student-secondhand-header'>
        {!embedded && <CapsulePageHeader
          variant='search'
          className='student-secondhand-header__title-row'
          left={<Text className='student-secondhand-header__title'>二手集市</Text>}
          center={visibility.search ? <View className='student-secondhand-search' onClick={() => void navigate('/pages/search/index?scope=secondhand')} hoverClass='student-secondhand-search--pressed' ariaRole='button' ariaLabel='搜索二手商品'>
            <View className='student-secondhand-search__field'>
              <AppIcon name='search' className='student-secondhand-search__icon' />
              <Text className='student-secondhand-search__placeholder'>搜二手好物</Text>
            </View>
          </View> : undefined}
          divider={false}
        />}
        <ScrollView className='student-secondhand-categories' scrollX enhanced showScrollbar={false}>
          <View className='student-secondhand-categories__inner'>
            {categories.map((item) => (
              <View key={item} className={'student-secondhand-category' + (activeCategory === item ? ' student-secondhand-category--active' : '')} onClick={() => {
                if (active && (!interaction || interaction.canActivate())) setActiveCategory(item)
              }} ariaRole='button' ariaLabel={item + (activeCategory === item ? '，已选中' : '')}>
                <Text>{item}</Text>
              </View>
            ))}
          </View>
        </ScrollView>
      </View>
      <ScrollView key={`${ownerKey}:${activeCategory}`} className='student-secondhand-scroll' scrollY enhanced showScrollbar={false}
        scrollTop={restoreScrollTop} onScroll={event => { scrollPosition.current = Math.max(0, event.detail.scrollTop) }}
        lowerThreshold={120} onScrollToLower={loadMoreProducts}>
        {!visibility.secondhand ? <AppState kind='empty' title='二手市场暂未开放' /> : <>
          <View className='student-secondhand-notice'>
            <AppIcon name='info' className='student-secondhand-notice__icon' />
            <Text>平台仅展示信息，请当面验货后付款。</Text>
          </View>
          {status === 'empty' || status === 'error' ? (
            <AppState kind={status} title={status === 'empty' ? '该分类暂无在售宝贝' : '商品加载失败'} description={status === 'empty' ? '可以看看其他分类。' : '请检查网络后重试。'} actionText={status === 'error' ? '重新加载' : undefined} onAction={status === 'error' ? retryCurrentCategory : undefined} />
          ) : <View className='student-secondhand-grid'>
            {status === 'loading' ? [0, 1, 2, 3].map((key) => (
              <View key={key} className='secondhand-card secondhand-card--skeleton'>
                <View className='secondhand-card__media' />
                <View className='secondhand-card__body'><View className='secondhand-card__skeleton-line' /><View className='secondhand-card__skeleton-line secondhand-card__skeleton-line--short' /></View>
              </View>
            )) : items.map(renderCard)}
          </View>}
          <View className='student-secondhand-footer'>
            {refreshError ? <Text className='student-secondhand-footer__retry' onClick={() => void refreshProducts()}>更新失败，已保留浏览内容 · 点击重试</Text>
              : refreshing ? <Text>正在更新商品状态…</Text>
                : pagingError ? <Text className='student-secondhand-footer__retry' onClick={loadMoreProducts}>更多商品加载失败，点击重试</Text>
                  : loadingMore ? <Text>正在加载更多…</Text>
                    : status === 'ready' && items.length >= total ? <Text>已显示全部在售宝贝</Text> : null}
          </View>
        </>}
      </ScrollView>
      {!embedded && <StudentBottomNav active='market' className='student-bottom-nav--secondhand' />}
    </View>
  )
}
