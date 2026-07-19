import React, { useMemo, useState } from 'react'
import { ScrollView, Text, View } from '@tarojs/components'
import IdentitySwitchModal from '../../components/IdentitySwitchModal'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import { identityRoute, useIdentityGuard } from '../../hooks/use-identity-guard'
import Taro from '../../services/taro'
import { useSessionStore } from '../../stores/session'
import { getCapsuleSafeStyle } from '../../utils/capsule-safe'
import './index.scss'

type MerchantTabKey = 'pending' | 'delivering' | 'completed'
type OrderStatusKind = 'waiting' | 'delivering' | 'pickup' | 'completed'

interface RiderInfo {
  name: string
  phone: string
  avatar: string
}

interface MerchantOrder {
  id: string
  orderTime: string
  deliveredTime?: string
  statusKind: OrderStatusKind
  statusText: string
  pickupName: string
  destination: string
  customerName: string
  customerPhone: string
  remark?: string
  fee: string
  rider?: RiderInfo
}

const tabs: Array<{ key: MerchantTabKey; label: string; count?: number }> = [
  { key: 'pending', label: '待接单', count: 2 },
  { key: 'delivering', label: '配送中', count: 2 },
  { key: 'completed', label: '已完成' }
]

const pendingOrders: MerchantOrder[] = [
  {
    id: 'B1085',
    orderTime: '12:47下单',
    statusKind: 'waiting',
    statusText: '待骑手接单',
    pickupName: '茶知味·大学城店',
    destination: '12栋 3楼 305室',
    customerName: '陈同学',
    customerPhone: '159 2201 8834',
    remark: '放宿舍楼下取餐架就行',
    fee: '1.5'
  },
  {
    id: 'B1083',
    orderTime: '12:44下单',
    statusKind: 'waiting',
    statusText: '待骑手接单',
    pickupName: '茶知味·大学城店',
    destination: '6栋 5楼 512室',
    customerName: '吴同学',
    customerPhone: '188 5530 2217',
    fee: '1.5'
  }
]

const deliveringOrders: MerchantOrder[] = [
  {
    id: 'B1079',
    orderTime: '12:31下单',
    statusKind: 'delivering',
    statusText: '配送中',
    pickupName: '茶知味·大学城店',
    destination: '3栋 4楼 412室',
    customerName: '林同学',
    customerPhone: '138 6620 2048',
    remark: '到楼下打电话',
    fee: '1.5',
    rider: {
      name: '李伟',
      phone: '137 5560 8814',
      avatar: '李'
    }
  },
  {
    id: 'B1077',
    orderTime: '12:26下单',
    statusKind: 'pickup',
    statusText: '待取货',
    pickupName: '茶知味·大学城店',
    destination: '7栋 2楼 205室',
    customerName: '黄同学',
    customerPhone: '176 3380 0093',
    fee: '1.5',
    rider: {
      name: '张强',
      phone: '150 2318 7745',
      avatar: '张'
    }
  }
]

const completedOrders: MerchantOrder[] = [
  {
    id: 'B1071',
    orderTime: '11:52下单',
    deliveredTime: '12:05送达',
    statusKind: 'completed',
    statusText: '已完成',
    pickupName: '茶知味·大学城店',
    destination: '9栋 5楼 523室',
    customerName: '刘同学',
    customerPhone: '133 9014 4501',
    fee: '1.5'
  },
  {
    id: 'B1066',
    orderTime: '11:20下单',
    deliveredTime: '11:38送达',
    statusKind: 'completed',
    statusText: '已完成',
    pickupName: '茶知味·大学城店',
    destination: '5栋 1楼 101室',
    customerName: '赵同学',
    customerPhone: '186 7712 3066',
    remark: '不要辣',
    fee: '1.5'
  }
]

const orderGroups: Record<MerchantTabKey, MerchantOrder[]> = {
  pending: pendingOrders,
  delivering: deliveringOrders,
  completed: completedOrders
}

const shortcuts = [
  { key: 'publish', label: '发布配送单', icon: '+' },
  { key: 'recharge', label: '充值', icon: '↟' },
  { key: 'bill', label: '账单', icon: '▤' },
  { key: 'message', label: '消息', icon: '□' }
]

function showPlaceholder(title: string) {
  void Taro.showToast({ title, icon: 'none', duration: 1400 })
}

