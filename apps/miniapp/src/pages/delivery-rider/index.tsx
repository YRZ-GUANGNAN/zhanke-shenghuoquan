import React, { useState } from 'react'
import { Text, View } from '@tarojs/components'
import Taro from '../../services/taro'
import './index.scss'

type RiderTab = 'grab' | 'pickup' | 'delivery'

interface GrabOrder {
  id: string
  time: string
  shop: string
  floor: string
  remark?: string
  phone: string
}

const tabs: Array<{ key: RiderTab; label: string; count: number }> = [
  { key: 'grab', label: '待抢单', count: 5 },
  { key: 'pickup', label: '待取货', count: 1 },
  { key: 'delivery', label: '待送达', count: 1 }
]

const grabOrders: GrabOrder[] = [
  {
    id: '#A2048',
    time: '12:32下单',
    shop: '蜜雪冰城·大学城店',
    floor: '3栋 · 4楼',
    remark: '冰淇淋打包好别洒了，放门口就行',
    phone: '138****2048'
  },
  {
    id: '#A2051',
    time: '12:35下单',
    shop: '华莱士·西门店',
    floor: '7栋 · 2楼',
    phone: '176****0093'
  },
  {
    id: '#A2053',
    time: '12:38下单',
    shop: '塔斯汀·大学城店',
    floor: '5栋 · 4楼',
    remark: '麻烦快一点，到了打电话',
    phone: '150****3390'
  }
]

function showStaticToast(title: string) {
  void Taro.showToast({ title, icon: 'none', duration: 1400 })
}

export default function DeliveryRiderPage() {
  const [activeTab, setActiveTab] = useState<RiderTab>('grab')

  return (
    <View className='rider-grab-page'>
      <View className='rider-grab-hero'>
        <View className='rider-grab-hero__orb rider-grab-hero__orb--one' />
        <View className='rider-grab-hero__orb rider-grab-hero__orb--two' />

        <View className='rider-grab-user'>
          <View className='rider-grab-user__avatar'>
            <Text>李</Text>
          </View>
          <View className='rider-grab-user__main'>
            <View className='rider-grab-user__name-row'>
              <Text className='rider-grab-user__name'>李伟</Text>
              <Text className='rider-grab-user__badge'>骑手</Text>
            </View>
            <Text className='rider-grab-user__meta'>已认证 · 湛江科技学院</Text>
          </View>
          <View
            className='rider-grab-user__switch'
            onClick={() => showStaticToast('身份切换阶段二接入')}
          >
            <Text className='rider-grab-user__switch-icon'>⇄</Text>
            <Text>切换身份</Text>
          </View>
        </View>

        <View className='rider-grab-wallet-summary'>
          <View className='rider-grab-wallet-summary__main'>
            <Text className='rider-grab-wallet-summary__label'>钱包余额（元）</Text>
            <View className='rider-grab-wallet-summary__amount-row'>
              <Text className='rider-grab-wallet-summary__currency'>¥</Text>
              <Text className='rider-grab-wallet-summary__amount'>128.50</Text>
            </View>
            <Text className='rider-grab-wallet-summary__sub'>今日收入 ¥18.00　｜　已完成 3 单</Text>
          </View>
          <View
            className='rider-grab-wallet-summary__button'
            onClick={() => showStaticToast('钱包页 UI-05 接入')}
          >
            <View className='rider-grab-wallet-summary__wallet-icon' />
            <Text>钱包</Text>
          </View>
        </View>
      </View>

      <View className='rider-grab-tabs'>
        {tabs.map((tab) => (
          <View
            key={tab.key}
            className={`rider-grab-tabs__item ${activeTab === tab.key ? 'rider-grab-tabs__item--active' : ''}`}
            onClick={() => setActiveTab(tab.key)}
          >
            <Text>{tab.label}</Text>
            <Text className={`rider-grab-tabs__count rider-grab-tabs__count--${tab.key}`}>{tab.count}</Text>
          </View>
        ))}
      </View>

      <View className='rider-grab-content'>
        {activeTab === 'grab' ? (
          <>
            <View className='rider-grab-list-head'>
              <Text>手快有 · 共 5 单可抢</Text>
              <View className='rider-grab-list-head__refresh' onClick={() => showStaticToast('刷新静态占位')}>
                <Text>↻</Text>
                <Text>刷新</Text>
              </View>
            </View>
            <View className='rider-grab-list'>
              {grabOrders.map((order) => (
                <View key={order.id} className='rider-order-card'>
                  <View className='rider-order-card__top'>
                    <View className='rider-order-card__serial'>
                      <Text>{order.id}</Text>
                      <Text className='rider-order-card__dot'>·</Text>
                      <Text>{order.time}</Text>
                    </View>
                    <View className='rider-order-card__fee'>
                      <Text>配送费 </Text>
                      <Text className='rider-order-card__fee-price'>¥1.5</Text>
                    </View>
                  </View>

                  <View className='rider-order-route'>
                    <View className='rider-order-route__line' />
                    <View className='rider-order-route__row'>
                      <View className='rider-order-route__marker rider-order-route__marker--pickup' />
                      <View className='rider-order-route__info'>
                        <Text className='rider-order-route__label'>取货点</Text>
                        <Text className='rider-order-route__place'>{order.shop}</Text>
                      </View>
                    </View>
                    <View className='rider-order-route__row rider-order-route__row--drop'>
                      <View className='rider-order-route__marker rider-order-route__marker--drop' />
                      <View className='rider-order-route__info'>
                        <Text className='rider-order-route__label'>送达点</Text>
                        <View className='rider-order-route__drop-row'>
                          <Text className='rider-order-route__place'>{order.floor}</Text>
                          <Text className='rider-order-route__lock'>锁 抢单后显示门牌</Text>
                        </View>
                      </View>
                    </View>
                  </View>

                  {order.remark && (
                    <View className='rider-order-card__remark'>
                      <Text className='rider-order-card__remark-label'>备注</Text>
                      <Text>{order.remark}</Text>
                    </View>
                  )}

                  <View className='rider-order-card__bottom'>
                    <View className='rider-order-card__phone'>
                      <Text className='rider-order-card__phone-icon'>☎</Text>
                      <Text>{order.phone}</Text>
                    </View>
                    <View
                      className='rider-order-card__grab'
                      onClick={() => showStaticToast('抢单逻辑阶段二接入')}
                    >
                      <Text>抢单</Text>
                    </View>
                  </View>
                </View>
              ))}
            </View>
          </>
        ) : (
          <View className='rider-grab-placeholder'>
            <View className='rider-grab-placeholder__icon'>
              <Text>{activeTab === 'pickup' ? '取' : '送'}</Text>
            </View>
            <Text className='rider-grab-placeholder__title'>
              {activeTab === 'pickup' ? '待取货内容将在 UI-04 完整还原' : '待送达内容将在 UI-04 完整还原'}
            </Text>
            <Text className='rider-grab-placeholder__desc'>本页聚焦抢单大厅静态门面，完整履约与拍照弹层后续页面接入。</Text>
          </View>
        )}
      </View>
    </View>
  )
}
