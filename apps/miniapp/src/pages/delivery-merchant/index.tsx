import React, { useEffect, useState } from 'react'
import { Button, Input, Text, Textarea, View } from '@tarojs/components'
import Taro from '../../services/taro'
import { deliveryMockFeeEnabled } from '../../feature-flags'
import {
  cancelMerchantDeliveryOrder,
  centsToYuan,
  confirmMockDeliveryFee,
  createDeliveryClientRequestId,
  createMerchantDeliveryOrder,
  getMerchantDeliveryOrder,
  listMerchantDeliveryOrders,
  type DeliveryStatus,
  type MerchantDeliveryOrder,
  type StageTimestamps
} from '../../services/delivery-merchant'
import './index.scss'

type Tab = 'create' | 'orders'

interface DeliveryForm {
  pickupName: string
  pickupPhone: string
  pickupAddress: string
  recipientName: string
  recipientPhone: string
  deliveryAddress: string
  remark: string
}

const emptyForm: DeliveryForm = {
  pickupName: '',
  pickupPhone: '',
  pickupAddress: '',
  recipientName: '',
  recipientPhone: '',
  deliveryAddress: '',
  remark: ''
}

const statusText: Record<DeliveryStatus, string> = {
  pending_fee_payment: '待付配送费',
  payment_failed: '费用确认失败',
  ready_for_claim: '待骑手抢单',
  claimed: '骑手已接单',
  to_pickup: '骑手前往取货',
  picked_up: '已取货',
  delivering: '配送中',
  delivered: '已送达',
  cancelled: '已取消'
}

const cancellable = new Set<DeliveryStatus>([
  'pending_fee_payment',
  'payment_failed',
  'ready_for_claim',
  'claimed',
  'to_pickup'
])