function openMerchantConversation(orderId = 'B1079') {
  void Taro.navigateTo({
    url: `/pages/delivery-conversation/index?orderId=${encodeURIComponent(orderId)}&role=merchant`
  })
}

function handleShortcutClick(key: string, label: string) {
  if (key === 'publish') {
    void Taro.navigateTo({ url: '/pages/merchant-publish/index' })
    return
  }
  if (key === 'recharge' || key === 'bill') {
    void Taro.navigateTo({ url: '/pages/merchant-wallet/index' })
    return
  }
  if (key === 'message') {
    void Taro.navigateTo({ url: '/pages/merchant-messages/index' })
    return
  }
  showPlaceholder(`${label}静态占位`)
}

function openOrderDetail(orderId: string) {
  void Taro.navigateTo({
    url: `/pages/merchant-order-detail/index?orderId=${encodeURIComponent(orderId)}&role=merchant`
  })
}

function OrderCard({ order, mode }: { order: MerchantOrder; mode: MerchantTabKey }) {
  const timeText = order.deliveredTime
    ? `#${order.id} · ${order.orderTime} · ${order.deliveredTime}`
    : `#${order.id} · ${order.orderTime}`

  return (
    <View className='merchant-order-card'>
      <View className='merchant-order-card__top'>
        <Text className='merchant-order-card__serial'>{timeText}</Text>
        <Text className={`merchant-order-card__status merchant-order-card__status--${order.statusKind}`}>
          {order.statusText}
        </Text>
      </View>

      {order.rider && (
        <View className='merchant-order-rider'>
          <View className='merchant-order-rider__avatar'>
            <Text>{order.rider.avatar}</Text>
          </View>
          <View className='merchant-order-rider__main'>
            <Text className='merchant-order-rider__name'>{order.rider.name} · 骑手</Text>
            <Text className='merchant-order-rider__phone'>{order.rider.phone}</Text>
          </View>
          <View className='merchant-order-rider__actions'>
            <View className='merchant-order-rider__circle' onClick={() => openMerchantConversation(order.id)}>
              <Text>□</Text>
            </View>
            <View className='merchant-order-rider__circle' onClick={() => showPlaceholder('电话静态占位')}>
              <Text>⌕</Text>
            </View>
          </View>
        </View>
      )}

      <View className='merchant-order-route'>
        <View className='merchant-order-route__rail'>
          <View className='merchant-order-route__dot merchant-order-route__dot--pickup' />
          <View className='merchant-order-route__line' />
          <View className='merchant-order-route__dot merchant-order-route__dot--dropoff' />
        </View>
        <View className='merchant-order-route__body'>
          <Text className='merchant-order-route__label'>取货点 · 本店</Text>
          <Text className='merchant-order-route__place'>{order.pickupName}</Text>
          <Text className='merchant-order-route__label merchant-order-route__label--dropoff'>送达点</Text>
          <Text className='merchant-order-route__place'>{order.destination}</Text>
          <Text className='merchant-order-route__contact'>
            {order.customerName} · {order.customerPhone}
          </Text>
        </View>
      </View>

      {order.remark && (
        <View className='merchant-order-remark'>
          <Text className='merchant-order-remark__label'>备注</Text>
          <Text>{order.remark}</Text>
        </View>
      )}

      <View className='merchant-order-card__footer'>
        <View className='merchant-order-fee'>
          <Text className='merchant-order-fee__label'>配送费</Text>
          <Text className='merchant-order-fee__amount'>¥{order.fee}</Text>
        </View>
        <View className='merchant-order-card__buttons'>
          {mode === 'pending' && (
            <View className='merchant-order-button merchant-order-button--ghost' onClick={() => showPlaceholder('取消并退款静态占位')}>
              <Text>取消并退款</Text>
            </View>
          )}
          <View className='merchant-order-button merchant-order-button--primary' onClick={() => openOrderDetail(order.id)}>
            <Text>详情</Text>
          </View>
        </View>
      </View>
    </View>
  )
}

