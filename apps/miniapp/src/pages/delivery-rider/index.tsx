import React, { useCallback, useMemo, useState } from 'react'
import { ScrollView, Text, View } from '@tarojs/components'
import { useDidShow } from '@tarojs/taro'
import { CapsuleSystemRow } from '../../components/capsule-page-header'
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
  const [assigned, setAssigned] = useState<AssignedDeliveryOrder[]>([])
  const [loading, setLoading] = useState(false)
  const [busyOrderId, setBusyOrderId] = useState('')
  const [error, setError] = useState('')

  const pickupOrders = useMemo(
    () => assigned.filter((order) => order.status === 'claimed' || order.status === 'to_pickup'),
    [assigned]
  )
  const deliveryOrders = useMemo(
    () => assigned.filter((order) => order.status === 'picked_up' || order.status === 'delivering'),
    [assigned]
  )
  const activeTaskCount = pickupOrders.length + deliveryOrders.length
  const completedCount = assigned.filter((order) => order.status === 'delivered').length

  const refreshOrders = async () => {
    const [claimablePage, assignedPage] = await Promise.all([
      listClaimableDeliveryOrders(),
      listAssignedDeliveryOrders()
    ])
    setClaimable(claimablePage.items)
    setAssigned(assignedPage.items)
  }

  const load = useCallback(async () => {
    if (!identityAllowed || loading) return
    setLoading(true)
    setError('')
    try {
      const rider = await getDeliveryRiderMe()
      setMe(rider)
      if (!rider.registered || !rider.rider) {
        setClaimable([])
        setAssigned([])
        return
      }
      const [claimablePage, assignedPage] = await Promise.all([
        listClaimableDeliveryOrders(),
        listAssignedDeliveryOrders()
      ])
      setClaimable(claimablePage.items)
      setAssigned(assignedPage.items)
    } catch (value) {
      const message = errorMessage(value, '骑手订单加载失败')
      setError(message)
      await Taro.showToast({ title: message, icon: 'none' })
    } finally {
      setLoading(false)
    }
  }, [identityAllowed, loading])

  useDidShow(() => {
    void load()
  })

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
    const modal = await Taro.showModal({
      title: '确认释放订单',
      content: '释放后订单会重新进入抢单池，请确认你尚未取到货物。',
      confirmText: '确认释放'
    })
    if (!modal.confirm) return
    await runCommand(order, releaseDeliveryOrder, '订单已释放')
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
        <View className='rider-grab-placeholder'>
          <View className='rider-grab-placeholder__icon'><Text>骑</Text></View>
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
          <View className='rider-grab-placeholder__icon'><Text>骑</Text></View>
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
    { key: 'grab', label: '待抢单', count: claimable.length },
    { key: 'pickup', label: '待取货', count: pickupOrders.length },
    { key: 'delivery', label: '待送达', count: deliveryOrders.length }
  ]

  return (
    <View className='rider-grab-page'>
      <View className='rider-grab-hero'>
        <View className='rider-grab-hero__orb rider-grab-hero__orb--one' />
        <View className='rider-grab-hero__orb rider-grab-hero__orb--two' />
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

        <View className='rider-grab-wallet-summary'>
          <View className='rider-grab-wallet-summary__main'>
            <Text className='rider-grab-wallet-summary__label'>当前配送任务</Text>
            <View className='rider-grab-wallet-summary__amount-row'>
              <Text className='rider-grab-wallet-summary__amount'>{activeTaskCount}</Text>
              <Text className='rider-grab-wallet-summary__unit'>单</Text>
            </View>
            <Text className='rider-grab-wallet-summary__sub'>
              可抢 {claimable.length} 单　｜　已完成 {completedCount} 单
            </Text>
          </View>
          <View
            className={`rider-grab-wallet-summary__button ${loading ? 'rider-grab-wallet-summary__button--loading' : ''}`}
            onClick={() => void load()}
          >
            <View className='rider-grab-wallet-summary__refresh-icon' />
            <Text>{loading ? '刷新中' : '刷新'}</Text>
          </View>
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

      <ScrollView scrollY enhanced showScrollbar={false} className='rider-grab-content'>
        <View className='rider-grab-content__inner'>
          {error ? (
            <View className='rider-grab-error' onClick={() => void load()}>
              <Text>{error}，点击重试</Text>
            </View>
          ) : null}

          {activeTab === 'grab' ? renderClaimableOrders() : null}
          {activeTab === 'pickup' ? renderAssignedOrders(pickupOrders, 'pickup') : null}
          {activeTab === 'delivery' ? renderAssignedOrders(deliveryOrders, 'delivery') : null}
        </View>
      </ScrollView>
    </View>
  )

  function renderClaimableOrders() {
    return (
      <>
        <View className='rider-grab-list-head'>
          <Text>当前共 {claimable.length} 单可抢</Text>
          <View className='rider-grab-list-head__refresh' onClick={() => void load()}>
            <Text>{loading ? '刷新中…' : '刷新订单'}</Text>
          </View>
        </View>
        {claimable.length === 0 ? (
          <EmptyState title='当前暂无可抢配送单' description='新配送单出现后会显示在这里。' />
        ) : (
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
        )}
      </>
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
            <Text>消息</Text>
          </View>
          <View
            className='rider-order-card__secondary'
            onClick={() => void Taro.makePhoneCall({ phoneNumber: contactPhone })}
          >
            <Text>电话</Text>
          </View>
          {releasable ? (
            <View
              className={`rider-order-card__secondary rider-order-card__secondary--danger ${busyOrderId ? 'rider-order-card__secondary--disabled' : ''}`}
              onClick={() => {
                if (!busyOrderId) void release(order)
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
    <View className='rider-grab-placeholder rider-grab-placeholder--inline'>
      <View className='rider-grab-placeholder__icon'><Text>骑</Text></View>
      <Text className='rider-grab-placeholder__title'>{title}</Text>
      <Text className='rider-grab-placeholder__desc'>{description}</Text>
    </View>
  )
}