export default function DeliveryMerchantPage() {
  const [tab, setTab] = useState<Tab>('create')
  const [form, setForm] = useState<DeliveryForm>(emptyForm)
  const [clientRequestId, setClientRequestId] = useState(createDeliveryClientRequestId)
  const [orders, setOrders] = useState<MerchantDeliveryOrder[]>([])
  const [detail, setDetail] = useState<MerchantDeliveryOrder | null>(null)
  const [loading, setLoading] = useState(false)
  const [busyOrderId, setBusyOrderId] = useState('')

  useEffect(() => {
    void loadOrders()
  }, [])

  const loadOrders = async () => {
    setLoading(true)
    try {
      const result = await listMerchantDeliveryOrders()
      setOrders(result.items)
    } catch (error) {
      showDeliveryError(error, '配送单加载失败')
    } finally {
      setLoading(false)
    }
  }

  const openDetail = async (order: MerchantDeliveryOrder) => {
    setBusyOrderId(order.deliveryOrderPublicId)
    try {
      setDetail(await getMerchantDeliveryOrder(order.deliveryOrderPublicId))
    } catch (error) {
      showDeliveryError(error, '配送单详情加载失败')
    } finally {
      setBusyOrderId('')
    }
  }

  const submitOrder = async () => {
    const input = normalizedForm(form)
    if (!input) return
    setLoading(true)
    try {
      const created = await createMerchantDeliveryOrder({ ...input, clientRequestId })
      setForm(emptyForm)
      setClientRequestId(createDeliveryClientRequestId())
      setTab('orders')
      await loadOrders()
      await openDetail(created)
      await Taro.showToast({ title: '配送单已创建，等待费用确认', icon: 'success' })
    } catch (error) {
      showDeliveryError(error, '配送单创建失败')
    } finally {
      setLoading(false)
    }
  }

  const runOrderCommand = async (
    order: MerchantDeliveryOrder,
    command: (value: MerchantDeliveryOrder) => Promise<MerchantDeliveryOrder>,
    success: string
  ) => {
    setBusyOrderId(order.deliveryOrderPublicId)
    try {
      await command(order)
      await Promise.all([loadOrders(), refreshDetail(order.deliveryOrderPublicId)])
      await Taro.showToast({ title: success, icon: 'success' })
    } catch (error) {
      showDeliveryError(error, '操作失败')
      await Promise.all([loadOrders(), refreshDetail(order.deliveryOrderPublicId)])
    } finally {
      setBusyOrderId('')
    }
  }

  const refreshDetail = async (orderId: string) => {
    try {
      setDetail(await getMerchantDeliveryOrder(orderId))
    } catch {
      setDetail(null)
    }
  }

  return (
    <View className='page delivery-merchant-page'>
      <View className='delivery-hero'>
        <Text className='delivery-hero__title'>校园配送派单</Text>
        <Text className='delivery-hero__desc'>平台只处理配送服务。配送费由服务端计算，页面不会提交或修改金额。</Text>
      </View>

      <View className='delivery-tabs'>
        <Text className={tab === 'create' ? 'delivery-tabs__active' : ''} onClick={() => setTab('create')}>创建配送单</Text>
        <Text className={tab === 'orders' ? 'delivery-tabs__active' : ''} onClick={() => { setTab('orders'); void loadOrders() }}>
          我的配送单
        </Text>
      </View>

      {tab === 'create' ? renderCreateForm() : renderOrders()}
      {detail && renderDetail(detail)}
    </View>
  )

  function renderCreateForm() {
    return (
      <View className='card delivery-card delivery-form'>
        <Text className='delivery-section-title'>取货信息</Text>
        {renderInput('取货联系人', 'pickupName')}
        {renderInput('取货联系电话', 'pickupPhone', 'number')}
        {renderInput('取货地址', 'pickupAddress')}
        <Text className='delivery-section-title delivery-section-title--spaced'>收货信息</Text>
        {renderInput('收货人', 'recipientName')}
        {renderInput('收货联系电话', 'recipientPhone', 'number')}
        {renderInput('送货地址（楼栋/房间）', 'deliveryAddress')}
        <Textarea
          className='cc-textarea'
          value={form.remark}
          maxlength={500}
          placeholder='备注（选填）'
          onInput={(event) => setForm((value) => ({ ...value, remark: String(event.detail.value) }))}
        />
        <View className='delivery-fee-note'>
          <Text>配送费由服务端固定快照</Text>
          <Text>当前验收价格：¥1.50</Text>
        </View>
        <Button className='cc-button cc-button--primary cc-button--block' disabled={loading} onClick={() => void submitOrder()}>
          {loading ? '提交中...' : '创建配送单'}
        </Button>
      </View>
    )
  }

  function renderInput(
    placeholder: string,
    key: Exclude<keyof DeliveryForm, 'remark'>,
    type?: 'number'
  ) {
    return (
      <Input
        className='cc-field'
        value={form[key]}
        type={type}
        placeholder={placeholder}
        onInput={(event) => setForm((value) => ({ ...value, [key]: String(event.detail.value) }))}
      />
    )
  }

  function renderOrders() {
    return (
      <>
        <View className='delivery-toolbar'>
          <Text className='delivery-hint'>只展示当前登录用户创建的配送单</Text>
          <Button className='cc-button cc-button--small' disabled={loading} onClick={() => void loadOrders()}>刷新</Button>
        </View>
        {!loading && orders.length === 0 && <View className='card delivery-card'>暂无配送单</View>}
        {orders.map((order) => (
          <View key={order.deliveryOrderPublicId} className='card delivery-card'>
            <View className='between'>
              <View>
                <Text className='delivery-item-title'>送至 {order.deliveryAddress}</Text>
                <Text className='delivery-hint'>{shortId(order.deliveryOrderPublicId)} · {formatTime(order.createdAt)}</Text>
              </View>
              <Text className={`delivery-status delivery-status--${order.status}`}>{statusText[order.status]}</Text>
            </View>
            <Text className='delivery-line'>取货：{order.pickupAddress}</Text>
            <Text className='delivery-line'>配送费：¥{centsToYuan(order.deliveryFeeCents)}</Text>
            <View className='delivery-actions'>
              <Button className='cc-button cc-button--small' disabled={!!busyOrderId} onClick={() => void openDetail(order)}>查看详情</Button>
              {deliveryMockFeeEnabled && order.status === 'pending_fee_payment' && (
                <Button
                  className='cc-button cc-button--small delivery-dev-button'
                  disabled={!!busyOrderId}
                  onClick={() => void runOrderCommand(order, confirmMockDeliveryFee, '本地费用确认成功')}
                >
                  本地开发·确认费用
                </Button>
              )}
              {cancellable.has(order.status) && (
                <Button
                  className='cc-button cc-button--small delivery-danger-button'
                  disabled={!!busyOrderId}
                  onClick={() => void runOrderCommand(order, cancelMerchantDeliveryOrder, '配送单已取消')}
                >
                  取消
                </Button>
              )}
            </View>
          </View>
        ))}
      </>
    )
  }

  function renderDetail(order: MerchantDeliveryOrder) {
    return (
      <View className='card delivery-card delivery-detail'>
        <View className='between'>
          <Text className='delivery-section-title'>配送详情</Text>
          <Text onClick={() => setDetail(null)}>关闭</Text>
        </View>
        <Text className='delivery-line'>单号：{order.deliveryOrderPublicId}</Text>
        <Text className='delivery-line'>取货：{order.pickupContactName} {order.pickupContactPhone}</Text>
        <Text className='delivery-line'>{order.pickupAddress}</Text>
        <Text className='delivery-line'>收货：{order.recipientName} {order.recipientPhone}</Text>
        <Text className='delivery-line'>{order.deliveryAddress}</Text>
        {order.remark && <Text className='delivery-line'>备注：{order.remark}</Text>}
        <Text className='delivery-line'>配送费：¥{centsToYuan(order.deliveryFeeCents)}</Text>
        {renderTimeline(order)}
      </View>
    )
  }
}

