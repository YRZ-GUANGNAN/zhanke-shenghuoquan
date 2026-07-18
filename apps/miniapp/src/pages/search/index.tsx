import { useEffect, useMemo, useState } from 'react'
import { Input, Text, View } from '@tarojs/components'
import type { CommunityPost, Product } from '../../types'
import Taro from '../../services/taro'
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

type SearchScope = 'community' | 'secondhand' | 'takeout'
type SearchResult = CommunityPost | Product | ExternalTakeoutMerchantLink

const scopeCopy: Record<SearchScope, { title: string; placeholder: string; empty: string }> = {
  community: { title: '帖子搜索', placeholder: '搜索帖子内容或标签', empty: '没有找到相关帖子' },
  secondhand: { title: '二手搜索', placeholder: '搜索二手商品', empty: '没有找到相关商品' },
  takeout: { title: '外卖搜索', placeholder: '搜索外卖商家', empty: '没有找到相关商家' },
}

export function isSearchScope(value: string | undefined): value is SearchScope {
  return value === 'community' || value === 'secondhand' || value === 'takeout'
}

function errorMessage(error: unknown): string {
  if (error instanceof Error && error.message.trim()) return error.message
  return '请求失败，请稍后重试'
}

export default function SearchPage() {
  const rawScope = Taro.getCurrentInstance().router?.params?.scope
  const scope = isSearchScope(rawScope) ? rawScope : null
  const copy = scope ? scopeCopy[scope] : null
  const initialKeyword = Taro.getCurrentInstance().router?.params?.keyword ?? ''
  const [keyword, setKeyword] = useState(initialKeyword)
  const [history, setHistory] = useState<string[]>([])
  const [hot, setHot] = useState<string[]>([])
  const [results, setResults] = useState<SearchResult[]>([])
  const [loading, setLoading] = useState(false)
  const [hasSearched, setHasSearched] = useState(false)
  const [error, setError] = useState('')

  const resultCount = useMemo(() => results.length, [results])

  const loadSuggestions = async (activeScope: SearchScope) => {
    try {
      if (activeScope === 'community') {
        const [historyItems, hotItems] = await Promise.all([
          listCommunitySearchHistory(),
          listCommunityHotSearches(),
        ])
        setHistory(historyItems)
        setHot(hotItems.map((item) => item.keyword))
        return
      }
      if (activeScope === 'secondhand') {
        const [historyItems, hotItems] = await Promise.all([
          listTradeSearchHistory(),
          listTradeHotSearches(),
        ])
        setHistory(historyItems)
        setHot(hotItems.map((item) => item.keyword))
        return
      }
      const [historyItems, hotItems] = await Promise.all([
        listExternalTakeoutSearchHistory(),
        listExternalTakeoutHotSearches(),
      ])
      setHistory(historyItems)
      setHot(hotItems.map((item) => item.keyword))
    } catch (loadError) {
      console.error('load domain search suggestions failed', loadError)
    }
  }

  useEffect(() => {
    if (!scope) return
    void loadSuggestions(scope)
  }, [scope])

  const submitSearch = async (candidate = keyword) => {
    if (!scope || loading) return
    const normalized = candidate.trim()
    if (!normalized) {
      void Taro.showToast({ title: '请输入搜索词', icon: 'none' })
      return
    }
    setKeyword(normalized)
    setLoading(true)
    setError('')
    setHasSearched(true)
    try {
      if (scope === 'community') {
        setResults((await searchCommunityPosts(normalized)).items)
      } else if (scope === 'secondhand') {
        setResults((await searchTradeProducts(normalized)).items)
      } else {
        setResults((await searchExternalTakeoutMerchants(normalized)).items)
      }
      await loadSuggestions(scope)
    } catch (searchError) {
      console.error('domain search failed', searchError)
      setResults([])
      setError(errorMessage(searchError))
    } finally {
      setLoading(false)
    }
  }

  const clearHistory = async () => {
    if (!scope) return
    try {
      if (scope === 'community') await clearCommunitySearchHistory()
      else if (scope === 'secondhand') await clearTradeSearchHistory()
      else await clearExternalTakeoutSearchHistory()
      setHistory([])
      void Taro.showToast({ title: '搜索历史已清空', icon: 'success' })
    } catch (clearError) {
      console.error('clear domain search history failed', clearError)
      void Taro.showToast({ title: '清空失败，请重试', icon: 'none' })
    }
  }

  const openResult = async (item: SearchResult) => {
    if (!scope) return
    if (scope === 'community') {
      await Taro.navigateTo({ url: `/pages/post-detail/index?id=${encodeURIComponent(item.id)}` })
      return
    }
    if (scope === 'secondhand') {
      await Taro.navigateTo({ url: `/pages/secondhand-detail/index?id=${encodeURIComponent(item.id)}` })
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
      return (
        <View className='student-search-result' key={post.id} onClick={() => void openResult(post)}>
          <Text className='student-search-result__title'>{post.title}</Text>
          <Text className='student-search-result__summary'>{post.content || '暂无正文'}</Text>
          {(post.tags ?? []).length > 0 && (
            <View className='student-search-result__tags'>
              {(post.tags ?? []).map((tag) => <Text key={tag}>#{tag.replace(/^#+/, '')}</Text>)}
            </View>
          )}
          <Text className='student-search-result__meta'>{post.author} · {post.createdAt}</Text>
        </View>
      )
    }
    if (scope === 'secondhand') {
      const product = item as Product
      return (
        <View className='student-search-result' key={product.id} onClick={() => void openResult(product)}>
          <View className='student-search-result__row'>
            <Text className='student-search-result__title'>{product.title}</Text>
            <Text className='student-search-result__price'>¥{product.price}</Text>
          </View>
          <Text className='student-search-result__summary'>{product.description || '暂无商品描述'}</Text>
          <Text className='student-search-result__meta'>{product.category || '其他'} · {product.campus || '校内交易'}</Text>
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
      <View className='student-search-page'>
        <View className='student-search-header'>
          <View className='student-search-back' onClick={() => void Taro.navigateBack()} />
          <Text className='student-search-header__title'>搜索入口无效</Text>
        </View>
        <View className='student-search-invalid'>请从帖子、二手或外卖页面重新进入搜索。</View>
      </View>
    )
  }

  return (
    <View className='student-search-page'>
      <View className='student-search-header'>
        <View className='student-search-back' onClick={() => void Taro.navigateBack()} />
        <View className='student-search-box'>
          <View className='student-search-box__icon' />
          <Input
            className='student-search-box__input'
            value={keyword}
            placeholder={copy.placeholder}
            confirmType='search'
            onInput={(event) => setKeyword(event.detail.value)}
            onConfirm={() => void submitSearch()}
          />
        </View>
        <Text className='student-search-submit' onClick={() => void submitSearch()}>{loading ? '搜索中' : '搜索'}</Text>
      </View>

      <View className='student-search-content'>
        {!hasSearched && (
          <>
            <View className='student-search-section'>
              <View className='student-search-section__title-row'>
                <Text className='student-search-section__title'>{copy.title}历史</Text>
                {history.length > 0 && <Text className='student-search-section__clear' onClick={() => void clearHistory()}>清空</Text>}
              </View>
              <View className='student-search-chip-row'>
                {history.length === 0 && <Text className='student-search-muted'>暂无搜索历史</Text>}
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
                {hot.length === 0 && <Text className='student-search-muted'>热搜数据正在积累</Text>}
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
              <Text>{error ? '搜索失败' : `找到 ${resultCount} 条结果`}</Text>
              <Text className='student-search-results__reset' onClick={() => setHasSearched(false)}>返回推荐</Text>
            </View>
            {error && <Text className='student-search-state student-search-state--error'>{error}</Text>}
            {!error && !loading && results.length === 0 && <Text className='student-search-state'>{copy.empty}</Text>}
            {results.map(renderResult)}
          </View>
        )}
      </View>
    </View>
  )
}
