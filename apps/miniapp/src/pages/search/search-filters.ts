import type { CommunitySearchFilters } from '../../services/b-community'
import type { TradeSearchFilters } from '../../services/c-trade'

export type FilterScope = 'community' | 'secondhand'
type Sort = NonNullable<CommunitySearchFilters['sort'] | TradeSearchFilters['sort']>
export interface SearchFilterValues {
  sort: Sort
  contentType: NonNullable<CommunitySearchFilters['contentType']>
  withinDays: NonNullable<CommunitySearchFilters['withinDays']>
  category: string
  minPrice: string
  maxPrice: string
}

export const postSortOptions: { value: Sort; label: string }[] = [
  { value: 'relevance', label: '综合' }, { value: 'recent', label: '最新' },
  { value: 'likes', label: '最多点赞' }, { value: 'comments', label: '最多评论' }, { value: 'favorites', label: '最多收藏' },
]
export const tradeSortOptions: { value: Sort; label: string }[] = [
  { value: 'relevance', label: '综合' }, { value: 'recent', label: '最新' },
  { value: 'price_asc', label: '价格从低到高' }, { value: 'price_desc', label: '价格从高到低' },
]
// Same categories as the existing market and publishing pages; no new product taxonomy.
export const categoryOptions = ['', '数码', '书籍资料', '生活用品', '代步', '其他']
export const contentOptions: { value: SearchFilterValues['contentType']; label: string }[] = [
  { value: 'all', label: '全部内容' }, { value: 'image', label: '有图' }, { value: 'text', label: '纯文字' },
]
export const timeOptions: { value: SearchFilterValues['withinDays']; label: string }[] = [
  { value: 0, label: '不限' }, { value: 1, label: '一天内' }, { value: 7, label: '一周内' }, { value: 180, label: '半年内' },
]

export function defaultSearchFilters(): SearchFilterValues {
  return { sort: 'relevance', contentType: 'all', withinDays: 0, category: '', minPrice: '', maxPrice: '' }
}

export function communitySearchFilters(value: SearchFilterValues): CommunitySearchFilters {
  const filters: CommunitySearchFilters = {}
  if (value.sort !== 'relevance' && postSortOptions.some((option) => option.value === value.sort)) filters.sort = value.sort as CommunitySearchFilters['sort']
  if (value.contentType !== 'all') filters.contentType = value.contentType
  if (value.withinDays) filters.withinDays = value.withinDays
  return filters
}

function priceInCents(value: string): number | undefined {
  const normalized = value.trim()
  if (!normalized) return undefined
  if (!/^\d{1,8}(\.\d{1,2})?$/.test(normalized)) throw new Error('价格须为不超过两位小数的非负金额')
  const [yuan, fraction = ''] = normalized.split('.')
  return Number(yuan) * 100 + Number(fraction.padEnd(2, '0'))
}

export function tradeSearchFilters(value: SearchFilterValues): TradeSearchFilters {
  const filters: TradeSearchFilters = {}
  if (value.sort !== 'relevance' && tradeSortOptions.some((option) => option.value === value.sort)) filters.sort = value.sort as TradeSearchFilters['sort']
  if (value.category) filters.category = value.category
  const min = priceInCents(value.minPrice), max = priceInCents(value.maxPrice)
  if (min !== undefined && max !== undefined && min > max) throw new Error('最低价格不能高于最高价格')
  if (min !== undefined) filters.minPrice = min
  if (max !== undefined) filters.maxPrice = max
  return filters
}

export function searchFilterKey(scope: FilterScope, value: SearchFilterValues): string {
  return JSON.stringify(scope === 'community' ? communitySearchFilters(value) : tradeSearchFilters(value))
}

export function searchFilterLabels(scope: FilterScope, value: SearchFilterValues): string[] {
  const labels: string[] = []
  const sorts = scope === 'community' ? postSortOptions : tradeSortOptions
  if (value.sort !== 'relevance') labels.push(sorts.find((option) => option.value === value.sort)?.label || '')
  if (scope === 'community') {
    if (value.contentType !== 'all') labels.push(contentOptions.find((option) => option.value === value.contentType)?.label || '')
    if (value.withinDays) labels.push(timeOptions.find((option) => option.value === value.withinDays)?.label || '')
  } else {
    if (value.category) labels.push(value.category)
    const { minPrice, maxPrice } = tradeSearchFilters(value)
    if (minPrice !== undefined && maxPrice !== undefined) labels.push(`¥${minPrice / 100}–${maxPrice / 100}`)
    else if (minPrice !== undefined) labels.push(`¥${minPrice / 100}起`)
    else if (maxPrice !== undefined) labels.push(`¥${maxPrice / 100}以内`)
  }
  return labels.filter(Boolean)
}