function normalizedForm(form: DeliveryForm): DeliveryForm | null {
  const value: DeliveryForm = {
    pickupName: form.pickupName.trim(),
    pickupPhone: form.pickupPhone.trim(),
    pickupAddress: form.pickupAddress.trim(),
    recipientName: form.recipientName.trim(),
    recipientPhone: form.recipientPhone.trim(),
    deliveryAddress: form.deliveryAddress.trim(),
    remark: form.remark.trim()
  }
  if (!value.pickupName || !value.pickupPhone || !value.pickupAddress ||
    !value.recipientName || !value.recipientPhone || !value.deliveryAddress) {
    void Taro.showToast({ title: '请完整填写取货和收货信息', icon: 'none' })
    return null
  }
  return value
}

const stages: Array<{ key: keyof StageTimestamps; label: string }> = [
  { key: 'readyForClaimAt', label: '费用已确认，进入抢单池' },
  { key: 'claimedAt', label: '骑手已接单' },
  { key: 'toPickupAt', label: '骑手前往取货' },
  { key: 'pickedUpAt', label: '骑手已取货' },
  { key: 'deliveringAt', label: '配送中' },
  { key: 'deliveredAt', label: '已送达' },
  { key: 'paymentFailedAt', label: '费用确认失败' },
  { key: 'cancelledAt', label: '已取消' }
]

function renderTimeline(order: MerchantDeliveryOrder) {
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

function friendlyDeliveryMessage(error: unknown, fallback: string): string {
  const message = error instanceof Error ? error.message : fallback
  if (/concurrent|invalid transition|idempotency conflict|rider mismatch/i.test(message)) {
    return '配送单状态已变化，已刷新最新数据'
  }
  if (/not found/i.test(message)) return '配送单不存在或无权访问'
  if (/invalid/i.test(message)) return '请检查填写内容'
  return message || fallback
}

function showDeliveryError(error: unknown, fallback: string) {
  void Taro.showToast({ title: friendlyDeliveryMessage(error, fallback), icon: 'none' })
}

function formatTime(value: string): string {
  return value ? value.replace('T', ' ').slice(0, 19) : '-'
}

function shortId(value: string): string {
  return value.length > 12 ? `${value.slice(0, 6)}...${value.slice(-4)}` : value
}
