import React, { useMemo, useState } from 'react'
import { Button, Text, View } from '@tarojs/components'
import { ImageTile, PageHeader, PageShell } from '../../components'
import Taro from '../../services/taro'
import './index.scss'

const shops = [
  ['茶知味·大学城店', '满 15 减 3 · 月售 1200+', '奶茶', '#2e7dd4'],
  ['华莱士·西门店', '30 分钟送达 · 热门小吃', '汉堡', '#ff7a45'],
  ['正新鸡排·三食堂', '下课高峰热门小吃', '鸡排', '#ff6b8a'],
  ['蜜雪冰城·大学城店', '冰淇淋与茶 · 新品上架', '雪', '#1d9e75']
]

const dishes = [
  ['芋泥波波奶茶', '绵密芋泥，加一份波波更香', '8.00', '奶'],
  ['柠檬茶', '清爽少糖，下午课续命款', '6.50', '茶'],
  ['招牌鸡排饭', '现炸鸡排，配青菜和例汤', '12.00', '饭'],
  ['冰淇淋红茶', '甜度适中，配送也不容易洒', '7.00', '冰']
]

export default function FoodDetailPage() {
  const params = Taro.getCurrentInstance().router?.params ?? {}
  const shopIndex = Number(params.shop ?? 0)
  const shop = shops[Number.isFinite(shopIndex) ? shopIndex % shops.length : 0]
  const [cart, setCart] = useState(0)
  const total = useMemo(() => (cart * 8).toFixed(2), [cart])

  return (
    <PageShell plain refreshTitle='商家菜单已刷新'>
      <PageHeader title='商家详情' right={<Text onClick={() => Taro.navigateTo({ url: '/pages/chat/index' })}>联系</Text>} />
      <View className='food-detail-hero'>
        <View className='food-detail-hero__top'>
          <View className='shop-row__logo' style={{ background: shop[3] }}>
            <Text>{shop[2]}</Text>
          </View>
          <View className='food-detail-hero__main'>
            <Text>{shop[0]}</Text>
            <Text>{shop[1]} · 配送费 ¥1.5</Text>
          </View>
          <Text className='zk-chip zk-chip--green'>营业中</Text>
        </View>
        <View className='food-tags-live'>
          <Text>学生优选</Text>
          <Text>可送宿舍楼下</Text>
          <Text>支持备注</Text>
        </View>
      </View>
      <View className='section-head-live section-head-live--pad'>
        <Text>热卖菜单</Text>
        <Text>{cart > 0 ? `已选 ${cart}` : '点一份试试'}</Text>
      </View>
      <View className='dish-list-live'>
        {dishes.map((dish, index) => (
          <View key={dish[0]} className='dish-row-live'>
            <ImageTile tone={index} label={dish[3]} />
            <View className='dish-row-live__main'>
              <Text>{dish[0]}</Text>
              <Text>{dish[1]}</Text>
              <Text>¥{dish[2]}</Text>
            </View>
            <Button onClick={() => setCart((value) => value + 1)}>+</Button>
          </View>
        ))}
      </View>
      <View className='cart-bar-live'>
        <View>
          <Text>{cart > 0 ? `合计 ¥${total}` : '购物车空空'}</Text>
          <Text>{cart > 0 ? `共 ${cart} 件 · 配送费另计` : '选择菜品后可提交订单'}</Text>
        </View>
        <Button
          className='primary-btn'
          onClick={() => Taro.showToast({ title: cart > 0 ? '订单已提交' : '先选一份菜品', icon: cart > 0 ? 'success' : 'none' })}
        >
          去结算
        </Button>
      </View>
    </PageShell>
  )
}
