import React, { useCallback, useRef, useState } from 'react'
import { ScrollView, Text, View } from '@tarojs/components'
import { useDidShow } from '@tarojs/taro'
import { CapsuleSystemRow } from '../../components/capsule-page-header'
import IdentitySwitchModal from '../../components/IdentitySwitchModal'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import { deliveryPaymentEnabled } from '../../feature-flags'
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
import MerchantHomeIcon, { type MerchantHomeIconName } from './merchant-home-icon'
import './index.scss'
import { useFeatureVisibility } from '../../hooks/use-feature-visibility'
import { useViewportRefresh } from '../../hooks/use-viewport-refresh'

type MerchantTabKey = 'pending' | 'delivering' | 'completed'
type MerchantOrdersByTab = Record<MerchantTabKey, MerchantDeliveryOrder[]>
type MerchantNumbersByTab = Record<MerchantTabKey, number>
type MerchantLoadedByTab = Record<MerchantTabKey, boolean>
type MerchantErrorsByTab = Record<MerchantTabKey, string>
type Settled<T> = { ok: true; value: T } | { ok: false; error: unknown }
type ApprovalState = 'checking' | 'approved' | 'blocked'

function emptyOrdersByTab(): MerchantOrdersByTab {
  return { pending: [], delivering: [], completed: [] }
}

function initialPagesByTab(): MerchantNumbersByTab {
  // 分页游标从第 1 页开始；是否已经拿到真实数量由 loadedByTab 记录。
  return { pending: 1, delivering: 1, completed: 1 }
}

function emptyTotalsByTab(): MerchantNumbersByTab {
  return { pending: 0, delivering: 0, completed: 0 }
}

function unloadedTabs(): MerchantLoadedByTab {
  return { pending: false, delivering: false, completed: false }
}

function emptyErrorsByTab(): MerchantErrorsByTab {
  return { pending: '', delivering: '', completed: '' }
}

function orderLoadError(result: Settled<unknown>): string {
  if (result.ok) return ''
  return result.error instanceof Error ? result.error.message : '配送单加载失败'
}

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
        <Text className={`merchant-order-card__status merchant-order-card__status--${tone}`}>
          {statusCopy[order.status]}
        </Text>
        <Text className='merchant-order-card__time'>{formatTime(order.createdAt)}</Text>
      </View>
      <Text className='merchant-order-card__serial'>配送单 #{order.deliveryOrderPublicId}</Text>

      <View className='merchant-order-route'>
        <View className='merchant-order-route__row'>
          <Text className='merchant-order-route__marker merchant-order-route__marker--pickup'>取</Text>
          <View className='merchant-order-route__body'>
            <Text className='merchant-order-route__place'>{order.pickupAddress}</Text>
            <Text className='merchant-order-route__contact'>
              {[order.pickupContactName, order.pickupContactPhone].filter(Boolean).join(' · ')}
            </Text>
          </View>
        </View>
        <View className='merchant-order-route__row'>
          <Text className='merchant-order-route__marker merchant-order-route__marker--dropoff'>送</Text>
          <View className='merchant-order-route__body'>
            <Text className='merchant-order-route__place'>{order.deliveryAddress}</Text>
            <Text className='merchant-order-route__contact'>
              {[order.recipientName, order.recipientPhone].filter(Boolean).join(' · ')}
            </Text>
          </View>
        </View>
      </View>

      {order.remark ? (
        <View className='merchant-order-remark'>
          <Text className='merchant-order-remark__label'>备注</Text>
          <Text>{order.remark}</Text>
        </View>
      ) : null}

      {order.riderUserId ? (
        <View className='merchant-order-rider'>
          <Text className='merchant-order-rider__name'>骑手 #{order.riderUserId}</Text>
          <View className='merchant-order-rider__circle' ariaRole='button' ariaLabel='联系本单骑手' hoverClass='merchant-home-control--pressed' onClick={() => openConversation(order)}>
            <MerchantHomeIcon name='message' className='merchant-order-rider__action-icon' />
            <Text>联系骑手</Text>
          </View>
        </View>
      ) : null}

      <View className='merchant-order-card__footer'>
        <View className='merchant-order-fee'>
          <Text className='merchant-order-fee__label'>配送费</Text>
          <Text className='merchant-order-fee__amount'>¥{centsToYuan(order.deliveryFeeCents)}</Text>
        </View>
        <View className='merchant-order-card__buttons'>
          <View className='merchant-order-button merchant-order-button--primary' ariaRole='button' ariaLabel='查看配送单详情' hoverClass='merchant-home-control--pressed' onClick={() => openOrderDetail(order)}>
            <Text>查看详情</Text>
          </View>
        </View>
      </View>
    </View>
  )
}

