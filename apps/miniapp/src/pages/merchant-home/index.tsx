import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { ScrollView, Text, View } from '@tarojs/components'
import IdentitySwitchModal from '../../components/IdentitySwitchModal'
import { CapsuleSystemRow } from '../../components/capsule-page-header'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import { identityRoute, useIdentityGuard } from '../../hooks/use-identity-guard'
import { getIdentityApplication } from '../../services/a-user'
import {
  centsToYuan,
  listMerchantDeliveryOrders,
  type DeliveryStatus,
  type MerchantDeliveryOrder,
} from '../../services/delivery-merchant'
import { getDeliveryWallet } from '../../services/delivery-payment'
import Taro from '../../services/taro'
import { useSessionStore } from '../../stores/session'
import { getCapsuleSafeStyle } from '../../utils/capsule-safe'
import MerchantHomeIcon, { type MerchantHomeIconName } from './merchant-home-icon'
import './index.scss'

type MerchantTabKey = 'pending' | 'delivering' | 'completed'
type Settled<T> = { ok: true; value: T } | { ok: false; error: unknown }
type ApprovalState = 'checking' | 'approved' | 'blocked'

async function settle<T>(promise: Promise<T>): Promise<Settled<T>> {
  try {
    return { ok: true, value: await promise }
  } catch (error) {
    return { ok: false, error }
  }
}

const tabCopy: Array<{ key: MerchantTabKey; label: string }> = [
  { key: 'pending', label: '待接单' },
  { key: 'delivering', label: '配送中' },
  { key: 'completed', label: '已完成' },
]

const shortcutCopy: Array<{
  key: string
  label: string
  description: string
  icon: MerchantHomeIconName
  url: string
}> = [
  { key: 'publish', label: '发布配送单', description: '创建新的校内配送', icon: 'publish', url: '/pages/merchant-publish/index' },
  { key: 'wallet', label: '钱包', description: '查看余额与充值', icon: 'wallet', url: '/pages/merchant-wallet/index' },
  { key: 'bill', label: '账单', description: '核对资金流水', icon: 'bill', url: '/pages/merchant-wallet/index' },
  { key: 'message', label: '消息', description: '联系正在配送的骑手', icon: 'message', url: '/pages/merchant-messages/index' },
]

const statusCopy: Record<DeliveryStatus, string> = {
  pending_fee_payment: '待支付配送费',
  payment_failed: '支付失败',
  ready_for_claim: '待骑手接单',
  claimed: '骑手已接单',
  to_pickup: '骑手前往取货',
  picked_up: '骑手已取货',
  delivering: '配送中',
  delivered: '已完成',
  cancelled: '已取消',
}

function orderTab(status: DeliveryStatus): MerchantTabKey {
  if (status === 'delivered' || status === 'cancelled') return 'completed'
  if (status === 'claimed' || status === 'to_pickup' || status === 'picked_up' || status === 'delivering') {
    return 'delivering'
  }
  return 'pending'
}

function orderTone(status: DeliveryStatus): 'waiting' | 'pickup' | 'delivering' | 'completed' {
  if (status === 'delivered' || status === 'cancelled') return 'completed'
  if (status === 'picked_up' || status === 'delivering') return 'delivering'
  if (status === 'claimed' || status === 'to_pickup') return 'pickup'
  return 'waiting'
}

function formatTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value.replace('T', ' ').slice(0, 16)
  return `${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
}

function createdToday(value: string): boolean {
  const date = new Date(value)
  const now = new Date()
  return !Number.isNaN(date.getTime())
    && date.getFullYear() === now.getFullYear()
    && date.getMonth() === now.getMonth()
    && date.getDate() === now.getDate()
}

function openConversation(order: MerchantDeliveryOrder) {
  if (!order.riderUserId) {
    void Taro.showToast({ title: '骑手接单后才能进入私聊', icon: 'none' })
    return
  }
  void Taro.navigateTo({
    url: `/pages/delivery-conversation/index?orderId=${encodeURIComponent(order.deliveryOrderPublicId)}&role=merchant`,
  })
}

function openOrderDetail(order: MerchantDeliveryOrder) {
  void Taro.navigateTo({
    url: `/pages/merchant-order-detail/index?orderId=${encodeURIComponent(order.deliveryOrderPublicId)}`,
  })
}

function OrderCard({ order }: { order: MerchantDeliveryOrder }) {
  const tone = orderTone(order.status)
  return (
    <View className='merchant-order-card'>
      <View className='merchant-order-card__top'>
        <Text className='merchant-order-card__serial'>
          #{order.deliveryOrderPublicId} · {formatTime(order.createdAt)}
        </Text>
        <Text className={`merchant-order-card__status merchant-order-card__status--${tone}`}>
          {statusCopy[order.status]}
        </Text>
      </View>

      {order.riderUserId ? (
        <View className='merchant-order-rider'>
          <View className='merchant-order-rider__avatar'><Text>骑</Text></View>
          <View className='merchant-order-rider__main'>
            <Text className='merchant-order-rider__name'>骑手 #{order.riderUserId}</Text>
            <Text className='merchant-order-rider__phone'>已参与本配送单</Text>
          </View>
          <View className='merchant-order-rider__actions'>
            <View className='merchant-order-rider__circle' onClick={() => openConversation(order)}>
              <Text>聊</Text>
            </View>
          </View>
        </View>
      ) : null}

      <View className='merchant-order-route'>
        <View className='merchant-order-route__rail'>
          <View className='merchant-order-route__dot merchant-order-route__dot--pickup' />
          <View className='merchant-order-route__line' />
          <View className='merchant-order-route__dot merchant-order-route__dot--dropoff' />
        </View>
        <View className='merchant-order-route__body'>
          <Text className='merchant-order-route__label'>取货点</Text>
          <Text className='merchant-order-route__place'>{order.pickupAddress}</Text>
          <Text className='merchant-order-route__contact'>
            {order.pickupContactName} · {order.pickupContactPhone}
          </Text>
          <Text className='merchant-order-route__label merchant-order-route__label--dropoff'>送达点</Text>
          <Text className='merchant-order-route__place'>{order.deliveryAddress}</Text>
          <Text className='merchant-order-route__contact'>
            {order.recipientName} · {order.recipientPhone}
          </Text>
        </View>
      </View>

      {order.remark ? (
        <View className='merchant-order-remark'>
          <Text className='merchant-order-remark__label'>备注</Text>
          <Text>{order.remark}</Text>
        </View>
      ) : null}

      <View className='merchant-order-card__footer'>
        <View className='merchant-order-fee'>
          <Text className='merchant-order-fee__label'>配送费</Text>
          <Text className='merchant-order-fee__amount'>¥{centsToYuan(order.deliveryFeeCents)}</Text>
        </View>
        <View className='merchant-order-card__buttons'>
          <View className='merchant-order-button merchant-order-button--primary' onClick={() => openOrderDetail(order)}>
            <Text>详情</Text>
          </View>
        </View>
      </View>
    </View>
  )
}

function MerchantHomeContent() {
  const [activeTab, setActiveTab] = useState<MerchantTabKey>('pending')
  const [orders, setOrders] = useState<MerchantDeliveryOrder[]>([])
  const [balanceCents, setBalanceCents] = useState<number | null>(null)
  const [merchantName, setMerchantName] = useState('')
  const [merchantAddress, setMerchantAddress] = useState('')
  const [approvalState, setApprovalState] = useState<ApprovalState>('checking')
  const [approvalMessage, setApprovalMessage] = useState('正在核验商家审核状态…')
  const [loading, setLoading] = useState(false)
  const [loadError, setLoadError] = useState('')
  const [showStudentSwitch, setShowStudentSwitch] = useState(false)
  const [switchingIdentity, setSwitchingIdentity] = useState(false)
  const [switchIdentityError, setSwitchIdentityError] = useState('')
  const profile = useSessionStore((state) => state.profile)
  const switchIdentity = useSessionStore((state) => state.switchIdentity)
  const identityAllowed = useIdentityGuard('merchant')

  const loadWorkbench = useCallback(async () => {
    if (!identityAllowed || loading) return
    setLoading(true)
    setLoadError('')
    setApprovalState('checking')
    setApprovalMessage('正在核验商家审核状态…')
    const applicationResult = await settle(getIdentityApplication('merchant'))
    if (!applicationResult.ok) {
      setApprovalState('blocked')
      setApprovalMessage('暂时无法确认商家审核状态，请返回“我的”重新进入')
      setLoading(false)
      return
    }
    if (applicationResult.value.status !== 'approved') {
      setApprovalState('blocked')
      setApprovalMessage(
        applicationResult.value.status === 'pending'
          ? '商家申请正在审核中，审核通过后才能进入'
          : '商家申请未通过，请修改资料后重新提交'
      )
      setLoading(false)
      return
    }
    setMerchantName(applicationResult.value.merchantName)
    setMerchantAddress(applicationResult.value.merchantAddress)
    setApprovalState('approved')
    const [ordersResult, walletResult] = await Promise.all([
      settle(listMerchantDeliveryOrders(1, 100)),
      settle(getDeliveryWallet()),
    ])
    if (ordersResult.ok) {
      setOrders(ordersResult.value.items)
    } else {
      setOrders([])
      setLoadError(ordersResult.error instanceof Error ? ordersResult.error.message : '配送单加载失败')
    }
    setBalanceCents(walletResult.ok ? walletResult.value.availableBalanceCents : null)
    setLoading(false)
  }, [identityAllowed, loading])

  useEffect(() => {
    if (identityAllowed) void loadWorkbench()
  }, [identityAllowed]) // eslint-disable-line react-hooks/exhaustive-deps

  const groupedOrders = useMemo(() => ({
    pending: orders.filter((item) => orderTab(item.status) === 'pending'),
    delivering: orders.filter((item) => orderTab(item.status) === 'delivering'),
    completed: orders.filter((item) => orderTab(item.status) === 'completed'),
  }), [orders])
  const todayCount = useMemo(() => orders.filter((item) => createdToday(item.createdAt)).length, [orders])
  const shopName = merchantName || profile.nickname || '已认证商家'
  const shopAddress = merchantAddress || profile.campus || profile.school || '商家资料已通过审核'

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
  if (approvalState !== 'approved') {
    return (
      <View className='merchant-home-page merchant-home-page--review' style={getCapsuleSafeStyle()}>
        <CapsuleSystemRow />
        <View className='merchant-home-review'>
          <Text className='merchant-home-review__title'>
            {approvalState === 'checking' ? '正在核验商家资格' : '暂时无法进入商家端'}
          </Text>
          <Text className='merchant-home-review__message'>{approvalMessage}</Text>
          {approvalState === 'blocked' ? (
            <View className='merchant-home-review__action' onClick={() => void Taro.reLaunch({ url: '/pages/profile/index' })}>
              <Text>返回我的</Text>
            </View>
          ) : null}
        </View>
      </View>
    )
  }

  return (
    <View className='merchant-home-page' style={getCapsuleSafeStyle()}>
      <View className='merchant-home-hero'>
        <CapsuleSystemRow />
        <View className='merchant-home-header'>
          <View className='merchant-home-shop'>
            <View className='merchant-home-shop__avatar'><Text>{shopName.slice(0, 1)}</Text></View>
            <View className='merchant-home-shop__main'>
              <View className='merchant-home-shop__name-row'>
                <Text className='merchant-home-shop__name'>{shopName}</Text>
                <Text className='merchant-home-shop__badge'>已审核</Text>
              </View>
              <View className='merchant-home-shop__status'>
                <View className='merchant-home-shop__green-dot' />
                <Text>{shopAddress}</Text>
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
            <Text className='merchant-home-wallet__amount'>
              {balanceCents == null ? '--' : `¥${centsToYuan(balanceCents)}`}
            </Text>
          </View>
          <View className='merchant-home-counts'>
            <View className='merchant-home-counts__item'>
              <Text className='merchant-home-counts__value'>{todayCount}</Text>
              <Text className='merchant-home-counts__label'>今日单量</Text>
            </View>
            <View className='merchant-home-counts__item'>
              <Text className='merchant-home-counts__value'>{groupedOrders.delivering.length}</Text>
              <Text className='merchant-home-counts__label'>配送中</Text>
            </View>
          </View>
        </View>
      </View>

      <View className='merchant-home-shortcuts'>
        {shortcutCopy.map((item) => (
          <View
            key={item.key}
            className={`merchant-home-shortcut merchant-home-shortcut--${item.key}`}
            onClick={() => void Taro.navigateTo({ url: item.url })}
          >
            <View className='merchant-home-shortcut__icon'>
              <MerchantHomeIcon name={item.icon} />
            </View>
            <View className='merchant-home-shortcut__copy'>
              <Text className='merchant-home-shortcut__label'>{item.label}</Text>
              <Text className='merchant-home-shortcut__description'>{item.description}</Text>
            </View>
          </View>
        ))}
      </View>

      <View className='merchant-home-tabs'>
        {tabCopy.map((tab) => (
          <View
            key={tab.key}
            className={`merchant-home-tab ${activeTab === tab.key ? 'merchant-home-tab--active' : ''}`}
            onClick={() => setActiveTab(tab.key)}
          >
            <Text className='merchant-home-tab__label'>{tab.label}</Text>
            <Text className={`merchant-home-tab__count merchant-home-tab__count--${tab.key}`}>
              {groupedOrders[tab.key].length}
            </Text>
          </View>
        ))}
      </View>

      <ScrollView scrollY className='merchant-home-orders' enhanced showScrollbar={false}>
        <View className='merchant-home-orders__inner'>
          {loading ? <Text className='merchant-home-orders__end'>正在加载真实配送单…</Text> : null}
          {!loading && loadError ? (
            <Text className='merchant-home-orders__end' onClick={() => void loadWorkbench()}>
              {loadError}，点击重试
            </Text>
          ) : null}
          {!loading && !loadError && groupedOrders[activeTab].length === 0 ? (
            <View className='merchant-home-empty'>
              <View className='merchant-home-empty__icon'>
                <MerchantHomeIcon name='empty' />
              </View>
              <Text className='merchant-home-empty__title'>
                暂无{tabCopy.find((item) => item.key === activeTab)?.label}配送单
              </Text>
              <Text className='merchant-home-empty__description'>
                新配送单和状态更新会显示在这里
              </Text>
            </View>
          ) : null}
          {groupedOrders[activeTab].map((order) => <OrderCard key={order.deliveryOrderPublicId} order={order} />)}
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
    <RuntimeSessionGate preserveAuthenticatedChildrenDuringRetry>
      <MerchantHomeContent />
    </RuntimeSessionGate>
  )
}
