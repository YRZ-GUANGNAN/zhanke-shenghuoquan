import React, { useEffect, useState } from 'react'
import { Button, Text, View } from '@tarojs/components'
import Taro from '../../services/taro'
import { centsToYuan, type DeliveryStatus, type StageTimestamps } from '../../services/delivery-merchant'
import {
  claimDeliveryOrder,
  getAssignedDeliveryOrder,
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
  type DeliveryRider
} from '../../services/delivery-rider'
import './index.scss'

type Tab = 'claimable' | 'assigned'

const statusText: Record<DeliveryStatus, string> = {
  pending_fee_payment: '待付配送费',
  payment_failed: '费用确认失败',
  ready_for_claim: '可抢',
  claimed: '已接单',
  to_pickup: '前往取货',
  picked_up: '已取货',
  delivering: '配送中',
  delivered: '已送达',
  cancelled: '已取消'
}

export default function DeliveryRiderPage() {
  const [loading, setLoading] = useState(true)
  const [rider, setRider] = useState<DeliveryRider | null>(null)
  const [unregisteredMessage, setUnregisteredMessage] = useState('')
  const [tab, setTab] = useState<Tab>('claimable')
  const [claimable, setClaimable] = useState<ClaimableDeliveryOrder[]>([])
  const [assigned, setAssigned] = useState<AssignedDeliveryOrder[]>([])
  const [detail, setDetail] = useState<AssignedDeliveryOrder | null>(null)
  const [busyOrderId, setBusyOrderId] = useState('')

  useEffect(() => {
    void loadIdentity()
  }, [])

  const loadIdentity = async () => {
    setLoading(true)
    try {
      const value = await getDeliveryRiderMe()
      if (!value.registered || !value.rider) {
        setRider(null)
        setUnregisteredMessage(value.message || '请联系管理员开通骑手权限')
        return
      }
      setRider(value.rider)
      setUnregisteredMessage('')
      if (value.rider.status === 'active') {
        await Promise.all([loadClaimable(), loadAssigned()])
      }
    } catch (error) {
      showRiderError(error, '骑手身份加载失败')
    } finally {
      setLoading(false)
    }
  }

  const loadClaimable = async () => {
    try {
      const result = await listClaimableDeliveryOrders()
      setClaimable(result.items)
    } catch (error) {
      showRiderError(error, '抢单池加载失败')
    }
  }

  const loadAssigned = async () => {
    try {
      const result = await listAssignedDeliveryOrders()
      setAssigned(result.items)
    } catch (error) {
      showRiderError(error, '我的配送加载失败')
    }
  }

  const openDetail = async (order: AssignedDeliveryOrder) => {
    setBusyOrderId(order.deliveryOrderPublicId)
    try {
      setDetail(await getAssignedDeliveryOrder(order.deliveryOrderPublicId))
    } catch (error) {
      showRiderError(error, '配送详情加载失败')
    } finally {
      setBusyOrderId('')
    }
  }

  const claim = async (order: ClaimableDeliveryOrder) => {
    setBusyOrderId(order.deliveryOrderPublicId)
    try {
      await claimDeliveryOrder(order)
      setTab('assigned')
      await Promise.all([loadClaimable(), loadAssigned()])
      await Taro.showToast({ title: '抢单成功', icon: 'success' })
    } catch (error) {
      showRiderError(error, '抢单失败')
      await Promise.all([loadClaimable(), loadAssigned()])
    } finally {
      setBusyOrderId('')
    }
  }

  const runCommand = async (
    order: AssignedDeliveryOrder,
    command: (value: AssignedDeliveryOrder) => Promise<AssignedDeliveryOrder>,
    success: string
  ) => {
    setBusyOrderId(order.deliveryOrderPublicId)
    try {
      await command(order)
      await Promise.all([loadClaimable(), loadAssigned(), refreshDetail(order.deliveryOrderPublicId)])
      await Taro.showToast({ title: success, icon: 'success' })
    } catch (error) {
      showRiderError(error, '履约操作失败')
      await Promise.all([loadClaimable(), loadAssigned(), refreshDetail(order.deliveryOrderPublicId)])
    } finally {
      setBusyOrderId('')
    }
  }

  const refreshDetail = async (orderId: string) => {
    try {
      setDetail(await getAssignedDeliveryOrder(orderId))
    } catch {
      setDetail(null)
    }
  }

  return (
    <View className='page delivery-rider-page'>
      <View className='delivery-hero delivery-rider-hero'>
        <Text className='delivery-hero__title'>配送骑手中心</Text>
        <Text className='delivery-hero__desc'>抢单池只展示最低必要信息；抢单成功后才能查看完整取货与送货信息。</Text>
      </View>

      {loading && <View className='card delivery-card'>正在核验骑手身份...</View>}
      {!loading && !rider && (
        <View className='card delivery-card delivery-rider-empty'>
          <Text className='delivery-section-title'>尚未开通配送骑手</Text>
          <Text className='delivery-line'>{unregisteredMessage || '请联系管理员开通骑手权限'}</Text>
          <Button className='cc-button cc-button--primary cc-button--block' onClick={() => void loadIdentity()}>重新检查</Button>
        </View>
      )}
      {!loading && rider && renderRiderHome(rider)}
      {detail && renderAssignedDetail(detail)}
    </View>
  )

  function renderRiderHome(value: DeliveryRider) {
    const active = value.status === 'active'
    return (
      <>
        <View className='card delivery-card'>
          <View className='between'>
            <View>
              <Text className='delivery-item-title'>{value.realName}</Text>
              <Text className='delivery-hint'>骑手 ID {value.riderUserId}</Text>
            </View>
            <Text className={`delivery-status delivery-status--${active ? 'delivered' : 'cancelled'}`}>
              {active ? '已开通' : value.status}
            </Text>
          </View>
          {!active && <Text className='delivery-line'>当前状态不可抢单或推进履约，请联系管理员。</Text>}
        </View>

        {active && <>
          <View className='delivery-tabs'>
            <Text className={tab === 'claimable' ? 'delivery-tabs__active' : ''} onClick={() => { setTab('claimable'); void loadClaimable() }}>
              抢单池
            </Text>
            <Text className={tab === 'assigned' ? 'delivery-tabs__active' : ''} onClick={() => { setTab('assigned'); void loadAssigned() }}>
              我的配送
            </Text>
          </View>
          {tab === 'claimable' ? renderClaimable() : renderAssigned()}
        </>}
      </>
    )
  }

  function renderClaimable() {
    return (
      <>
        <View className='delivery-toolbar'>
          <Text className='delivery-hint'>抢单前不展示联系人、电话和详细地址</Text>
          <Button className='cc-button cc-button--small' onClick={() => void loadClaimable()}>刷新</Button>
        </View>
        {claimable.length === 0 && <View className='card delivery-card'>暂无可抢配送单</View>}
        {claimable.map((order) => (
          <View key={order.deliveryOrderPublicId} className='card delivery-card'>
            <View className='between'>
              <View>
                <Text className='delivery-item-title'>{order.routeSummary}</Text>
                <Text className='delivery-hint'>{shortId(order.deliveryOrderPublicId)} · {formatTime(order.createdAt)}</Text>
              </View>
              <Text className='delivery-status'>配送费 ¥{centsToYuan(order.deliveryFeeCents)}</Text>
            </View>
            <Button
              className='cc-button cc-button--primary cc-button--block delivery-rider-action'
              disabled={!!busyOrderId}
              onClick={() => void claim(order)}
            >
              {busyOrderId === order.deliveryOrderPublicId ? '抢单中...' : '抢单'}
            </Button>
          </View>
        ))}
      </>
    )
  }

  function renderAssigned() {
    return (
      <>
        <View className='delivery-toolbar'>
          <Text className='delivery-hint'>只有分配给你的订单会出现在这里</Text>
          <Button className='cc-button cc-button--small' onClick={() => void loadAssigned()}>刷新</Button>
        </View>
        {assigned.length === 0 && <View className='card delivery-card'>暂无配送任务</View>}
        {assigned.map((order) => (
          <View key={order.deliveryOrderPublicId} className='card delivery-card'>
            <View className='between'>
              <View>
                <Text className='delivery-item-title'>送至 {order.deliveryAddress}</Text>
                <Text className='delivery-hint'>{shortId(order.deliveryOrderPublicId)}</Text>
              </View>
              <Text className={`delivery-status delivery-status--${order.status}`}>{statusText[order.status]}</Text>
            </View>
            <Text className='delivery-line'>取货：{order.pickupAddress}</Text>
            <Text className='delivery-line'>收货：{order.deliveryAddress}</Text>
            <View className='delivery-actions'>
              <Button className='cc-button cc-button--small' disabled={!!busyOrderId} onClick={() => void openDetail(order)}>详情</Button>
              {nextCommand(order) && (
                <Button
                  className='cc-button cc-button--small cc-button--primary'
                  disabled={!!busyOrderId}
                  onClick={() => {
                    const command = nextCommand(order)
                    if (command) void runCommand(order, command.run, command.label)
                  }}
                >
                  {nextCommand(order)?.label}
                </Button>
              )}
              {(order.status === 'claimed' || order.status === 'to_pickup') && (
                <Button
                  className='cc-button cc-button--small delivery-danger-button'
                  disabled={!!busyOrderId}
                  onClick={() => void runCommand(order, releaseDeliveryOrder, '配送单已释放回抢单池')}
                >
                  释放回池
                </Button>
              )}
            </View>
          </View>
        ))}
      </>
    )
  }

  function renderAssignedDetail(order: AssignedDeliveryOrder) {
    return (
      <View className='card delivery-card delivery-detail'>
        <View className='between'>
          <Text className='delivery-section-title'>配送任务详情</Text>
          <Text onClick={() => setDetail(null)}>关闭</Text>
        </View>
        <Text className='delivery-line'>取货联系人：{order.pickupContactName} {order.pickupContactPhone}</Text>
        <Text className='delivery-line'>取货地址：{order.pickupAddress}</Text>
        <Text className='delivery-line'>收货人：{order.recipientName} {order.recipientPhone}</Text>
        <Text className='delivery-line'>送货地址：{order.deliveryAddress}</Text>
        {order.remark && <Text className='delivery-line'>备注：{order.remark}</Text>}
        {renderTimeline(order)}
      </View>
    )
  }
}

