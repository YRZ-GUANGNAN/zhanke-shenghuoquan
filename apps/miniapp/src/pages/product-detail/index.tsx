import React from 'react'
import { Button, Text, View } from '@tarojs/components'
import { AvatarMark, ImageTile, PageHeader, PageShell } from '../../components'
import Taro from '../../services/taro'
import './index.scss'

export default function ProductDetailPage() {
  return (
    <PageShell plain refreshTitle='宝贝详情已刷新'>
      <PageHeader title='宝贝详情' />
      <View className='detail-hero-image'>
        <ImageTile tone={0} label='自行车' />
      </View>
      <View className='zk-card'>
        <View className='detail-price-row'>
          <Text>¥150</Text>
          <Text className='zk-chip zk-chip--green'>出售中</Text>
        </View>
        <Text className='detail-title-live'>九成新自行车，毕业急出，车况好无锈</Text>
        <Text className='detail-copy-live'>大概 10 月买的，平时只在校内骑，刹车正常，车锁一起送。北门或宿舍区自提，诚心可小刀。</Text>
        <View className='tag-row-live'>
          <Text>代步工具</Text>
          <Text>可小刀</Text>
          <Text>自提优先</Text>
        </View>
      </View>
      <View className='seller-row-live'>
        <AvatarMark label='向' />
        <View>
          <Text>向北</Text>
          <Text>湛科校区 · 学生认证</Text>
        </View>
        <Text onClick={() => Taro.navigateTo({ url: '/pages/chat/index' })}>联系</Text>
      </View>
      <View className='safe-note-live'>
        <Text>交易提醒</Text>
        <Text>线下验货后再付款，不要提前转账给陌生人。</Text>
      </View>
      <Button className='primary-btn page-bottom-action' onClick={() => Taro.navigateTo({ url: '/pages/chat/index' })}>联系卖家</Button>
    </PageShell>
  )
}
