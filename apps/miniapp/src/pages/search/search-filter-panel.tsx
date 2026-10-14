import { useState } from 'react'
import { Input, ScrollView, Text, View } from '@tarojs/components'
import {
  categoryOptions, contentOptions, defaultSearchFilters, postSortOptions, timeOptions, tradeSortOptions,
  searchFilterKey, type FilterScope, type SearchFilterValues,
} from './search-filters'

interface Props {
  scope: FilterScope
  value: SearchFilterValues
  onCancel: () => void
  onApply: (value: SearchFilterValues) => void
}

export default function SearchFilterPanel({ scope, value, onCancel, onApply }: Props) {
  const [draft, setDraft] = useState(() => ({ ...value }))
  const [error, setError] = useState('')
  const update = (patch: Partial<SearchFilterValues>) => { setDraft((current) => ({ ...current, ...patch })); setError('') }
  const sorts = scope === 'community' ? postSortOptions : tradeSortOptions
  const apply = () => {
    try { searchFilterKey(scope, draft); onApply(draft) }
    catch (cause) { setError(cause instanceof Error ? cause.message : '请检查筛选条件') }
  }
  const option = (id: string, label: string, selected: boolean, onClick: () => void) => (
    <View id={id} key={id} className={`student-search-filter-option${selected ? ' student-search-filter-option--selected' : ''}`} hoverClass='student-search-control--pressed' role='button' ariaLabel={`${label}${selected ? '，已选中' : ''}`} onClick={onClick}>
      <Text>{label}</Text>
    </View>
  )
  return (
    <View className='student-search-filter-layer'>
      <View className='student-search-filter-backdrop' onClick={onCancel} catchMove />
      <View className='student-search-filter-panel' role='dialog' ariaLabel={scope === 'community' ? '帖子筛选' : '商品筛选'}>
        <View className='student-search-filter-heading'>
          <View><Text className='student-search-filter-title'>{scope === 'community' ? '帖子筛选' : '商品筛选'}</Text><Text className='student-search-filter-description'>选好条件后，点击确认筛选</Text></View>
          <View className='student-search-filter-cancel' role='button' onClick={onCancel}>取消</View>
        </View>
        <ScrollView scrollY showScrollbar={false} className='student-search-filter-scroll'>
          <View className='student-search-filter-group'>
            <Text className='student-search-filter-label'>排序方式</Text>
            <View className='student-search-filter-options'>{sorts.map((item) => option(`search-filter-sort-${item.value}`, item.label, draft.sort === item.value, () => update({ sort: item.value })))}</View>
          </View>
          {scope === 'community' ? <>
            <View className='student-search-filter-group'>
              <Text className='student-search-filter-label'>内容类型</Text>
              <View className='student-search-filter-options'>{contentOptions.map((item) => option(`search-filter-content-${item.value}`, item.label, draft.contentType === item.value, () => update({ contentType: item.value })))}</View>
            </View>
            <View className='student-search-filter-group'>
              <Text className='student-search-filter-label'>发布时间</Text>
              <View className='student-search-filter-options'>{timeOptions.map((item) => option(`search-filter-days-${item.value}`, item.label, draft.withinDays === item.value, () => update({ withinDays: item.value })))}</View>
            </View>
          </> : <>
            <View className='student-search-filter-group'>
              <Text className='student-search-filter-label'>商品分类</Text>
              <View className='student-search-filter-options'>{categoryOptions.map((item, index) => option(`search-filter-category-${index}`, item || '全部分类', draft.category === item, () => update({ category: item })))}</View>
            </View>
            <View className='student-search-filter-group'>
              <Text className='student-search-filter-label'>价格区间<Text className='student-search-filter-unit'>（元）</Text></Text>
              <View className='student-search-price-range'>
                <Input className='student-search-price-min' type='digit' maxlength={11} value={draft.minPrice} placeholder='最低价格' onInput={(event) => update({ minPrice: event.detail.value })} />
                <Text>至</Text>
                <Input className='student-search-price-max' type='digit' maxlength={11} value={draft.maxPrice} placeholder='最高价格' onInput={(event) => update({ maxPrice: event.detail.value })} />
              </View>
              <Text className='student-search-filter-hint'>可以只填一端，留空表示不限</Text>
            </View>
          </>}
        </ScrollView>
        {error && <View className='student-search-filter-error' role='alert'>{error}</View>}
        <View className='student-search-filter-footer'>
          <View className='student-search-filter-reset' role='button' hoverClass='student-search-control--pressed' onClick={() => { setDraft(defaultSearchFilters()); setError('') }}>重置</View>
          <View className='student-search-filter-apply' role='button' hoverClass='student-search-control--pressed' onClick={apply}>确认筛选</View>
        </View>
      </View>
    </View>
  )
}