function nextCommand(order: AssignedDeliveryOrder): {
  label: string
  run: (value: AssignedDeliveryOrder) => Promise<AssignedDeliveryOrder>
} | null {
  if (order.status === 'claimed') return { label: '前往取货', run: startToPickup }
  if (order.status === 'to_pickup') return { label: '确认已取货', run: markPickedUp }
  if (order.status === 'picked_up') return { label: '开始配送', run: startDelivery }
  if (order.status === 'delivering') return { label: '确认送达', run: markDelivered }
  return null
}

const stages: Array<{ key: keyof StageTimestamps; label: string }> = [
  { key: 'readyForClaimAt', label: '进入抢单池' },
  { key: 'claimedAt', label: '骑手已接单' },
  { key: 'toPickupAt', label: '前往取货' },
  { key: 'pickedUpAt', label: '已取货' },
  { key: 'deliveringAt', label: '配送中' },
  { key: 'deliveredAt', label: '已送达' },
  { key: 'paymentFailedAt', label: '费用确认失败' },
  { key: 'cancelledAt', label: '已取消' }
]

function renderTimeline(order: AssignedDeliveryOrder) {
  return (
    <View className='delivery-timeline'>
      <Text className='delivery-section-title'>阶段时间线</Text>
      <View className='delivery-timeline__item delivery-timeline__item--done'>
        <Text>配送单已创建</Text><Text>{formatTime(order.createdAt)}</Text>
      </View>
      {stages.map((stage) => {
        const timestamp = order[stage.key]
        return (
          <View key={stage.key} className={`delivery-timeline__item ${timestamp ? 'delivery-timeline__item--done' : ''}`}>
            <Text>{stage.label}</Text><Text>{timestamp ? formatTime(timestamp) : '未发生'}</Text>
          </View>
        )
      })}
    </View>
  )
}

function showRiderError(error: unknown, fallback: string) {
  const message = error instanceof Error ? error.message : fallback
  let friendly = message || fallback
  if (/concurrent|invalid transition|rider mismatch|already claimed/i.test(message)) {
    friendly = '该配送单已被抢或状态已变化，已刷新最新数据'
  } else if (/not registered|联系管理员/i.test(message)) {
    friendly = '请联系管理员开通骑手权限'
  } else if (/not active|unavailable|suspended/i.test(message)) {
    friendly = '当前骑手状态不可操作'
  } else if (/not found/i.test(message)) {
    friendly = '配送单不存在或不属于当前骑手'
  }
  void Taro.showToast({ title: friendly, icon: 'none' })
}

function formatTime(value: string): string {
  return value ? value.replace('T', ' ').slice(0, 19) : '-'
}

function shortId(value: string): string {
  return value.length > 12 ? `${value.slice(0, 6)}...${value.slice(-4)}` : value
}
