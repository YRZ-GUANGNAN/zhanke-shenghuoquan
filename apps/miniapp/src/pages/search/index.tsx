import React from 'react'
import { Text, View } from '@tarojs/components'
import Taro from '../../services/taro'
import './index.scss'

type SearchKeyword = {
  label: string
  hot?: boolean
}

const historyKeywords = ['芋泥波波', '自行车', '四级']

const popularKeywords: SearchKeyword[] = [
  { label: '芋泥波波', hot: true },
  { label: '自行车', hot: true },
  { label: '兼职' },
  { label: '四级资料' },
  { label: '搭子' },
  { label: '宿舍改造' }
]

function goBack() {
  void Taro.navigateBack()
}

function previewSearch(keyword: string) {
  void Taro.showToast({ title: `搜索：${keyword}`, icon: 'none', duration: 1200 })
}

function clearHistory() {
  void Taro.showToast({ title: '静态预览，不清空', icon: 'none', duration: 1200 })
}

export default function SearchPage() {
  const rawKeyword = Taro.getCurrentInstance().router?.params?.keyword
  let keyword = rawKeyword || ''
  try {
    keyword = rawKeyword ? decodeURIComponent(rawKeyword) : ''
  } catch {
    keyword = rawKeyword || ''
  }

  return (
    <View className='student-search-page'>
      <View className='student-search-header'>
        <View className='student-search-back' onClick={goBack} />
        <View className='student-search-box'>
          <View className='student-search-box__icon' />
          <Text className='student-search-box__placeholder'>{keyword || '搜帖子、二手好物'}</Text>
        </View>
      </View>

      <View className='student-search-content'>
        <View className='student-search-section'>
          <View className='student-search-section__title-row'>
            <Text className='student-search-section__title'>搜索历史</Text>
            <Text className='student-search-section__clear' onClick={clearHistory}>清空</Text>
          </View>

          <View className='student-search-chip-row'>
            {historyKeywords.map((keyword) => (
              <View
                key={keyword}
                className='student-search-chip student-search-chip--history'
                onClick={() => previewSearch(keyword)}
              >
                <Text>{keyword}</Text>
              </View>
            ))}
          </View>
        </View>

        <View className='student-search-section student-search-section--popular'>
          <Text className='student-search-section__title'>大家都在搜</Text>
          <View className='student-search-chip-row student-search-chip-row--popular'>
            {popularKeywords.map((keyword) => (
              <View
                key={keyword.label}
                className='student-search-chip student-search-chip--popular'
                onClick={() => previewSearch(keyword.label)}
              >
                {keyword.hot && <Text className='student-search-hot'>热</Text>}
                <Text>{keyword.label}</Text>
              </View>
            ))}
          </View>
        </View>
      </View>
    </View>
  )
}
