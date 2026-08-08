import React, { useCallback, useRef, useState } from 'react'
import { ScrollView, Text, View } from '@tarojs/components'
import { useDidShow } from '@tarojs/taro'
import { CapsuleSystemRow } from '../../components/capsule-page-header'
import { AppDialog } from '../../components/ui'
import { identityRoute, useIdentityGuard } from '../../hooks/use-identity-guard'
import { isApiError } from '../../services/api'
import {
  claimDeliveryOrder,
  getDeliveryRiderMe,
  listAssignedDeliveryOrders,
  listClaimableDeliveryOrders,
  markDelivered,
  markPickedUp,
  releaseDeliveryOrder,
  startDelivery,
  startToPickup,
  type AssignedDeliveryOrder,
  type ClaimableDeliveryOrder,
  type DeliveryRiderMe
} from '../../services/delivery-rider'
import { centsToYuan, type DeliveryStatus } from '../../services/delivery-merchant'
import { chooseAndUploadImages, waitForMediaApproval } from '../../services/media'
import Taro from '../../services/taro'
import RiderWorkbenchIcon from './rider-workbench-icon'
import './index.scss'

type RiderTab = 'grab' | 'pickup' | 'delivery'

const statusText: Record<DeliveryStatus, string> = {
  pending_fee_payment: '待付配送费',
  payment_failed: '费用确认失败',
  ready_for_claim: '待抢单',
  claimed: '已接单',
  to_pickup: '前往取货',
  picked_up: '已取货',
  delivering: '配送中',
  delivered: '已送达',
  cancelled: '已取消'
}

function errorMessage(error: unknown, fallback: string): string {
  if (isApiError(error)) return error.message || fallback
  return error instanceof Error ? error.message : fallback
}

function formatTime(value: string): string {
  const timestamp = Date.parse(value)
  if (!Number.isFinite(timestamp)) return value
  const date = new Date(timestamp)
  return `${date.getMonth() + 1}月${date.getDate()}日 ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
}

function formatRefreshTime(value: Date | null): string {
  if (!value) return '等待首次同步'
  return `${String(value.getHours()).padStart(2, '0')}:${String(value.getMinutes()).padStart(2, '0')} 更新`
}

function shortOrderId(value: string): string {
  return `#${value.slice(-8).toUpperCase()}`
}

function parseRouteSummary(value: string): { pickup: string; delivery: string } {
  const summary = value.trim()
  if (!summary) return { pickup: '校园取货点', delivery: '抢单后显示详细地址' }
  const parts = summary.split(/\s*(?:→|->|至)\s*/).filter(Boolean)
  if (parts.length >= 2) {
    return { pickup: parts[0], delivery: parts.slice(1).join(' → ') }
  }
  return { pickup: summary, delivery: '抢单后显示详细地址' }
}

function riderStatusLabel(status: NonNullable<DeliveryRiderMe['rider']>['status']): string {
  if (status === 'active') return '可接单'
  if (status === 'resting') return '休息中'
  return '已停用'
}

