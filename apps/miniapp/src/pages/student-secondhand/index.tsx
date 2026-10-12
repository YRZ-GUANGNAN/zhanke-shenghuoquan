import { withFeaturePage } from '../../components/feature-page-gate'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useDidShow } from '@tarojs/taro'
import { Image, ScrollView, Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import StudentBottomNav from '../../components/student-bottom-nav'
import { getProductsPage } from '../../services/c-trade'
import Taro from '../../services/taro'
import type { Product } from '../../types'
import { getCapsuleSafeStyle } from '../../utils/capsule-safe'
import { useFeatureVisibility } from '../../hooks/use-feature-visibility'
import { usePageShare } from '../../hooks/use-page-share'
import './index.scss'

type CategoryKey = '全部' | '数码' | '书籍资料' | '生活用品' | '代步'
type ProductTone = 'lemon' | 'lilac' | 'sky' | 'peach' | 'mint'
type ProductStatus = 'selling' | 'sold'
type ProductHeight = 'short' | 'medium' | 'tall'

interface ProductCard {
  id: string
  title: string
  price: number
  category: string
  imageLabel: string
  imageUrl?: string
  status: ProductStatus
  tone: ProductTone
  height: ProductHeight
}

const categories: CategoryKey[] = ['全部', '数码', '书籍资料', '生活用品', '代步']
const productTones: ProductTone[] = ['lemon', 'lilac', 'sky', 'peach', 'mint']
const productHeights: ProductHeight[] = ['short', 'medium', 'tall']

function stableNumber(value: string) {
  return Array.from(value).reduce((hash, char) => ((hash * 31) + char.charCodeAt(0)) >>> 0, 7)
}

function mergeProductsById(current: Product[], incoming: Product[]) {
  const values = new Map(current.map((item) => [item.id, item]))
  incoming.forEach((item) => values.set(item.id, item))
  return Array.from(values.values())
}

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

function splitProducts(cards: ProductCard[]) {
  return {
    left: cards.filter((_, index) => index % 2 === 0),
    right: cards.filter((_, index) => index % 2 === 1)
  }
}

function StudentSecondhandPage() {
  const [activeCategory, setActiveCategory] = useState<CategoryKey>('全部')
  const [items, setItems] = useState<Product[]>([])
  const [status, setStatus] = useState<'loading' | 'ready' | 'empty' | 'error'>('loading')
  const [page, setPage] = useState(0)
  const [total, setTotal] = useState(0)
  const [loadingMore, setLoadingMore] = useState(false)
  const [pagingError, setPagingError] = useState(false)
  const visibility = useFeatureVisibility()
  usePageShare({ kind: 'secondhand', enabled: visibility.secondhand })
  const [failedImageKeys, setFailedImageKeys] = useState<Set<string>>(() => new Set())
  const queryVersion = useRef(0)
  const categoryRef = useRef<CategoryKey>('全部')
  const loadingKeys = useRef(new Set<string>())
  const didShowOnce = useRef(false)

  const fetchProductsPage = useCallback(async (
    category: CategoryKey,
    targetPage: number,
    version: number,
    reset: boolean
  ) => {
    const key = `${version}:${targetPage}`
    if (loadingKeys.current.has(key)) return
    loadingKeys.current.add(key)
    if (reset) {
      setStatus('loading')
      setLoadingMore(false)
    } else {
      setLoadingMore(true)
    }
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
    if (!visibility) return
    if (!visibility.secondhand) {
      setItems([])
      setStatus('empty')
      return
    }
    categoryRef.current = activeCategory
    const version = queryVersion.current + 1
    queryVersion.current = version
    setItems([])
    setPage(0)
    setTotal(0)
    void fetchProductsPage(activeCategory, 1, version, true)

    return () => {
      queryVersion.current += 1
    }
  }, [activeCategory, fetchProductsPage, visibility])

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

  useDidShow(() => {
    if (!didShowOnce.current) {
      didShowOnce.current = true
      return
    }
    retryCurrentCategory()
  })

  const cards = useMemo(() => items.map(toCard), [items])
  const columns = useMemo(() => splitProducts(cards), [cards])

  const openDetail = (item: ProductCard) => {
    void Taro.navigateTo({ url: `/pages/secondhand-detail/index?id=${encodeURIComponent(item.id)}` })
  }

  const openSearch = () => {
    void Taro.navigateTo({ url: '/pages/search/index?scope=secondhand' })
  }

  const markImageFailed = (id: string) => {
    setFailedImageKeys((current) => {
      const next = new Set(current)
      next.add(id)
      return next
    })
  }

  const renderCard = (item: ProductCard) => (
    <View
      key={item.id}
      className={`secondhand-card ${item.status === 'sold' ? 'secondhand-card--sold' : ''}`}
      onClick={() => openDetail(item)}
    >
      <View className={`secondhand-card__media secondhand-card__media--${item.tone} secondhand-card__media--${item.height}`}>
        {item.imageUrl && !failedImageKeys.has(item.id) ? (
          <Image
            className='secondhand-card__image'
            src={item.imageUrl}
            mode='aspectFill'
            onError={() => markImageFailed(item.id)}
          />
        ) : (
          <Text>{item.imageLabel}</Text>
        )}
        {item.status === 'sold' && (
          <View className='secondhand-card__sold-badge'>
            <Text>已售出</Text>
          </View>
        )}
      </View>
      <View className='secondhand-card__body'>
        <Text className='secondhand-card__title'>{item.title}</Text>
        <View className='secondhand-card__bottom'>
          <Text className='secondhand-card__price'>¥{item.price}</Text>
          <View className={`secondhand-card__status secondhand-card__status--${item.status}`}>
            <Text>{item.status === 'selling' ? '出售中' : '已售'}</Text>
          </View>
        </View>
      </View>
    </View>
  )

  const renderSkeleton = (key: string, tone: ProductTone, height: ProductHeight) => (
    <View key={key} className='secondhand-card secondhand-card--skeleton'>
      <View className={`secondhand-card__media secondhand-card__media--${tone} secondhand-card__media--${height}`} />
      <View className='secondhand-card__body'>
        <View className='secondhand-card__skeleton-line secondhand-card__skeleton-line--title' />
        <View className='secondhand-card__skeleton-line secondhand-card__skeleton-line--meta' />
      </View>
    </View>
  )

  const renderStateCard = () => (
    <View className='secondhand-card secondhand-card--state'>
      <View className='secondhand-card__media secondhand-card__media--sky secondhand-card__media--medium'>
        <Text>{status === 'empty' ? '该分类暂无在售宝贝' : '加载失败'}</Text>
      </View>
      {status === 'error' && (
        <View className='secondhand-card__state-action' onClick={retryCurrentCategory}>
          <Text>点击重试</Text>
        </View>
      )}
    </View>
  )

  return (
    <View className='student-secondhand-page' style={getCapsuleSafeStyle()}>
      <View className='student-secondhand-header'>
        <CapsulePageHeader
          variant='search'
          className='student-secondhand-header__title-row'
          left={<Text className='student-secondhand-header__title'>二手集市</Text>}
          center={
            visibility.search ? <View className='student-secondhand-search' onClick={openSearch}>
              <View className='student-secondhand-search__icon' />
              <Text>搜二手好物</Text>
            </View> : undefined
          }
          divider={false}
        />

        <ScrollView className='student-secondhand-categories' scrollX enhanced showScrollbar={false}>
          <View className='student-secondhand-categories__inner'>
            {categories.map((item) => (
              <View
                key={item}
                className={`student-secondhand-category ${activeCategory === item ? 'student-secondhand-category--active' : ''}`}
                onClick={() => setActiveCategory(item)}
              >
                <Text>{item}</Text>
              </View>
            ))}
          </View>
        </ScrollView>
      </View>

      <ScrollView
        className='student-secondhand-scroll'
        scrollY
        enhanced
        showScrollbar={false}
        lowerThreshold={120}
        onScrollToLower={loadMoreProducts}
      >
        {!visibility && <View className='student-secondhand-notice'><Text>正在读取功能状态…</Text></View>}
        {visibility && !visibility.secondhand && <View className='student-secondhand-notice'><Text>二手市场暂未开放</Text></View>}
        {visibility?.secondhand && (
        <View className='student-secondhand-notice'>
          <Text className='student-secondhand-notice__icon'>i</Text>
          <Text>平台仅展示信息，交易请与卖家私下当面交付，验货后再付款。</Text>
        </View>
        )}

        {visibility?.secondhand && <View className='student-secondhand-waterfall'>
          <View className='student-secondhand-waterfall__column'>
            {status === 'loading'
              ? [
                  renderSkeleton('skeleton-left-1', 'lemon', 'short'),
                  renderSkeleton('skeleton-left-2', 'mint', 'medium')
                ]
              : (status === 'ready' ? columns.left.map(renderCard) : renderStateCard())}
          </View>
          <View className='student-secondhand-waterfall__column'>
            {status === 'loading'
              ? [
                  renderSkeleton('skeleton-right-1', 'lilac', 'medium'),
                  renderSkeleton('skeleton-right-2', 'peach', 'tall')
                ]
              : (status === 'ready' ? columns.right.map(renderCard) : null)}
          </View>
          {loadingMore && (
            <View className='student-secondhand-waterfall__footer'>
              <Text>正在加载更多...</Text>
            </View>
          )}
          {pagingError && (
            <View className='student-secondhand-waterfall__footer student-secondhand-waterfall__footer--retry' onClick={loadMoreProducts}>
              <Text>加载失败，点击重试</Text>
            </View>
          )}
          {status === 'ready' && items.length >= total && total > 0 && (
            <View className='student-secondhand-waterfall__footer'>
              <Text>— 到底了 —</Text>
            </View>
          )}
        </View>}
      </ScrollView>

      <StudentBottomNav active='market' className='student-bottom-nav--secondhand' />
    </View>
  )
}

export default withFeaturePage(StudentSecondhandPage)