function MerchantHomeContent() {
  const [activeTab, setActiveTab] = useState<MerchantTabKey>('pending')
  const [showStudentSwitch, setShowStudentSwitch] = useState(false)
  const [switchingIdentity, setSwitchingIdentity] = useState(false)
  const [switchIdentityError, setSwitchIdentityError] = useState('')
  const switchIdentity = useSessionStore((state) => state.switchIdentity)
  const identityAllowed = useIdentityGuard('merchant')
  const currentOrders = useMemo(() => orderGroups[activeTab], [activeTab])

  const confirmStudentSwitch = async () => {
    if (switchingIdentity) return
    setSwitchingIdentity(true)
    setSwitchIdentityError('')
    try {
      await switchIdentity('student')
      await Taro.reLaunch({ url: identityRoute('student') })
      setShowStudentSwitch(false)
    } catch (error) {
      setSwitchIdentityError(error instanceof Error ? error.message : '身份切换失败，请重试')
    } finally {
      setSwitchingIdentity(false)
    }
  }

  if (!identityAllowed) return <View className='merchant-home-page' />

  return (
    <View className='merchant-home-page' style={getCapsuleSafeStyle()}>
      <View className='merchant-home-hero'>
        <View className='merchant-home-hero__orb merchant-home-hero__orb--left' />
        <View className='merchant-home-hero__orb merchant-home-hero__orb--right' />
        <View className='merchant-home-header'>
          <View className='merchant-home-shop'>
            <View className='merchant-home-shop__avatar'>
              <Text>茶</Text>
            </View>
            <View className='merchant-home-shop__main'>
              <View className='merchant-home-shop__name-row'>
                <Text className='merchant-home-shop__name'>茶知味·大学城店</Text>
                <Text className='merchant-home-shop__badge'>商家</Text>
              </View>
              <View className='merchant-home-shop__status'>
                <View className='merchant-home-shop__green-dot' />
                <Text>营业中 · 大学城二食堂</Text>
              </View>
            </View>
          </View>
          <View className='merchant-home-switch' onClick={() => {
            setSwitchIdentityError('')
            setShowStudentSwitch(true)
          }}>
            <Text>⇄ 切换身份</Text>
          </View>
        </View>

        <View className='merchant-home-metrics'>
          <View className='merchant-home-wallet'>
            <Text className='merchant-home-wallet__label'>钱包余额（元）</Text>
            <Text className='merchant-home-wallet__amount'>¥86.50</Text>
          </View>
          <View className='merchant-home-counts'>
            <View className='merchant-home-counts__item'>
              <Text className='merchant-home-counts__value'>12</Text>
              <Text className='merchant-home-counts__label'>今日单量</Text>
            </View>
            <View className='merchant-home-counts__item'>
              <Text className='merchant-home-counts__value'>2</Text>
              <Text className='merchant-home-counts__label'>配送中</Text>
            </View>
          </View>
        </View>
      </View>

      <View className='merchant-home-shortcuts'>
        {shortcuts.map((item, index) => (
          <View key={item.key} className='merchant-home-shortcut' onClick={() => handleShortcutClick(item.key, item.label)}>
            <View className={`merchant-home-shortcut__icon ${index === 0 ? 'merchant-home-shortcut__icon--primary' : ''}`}>
              <Text>{item.icon}</Text>
            </View>
            <Text className='merchant-home-shortcut__label'>{item.label}</Text>
          </View>
        ))}
      </View>

      <View className='merchant-home-tabs'>
        {tabs.map((tab) => (
          <View
            key={tab.key}
            className={`merchant-home-tab ${activeTab === tab.key ? 'merchant-home-tab--active' : ''}`}
            onClick={() => setActiveTab(tab.key)}
          >
            <Text className='merchant-home-tab__label'>{tab.label}</Text>
            {tab.count ? <Text className={`merchant-home-tab__count merchant-home-tab__count--${tab.key}`}>{tab.count}</Text> : null}
          </View>
        ))}
      </View>

      <ScrollView scrollY className='merchant-home-orders' enhanced showScrollbar={false}>
        <View className='merchant-home-orders__inner'>
          {currentOrders.map((order) => (
            <OrderCard key={order.id} order={order} mode={activeTab} />
          ))}
          <Text className='merchant-home-orders__end'>— 到底了 —</Text>
        </View>
      </ScrollView>

      <IdentitySwitchModal
        visible={showStudentSwitch}
        currentRole='merchant'
        targetRole='student'
        onCancel={() => setShowStudentSwitch(false)}
        onConfirm={() => void confirmStudentSwitch()}
        confirming={switchingIdentity}
        error={switchIdentityError}
      />
    </View>
  )
}

export default function MerchantHomePage() {
  return (
    <RuntimeSessionGate>
      <MerchantHomeContent />
    </RuntimeSessionGate>
  )
}
