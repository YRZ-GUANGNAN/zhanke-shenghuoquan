import React from 'react'
import { Text, View } from '@tarojs/components'
import { PageShell, SectionTitle } from '../../components'
import Taro from '../../services/taro'
import './index.scss'

const shops = [
  ['茶', '茶知味·大学城店', '奶茶果茶 · 芋泥波波', '营业中', '#2E7DD4'],
  ['华', '华莱士·西门店', '炸鸡汉堡 · 全鸡桶', '营业中', '#FF7A45'],
  ['正', '正新鸡排·三食堂', '小吃炸串 · 香辣鸡排', '营业中', '#FF6B8A'],
  ['蜜', '蜜雪冰城·大学城店', '冰淇淋与茶 · 柠檬水', '休息中', '#1D9E75']
]

export default function FoodPage() {
  return (
    <PageShell active='food' refreshTitle='商家已刷新'>
      <View className='plain-hero'>
        <Text className='plain-hero__kicker'>合作商家</Text>
        <Text className='plain-hero__title'>外卖聚合</Text>
        <Text className='plain-hero__sub'>在商家小程序下单后，商家通过湛科生活圈发布配送单，由校园骑手配送。</Text>
      </View>
      <SectionTitle title='附近商家' sub='点击跳转' />
      <View className='list-stack'>
        {shops.map((shop, index) => (
          <View key={shop[1]} className='shop-row' onClick={() => Taro.navigateTo({ url: `/pages/food-detail/index?shop=${index}` })}>
            <View className='shop-row__logo' style={{ background: String(shop[4]) }}>{shop[0]}</View>
            <View className='shop-row__main'>
              <Text>{shop[1]}</Text>
              <Text>{shop[2]}</Text>
            </View>
            <Text className={shop[3] === '营业中' ? 'zk-chip zk-chip--green' : 'zk-chip'}>{shop[3]}</Text>
          </View>
        ))}
      </View>
    </PageShell>
  )
}
