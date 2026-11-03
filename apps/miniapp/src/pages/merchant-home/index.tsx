import React, { useCallback, useEffect, useRef, useState } from 'react'
import { Text, View } from '@tarojs/components'
import { useDidShow } from '@tarojs/taro'
import { useNotificationRefresh } from '../../hooks/use-notification-refresh'
import { CapsuleSystemRow } from '../../components/capsule-page-header'
import { useChannelInteraction } from '../../components/channel-pager'
import DeliveryStagePager from '../../components/delivery-stage-pager'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import DeliveryWorkspaceNav, { useDeliveryWorkspaceNavigation } from '../../components/delivery-workspace-nav'
import RefreshScrollView from '../../components/refresh-scroll-view'
import { DeliveryLoadDots, DeliveryOrderLoading, DeliveryOrderState } from '../../components/delivery-order-feedback'
import { useIdentityGuard } from '../../hooks/use-identity-guard'
import { getIdentityApplication } from '../../services/a-user'
import {
  centsToYuan,
  listMerchantDeliveryOrders,
  type DeliveryStatus,
  type MerchantDeliveryOrder,
} from '../../services/delivery-merchant'
import Taro from '../../services/taro'
import { useSessionStore } from '../../stores/session'
import { deliveryPickupLabel, deliverySourceLabel } from '../../utils/delivery-order-source'
import MerchantHomeIcon from './merchant-home-icon'
import './index.scss'
import { useFeatureVisibility } from '../../hooks/use-feature-visibility'
import { useViewportRefresh } from '../../hooks/use-viewport-refresh'

type MerchantTabKey = 'pending' | 'delivering' | 'completed'
type MerchantOrdersByTab = Record<MerchantTabKey, MerchantDeliveryOrder[]>
type MerchantNumbersByTab = Record<MerchantTabKey, number>
type MerchantLoadedByTab = Record<MerchantTabKey, boolean>
type MerchantErrorsByTab = Record<MerchantTabKey, string>
type Settled<T> = { ok: true; value: T } | { ok: false; error: unknown }
type ApprovalState = 'checking' | 'approved' | 'blocked' | 'error'

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
  { key: 'pending', label: '待处理' },
  { key: 'delivering', label: '配送中' },
  { key: 'completed', label: '已结束' },
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

