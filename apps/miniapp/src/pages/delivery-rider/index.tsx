import React, { useState } from 'react'
import { Button, Text, View } from '@tarojs/components'
import { PageShell, SegmentedTabs } from '../../components'
import Taro from '../../services/taro'
import './index.scss'

type RiderTab = 'claim' | 'pickup' | 'deliver'

const tabs = [
  { key: 'claim' as const, label: '待抢单', count: 5 },
  { key: 'pickup' as const, label: '待取货', count: 1 },
  { key: 'deliver' as const, label: '待送达', count: 1 }
]

const orders = [
  ['A2048', '蜜雪冰城·大学城店', '3栋 · 4楼', '冰淇淋打包好别洒了，放门口就行', '138****2048'],
  ['A2051', '华莱士·西门店', '7栋 · 2楼', '到楼下打电话', '176****0093'],
  ['A2053', '正新鸡排·三食堂', '5栋 · 4楼', '轻放，谢谢', '150****7723']
]

export default function DeliveryRiderPage() {
  const [active, setActive] = useState<RiderTab>('claim')

  return (
    <PageShell plain refreshTitle='骑手大厅已刷新'>
      <View className='hero-blue'>
        <View className='hero-blue__top'>
          <View>
            <Text className='hero-blue__title'>李伟 · 骑手</Text>
            <Text className='hero-blue__sub'>已认证 · 湛江科技学院</Text>
          </View>
          <Text className='hero-blue__switch' onClick={() => Taro.redirectTo({ url: '/pages/profile/index' })}>切换身份</Text>
        </View>
        <View className='rider-balance-live'>
          <Text>钱包余额（元）</Text>
          <Text>¥128.50</Text>
          <Text>今日收入 ¥18.00 ｜ 已完成 3 单</Text>
        </View>
      </View>
      <SegmentedTabs items={tabs} active={active} onChange={setActive} />
      <View className='section-head-live section-head-live--pad'>
        <Text>{active === 'claim' ? '手快有单 · 共 5 单可抢' : '我的配送'}</Text>
        <Text onClick={() => Taro.showToast({ title: '已刷新', icon: 'success' })}>刷新</Text>
      </View>
      <View className='list-stack'>
        {orders.map((order, index) => (
          <View key={order[0]} className='delivery-card-live'>
            <View className='delivery-card-live__top'>
              <Text>#{order[0]} · 12:{32 + index * 3}下单</Text>
              <Text>配送费 ¥1.5</Text>
            </View>
            <RouteLine dot='blue' label='取货点' value={order[1]} />
            <RouteLine dot='orange' label='送达点' value={active === 'claim' ? order[2] : `${order[2]} 412室`} />
            <View className='delivery-note-live'>备注　{order[3]}</View>
            <View className='delivery-card-live__foot'>
              <Text>{active === 'claim' ? `${order[4].slice(0, 3)}****${order[4].slice(-4)}` : order[4]}</Text>
              {active === 'claim' ? (
                <Button className='primary-btn' onClick={() => setActive('pickup')}>抢单</Button>
              ) : (
                <Button className='secondary-btn' onClick={() => Taro.navigateTo({ url: '/pages/delivery-conversation/index' })}>联系商家</Button>
              )}
            </View>
          </View>
        ))}
      </View>
    </PageShell>
  )
}

function RouteLine({ dot, label, value }: { dot: 'blue' | 'orange'; label: string; value: string }) {
  return (
    <View className='route-line-live'>
      <Text className={`route-line-live__dot route-line-live__dot--${dot}`} />
      <Text>{label}</Text>
      <Text>{value}</Text>
    </View>
  )
}
