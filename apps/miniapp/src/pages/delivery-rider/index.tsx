import React, { useCallback, useState } from 'react'
import { Button, Text, View } from '@tarojs/components'
import { useDidShow } from '@tarojs/taro'
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

type Tab = 'claimable' | 'assigned'

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

export default function DeliveryRiderPage() {
  const identityAllowed = useIdentityGuard('rider')
  const [tab, setTab] = useState<Tab>('claimable')
  const [me, setMe] = useState<DeliveryRiderMe | null>(null)
  const [claimable, setClaimable] = useState<ClaimableDeliveryOrder[]>([])
  const [assigned, setAssigned] = useState<AssignedDeliveryOrder[]>([])
  const [loading, setLoading] = useState(false)
  const [busyOrderId, setBusyOrderId] = useState('')
  const [error, setError] = useState('')

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
      setTab('assigned')
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

  const refreshOrders = async () => {
    const [claimablePage, assignedPage] = await Promise.all([
      listClaimableDeliveryOrders(),
      listAssignedDeliveryOrders()
    ])
    setClaimable(claimablePage.items)
    setAssigned(assignedPage.items)
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

  if (!identityAllowed) return <View className='page rider-page' />

  if (!me || loading && !me) {
    return <View className='page rider-page'><View className='card rider-state'>正在加载骑手工作台…</View></View>
  }

  if (!me.registered || !me.rider) {
    return (
      <View className='page rider-page'>
        <View className='card rider-state'>
          <Text className='rider-state__title'>骑手权限尚未开通</Text>
          <Text className='rider-muted'>{me.message || '请联系管理员完成骑手身份审核。'}</Text>
          <Button className='cc-button cc-button--primary cc-button--block' onClick={() => void switchToStudent()}>返回学生端</Button>
        </View>
      </View>
    )
  }

  const rider = me.rider

  return (
    <View className='page rider-page'>
      <View className='rider-hero'>
        <View>
          <Text className='rider-hero__title'>{rider.realName || '校园骑手'}</Text>
          <Text className='rider-hero__meta'>状态：{rider.status === 'active' ? '可接单' : rider.status === 'resting' ? '休息中' : '已停用'}</Text>
        </View>
        <Button className='cc-button cc-button--small' disabled={loading} onClick={() => void load()}>{loading ? '刷新中' : '刷新'}</Button>
      </View>

      <View className='rider-note'>收入结算由平台后台处理；当前小程序不展示虚假余额，也不提供未接通的提现入口。</View>
      {error ? <View className='card rider-error' onClick={() => void load()}>{error}，点击重试</View> : null}

      <View className='rider-tabs'>
        <Text className={tab === 'claimable' ? 'rider-tabs__active' : ''} onClick={() => setTab('claimable')}>可抢订单（{claimable.length}）</Text>
        <Text className={tab === 'assigned' ? 'rider-tabs__active' : ''} onClick={() => setTab('assigned')}>我的订单（{assigned.length}）</Text>
      </View>

      {tab === 'claimable' ? (
        claimable.length === 0 ? <View className='card rider-state'>当前暂无可抢配送单</View> : claimable.map((order) => (
          <View key={order.deliveryOrderPublicId} className='card rider-order'>
            <View className='rider-between'>
              <Text className='rider-order__title'>{order.routeSummary || '校园配送单'}</Text>
              <Text className='rider-fee'>¥{centsToYuan(order.deliveryFeeCents)}</Text>
            </View>
            <Text className='rider-muted'>{formatTime(order.createdAt)} · {order.deliveryOrderPublicId.slice(-8)}</Text>
            <Button className='cc-button cc-button--primary cc-button--block' disabled={!!busyOrderId || rider.status !== 'active'} onClick={() => void claim(order)}>
              {busyOrderId === order.deliveryOrderPublicId ? '接单中…' : '确认接单'}
            </Button>
          </View>
        ))
      ) : (
        assigned.length === 0 ? <View className='card rider-state'>暂无已接配送单</View> : assigned.map((order) => renderAssigned(order))
      )}
    </View>
  )

  function renderAssigned(order: AssignedDeliveryOrder) {
    const busy = busyOrderId === order.deliveryOrderPublicId
    const releasable = order.status === 'claimed' || order.status === 'to_pickup'
    return (
      <View key={order.deliveryOrderPublicId} className='card rider-order'>
        <View className='rider-between'>
          <Text className='rider-order__title'>{statusText[order.status]}</Text>
          <Text className='rider-fee'>¥{centsToYuan(order.deliveryFeeCents)}</Text>
        </View>
        <Text className='rider-line'>取货：{order.pickupContactName} {order.pickupContactPhone}</Text>
        <Text className='rider-line'>{order.pickupAddress}</Text>
        <Text className='rider-line'>送达：{order.recipientName} {order.recipientPhone}</Text>
        <Text className='rider-line'>{order.deliveryAddress}</Text>
        {order.remark ? <Text className='rider-line'>备注：{order.remark}</Text> : null}
        <Text className='rider-muted'>单号 {order.deliveryOrderPublicId}</Text>
        <View className='rider-actions'>
          <Button className='cc-button cc-button--small' disabled={!!busyOrderId} onClick={() => openConversation(order.deliveryOrderPublicId)}>联系商家</Button>
          <Button className='cc-button cc-button--small' onClick={() => void Taro.makePhoneCall({ phoneNumber: order.pickupContactPhone })}>联系取货人</Button>
          {order.status === 'claimed' ? <Button className='cc-button cc-button--small cc-button--primary' disabled={!!busyOrderId} onClick={() => void runCommand(order, startToPickup, '已开始前往取货')}>{busy ? '处理中…' : '前往取货'}</Button> : null}
          {order.status === 'to_pickup' ? <Button className='cc-button cc-button--small cc-button--primary' disabled={!!busyOrderId} onClick={() => void runProofCommand(order, markPickedUp, '取货凭证已提交')}>{busy ? '处理中…' : '拍照确认取货'}</Button> : null}
          {order.status === 'picked_up' ? <Button className='cc-button cc-button--small cc-button--primary' disabled={!!busyOrderId} onClick={() => void runCommand(order, startDelivery, '已开始配送')}>{busy ? '处理中…' : '开始配送'}</Button> : null}
          {order.status === 'delivering' ? <Button className='cc-button cc-button--small cc-button--primary' disabled={!!busyOrderId} onClick={() => void runProofCommand(order, markDelivered, '送达凭证已提交')}>{busy ? '处理中…' : '拍照确认送达'}</Button> : null}
          {releasable ? <Button className='cc-button cc-button--small rider-danger' disabled={!!busyOrderId} onClick={() => void release(order)}>释放订单</Button> : null}
        </View>
      </View>
    )
  }
}
