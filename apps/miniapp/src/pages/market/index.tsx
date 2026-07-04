import React, { useState } from 'react'
import { Input, Text, View } from '@tarojs/components'
import { ImageTile, PageShell } from '../../components'
import Taro from '../../services/taro'
import './index.scss'

const filters = ['全部', '教材资料', '宿舍好物', '数码闲置', '代步']
const products = [
  ['九成新自行车，毕业急出，车况好无锈', '150', '出售中', '自行车', 0],
  ['四级真题一套 + 听力网课，半价出', '25', '出售中', '资料', 2],
  ['宿舍小冰箱 70L，自提优先', '188', '预订', '冰箱', 1],
  ['考研政治资料，笔记已经整理好', '38', '已售', '资料', 4]
]

export default function MarketPage() {
  const [active, setActive] = useState(filters[0])

  return (
    <PageShell active='market' refreshTitle='二手市场已刷新'>
      <View className='market-head-live'>
        <View>
          <Text>二手集市</Text>
          <Text>学生私下交易，平台只做展示与私聊连接</Text>
        </View>
        <Text onClick={() => Taro.navigateTo({ url: '/pages/publish/index?type=product' })}>发布</Text>
      </View>
      <View className='search-pill'>
        <Text>⌕</Text>
        <Input disabled placeholder='搜索自行车、教材、宿舍好物' />
      </View>
      <View className='filter-row'>
        {filters.map((item) => (
          <Text key={item} className={active === item ? 'filter-row__active' : ''} onClick={() => setActive(item)}>
            {item}
          </Text>
        ))}
      </View>
      <View className='product-grid'>
        {products.map((item) => (
          <View key={item[0]} className='product-card' onClick={() => Taro.navigateTo({ url: '/pages/product-detail/index' })}>
            <ImageTile tone={Number(item[4])} label={String(item[3])} />
            <View className='product-card__body'>
              <Text className='product-card__title'>{item[0]}</Text>
              <View className='product-card__foot'>
                <Text>¥{item[1]}</Text>
                <Text className={item[2] === '出售中' ? 'zk-chip zk-chip--green' : 'zk-chip'}>{item[2]}</Text>
              </View>
            </View>
          </View>
        ))}
      </View>
    </PageShell>
  )
}
