import ProductPrice from '../../components/product-price'
import { withFeaturePage } from '../../components/feature-page-gate'
import { useEffect, useRef, useState } from 'react'
import { hideKeyboard, nextTick } from '@tarojs/taro'
import { Image, Input, ScrollView, Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import AppIcon from '../../components/app-icon'
import type { CommunityPost, Product } from '../../types'
import Taro from '../../services/taro'
import { useViewportRefresh } from '../../hooks/use-viewport-refresh'
import {
  clearCommunitySearchHistory,
  listCommunityHotSearches,
  listCommunitySearchHistory,
  searchCommunityPosts,
} from '../../services/b-community'
import {
  clearTradeSearchHistory,
  listTradeHotSearches,
  listTradeSearchHistory,
  searchTradeProducts,
} from '../../services/c-trade'
import {
  clearExternalTakeoutSearchHistory,
  listExternalTakeoutHotSearches,
  listExternalTakeoutSearchHistory,
  searchExternalTakeoutMerchants,
  type ExternalTakeoutMerchantLink,
} from '../../services/d-navigation'
import './index.scss'
import { formatPostDate, postBodyWithoutTitle } from '../../utils/post-presentation'
import SearchFilterPanel from './search-filter-panel'
import {
  communitySearchFilters,
  defaultSearchFilters,
  searchFilterKey,
  searchFilterLabels,
  tradeSearchFilters,
  type SearchFilterValues,
} from './search-filters'
import { useSessionStore } from '../../stores/session'

type SearchScope = 'community' | 'secondhand' | 'takeout'
type SearchResult = CommunityPost | Product | ExternalTakeoutMerchantLink

const scopeCopy: Record<SearchScope, { title: string; placeholder: string; empty: string }> = {
  community: { title: '帖子搜索', placeholder: '搜索帖子内容或标签', empty: '没有找到相关帖子' },
  secondhand: { title: '二手搜索', placeholder: '搜索二手商品', empty: '没有找到相关商品' },
  takeout: { title: '外卖搜索', placeholder: '搜索外卖商家', empty: '没有找到相关商家' },
}
const SEARCH_PAGE_SIZE = 20

function mergeResults(current: SearchResult[], incoming: SearchResult[]): SearchResult[] {
  const byId = new Map(current.map((item) => [item.id, item]))
  incoming.forEach((item) => byId.set(item.id, item))
  return Array.from(byId.values())
}

export function isSearchScope(value: string | undefined): value is SearchScope {
  return value === 'community' || value === 'secondhand' || value === 'takeout'
}

function errorMessage(error: unknown): string {
  if (error instanceof Error && error.message.trim()) return error.message
  return '请求失败，请稍后重试'
}

function SearchContent() {
  const capsuleStyle = useViewportRefresh()
  const rawScope = Taro.getCurrentInstance().router?.params?.scope
  const scope = isSearchScope(rawScope) ? rawScope : null
  const copy = scope ? scopeCopy[scope] : null
  const initialKeyword = Taro.getCurrentInstance().router?.params?.keyword ?? ''
  const [keyword, setKeyword] = useState(initialKeyword)
  const [submittedKeyword, setSubmittedKeyword] = useState('')
  const [history, setHistory] = useState<string[]>([])
  const [hot, setHot] = useState<string[]>([])
  const [results, setResults] = useState<SearchResult[]>([])
  const [loading, setLoading] = useState(false)
  const [loadingMore, setLoadingMore] = useState(false)
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [hasSearched, setHasSearched] = useState(false)
  const [error, setError] = useState('')
  const [readPostIds, setReadPostIds] = useState<Set<string>>(() => new Set())
  const [failedCovers, setFailedCovers] = useState<Set<string>>(() => new Set())
  const [scrollAnchor, setScrollAnchor] = useState('')
  const [appliedFilters, setAppliedFilters] = useState<SearchFilterValues>(() => defaultSearchFilters())
  const [filterPanelOpen, setFilterPanelOpen] = useState(false)
  const [endReached, setEndReached] = useState(false)
  const searchVersion = useRef(0)
  const searchBusy = useRef(false)
  const pageBusy = useRef(false)
  const openingResult = useRef(false)
  const activeQuery = useRef('')
  const activeFilterKey = useRef(searchFilterKey('community', defaultSearchFilters()))
  const alive = useRef(true)
  const suggestionVersion = useRef(0)
  const clearingHistory = useRef(false)
  const [suggestionState, setSuggestionState] = useState<'loading' | 'ready' | 'error'>('loading')

  const loadSuggestions = async (activeScope: SearchScope, version = searchVersion.current) => {
    const suggestion = ++suggestionVersion.current
    setSuggestionState('loading')
    try {
      const [historyItems, hotItems] = await (
        activeScope === 'community' ? Promise.all([
          listCommunitySearchHistory(),
          listCommunityHotSearches(),
        ]) : activeScope === 'secondhand' ? Promise.all([
          listTradeSearchHistory(),
          listTradeHotSearches(),
        ]) : Promise.all([
          listExternalTakeoutSearchHistory(),
          listExternalTakeoutHotSearches(),
        ])
      )
      if (!alive.current || version !== searchVersion.current || suggestion !== suggestionVersion.current) return
      setHistory(historyItems)
      setHot(hotItems.map((item) => item.keyword))
      setSuggestionState('ready')
    } catch (loadError) {
      if (!alive.current || version !== searchVersion.current || suggestion !== suggestionVersion.current) return
      console.error('load domain search suggestions failed', loadError)
      setSuggestionState('error')
    }
  }

  useEffect(() => {
    if (!scope) return
    alive.current = true
    void loadSuggestions(scope)
    return () => {
      searchVersion.current += 1
      searchBusy.current = false
      pageBusy.current = false
      alive.current = false
      suggestionVersion.current += 1
    }
  }, [scope])

  const scrollToStart = () => {
    setScrollAnchor('')
    nextTick(() => { if (alive.current) setScrollAnchor('search-content-start') })
  }

  const searchPage = async (activeScope: SearchScope, value: string, nextPage: number, filters: SearchFilterValues) => {
    if (activeScope === 'community') return searchCommunityPosts(value, nextPage, SEARCH_PAGE_SIZE, communitySearchFilters(filters))
    if (activeScope === 'secondhand') return searchTradeProducts(value, nextPage, SEARCH_PAGE_SIZE, tradeSearchFilters(filters))
    return searchExternalTakeoutMerchants(value, nextPage, SEARCH_PAGE_SIZE)
  }

  const submitSearch = async (candidate = keyword, filters = appliedFilters) => {
    if (!scope) return
    const normalized = candidate.trim()
    if (!normalized) {
      void Taro.showToast({ title: '请输入搜索词', icon: 'none' })
      return
    }
    const nextFilterKey = searchFilterKey(scope === 'community' || scope === 'secondhand' ? scope : 'community', filters)
    if (searchBusy.current && activeQuery.current === normalized && activeFilterKey.current === nextFilterKey) return
    searchBusy.current = true
    pageBusy.current = false
    activeQuery.current = normalized
    activeFilterKey.current = nextFilterKey
    setAppliedFilters(filters)
    setKeyword(normalized)
    setSubmittedKeyword(normalized)
    setLoading(true)
    setLoadingMore(false)
    setError('')
    setHasSearched(true)
    setEndReached(false)
    setResults([])
    setPage(1)
    setTotal(0)
    setFailedCovers(new Set())
    void hideKeyboard().catch(() => undefined)
    scrollToStart()
    const version = searchVersion.current + 1
    searchVersion.current = version
    try {
      const result = await searchPage(scope, normalized, 1, filters)
      if (searchVersion.current !== version) return
      setResults(result.items)
      setPage(result.page || 1)
      setTotal(result.total)
      setEndReached(result.items.length === 0)
      void loadSuggestions(scope, version)
    } catch (searchError) {
      if (searchVersion.current !== version) return
      console.error('domain search failed', searchError)
      setResults([])
      setError(errorMessage(searchError))
    } finally {
      if (searchVersion.current === version) {
        searchBusy.current = false
        setLoading(false)
      }
    }
  }

  const loadMore = async () => {
    if (!scope || !hasSearched || !submittedKeyword || filterPanelOpen || endReached || searchBusy.current || pageBusy.current || results.length >= total) return
    pageBusy.current = true
    const nextPage = page + 1
    const normalized = submittedKeyword
    const version = searchVersion.current
    setLoadingMore(true)
    setError('')
    try {
      const result = await searchPage(scope, normalized, nextPage, appliedFilters)
      if (searchVersion.current !== version) return
      setResults((current) => mergeResults(current, result.items))
      setPage(result.page || nextPage)
      setTotal(result.total)
      setEndReached(result.items.length === 0)
    } catch (searchError) {
      if (searchVersion.current !== version) return
      console.error('load more search results failed', searchError)
      setError('更多结果加载失败，点击重试')
    } finally {
      if (searchVersion.current === version) {
        pageBusy.current = false
        setLoadingMore(false)
      }
    }
  }

  const showSuggestions = () => {
    searchVersion.current += 1
    searchBusy.current = false
    pageBusy.current = false
    activeQuery.current = ''
    activeFilterKey.current = searchFilterKey('community', defaultSearchFilters())
    setLoading(false)
    setLoadingMore(false)
    setHasSearched(false)
    setError('')
    scrollToStart()
    if (scope) void loadSuggestions(scope)
  }

  const applyFilters = (nextFilters: SearchFilterValues) => {
    void hideKeyboard().catch(() => undefined)
    setFilterPanelOpen(false)
    setAppliedFilters(nextFilters)
    if (!scope || scope === 'takeout') return
    if (hasSearched && submittedKeyword && searchFilterKey(scope, nextFilters) !== activeFilterKey.current) void submitSearch(submittedKeyword, nextFilters)
  }

  const openFilters = () => {
    void hideKeyboard().catch(() => undefined)
    setFilterPanelOpen(true)
  }

  const handleBack = () => {
    if (filterPanelOpen) {
      setFilterPanelOpen(false)
      return
    }
    void Taro.navigateBack()
  }

  const clearHistory = async () => {
    if (!scope || clearingHistory.current) return
    clearingHistory.current = true
    suggestionVersion.current += 1
    try {
      if (scope === 'community') await clearCommunitySearchHistory()
      else if (scope === 'secondhand') await clearTradeSearchHistory()
      else await clearExternalTakeoutSearchHistory()
      if (!alive.current) return
      setHistory([])
      void loadSuggestions(scope)
      void Taro.showToast({ title: '搜索历史已清空', icon: 'success' })
    } catch (clearError) {
      if (!alive.current) return
      console.error('clear domain search history failed', clearError)
      void Taro.showToast({ title: '清空失败，请重试', icon: 'none' })
    } finally {
      clearingHistory.current = false
    }
  }

  const openResult = async (item: SearchResult) => {
    if (!scope || openingResult.current) return
    if (scope === 'community' || scope === 'secondhand') {
      openingResult.current = true
      try {
        await Taro.navigateTo({ url: scope === 'community'
          ? `/pages/post-detail/index?id=${encodeURIComponent(item.id)}`
          : `/pages/secondhand-detail/index?id=${encodeURIComponent(item.id)}` })
        if (scope === 'community') setReadPostIds((current) => new Set(current).add(item.id))
      } catch (jumpError) {
        console.error('open search result failed', jumpError)
        await Taro.showToast({ title: '暂时无法打开，请重试', icon: 'none' })
      } finally {
        openingResult.current = false
      }
      return
    }
    const merchant = item as ExternalTakeoutMerchantLink
    if (merchant.operatingStatus === 'resting') {
      await Taro.showToast({ title: '商家当前休息中', icon: 'none' })
      return
    }
    try {
      await Taro.navigateToMiniProgram({ appId: merchant.appId, path: merchant.path })
    } catch (jumpError) {
      console.error('open external takeout merchant failed', jumpError)
      await Taro.showToast({ title: '暂时无法打开该商家', icon: 'none' })
    }
  }

  const renderResult = (item: SearchResult) => {
    if (scope === 'community') {
      const post = item as CommunityPost
      const cover = post.images.find((src) => src.trim())?.trim()
      const coverKey = `${post.id}:${cover}`
      const summary = postBodyWithoutTitle(post.title, post.content)
      return (
        <View className={`student-search-result student-search-result--post${readPostIds.has(post.id) ? ' student-search-result--read' : ''}`} key={post.id} onClick={() => void openResult(post)}>
          <View className='student-search-result__main'>
            <Text className='student-search-result__title'>{post.title}</Text>
            {summary && <Text className='student-search-result__summary'>{summary}</Text>}
            {(post.tags ?? []).length > 0 && (
              <View className='student-search-result__tags'>
                {(post.tags ?? []).map((tag) => <Text key={tag}>#{tag.replace(/^#+/, '')}</Text>)}
              </View>
            )}
            <Text className='student-search-result__meta'>{[post.author, formatPostDate(post.createdAt)].filter(Boolean).join(' · ')}</Text>
          </View>
          {cover && <View className='student-search-result__media'>
            {failedCovers.has(coverKey) ? <Text className='student-search-result__media-error'>图片未加载</Text> : <Image
              className='student-search-result__cover'
              src={cover}
              mode='aspectFill'
              lazyLoad
              onError={() => setFailedCovers((current) => new Set(current).add(coverKey))}
            />}
          </View>}
        </View>
      )
    }
    if (scope === 'secondhand') {
      const product = item as Product
      const cover = product.images.find((src) => src.trim())?.trim()
      const coverKey = `${product.id}:${cover}`
      return (
        <View className='student-search-result student-search-result--product' key={product.id} onClick={() => void openResult(product)}>
          {cover && <View className='student-search-result__media student-search-result__media--product'>
            {failedCovers.has(coverKey) ? <Text className='student-search-result__media-error'>图片未加载</Text> : <Image className='student-search-result__cover' src={cover} mode='aspectFill' lazyLoad onError={() => setFailedCovers((current) => new Set(current).add(coverKey))} />}
          </View>}
          <View className='student-search-result__main'>
            <View className='student-search-result__row'>
              <Text className='student-search-result__title'>{product.title}</Text>
              <ProductPrice className='student-search-result__price' value={product.price} size='compact' />
            </View>
            <Text className='student-search-result__summary'>{product.description || '暂无商品描述'}</Text>
            <Text className='student-search-result__meta'>{[product.category || '其他', product.campus || '校内交易', product.sellerName, formatPostDate(product.createdAt)].filter(Boolean).join(' · ')}</Text>
          </View>
        </View>
      )
    }
    const merchant = item as ExternalTakeoutMerchantLink
    return (
      <View className='student-search-result' key={merchant.id} onClick={() => void openResult(merchant)}>
        <View className='student-search-result__row'>
          <Text className='student-search-result__title'>{merchant.title}</Text>
          <Text className={`student-search-result__status student-search-result__status--${merchant.operatingStatus}`}>
            {merchant.operatingStatus === 'open' ? '营业中' : '休息中'}
          </Text>
        </View>
        <Text className='student-search-result__summary'>{merchant.subtitle || '点击进入商家小程序'}</Text>
      </View>
    )
  }

  if (!scope || !copy) {
    return (
      <View className='student-search-page' style={capsuleStyle}>
        <CapsulePageHeader
          className='student-search-header'
          title='搜索入口无效'
          left={<View className='student-search-back' onClick={handleBack} hoverClass='student-search-control--pressed' ariaRole='button' ariaLabel='返回'>
            <AppIcon name='back' className='student-search-back__icon' />
          </View>}
        />
        <View className='student-search-invalid'>请从帖子、二手或外卖页面重新进入搜索。</View>
      </View>
    )
  }

  return (
    <View className='student-search-page' style={capsuleStyle}>
      <CapsulePageHeader
        variant='search'
        className='student-search-header'
        left={<View className='student-search-back' onClick={handleBack} hoverClass='student-search-control--pressed' ariaRole='button' ariaLabel='返回'>
          <AppIcon name='back' className='student-search-back__icon' />
        </View>}
        center={
          <View className='student-search-box'>
            <AppIcon name='search' className='student-search-box__icon' />
            <Input
              className='student-search-box__input'
              value={keyword}
              placeholder={copy.placeholder}
              confirmType='search'
              onInput={(event) => setKeyword(event.detail.value)}
              onConfirm={() => void submitSearch()}
            />
          </View>
        }
        right={<View className='student-search-submit' onClick={() => void submitSearch()} hoverClass='student-search-control--pressed' ariaRole='button' ariaLabel='搜索'>
          <Text>{loading ? '搜索中' : '搜索'}</Text>
        </View>}
      />

      {(scope === 'community' || scope === 'secondhand') && <View className='student-search-tools'>
        <View className='student-search-filter-bar'>
          <View className='student-search-sort-options'>
            {([{ value: 'relevance', label: '综合' }, { value: 'recent', label: '最新' }] as const).map((item) => <View
              id={`search-sort-${item.value}`} key={item.value}
              className={`student-search-sort${appliedFilters.sort === item.value ? ' student-search-sort--active' : ''}`}
              role='button' ariaLabel={`${item.label}${appliedFilters.sort === item.value ? '，已选中' : ''}`} hoverClass='student-search-control--pressed'
              onClick={() => applyFilters({ ...appliedFilters, sort: item.value })}
            >{item.label}</View>)}
          </View>
          <View className={`student-search-filter-toggle${searchFilterLabels(scope, appliedFilters).length ? ' student-search-filter-toggle--active' : ''}`} role='button' hoverClass='student-search-control--pressed' onClick={openFilters}>
            <Text>筛选{searchFilterLabels(scope, appliedFilters).length ? ` ${searchFilterLabels(scope, appliedFilters).length}` : ''}</Text>
            <View className='student-search-filter-toggle__icon' />
          </View>
        </View>
        {searchFilterLabels(scope, appliedFilters).length > 0 && <View className='student-search-filter-summary'>
          <ScrollView scrollX showScrollbar={false} className='student-search-filter-summary__scroll'>
            <Text className='student-search-filter-summary__text' onClick={openFilters}>{searchFilterLabels(scope, appliedFilters).join(' · ')}</Text>
          </ScrollView>
          <View className='student-search-clear-filters' role='button' onClick={() => applyFilters(defaultSearchFilters())}>清除</View>
        </View>}
      </View>}

      <ScrollView
        className='student-search-content'
        scrollY={!filterPanelOpen}
        enhanced
        scrollIntoView={scrollAnchor}
        showScrollbar={false}
        lowerThreshold={120}
        onScrollToLower={() => void loadMore()}
      >
        <View id='search-content-start' />
        {!hasSearched && (
          <>
            {suggestionState === 'loading' && <Text className='student-search-suggestion-status'>正在读取历史与热搜…</Text>}
            {suggestionState === 'error' && <Text className='student-search-suggestion-status student-search-state--error' onClick={() => void loadSuggestions(scope)}>历史与热搜读取失败，点击重试</Text>}
            <View className='student-search-section'>
              <View className='student-search-section__title-row'>
                <Text className='student-search-section__title'>{copy.title}历史</Text>
                {history.length > 0 && <Text className='student-search-section__clear' onClick={() => void clearHistory()}>清空</Text>}
              </View>
              <View className='student-search-chip-row'>
                {history.length === 0 && suggestionState === 'ready' && <Text className='student-search-muted'>暂无搜索历史</Text>}
                {history.map((item) => (
                  <View key={item} className='student-search-chip student-search-chip--history' onClick={() => void submitSearch(item)}>
                    <Text>{item}</Text>
                  </View>
                ))}
              </View>
            </View>

            <View className='student-search-section student-search-section--popular'>
              <Text className='student-search-section__title'>{copy.title}热搜</Text>
              <View className='student-search-chip-row student-search-chip-row--popular'>
                {hot.length === 0 && suggestionState === 'ready' && <Text className='student-search-muted'>热搜数据正在积累</Text>}
                {hot.map((item, index) => (
                  <View key={item} className='student-search-chip student-search-chip--popular' onClick={() => void submitSearch(item)}>
                    {index < 3 && <Text className='student-search-hot'>热</Text>}
                    <Text>{item}</Text>
                  </View>
                ))}
              </View>
            </View>
          </>
        )}

        {hasSearched && (
          <View className='student-search-results'>
            <View className='student-search-results__head'>
              <Text>{loading ? '正在搜索…' : error && results.length === 0 ? '搜索失败' : `找到 ${total} 条结果`}</Text>
              <Text className='student-search-results__reset' onClick={showSuggestions}>返回推荐</Text>
            </View>
            {loading && [0, 1, 2].map((row) => <View className='student-search-skeleton' key={row}>
              <View className='student-search-skeleton__lines'><View /><View /><View /></View><View className='student-search-skeleton__media' />
            </View>)}
            {error && results.length === 0 && (
              <Text
                className='student-search-state student-search-state--error'
                onClick={() => void submitSearch(submittedKeyword)}
              >
                {error}
              </Text>
            )}
            {!error && !loading && results.length === 0 && <>
              <Text className='student-search-state'>{copy.empty}</Text>
              {searchFilterLabels(scope === 'community' || scope === 'secondhand' ? scope : 'community', appliedFilters).length > 0 && <Text className='student-search-state student-search-state--action' onClick={() => applyFilters(defaultSearchFilters())}>清除筛选</Text>}
            </>}
            {results.map(renderResult)}
            {error && results.length > 0 && <Text className='student-search-state student-search-state--error' onClick={() => void loadMore()}>{error}</Text>}
            {loadingMore && <Text className='student-search-state'>正在加载更多结果…</Text>}
            {!error && results.length > 0 && (endReached || results.length >= total) && (
              <Text className='student-search-state'>— 已显示全部结果 —</Text>
            )}
          </View>
        )}
      </ScrollView>
      {filterPanelOpen && (scope === 'community' || scope === 'secondhand') && <SearchFilterPanel scope={scope} value={appliedFilters} onCancel={() => setFilterPanelOpen(false)} onApply={applyFilters} />}
    </View>
  )
}

function SearchPage() {
  const session = useSessionStore((state) => `${state.profile.id}:${state.token}`)
  const scope = Taro.getCurrentInstance().router?.params?.scope ?? ''
  return <SearchContent key={`${session}:${scope}`} />
}

export default withFeaturePage(SearchPage)