function orderTone(status: DeliveryStatus): 'waiting' | 'pickup' | 'delivering' | 'completed' | 'danger' {
  if (status === 'payment_failed') return 'danger'
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

function OrderCard({ order, canActivate }: { order: MerchantDeliveryOrder; canActivate: () => boolean }) {
  const tone = orderTone(order.status)
  return (
    <View className='merchant-order-card'>
      <View className='merchant-order-card__identity'>
        <Text className='merchant-order-card__source'>{deliverySourceLabel(order.sourcePlatform)}</Text>
        <Text className={`merchant-order-card__pickup-code${order.pickupCode?.trim() ? '' : ' merchant-order-card__pickup-code--fallback'}${(order.pickupCode?.trim().length ?? 0) > 8 ? ' merchant-order-card__pickup-code--long' : ''}`}>
          {order.pickupCode?.trim() ? <>{deliveryPickupLabel(order).slice(0, -1)}<Text className='merchant-order-card__pickup-unit'>号</Text></> : deliveryPickupLabel(order)}
        </Text>
      </View>
      <View className='merchant-order-card__top'>
        <Text className={`merchant-order-card__status merchant-order-card__status--${tone}`}>
          {statusCopy[order.status]}
        </Text>
      </View>

      <View className='merchant-order-route'>
        <View className='merchant-order-route__row'>
          <Text className='merchant-order-route__marker merchant-order-route__marker--pickup'>取</Text>
          <View className='merchant-order-route__body'>
            <Text className='merchant-order-route__place'>{order.pickupAddress}</Text>
          </View>
        </View>
        <View className='merchant-order-route__row'>
          <Text className='merchant-order-route__marker merchant-order-route__marker--dropoff'>送</Text>
          <View className='merchant-order-route__body'>
            <Text className='merchant-order-route__place'>{order.deliveryAddress}</Text>
          </View>
        </View>
      </View>

      {order.remark ? <View className='merchant-order-remark'>
        <Text className='merchant-order-remark__label'>备注</Text>
        <Text className='merchant-order-remark__text'>{order.remark}</Text>
      </View> : null}

      <View className='merchant-order-card__footer'>
        <Text className='merchant-order-card__time'>{formatTime(order.createdAt)} 下单</Text>
        <View className='merchant-order-fee'>
          <Text className='merchant-order-fee__label'>配送费</Text>
          <Text className='merchant-order-fee__amount'>¥{centsToYuan(order.deliveryFeeCents)}</Text>
        </View>
        {order.riderUserId ? (
          <View className='merchant-order-rider' ariaRole='button' ariaLabel='联系本单骑手' hoverClass='merchant-home-control--pressed' onClick={() => { if (canActivate()) openConversation(order) }}>
            <MerchantHomeIcon name='message' className='merchant-order-rider__action-icon' />
            <Text>联系骑手</Text>
          </View>
        ) : null}
        <View className='merchant-order-card__buttons'>
          <View className='merchant-order-button merchant-order-button--primary' ariaRole='button' ariaLabel={`查看${deliverySourceLabel(order.sourcePlatform)}${deliveryPickupLabel(order)}详情`} hoverClass='merchant-home-control--pressed' onClick={() => { if (canActivate()) openOrderDetail(order) }}>
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
  const [pageErrorsByTab, setPageErrorsByTab] = useState<MerchantErrorsByTab>(emptyErrorsByTab)
  const [loadingMore, setLoadingMore] = useState(false)
  const [approvalState, setApprovalState] = useState<ApprovalState>('checking')
  const [approvalMessage, setApprovalMessage] = useState('正在核验商家审核状态…')
  const [loading, setLoading] = useState(false)
  const channelInteraction = useChannelInteraction()
  const profile = useSessionStore((state) => state.profile)
  const identityAllowed = useIdentityGuard('merchant')
  const workspace = useDeliveryWorkspaceNavigation({ role: 'merchant', active: 'orders' }, identityAllowed && approvalState === 'approved')
  const canActivateWorkspace = () => workspace.swipe.canActivate() && channelInteraction.canActivate()
  const requestVersion = useRef(0)
  const activeLoad = useRef<number | null>(null)
  const activePageLoad = useRef<{ version: number; tab: MerchantTabKey } | null>(null)
  const identityRef = useRef(identityAllowed)
  const approvalRef = useRef(approvalState)
  const accountRef = useRef(profile.id)
  const dataAccountRef = useRef(profile.id)
  const mountedRef = useRef(true)
  identityRef.current = identityAllowed
  approvalRef.current = approvalState
  accountRef.current = profile.id

  useEffect(() => {
    mountedRef.current = true
    return () => { mountedRef.current = false }
  }, [])

  const loadWorkbench = useCallback(async (): Promise<boolean> => {
    if (!mountedRef.current || !identityAllowed || !identityRef.current || profile.id !== accountRef.current || activeLoad.current !== null) return false
    const account = accountRef.current
    const version = requestVersion.current + 1
    requestVersion.current = version
    activeLoad.current = version
    activePageLoad.current = null
    const current = () => mountedRef.current && identityRef.current && accountRef.current === account && requestVersion.current === version
    setLoading(true)
    setLoadingMore(false)
    setLoadErrorsByTab(emptyErrorsByTab())
    setPageErrorsByTab(emptyErrorsByTab())
    // A background check must not unmount the list and native refresh slot.
    // A failed check or revoked approval still leaves the guarded workbench.
    if (approvalRef.current !== 'approved') setApprovalState('checking')
    setApprovalMessage('正在核验商家审核状态…')
    const applicationResult = await settle(getIdentityApplication('merchant'))
    if (!current()) return false
    if (!applicationResult.ok) {
      approvalRef.current = 'error'
      setApprovalState('error')
      setApprovalMessage('暂时无法确认商家审核状态，请稍后重试')
      setLoading(false)
      activeLoad.current = null
      return false
    }
    if (applicationResult.value.status !== 'approved') {
      approvalRef.current = 'blocked'
      setApprovalState('blocked')
      setApprovalMessage(
        applicationResult.value.status === 'pending'
          ? '商家申请正在审核中，审核通过后才能进入'
          : '商家申请未通过，请修改资料后重新提交'
      )
      setLoading(false)
      activeLoad.current = null
      return false
    }
    approvalRef.current = 'approved'
    setApprovalState('approved')
    const [pendingResult, deliveringResult, completedResult] = await Promise.all([
      settle(listMerchantDeliveryOrders(1, 50, 'pending')),
      settle(listMerchantDeliveryOrders(1, 50, 'delivering')),
      settle(listMerchantDeliveryOrders(1, 50, 'completed')),
    ])
    if (!current()) return false
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
    setLoading(false)
    activeLoad.current = null
    return pendingResult.ok && deliveringResult.ok && completedResult.ok
  }, [identityAllowed, profile.id])

  useEffect(() => {
    requestVersion.current += 1
    activeLoad.current = null
    activePageLoad.current = null
    setLoading(false)
    setLoadingMore(false)
    // A transient authentication retry keeps this account's known data. A new
    // account must never inherit the previous merchant's orders or balance.
    if (profile.id && profile.id !== dataAccountRef.current) {
      dataAccountRef.current = profile.id
      setOrdersByTab(emptyOrdersByTab())
      setOrdersPageByTab(initialPagesByTab())
      setOrdersTotalByTab(emptyTotalsByTab())
      setLoadedByTab(unloadedTabs())
      setLoadErrorsByTab(emptyErrorsByTab())
      setPageErrorsByTab(emptyErrorsByTab())
      setApprovalState('checking')
      setActiveTab('pending')
    }
    if (identityAllowed) void loadWorkbench()
    return () => {
      requestVersion.current += 1
      activeLoad.current = null
      activePageLoad.current = null
    }
  }, [identityAllowed, profile.id, loadWorkbench])

  useDidShow(() => {
    if (identityAllowed) void loadWorkbench()
  })

  const loadMoreOrders = useCallback(async () => {
    const requestedTab = activeTab
    const currentOrders = ordersByTab[requestedTab]
    const currentTotal = ordersTotalByTab[requestedTab]
    if (!mountedRef.current || !identityAllowed || !identityRef.current || profile.id !== accountRef.current
      || approvalRef.current !== 'approved' || activeLoad.current !== null || activePageLoad.current !== null
      || loading || loadingMore || currentOrders.length >= currentTotal) return
    const account = accountRef.current
    const version = requestVersion.current
    const pageRequest = { version, tab: requestedTab }
    activePageLoad.current = pageRequest
    const current = () => mountedRef.current && identityRef.current && accountRef.current === account
      && requestVersion.current === version && activePageLoad.current === pageRequest
    const nextPage = ordersPageByTab[requestedTab] + 1
    setLoadingMore(true)
    setPageErrorsByTab((current) => ({ ...current, [requestedTab]: '' }))
    try {
      const result = await listMerchantDeliveryOrders(nextPage, 50, requestedTab)
      if (!current()) return
      setOrdersByTab((current) => {
        const byId = new Map(current[requestedTab].map((order) => [order.deliveryOrderPublicId, order]))
        result.items.forEach((order) => byId.set(order.deliveryOrderPublicId, order))
        return { ...current, [requestedTab]: Array.from(byId.values()) }
      })
      setOrdersPageByTab((current) => ({ ...current, [requestedTab]: result.page || nextPage }))
      setOrdersTotalByTab((current) => ({ ...current, [requestedTab]: result.total }))
    } catch (error) {
      if (current()) {
        setPageErrorsByTab((current) => ({
          ...current,
          [requestedTab]: error instanceof Error ? error.message : '更多配送单加载失败',
        }))
      }
    } finally {
      if (current()) {
        activePageLoad.current = null
        setLoadingMore(false)
      }
    }
  }, [activeTab, identityAllowed, profile.id, loading, loadingMore, ordersByTab, ordersPageByTab, ordersTotalByTab])

  const requestRefresh = async (): Promise<boolean> => {
    if (activePageLoad.current !== null) return false
    return loadWorkbench()
  }

  const notificationRefresh = useNotificationRefresh(identityAllowed && approvalState === 'approved',
    !loading && !loadingMore && Object.values(ordersPageByTab).every(page => page <= 1), requestRefresh)

  const totalOrderCount = ordersTotalByTab.pending + ordersTotalByTab.delivering + ordersTotalByTab.completed
  const allCountsLoaded = tabCopy.every((tab) => loadedByTab[tab.key])

  if (!identityAllowed || profile.id && profile.id !== dataAccountRef.current) return <View className='merchant-home-page' />
  if (approvalState !== 'approved') {
    return (
      <View className='merchant-home-page merchant-home-page--review' style={capsuleStyle}>
        <CapsuleSystemRow />
        <View className='merchant-home-review'>
          {approvalState === 'checking' ? <DeliveryOrderLoading compact label='正在核验商家资格' /> :
            <DeliveryOrderState kind={approvalState === 'error' ? 'error' : 'access'}
              title={approvalState === 'error' ? '商家资格核验失败' : '暂时无法进入商家端'} description={approvalMessage} />}
          {approvalState === 'error' ? (
            <View className='merchant-home-review__action merchant-home-review__retry' ariaRole='button' ariaLabel='重新核验商家资格' hoverClass='merchant-home-control--pressed' onClick={() => void loadWorkbench()}>
              <Text>重新尝试</Text>
            </View>
          ) : null}
          {approvalState === 'blocked' || approvalState === 'error' ? (
            <View className={`merchant-home-review__action${approvalState === 'error' ? ' merchant-home-review__action--secondary' : ''}`} ariaRole='button' ariaLabel='返回我的' hoverClass='merchant-home-control--pressed' onClick={() => void Taro.reLaunch({ url: '/pages/profile/index' })}>
              <Text>返回我的</Text>
            </View>
          ) : null}
        </View>
      </View>
    )
  }

  return (
    <View className='merchant-home-page' style={capsuleStyle} {...workspace.swipe.handlers}>
      <View className='merchant-home-hero'>
        <CapsuleSystemRow />
        <View className='merchant-home-workspace-heading'>
          <View className='merchant-home-workspace-heading__copy'>
            <Text className='merchant-home-workspace-heading__title'>订单管理</Text>
          </View>
          <View className='merchant-home-metrics'>
            <View className='merchant-home-metric'>
              <Text className='merchant-home-metric__label'>全部订单</Text>
              <Text className={`merchant-home-metric__value${!allCountsLoaded ? ' merchant-home-metric__value--pending' : ''}`}>
                {allCountsLoaded ? totalOrderCount : '—'}
              </Text>
            </View>
          </View>
        </View>
      </View>

      {(!visibility.delivery || !visibility.merchant) && <View className='merchant-home-section-head'><Text>新业务暂未开放，可继续处理已有订单与资金。</Text></View>}

      <View className='merchant-home-tabs'>
        {tabCopy.map((tab) => (
          <View
            key={tab.key}
            className={`merchant-home-tab ${activeTab === tab.key ? 'merchant-home-tab--active' : ''}`}
            ariaRole='button'
            ariaLabel={`${tab.label}${activeTab === tab.key ? '，已选中' : ''}`}
            hoverClass='merchant-home-control--pressed'
            onClick={() => { if (canActivateWorkspace()) setActiveTab(tab.key) }}
          >
            <Text className='merchant-home-tab__label'>{tab.label}</Text>
            <Text className={`merchant-home-tab__count merchant-home-tab__count--${tab.key}${!loadedByTab[tab.key] ? ' merchant-home-tab__count--pending' : ''}`}>
              {loadedByTab[tab.key] ? ordersTotalByTab[tab.key] : '—'}
            </Text>
          </View>
        ))}
      </View>

      <DeliveryStagePager keys={tabCopy.map(tab => tab.key)} activeKey={activeTab} onChange={setActiveTab} interaction={channelInteraction}
        scopeKey={`merchant:${profile.id}`} enabled={identityAllowed && approvalState === 'approved'} onBoundarySwipe={workspace.step}
        renderPage={tab => (
      <RefreshScrollView
        scrollY
        className='merchant-home-orders delivery-orders-scroll'
        refreshKey={`merchant:${profile.id}:${tab}`}
        enabled={identityAllowed && activeTab === tab}
        busy={loading || loadingMore}
        onRefresh={() => activeTab === tab ? requestRefresh() : Promise.resolve(false)}
        enhanced
        showScrollbar={false}
        lowerThreshold={120}
        onScrollToLower={() => { if (activeTab === tab && channelInteraction.canActivate()) void loadMoreOrders() }}
      >
        <View className='merchant-home-orders__inner'>
          {loadedByTab[tab] || !loading ? <View className='delivery-order-toolbar'>
            <Text>{loading ? '正在更新订单' : notificationRefresh.pending ? '订单有更新，可下拉刷新' : '下拉刷新订单'}</Text>
            <View className={`delivery-order-toolbar__refresh${loading || loadingMore ? ' delivery-order-toolbar__refresh--disabled' : ''}`}
              ariaRole='button' ariaLabel={loading ? '正在刷新配送订单' : '刷新配送订单'} hoverClass='delivery-order-control--pressed'
              onClick={() => { void requestRefresh() }}>
              {loading ? <DeliveryLoadDots /> : null}<Text>{loading ? '刷新中' : '刷新'}</Text>
            </View>
          </View> : null}
          {loading && !loadedByTab[tab] ? <DeliveryOrderLoading columns={2} /> : null}
          {!loading && loadErrorsByTab[tab] ? (
            <DeliveryOrderState kind='error' compact={ordersByTab[tab].length > 0}
              title={`配送单${loadedByTab[tab] ? '刷新' : '加载'}失败`} description={loadErrorsByTab[tab]}
              actionLabel='重新加载' onAction={() => void requestRefresh()} />
          ) : null}
          {loadedByTab[tab] && !loadErrorsByTab[tab] && ordersByTab[tab].length === 0 ? (
            <DeliveryOrderState kind='empty' title={`暂无${tabCopy.find((item) => item.key === tab)?.label}配送单`}
              description='新配送单和状态更新会显示在这里，可下拉刷新。' />
          ) : null}
          <View className='merchant-home-order-grid'>
            {ordersByTab[tab].map((order) => <OrderCard key={order.deliveryOrderPublicId} order={order} canActivate={() => activeTab === tab && channelInteraction.canActivate()} />)}
          </View>
          {pageErrorsByTab[tab] ? <DeliveryOrderState kind='error' compact title='更多订单暂未加载'
            description={pageErrorsByTab[tab]} actionLabel='重试加载' onAction={() => { if (activeTab === tab && channelInteraction.canActivate()) void loadMoreOrders() }} /> : null}
          {loadingMore ? <DeliveryOrderLoading compact label='正在加载更多订单' /> : null}
          {!loading && !loadingMore && !loadErrorsByTab[tab] && ordersByTab[tab].length > 0 && ordersByTab[tab].length >= ordersTotalByTab[tab] ? (
            <Text className='merchant-home-orders__end'>— 已显示全部配送单 —</Text>
          ) : null}
        </View>
      </RefreshScrollView>
        )} />

      <DeliveryWorkspaceNav role='merchant' active='orders' gestureGuard={canActivateWorkspace} />

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
