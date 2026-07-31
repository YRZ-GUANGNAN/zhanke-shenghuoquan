import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { Image, ScrollView, Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import { AppIconButton } from '../../components/ui'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import { deliveryPaymentEnabled } from '../../feature-flags'
import { useIdentityGuard } from '../../hooks/use-identity-guard'
import {
  centsToYuan,
  getMerchantDeliveryOrder,
  type DeliveryStatus,
  type MerchantDeliveryOrder,
  type StageTimestamps,
} from '../../services/delivery-merchant'
import { getDeliveryOrderFunds, type DeliveryOrderFunds } from '../../services/delivery-payment'
import { loadPrivateDeliveryProofs, type DeliveryProofImage } from '../../services/delivery-proof'
import Taro from '../../services/taro'
import './index.scss'

type StepState = 'done' | 'current' | 'todo'
type Settled<T> = { ok: true; value: T } | { ok: false }

async function settle<T>(promise: Promise<T>): Promise<Settled<T>> {
  try {
    return { ok: true, value: await promise }
  } catch {
    return { ok: false }
  }
}

interface ProgressStep {
  key: string
  title: string
  note?: string
  time?: string
  state: StepState
}

const statusCopy: Record<DeliveryStatus, { label: string; sub: string }> = {
  pending_fee_payment: { label: '待支付', sub: '配送单已创建，等待支付配送费' },
  payment_failed: { label: '支付失败', sub: '支付未完成，可返回商家工作台重试' },
  ready_for_claim: { label: '待接单', sub: '配送费已支付，正在等待骑手接单' },
  claimed: { label: '已接单', sub: '骑手已接单，准备前往取货点' },
  to_pickup: { label: '前往取货', sub: '骑手正在前往取货点' },
  picked_up: { label: '已取货', sub: '骑手已完成取货' },
  delivering: { label: '配送中', sub: '骑手正在前往送达点' },
  delivered: { label: '已完成', sub: '配送单已送达并完成' },
  cancelled: { label: '已取消', sub: '配送单已取消' },
}

const stepTemplates: Array<{ key: keyof StageTimestamps | 'createdAt'; title: string; note: string }> = [
  { key: 'createdAt', title: '创建配送单', note: '配送信息已提交' },
  { key: 'readyForClaimAt', title: '进入抢单池', note: '配送费已确认' },
  { key: 'claimedAt', title: '骑手接单', note: '已建立配送关系' },
  { key: 'toPickupAt', title: '前往取货', note: '骑手前往取货点' },
  { key: 'pickedUpAt', title: '完成取货', note: '骑手已取到物品' },
  { key: 'deliveringAt', title: '配送中', note: '骑手前往送达点' },
  { key: 'deliveredAt', title: '完成送达', note: '本单配送完成' },
]

function formatTime(value?: string | null) {
  return value ? value.replace('T', ' ').slice(0, 16) : ''
}

function buildProgressSteps(order: MerchantDeliveryOrder): ProgressStep[] {
  const timestamps = stepTemplates.map((step) => (
    step.key === 'createdAt' ? order.createdAt : order[step.key]
  ))
  const currentIndex = Math.min(
    timestamps.findIndex((value) => !value),
    stepTemplates.length - 1,
  )
  return stepTemplates.map((step, index) => {
    const timestamp = timestamps[index]
    return {
      ...step,
      time: formatTime(timestamp),
      state: timestamp ? 'done' : index === (currentIndex < 0 ? stepTemplates.length - 1 : currentIndex) ? 'current' : 'todo',
    }
  })
}

function MerchantOrderDetailContent() {
  const identityAllowed = useIdentityGuard('merchant')
  const orderId = String(Taro.getCurrentInstance().router?.params?.orderId || '')
  const [order, setOrder] = useState<MerchantDeliveryOrder | null>(null)
  const [funds, setFunds] = useState<DeliveryOrderFunds | null>(null)
  const [proofs, setProofs] = useState<DeliveryProofImage[]>([])
  const [loading, setLoading] = useState(false)
  const [loadError, setLoadError] = useState('')

  const loadDetail = useCallback(async () => {
    if (!identityAllowed || !orderId || loading) return
    setLoading(true)
    setLoadError('')
    try {
      const orderValue = await getMerchantDeliveryOrder(orderId)
      setOrder(orderValue)
      const [fundsResult, proofsResult] = await Promise.all([
        deliveryPaymentEnabled ? settle(getDeliveryOrderFunds(orderId)) : null,
        settle(loadPrivateDeliveryProofs(orderId)),
      ])
      setFunds(fundsResult?.ok ? fundsResult.value : null)
      setProofs(proofsResult.ok ? proofsResult.value : [])
    } catch (error) {
      setOrder(null)
      setFunds(null)
      setProofs([])
      setLoadError(error instanceof Error ? error.message : '配送单详情加载失败')
    } finally {
      setLoading(false)
    }
  }, [identityAllowed, loading, orderId])

  useEffect(() => {
    if (identityAllowed) void loadDetail()
  }, [identityAllowed, orderId]) // eslint-disable-line react-hooks/exhaustive-deps

  const progressSteps = useMemo(() => order ? buildProgressSteps(order) : [], [order])

  const openConversation = () => {
    if (!order?.riderUserId) {
      void Taro.showToast({ title: '骑手接单后才能进入私聊', icon: 'none' })
      return
    }
    void Taro.navigateTo({
      url: `/pages/delivery-conversation/index?orderId=${encodeURIComponent(order.deliveryOrderPublicId)}&role=merchant`,
    })
  }

  if (!identityAllowed) return <View className='merchant-order-detail-page' />

  return (
    <View className='merchant-order-detail-page'>
      <CapsulePageHeader
        className='merchant-detail-nav'
        title='配送单详情'
        left={<AppIconButton name='back' label='返回' className='merchant-detail-nav__back' onClick={() => void Taro.navigateBack()} />}
        right={<Text className='merchant-detail-nav__refresh' onClick={() => void loadDetail()}>刷新</Text>}
      />

      <ScrollView scrollY className='merchant-detail-scroll' enhanced showScrollbar={false}>
        <View className='merchant-detail-content'>
          {loading ? <View className='merchant-detail-status-card'><Text>正在加载真实配送单…</Text></View> : null}
          {!loading && loadError ? (
            <View className='merchant-detail-status-card' onClick={() => void loadDetail()}>
              <Text>{loadError}，点击重试</Text>
            </View>
          ) : null}
          {!loading && !loadError && !order ? (
            <View className='merchant-detail-status-card'><Text>缺少有效配送单号</Text></View>
          ) : null}

          {order ? (
            <>
              <View className='merchant-detail-status-card'>
                <View className='merchant-detail-status-card__main'>
                  <View className='merchant-detail-status-card__state-row'>
                    <Text className='merchant-detail-status-card__state'>{statusCopy[order.status].label}</Text>
                    <View className='merchant-detail-status-card__dot' />
                  </View>
                  <Text className='merchant-detail-status-card__sub'>{statusCopy[order.status].sub}</Text>
                </View>
                <View className='merchant-detail-status-card__fee'>
                  <Text className='merchant-detail-status-card__amount'>¥{centsToYuan(order.deliveryFeeCents)}</Text>
                  <Text className='merchant-detail-status-card__pay'>
                    {funds?.settlement ? '配送费已支付' : '配送费未支付'}
                  </Text>
                </View>
                <View className='merchant-detail-status-card__divider' />
                <View className='merchant-detail-status-card__meta'>
                  <Text>#{order.deliveryOrderPublicId} · {formatTime(order.createdAt)}</Text>
                  <Text>送达点：{order.deliveryAddress}</Text>
                </View>
              </View>

              <View className='merchant-detail-rider-card'>
                <View className='merchant-detail-rider-card__avatar'><Text>骑</Text></View>
                <View className='merchant-detail-rider-card__main'>
                  <Text className='merchant-detail-rider-card__name'>
                    {order.riderUserId ? `骑手 #${order.riderUserId}` : '等待骑手接单'}
                  </Text>
                  <Text className='merchant-detail-rider-card__phone'>
                    {order.riderUserId ? '可通过本单加密私聊联系' : '接单后开放本单私聊'}
                  </Text>
                </View>
                {order.riderUserId ? (
                  <View className='merchant-detail-rider-card__actions'>
                    <View className='merchant-detail-rider-card__circle' onClick={openConversation}><Text>聊</Text></View>
                  </View>
                ) : null}
              </View>

              <View className='merchant-detail-progress-card'>
                <Text className='merchant-detail-section-title'>配送进度</Text>
                <View className='merchant-detail-timeline'>
                  {progressSteps.map((step, index) => (
                    <View key={step.key} className='merchant-detail-timeline__row'>
                      <View className='merchant-detail-timeline__rail'>
                        <View className={`merchant-detail-timeline__marker merchant-detail-timeline__marker--${step.state}`}>
                          {step.state === 'done' ? <Text>✓</Text> : null}
                        </View>
                        {index < progressSteps.length - 1 ? (
                          <View className={`merchant-detail-timeline__line merchant-detail-timeline__line--${step.state}`} />
                        ) : null}
                      </View>
                      <View className='merchant-detail-timeline__body'>
                        <View className='merchant-detail-timeline__head'>
                          <Text className={`merchant-detail-timeline__title merchant-detail-timeline__title--${step.state}`}>{step.title}</Text>
                          {step.time ? <Text className='merchant-detail-timeline__time'>{step.time}</Text> : null}
                        </View>
                        <Text className='merchant-detail-timeline__note'>{step.note}</Text>
                      </View>
                    </View>
                  ))}
                </View>
              </View>

              <View className='merchant-detail-photo-card'>
                <View className='merchant-detail-photo-card__header'>
                  <Text className='merchant-detail-section-title'>配送照片</Text>
                  <View className='merchant-detail-photo-card__privacy'><Text>仅本单商家与骑手可见</Text></View>
                </View>
                {proofs.length === 0 ? <Text className='merchant-detail-timeline__note'>骑手尚未提交取货或送达凭证</Text> : null}
                <View className='merchant-detail-photo-grid'>
                  {proofs.map((proof) => (
                    <View key={proof.proofPublicId} className='merchant-detail-photo-item'>
                      <View className={`merchant-detail-photo-item__thumb ${proof.url ? 'merchant-detail-photo-item__thumb--ready' : 'merchant-detail-photo-item__thumb--pending'}`}>
                        {proof.url ? (
                          <Image className='merchant-detail-photo-item__image' src={proof.url} mode='aspectFill' />
                        ) : (
                          <Text>{proof.status === 'binding' ? '照片处理中' : '照片暂不可用'}</Text>
                        )}
                      </View>
                      <Text className={`merchant-detail-photo-item__caption ${proof.url ? 'merchant-detail-photo-item__caption--ready' : 'merchant-detail-photo-item__caption--pending'}`}>
                        {proof.proofType === 'pickup' ? '取货凭证' : '送达凭证'}
                      </Text>
                    </View>
                  ))}
                </View>
              </View>
            </>
          ) : null}
        </View>
      </ScrollView>
    </View>
  )
}

export default function MerchantOrderDetailPage() {
  return (
    <RuntimeSessionGate preserveAuthenticatedChildrenDuringRetry>
      <MerchantOrderDetailContent />
    </RuntimeSessionGate>
  )
}