export default function DeliveryRiderPage() {
  const identityAllowed = useIdentityGuard('rider')
  const [activeTab, setActiveTab] = useState<RiderTab>('grab')
  const [me, setMe] = useState<DeliveryRiderMe | null>(null)
  const [claimable, setClaimable] = useState<ClaimableDeliveryOrder[]>([])
  const [pickupOrders, setPickupOrders] = useState<AssignedDeliveryOrder[]>([])
  const [deliveryOrders, setDeliveryOrders] = useState<AssignedDeliveryOrder[]>([])
  const [claimablePage, setClaimablePage] = useState(1)
  const [claimableTotal, setClaimableTotal] = useState(0)
  const [pickupPage, setPickupPage] = useState(1)
  const [pickupTotal, setPickupTotal] = useState(0)
  const [deliveryPage, setDeliveryPage] = useState(1)
  const [deliveryTotal, setDeliveryTotal] = useState(0)
  const [completedTotal, setCompletedTotal] = useState(0)
  const [loadingMore, setLoadingMore] = useState(false)
  const [loading, setLoading] = useState(false)
  const [refreshing, setRefreshing] = useState(false)
  const [lastUpdatedAt, setLastUpdatedAt] = useState<Date | null>(null)
  const [busyOrderId, setBusyOrderId] = useState('')
  const [error, setError] = useState('')
  const [loadMoreError, setLoadMoreError] = useState('')
  const [releaseCandidate, setReleaseCandidate] = useState<AssignedDeliveryOrder | null>(null)
  const requestVersion = useRef(0)

  const activeTaskCount = pickupTotal + deliveryTotal

  const refreshOrders = async () => {
    const version = requestVersion.current + 1
    requestVersion.current = version
    setLoading(true)
    setLoadingMore(false)
    setError('')
    setLoadMoreError('')
    try {
      const [nextClaimable, nextPickup, nextDelivery, nextCompleted] = await Promise.all([
        listClaimableDeliveryOrders(1, 50),
        listAssignedDeliveryOrders(1, 50, 'pickup'),
        listAssignedDeliveryOrders(1, 50, 'delivery'),
        listAssignedDeliveryOrders(1, 1, 'completed')
      ])
      if (requestVersion.current !== version) return
      setClaimable(nextClaimable.items)
      setClaimablePage(nextClaimable.page || 1)
      setClaimableTotal(nextClaimable.total)
      setPickupOrders(nextPickup.items)
      setPickupPage(nextPickup.page || 1)
      setPickupTotal(nextPickup.total)
      setDeliveryOrders(nextDelivery.items)
      setDeliveryPage(nextDelivery.page || 1)
      setDeliveryTotal(nextDelivery.total)
      setCompletedTotal(nextCompleted.total)
      setLastUpdatedAt(new Date())
    } finally {
      if (requestVersion.current === version) setLoading(false)
    }
  }

  const load = useCallback(async () => {
    if (!identityAllowed) return
    const version = requestVersion.current + 1
    requestVersion.current = version
    setLoading(true)
    setLoadingMore(false)
    setError('')
    setLoadMoreError('')
    try {
      const rider = await getDeliveryRiderMe()
      if (requestVersion.current !== version) return
      setMe(rider)
      if (!rider.registered || !rider.rider) {
        setClaimable([])
        setPickupOrders([])
        setDeliveryOrders([])
        setClaimableTotal(0)
        setPickupTotal(0)
        setDeliveryTotal(0)
        setCompletedTotal(0)
        return
      }
      const [claimablePage, pickupPage, deliveryPage, completedPage] = await Promise.all([
        listClaimableDeliveryOrders(1, 50),
        listAssignedDeliveryOrders(1, 50, 'pickup'),
        listAssignedDeliveryOrders(1, 50, 'delivery'),
        listAssignedDeliveryOrders(1, 1, 'completed')
      ])
      if (requestVersion.current !== version) return
      setClaimable(claimablePage.items)
      setClaimablePage(claimablePage.page || 1)
      setClaimableTotal(claimablePage.total)
      setPickupOrders(pickupPage.items)
      setPickupPage(pickupPage.page || 1)
      setPickupTotal(pickupPage.total)
      setDeliveryOrders(deliveryPage.items)
      setDeliveryPage(deliveryPage.page || 1)
      setDeliveryTotal(deliveryPage.total)
      setCompletedTotal(completedPage.total)
      setLastUpdatedAt(new Date())
    } catch (value) {
      if (requestVersion.current !== version) return
      const message = errorMessage(value, '骑手订单加载失败')
      setError(message)
      await Taro.showToast({ title: message, icon: 'none' })
    } finally {
      if (requestVersion.current === version) setLoading(false)
    }
  }, [identityAllowed])

  useDidShow(() => {
    void load()
  })

  const handlePullDownRefresh = async () => {
    if (refreshing || loading) return
    setRefreshing(true)
    try {
      await load()
    } finally {
      setRefreshing(false)
    }
  }

  const loadMoreOrders = async () => {
    if (loading || loadingMore) return
    const isClaimable = activeTab === 'grab'
    const items = isClaimable ? claimable : activeTab === 'pickup' ? pickupOrders : deliveryOrders
    const total = isClaimable ? claimableTotal : activeTab === 'pickup' ? pickupTotal : deliveryTotal
    if (items.length >= total) return
    const nextPage = (isClaimable ? claimablePage : activeTab === 'pickup' ? pickupPage : deliveryPage) + 1
    const requestedTab = activeTab
    const version = requestVersion.current
    setLoadingMore(true)
    setLoadMoreError('')
    try {
      if (isClaimable) {
        const result = await listClaimableDeliveryOrders(nextPage, 50)
        if (requestVersion.current !== version) return
        setClaimable((current) => {
          const byId = new Map(current.map((order) => [order.deliveryOrderPublicId, order]))
          result.items.forEach((order) => byId.set(order.deliveryOrderPublicId, order))
          return Array.from(byId.values())
        })
        setClaimablePage(result.page || nextPage)
        setClaimableTotal(result.total)
      } else {
        const statusGroup = requestedTab === 'pickup' ? 'pickup' : 'delivery'
        const result = await listAssignedDeliveryOrders(nextPage, 50, statusGroup)
        if (requestVersion.current !== version) return
        const mergeOrders = (current: AssignedDeliveryOrder[]) => {
          const byId = new Map(current.map((order) => [order.deliveryOrderPublicId, order]))
          result.items.forEach((order) => byId.set(order.deliveryOrderPublicId, order))
          return Array.from(byId.values())
        }
        if (requestedTab === 'pickup') {
          setPickupOrders(mergeOrders)
          setPickupPage(result.page || nextPage)
          setPickupTotal(result.total)
        } else {
          setDeliveryOrders(mergeOrders)
          setDeliveryPage(result.page || nextPage)
          setDeliveryTotal(result.total)
        }
      }
    } catch (value) {
      if (requestVersion.current === version) {
        setLoadMoreError(errorMessage(value, '更多订单加载失败，点击重试'))
      }
    } finally {
      if (requestVersion.current === version) setLoadingMore(false)
    }
  }

  const claim = async (order: ClaimableDeliveryOrder) => {
    if (busyOrderId) return
    setBusyOrderId(order.deliveryOrderPublicId)
    try {
      await claimDeliveryOrder(order)
      setActiveTab('pickup')
      await Taro.showToast({ title: '接单成功', icon: 'success' })
      await refreshOrders()
    } catch (value) {
      await Taro.showToast({ title: errorMessage(value, '接单失败，请刷新后重试'), icon: 'none' })
      await refreshOrders()
    } finally {
      setBusyOrderId('')
    }
  }

  const runCommand = async (
    order: AssignedDeliveryOrder,
    command: (value: AssignedDeliveryOrder) => Promise<AssignedDeliveryOrder>,
    success: string
  ) => {
    if (busyOrderId) return
    setBusyOrderId(order.deliveryOrderPublicId)
    try {
      await command(order)
      await Taro.showToast({ title: success, icon: 'success' })
      await refreshOrders()
    } catch (value) {
      await Taro.showToast({ title: errorMessage(value, '操作失败，请刷新后重试'), icon: 'none' })
      await refreshOrders()
    } finally {
      setBusyOrderId('')
    }
  }

  const runProofCommand = async (
    order: AssignedDeliveryOrder,
    command: (value: AssignedDeliveryOrder, proofAssetPublicId: string) => Promise<AssignedDeliveryOrder>,
    success: string
  ) => {
    if (busyOrderId) return
    setBusyOrderId(order.deliveryOrderPublicId)
    try {
      const [uploaded] = await chooseAndUploadImages('delivery_proof', 1, (progress) => {
        void Taro.showLoading({ title: `${progress.message} ${progress.percent}%`, mask: true })
      })
      if (!uploaded) return
      await waitForMediaApproval(uploaded.assetId, (status) => {
        void Taro.showLoading({ title: status.message || '凭证审核中', mask: true })
      })
      await command(order, uploaded.assetId)
      await Taro.showToast({ title: success, icon: 'success' })
      await refreshOrders()
    } catch (value) {
      await Taro.showToast({ title: errorMessage(value, '凭证提交失败，请重试'), icon: 'none' })
      await refreshOrders()
    } finally {
      Taro.hideLoading()
      setBusyOrderId('')
    }
  }

  const release = async (order: AssignedDeliveryOrder) => {
    await runCommand(order, releaseDeliveryOrder, '订单已释放')
    setReleaseCandidate(null)
  }

  const openConversation = (orderId: string) => {
    void Taro.navigateTo({
      url: `/pages/delivery-conversation/index?orderId=${encodeURIComponent(orderId)}&role=rider`
    })
  }

  const switchToStudent = async () => {
    const store = await import('../../stores/session')
    try {
      await store.useSessionStore.getState().switchIdentity('student')
      await Taro.reLaunch({ url: identityRoute('student') })
    } catch (value) {
      await Taro.showToast({ title: errorMessage(value, '身份切换失败'), icon: 'none' })
    }
  }

  if (!identityAllowed) return <View className='rider-grab-page' />

  if (!me || loading && !me) {
    return (
      <View className='rider-grab-page'>
        <View className='rider-access-hero'>
          <CapsuleSystemRow />
          <Text className='rider-access-hero__title'>骑手工作台</Text>
        </View>
        <View className='rider-grab-placeholder rider-grab-placeholder--loading'>
          <RiderWorkbenchIcon name='empty' className='rider-grab-placeholder__icon' />
          <Text className='rider-grab-placeholder__title'>正在加载骑手工作台</Text>
          <Text className='rider-grab-placeholder__desc'>正在同步身份与配送订单，请稍候。</Text>
        </View>
      </View>
    )
  }

  if (!me.registered || !me.rider) {
    return (
      <View className='rider-grab-page'>
        <View className='rider-access-hero'>
          <CapsuleSystemRow />
          <Text className='rider-access-hero__title'>骑手工作台</Text>
        </View>
        <View className='rider-grab-placeholder'>
          <RiderWorkbenchIcon name='empty' className='rider-grab-placeholder__icon' />
          <Text className='rider-grab-placeholder__title'>骑手权限尚未开通</Text>
          <Text className='rider-grab-placeholder__desc'>
            {me.message || '请先提交骑手身份申请，并等待平台审核。'}
          </Text>
          <View className='rider-grab-placeholder__action' onClick={() => void switchToStudent()}>
            <Text>返回学生端</Text>
          </View>
        </View>
      </View>
    )
  }

  const rider = me.rider
  const riderInitial = rider.realName.trim().slice(0, 1) || '骑'
  const tabs: Array<{ key: RiderTab; label: string; count: number }> = [
    { key: 'grab', label: '待抢单', count: claimableTotal },
    { key: 'pickup', label: '待取货', count: pickupTotal },
    { key: 'delivery', label: '待送达', count: deliveryTotal }
  ]
  const activeOrderCount = activeTab === 'grab'
    ? claimable.length
    : activeTab === 'pickup'
      ? pickupOrders.length
      : deliveryOrders.length

  return (
    <View className='rider-grab-page'>
      <View className='rider-grab-hero'>
        <CapsuleSystemRow className='rider-grab-system-row' />

        <View className='rider-grab-user'>
          <View className='rider-grab-user__avatar'><Text>{riderInitial}</Text></View>
          <View className='rider-grab-user__main'>
            <View className='rider-grab-user__name-row'>
              <Text className='rider-grab-user__name'>{rider.realName || '校园骑手'}</Text>
              <Text className='rider-grab-user__badge'>骑手</Text>
            </View>
            <Text className='rider-grab-user__meta'>已认证 · {riderStatusLabel(rider.status)}</Text>
          </View>
          <View className='rider-grab-user__switch' onClick={() => void switchToStudent()}>
            <Text>切换身份</Text>
          </View>
        </View>

        <View className='rider-grab-task-summary'>
          <View className='rider-grab-task-summary__primary'>
            <Text className='rider-grab-task-summary__label'>当前配送任务</Text>
            <View className='rider-grab-task-summary__value-row'>
              <Text className='rider-grab-task-summary__value'>{activeTaskCount}</Text>
              <Text className='rider-grab-task-summary__unit'>单</Text>
            </View>
          </View>
          <View className='rider-grab-task-summary__stats'>
            <View className='rider-grab-task-summary__stat'>
              <Text className='rider-grab-task-summary__stat-value'>{claimableTotal}</Text>
              <Text className='rider-grab-task-summary__stat-label'>可抢</Text>
            </View>
            <View className='rider-grab-task-summary__divider' />
            <View className='rider-grab-task-summary__stat'>
              <Text className='rider-grab-task-summary__stat-value'>{completedTotal}</Text>
              <Text className='rider-grab-task-summary__stat-label'>已结束</Text>
            </View>
          </View>
        </View>
        <View className='rider-grab-refresh-hint'>
          <View className={`rider-grab-refresh-hint__dot ${refreshing ? 'rider-grab-refresh-hint__dot--active' : ''}`} />
          <Text className='rider-grab-refresh-hint__label'>{refreshing ? '正在刷新订单' : '下拉刷新订单'}</Text>
          <Text className='rider-grab-refresh-hint__time'>· {formatRefreshTime(lastUpdatedAt)}</Text>
        </View>
      </View>

      <View className='rider-grab-tabs'>
        {tabs.map((item) => (
          <View
            key={item.key}
            className={`rider-grab-tabs__item ${activeTab === item.key ? 'rider-grab-tabs__item--active' : ''}`}
            onClick={() => setActiveTab(item.key)}
          >
            <Text>{item.label}</Text>
            <Text className={`rider-grab-tabs__count rider-grab-tabs__count--${item.key}`}>{item.count}</Text>
          </View>
        ))}
      </View>

      <ScrollView
        scrollY
        enhanced
        showScrollbar={false}
        refresherEnabled
        refresherTriggered={refreshing}
        refresherBackground='#F3F6FA'
        onRefresherRefresh={() => void handlePullDownRefresh()}
        className='rider-grab-content'
        lowerThreshold={120}
        onScrollToLower={() => void loadMoreOrders()}
      >
        <View className='rider-grab-content__inner'>
          {error && activeOrderCount === 0 ? (
            <View className='rider-load-error' onClick={() => void load()}>
              <Text className='rider-load-error__title'>订单加载失败</Text>
              <Text className='rider-load-error__desc'>{error}</Text>
              <Text className='rider-load-error__action'>点击重试</Text>
            </View>
          ) : (
            <>
              {error ? (
                <View className='rider-grab-error' onClick={() => void load()}>
                  <Text>{error}，当前显示上次加载的数据，点击重试</Text>
                </View>
              ) : null}
              {activeTab === 'grab' ? renderClaimableOrders() : null}
              {activeTab === 'pickup' ? renderAssignedOrders(pickupOrders, 'pickup') : null}
              {activeTab === 'delivery' ? renderAssignedOrders(deliveryOrders, 'delivery') : null}
              {loadMoreError ? (
                <Text className='rider-grab-content__loading-more rider-grab-content__loading-more--retry' onClick={() => void loadMoreOrders()}>{loadMoreError}</Text>
              ) : null}
              {loadingMore ? <Text className='rider-grab-content__loading-more'>正在加载更多订单…</Text> : null}
            </>
          )}
        </View>
      </ScrollView>
      <AppDialog
        visible={releaseCandidate !== null}
        title='确认释放订单'
        description='释放后订单会重新进入抢单池，请确认你尚未取到货物。'
        confirmText='确认释放'
        tone='danger'
        loading={Boolean(busyOrderId)}
        onCancel={() => setReleaseCandidate(null)}
        onConfirm={() => { if (releaseCandidate) void release(releaseCandidate) }}
      />
    </View>
  )

  function renderClaimableOrders() {
    if (claimable.length === 0) {
      return <EmptyState title='当前暂无可抢配送单' description='新配送单出现后会显示在这里。' />
    }

    return (
      <View className='rider-grab-list'>
        {claimable.map((order) => {
          const route = parseRouteSummary(order.routeSummary)
          const busy = busyOrderId === order.deliveryOrderPublicId
          return (
            <View key={order.deliveryOrderPublicId} className='rider-order-card'>
              <View className='rider-order-card__top'>
                <View className='rider-order-card__serial'>
                  <Text>{shortOrderId(order.deliveryOrderPublicId)}</Text>
                  <Text className='rider-order-card__dot'>·</Text>
                  <Text>{formatTime(order.createdAt)}</Text>
                </View>
                <View className='rider-order-card__fee'>
                  <Text>配送费 </Text>
                  <Text className='rider-order-card__fee-price'>¥{centsToYuan(order.deliveryFeeCents)}</Text>
                </View>
              </View>

              <View className='rider-order-route'>
                <View className='rider-order-route__line' />
                <View className='rider-order-route__row'>
                  <View className='rider-order-route__marker rider-order-route__marker--pickup' />
                  <View className='rider-order-route__info'>
                    <Text className='rider-order-route__label'>取货点</Text>
                    <Text className='rider-order-route__place'>{route.pickup}</Text>
                  </View>
                </View>
                <View className='rider-order-route__row rider-order-route__row--drop'>
                  <View className='rider-order-route__marker rider-order-route__marker--drop' />
                  <View className='rider-order-route__info'>
                    <Text className='rider-order-route__label'>送达点</Text>
                    <View className='rider-order-route__drop-row'>
                      <Text className='rider-order-route__place'>{route.delivery}</Text>
                      <Text className='rider-order-route__lock'>抢单后显示门牌</Text>
                    </View>
                  </View>
                </View>
              </View>

              <View className='rider-order-card__bottom'>
                <Text className='rider-order-card__privacy'>隐私保护 · 接单后可联系</Text>
                <View
                  className={`rider-order-card__grab ${busy || rider.status !== 'active' ? 'rider-order-card__grab--disabled' : ''}`}
                  onClick={() => {
                    if (!busy && rider.status === 'active') void claim(order)
                  }}
                >
                  <Text>{busy ? '接单中…' : rider.status === 'active' ? '抢单' : '当前不可接单'}</Text>
                </View>
              </View>
            </View>
          )
        })}
      </View>
    )
  }

  function renderAssignedOrders(orders: AssignedDeliveryOrder[], kind: 'pickup' | 'delivery') {
    if (orders.length === 0) {
      return (
        <EmptyState
          title={kind === 'pickup' ? '当前暂无待取货订单' : '当前暂无待送达订单'}
          description={kind === 'pickup' ? '抢单成功后，订单会进入待取货列表。' : '完成取货后，订单会进入待送达列表。'}
        />
      )
    }
    return <View className='rider-grab-list rider-grab-list--assigned'>{orders.map(renderAssignedOrder)}</View>
  }

  function renderAssignedOrder(order: AssignedDeliveryOrder) {
    const busy = busyOrderId === order.deliveryOrderPublicId
    const releasable = order.status === 'claimed' || order.status === 'to_pickup'
    const deliveryStage = order.status === 'picked_up' || order.status === 'delivering'
    const contactPhone = deliveryStage ? order.recipientPhone : order.pickupContactPhone
    const primaryAction = getPrimaryAction(order)

    return (
      <View key={order.deliveryOrderPublicId} className='rider-order-card rider-order-card--assigned'>
        <View className='rider-order-card__top'>
          <View className='rider-order-card__serial'>
            <Text>{shortOrderId(order.deliveryOrderPublicId)}</Text>
            <Text className='rider-order-card__dot'>·</Text>
            <Text>{statusText[order.status]}</Text>
          </View>
          <View className='rider-order-card__fee'>
            <Text>配送费 </Text>
            <Text className='rider-order-card__fee-price'>¥{centsToYuan(order.deliveryFeeCents)}</Text>
          </View>
        </View>

        <View className='rider-order-route rider-order-route--assigned'>
          <View className='rider-order-route__line' />
          <View className='rider-order-route__row'>
            <View className='rider-order-route__marker rider-order-route__marker--pickup' />
            <View className='rider-order-route__info'>
              <Text className='rider-order-route__label'>取货点 · {order.pickupContactName}</Text>
              <Text className='rider-order-route__place'>{order.pickupAddress}</Text>
            </View>
          </View>
          <View className='rider-order-route__row rider-order-route__row--drop'>
            <View className='rider-order-route__marker rider-order-route__marker--drop' />
            <View className='rider-order-route__info'>
              <Text className='rider-order-route__label'>送达点 · {order.recipientName}</Text>
              <View className='rider-order-route__drop-row'>
                <Text className='rider-order-route__place'>{order.deliveryAddress}</Text>
                <Text className='rider-order-route__complete'>完整地址</Text>
              </View>
            </View>
          </View>
        </View>

        {order.remark ? (
          <View className='rider-order-card__remark rider-order-card__remark--assigned'>
            <Text className='rider-order-card__remark-label'>备注</Text>
            <Text>{order.remark}</Text>
          </View>
        ) : null}

        <View className='rider-order-card__quick-actions'>
          <View className='rider-order-card__secondary' onClick={() => openConversation(order.deliveryOrderPublicId)}>
            <RiderWorkbenchIcon name='message' className='rider-order-card__secondary-icon' />
            <Text>消息</Text>
          </View>
          <View
            className='rider-order-card__secondary'
            onClick={() => void Taro.makePhoneCall({ phoneNumber: contactPhone })}
          >
            <RiderWorkbenchIcon name='phone' className='rider-order-card__secondary-icon' />
            <Text>电话</Text>
          </View>
          {releasable ? (
            <View
              className={`rider-order-card__secondary rider-order-card__secondary--danger ${busyOrderId ? 'rider-order-card__secondary--disabled' : ''}`}
              onClick={() => {
                if (!busyOrderId) setReleaseCandidate(order)
              }}
            >
              <Text>释放</Text>
            </View>
          ) : null}
          {primaryAction ? (
            <View
              className={`rider-order-card__grab ${busyOrderId ? 'rider-order-card__grab--disabled' : ''}`}
              onClick={() => {
                if (!busyOrderId) primaryAction.run()
              }}
            >
              <Text>{busy ? '处理中…' : primaryAction.label}</Text>
            </View>
          ) : (
            <View className='rider-order-card__status'><Text>{statusText[order.status]}</Text></View>
          )}
        </View>
      </View>
    )
  }

  function getPrimaryAction(order: AssignedDeliveryOrder): { label: string; run: () => void } | null {
    if (order.status === 'claimed') {
      return { label: '前往取货', run: () => void runCommand(order, startToPickup, '已开始前往取货') }
    }
    if (order.status === 'to_pickup') {
      return { label: '确认取货', run: () => void runProofCommand(order, markPickedUp, '取货凭证已提交') }
    }
    if (order.status === 'picked_up') {
      return { label: '开始配送', run: () => void runCommand(order, startDelivery, '已开始配送') }
    }
    if (order.status === 'delivering') {
      return { label: '确认送达', run: () => void runProofCommand(order, markDelivered, '送达凭证已提交') }
    }
    return null
  }
}

function EmptyState({ title, description }: { title: string; description: string }) {
  return (
    <View className='rider-empty-state'>
      <RiderWorkbenchIcon name='empty' className='rider-empty-state__icon' />
      <View className='rider-empty-state__copy'>
        <Text className='rider-empty-state__title'>{title}</Text>
        <Text className='rider-empty-state__desc'>{description}</Text>
      </View>
    </View>
  )
}
