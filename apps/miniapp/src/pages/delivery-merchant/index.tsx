import React, { useState } from 'react'
import { Button, Input, Text, Textarea, View } from '@tarojs/components'
import { PageShell, SegmentedTabs } from '../../components'
import Taro from '../../services/taro'
import './index.scss'

type MerchantTab = 'waiting' | 'running' | 'done'

const tabs = [
  { key: 'waiting' as const, label: '待接单', count: 2 },
  { key: 'running' as const, label: '配送中', count: 1 },
  { key: 'done' as const, label: '已完成', count: 8 }
]

export default function DeliveryMerchantPage() {
  const [active, setActive] = useState<MerchantTab>('waiting')
  const [creating, setCreating] = useState(false)

  return (
    <PageShell plain refreshTitle='商家工作台已刷新'>
      <View className='hero-blue'>
        <View className='hero-blue__top'>
          <View>
            <Text className='hero-blue__title'>茶知味·大学城店</Text>
            <Text className='hero-blue__sub'>营业中 · 二食堂一楼</Text>
          </View>
          <Text className='hero-blue__switch' onClick={() => Taro.redirectTo({ url: '/pages/profile/index' })}>切换身份</Text>
        </View>
        <View className='stat-grid'>
          <View className='stat-cell' onClick={() => Taro.navigateTo({ url: '/pages/delivery-wallet/index?role=merchant' })}>
            <Text>86.50</Text>
            <Text>钱包余额</Text>
          </View>
          <View className='stat-cell'>
            <Text>12</Text>
            <Text>今日单量</Text>
          </View>
          <View className='stat-cell'>
            <Text>2</Text>
            <Text>配送中</Text>
          </View>
        </View>
      </View>
      <View className='quick-grid'>
        <Quick icon='+' label='发布' onClick={() => setCreating(true)} />
        <Quick icon='充' label='充值' onClick={() => Taro.navigateTo({ url: '/pages/delivery-wallet/index?role=merchant&action=recharge' })} />
        <Quick icon='账' label='账单' onClick={() => Taro.navigateTo({ url: '/pages/delivery-wallet/index?role=merchant' })} />
        <Quick icon='信' label='消息' onClick={() => Taro.navigateTo({ url: '/pages/delivery-conversation/index' })} />
      </View>
      <SegmentedTabs items={tabs} active={active} onChange={setActive} />
      <View className='list-stack'>
        {[0, 1].map((index) => (
          <View key={index} className='delivery-card-live'>
            <View className='delivery-card-live__top'>
              <Text>#B10{index + 1} · 12:{36 + index * 2}下单</Text>
              <Text>{active === 'done' ? '已完成' : active === 'running' ? '配送中' : '待接单'}</Text>
            </View>
            <RouteRow label='取货点' value='茶知味·大学城店' />
            <RouteRow label='送达点' value={index === 0 ? '12栋 3楼 305室' : '6栋 5楼 512室'} />
            <RouteRow label='备注' value='放门口取餐架，到楼下打电话' />
            <View className='delivery-card-live__foot'>
              <Text>配送费 ¥1.5</Text>
              <Button className='secondary-btn' onClick={() => Taro.navigateTo({ url: '/pages/delivery-conversation/index' })}>联系骑手</Button>
            </View>
          </View>
        ))}
      </View>
      {creating && <CreateSheet onClose={() => setCreating(false)} />}
    </PageShell>
  )
}

function Quick({ icon, label, onClick }: { icon: string; label: string; onClick: () => void }) {
  return (
    <View className='quick-grid__item' onClick={onClick}>
      <Text className='quick-grid__icon'>{icon}</Text>
      <Text>{label}</Text>
    </View>
  )
}

function RouteRow({ label, value }: { label: string; value: string }) {
  return (
    <View className='route-row-live'>
      <Text>{label}</Text>
      <Text>{value}</Text>
    </View>
  )
}

function CreateSheet({ onClose }: { onClose: () => void }) {
  return (
    <View className='sheet-live'>
      <View className='sheet-live__panel'>
        <View className='sheet-live__head'>
          <Text>发布配送单</Text>
          <Text onClick={onClose}>关闭</Text>
        </View>
        <View className='form-card merchant-create-form'>
          <View className='form-row'>
            <Text className='form-row__label'>联系人</Text>
            <Input value='王老板' disabled />
          </View>
          <View className='form-row'>
            <Text className='form-row__label'>电话</Text>
            <Input value='139 2280 6612' disabled />
          </View>
          <View className='form-row'>
            <Text className='form-row__label'>送达地址</Text>
            <Input placeholder='如：3栋 4楼 412室' />
          </View>
          <View className='form-row'>
            <Text className='form-row__label'>备注</Text>
            <Textarea placeholder='选填，给骑手的提示' />
          </View>
          <View className='fee-row-live'>
            <Text>配送费</Text>
            <Text>¥1.5</Text>
          </View>
          <Button className='primary-btn' onClick={onClose}>发布并支付</Button>
        </View>
      </View>
    </View>
  )
}