function MerchantHomeContent() {
  const capsuleStyle = useViewportRefresh()
  const visibility = useFeatureVisibility()
  const [activeTab, setActiveTab] = useState<MerchantTabKey>('pending')
  const [ordersByTab, setOrdersByTab] = useState<MerchantOrdersByTab>(emptyOrdersByTab)
  const [ordersPageByTab, setOrdersPageByTab] = useState<MerchantNumbersByTab>(initialPagesByTab)
  const [ordersTotalByTab, setOrdersTotalByTab] = useState<MerchantNumbersByTab>(emptyTotalsByTab)
  const [loadedByTab, setLoadedByTab] = useState<MerchantLoadedByTab>(unloadedTabs)
  const [loadErrorsByTab, setLoadErrorsByTab] = useState<MerchantErrorsByTab>(emptyErrorsByTab)
  const [loadingMore, setLoadingMore] = useState(false)
  const [balanceCents, setBalanceCents] = useState<number | null>(null)
  const [merchantName, setMerchantName] = useState('')
  const [merchantAddress, setMerchantAddress] = useState('')
  const [approvalState, setApprovalState] = useState<ApprovalState>('checking')
  const [approvalMessage, setApprovalMessage] = useState('正在核验商家审核状态…')
  const [loading, setLoading] = useState(false)
  const [showStudentSwitch, setShowStudentSwitch] = useState(false)
  const [switchingIdentity, setSwitchingIdentity] = useState(false)
  const [switchIdentityError, setSwitchIdentityError] = useState('')
  const profile = useSessionStore((state) => state.profile)
  const switchIdentity = useSessionStore((state) => state.switchIdentity)
  const identityAllowed = useIdentityGuard('merchant')
  const requestVersion = useRef(0)

  const loadWorkbench = useCallback(async () => {
    if (!identityAllowed) return
    const version = requestVersion.current + 1
    requestVersion.current = version
    setLoading(true)
    setLoadingMore(false)
    setLoadErrorsByTab(emptyErrorsByTab())
    setApprovalState('checking')
    setApprovalMessage('正在核验商家审核状态…')
    const applicationResult = await settle(getIdentityApplication('merchant'))
    if (requestVersion.current !== version) return
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
    const [pendingResult, deliveringResult, completedResult, walletResult] = await Promise.all([
      settle(listMerchantDeliveryOrders(1, 50, 'pending')),
      settle(listMerchantDeliveryOrders(1, 50, 'delivering')),
      settle(listMerchantDeliveryOrders(1, 50, 'completed')),
      deliveryPaymentEnabled ? settle(getDeliveryWallet()) : null,
    ])
    if (requestVersion.current !== version) return
    setLoadedByTab((current) => ({
      pending: pendingResult.ok || current.pending,
      delivering: deliveringResult.ok || current.delivering,
      completed: completedResult.ok || current.completed,
    }))
    setOrdersByTab((current) => ({
      pending: pendingResult.ok ? pendingResult.value.items : current.pending,
      delivering: deliveringResult.ok ? deliveringResult.value.items : current.delivering,
      completed: completedResult.ok ? completedResult.value.items : current.completed,
    }))
    setOrdersPageByTab((current) => ({
      pending: pendingResult.ok ? pendingResult.value.page || 1 : current.pending,
      delivering: deliveringResult.ok ? deliveringResult.value.page || 1 : current.delivering,
      completed: completedResult.ok ? completedResult.value.page || 1 : current.completed,
    }))
    setOrdersTotalByTab((current) => ({
      pending: pendingResult.ok ? pendingResult.value.total : current.pending,
      delivering: deliveringResult.ok ? deliveringResult.value.total : current.delivering,
      completed: completedResult.ok ? completedResult.value.total : current.completed,
    }))
    setLoadErrorsByTab({
      pending: orderLoadError(pendingResult),
      delivering: orderLoadError(deliveringResult),
      completed: orderLoadError(completedResult),
    })
    setBalanceCents(walletResult?.ok ? walletResult.value.availableBalanceCents : null)
    setLoading(false)
  }, [identityAllowed])

  useDidShow(() => {
    if (identityAllowed) void loadWorkbench()
  })

  const loadMoreOrders = useCallback(async () => {
    const requestedTab = activeTab
    const currentOrders = ordersByTab[requestedTab]
    const currentTotal = ordersTotalByTab[requestedTab]
    if (!identityAllowed || loading || loadingMore || currentOrders.length >= currentTotal) return
    const version = requestVersion.current
    const nextPage = ordersPageByTab[requestedTab] + 1
    setLoadingMore(true)
    setLoadErrorsByTab((current) => ({ ...current, [requestedTab]: '' }))
    try {
      const result = await listMerchantDeliveryOrders(nextPage, 50, requestedTab)
      if (requestVersion.current !== version) return
      setOrdersByTab((current) => {
        const byId = new Map(current[requestedTab].map((order) => [order.deliveryOrderPublicId, order]))
        result.items.forEach((order) => byId.set(order.deliveryOrderPublicId, order))
        return { ...current, [requestedTab]: Array.from(byId.values()) }
      })
      setOrdersPageByTab((current) => ({ ...current, [requestedTab]: result.page || nextPage }))
      setOrdersTotalByTab((current) => ({ ...current, [requestedTab]: result.total }))
    } catch (error) {
      if (requestVersion.current === version) {
        setLoadErrorsByTab((current) => ({
          ...current,
          [requestedTab]: error instanceof Error ? error.message : '更多配送单加载失败',
        }))
      }
    } finally {
      if (requestVersion.current === version) setLoadingMore(false)
    }
  }, [activeTab, identityAllowed, loading, loadingMore, ordersByTab, ordersPageByTab, ordersTotalByTab])

  const totalOrderCount = ordersTotalByTab.pending + ordersTotalByTab.delivering + ordersTotalByTab.completed
  const loadError = loadErrorsByTab[activeTab]
  const allCountsLoaded = tabCopy.every((tab) => loadedByTab[tab.key])
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
      <View className='merchant-home-page merchant-home-page--review' style={capsuleStyle}>
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
    <View className='merchant-home-page' style={capsuleStyle}>
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
          <View className='merchant-home-switch' ariaRole='button' ariaLabel='切换身份' hoverClass='merchant-home-control--pressed' onClick={() => {
            setSwitchIdentityError('')
            setShowStudentSwitch(true)
          }}>
            <Text>⇄ 切换身份</Text>
          </View>
        </View>

        <View className='merchant-home-metrics'>
          <View className='merchant-home-metric merchant-home-metric--wallet'>
            <Text className='merchant-home-metric__label'>钱包余额</Text>
            <Text className='merchant-home-metric__value merchant-home-metric__value--wallet'>
              {!deliveryPaymentEnabled ? '未开通' : balanceCents == null ? '--' : `¥${centsToYuan(balanceCents)}`}
            </Text>
          </View>
          <View className='merchant-home-metric'>
            <Text className='merchant-home-metric__label'>累计单量</Text>
            <Text className={`merchant-home-metric__value${!allCountsLoaded ? ' merchant-home-metric__value--pending' : ''}`}>
              {allCountsLoaded ? totalOrderCount : '—'}
            </Text>
          </View>
          <View className='merchant-home-metric'>
            <Text className='merchant-home-metric__label'>配送中</Text>
            <Text className={`merchant-home-metric__value${!loadedByTab.delivering ? ' merchant-home-metric__value--pending' : ''}`}>
              {loadedByTab.delivering ? ordersTotalByTab.delivering : '—'}
            </Text>
          </View>
        </View>
      </View>

      <View className='merchant-home-tools'>
        {(!visibility.delivery || !visibility.merchant) && <View className='merchant-home-section-head'><Text>新业务暂未开放，以下仅供处理已有订单与资金。</Text></View>}
        <View className={`merchant-home-shortcuts${visibility.delivery && visibility.merchant ? '' : ' merchant-home-shortcuts--existing-only'}`}>
          {shortcutCopy.filter(item => item.key === 'publish' && visibility.delivery && visibility.merchant).map((item) => (
            <View
              key={item.key}
              className={`merchant-home-shortcut merchant-home-shortcut--${item.key}`}
              ariaRole='button'
              ariaLabel={`${item.label}，${item.description}`}
              hoverClass='merchant-home-control--pressed'
              onClick={() => void Taro.navigateTo({ url: item.url })}
            >
              <View className='merchant-home-shortcut__icon'>
                <MerchantHomeIcon name={item.icon} />
              </View>
              <View className='merchant-home-shortcut__copy'>
                <Text className='merchant-home-shortcut__label'>{item.label}</Text>
              </View>
            </View>
          ))}
          <View className='merchant-home-shortcuts__secondary'>
            {shortcutCopy.filter(item => item.key !== 'publish').map((item) => (
              <View
                key={item.key}
                className={`merchant-home-shortcut merchant-home-shortcut--${item.key}`}
                ariaRole='button'
                ariaLabel={`${item.label}，${item.description}`}
                hoverClass='merchant-home-control--pressed'
                onClick={() => void Taro.navigateTo({ url: item.url })}
              >
                <MerchantHomeIcon name={item.icon} className='merchant-home-shortcut__aux-icon' />
                <Text className='merchant-home-shortcut__label'>{item.label}</Text>
              </View>
            ))}
          </View>
        </View>
      </View>

      <View className='merchant-home-tabs'>
        {tabCopy.map((tab) => (
          <View
            key={tab.key}
            className={`merchant-home-tab ${activeTab === tab.key ? 'merchant-home-tab--active' : ''}`}
            ariaRole='button'
            ariaLabel={`${tab.label}${activeTab === tab.key ? '，已选中' : ''}`}
            hoverClass='merchant-home-control--pressed'
            onClick={() => setActiveTab(tab.key)}
          >
            <Text className='merchant-home-tab__label'>{tab.label}</Text>
            <Text className={`merchant-home-tab__count merchant-home-tab__count--${tab.key}${!loadedByTab[tab.key] ? ' merchant-home-tab__count--pending' : ''}`}>
              {loadedByTab[tab.key] ? ordersTotalByTab[tab.key] : '—'}
            </Text>
          </View>
        ))}
      </View>

      <ScrollView
        scrollY
        className='merchant-home-orders'
        enhanced
        showScrollbar={false}
        lowerThreshold={120}
        onScrollToLower={() => void loadMoreOrders()}
      >
        <View className='merchant-home-orders__inner'>
          {loading ? <Text className='merchant-home-orders__notice merchant-home-orders__notice--loading'>正在加载真实配送单…</Text> : null}
          {!loading && loadError ? (
            <View className='merchant-home-orders__notice merchant-home-orders__notice--error' hoverClass='merchant-home-orders__notice--pressed' ariaRole='button' ariaLabel='重试配送单加载' onClick={() => void loadWorkbench()}>
              <Text>配送单{loadedByTab[activeTab] ? '刷新' : '加载'}失败</Text>
              <Text>{loadError}</Text>
              <Text className='merchant-home-orders__retry'>点击重试</Text>
            </View>
          ) : null}
          {!loading && !loadError && ordersByTab[activeTab].length === 0 ? (
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
          {ordersByTab[activeTab].map((order) => <OrderCard key={order.deliveryOrderPublicId} order={order} />)}
          {loadingMore ? <Text className='merchant-home-orders__end'>正在加载更多配送单…</Text> : null}
          {!loading && !loadingMore && !loadError && ordersByTab[activeTab].length > 0 && ordersByTab[activeTab].length >= ordersTotalByTab[activeTab] ? (
            <Text className='merchant-home-orders__end'>— 已显示全部配送单 —</Text>
          ) : null}
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
