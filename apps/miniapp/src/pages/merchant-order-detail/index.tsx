import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
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
import MerchantHomeIcon from '../merchant-home/merchant-home-icon'
import './index.scss'

type StepState = 'done' | 'current' | 'todo'
type Settled<T> = { ok: true; value: T } | { ok: false; error: unknown }

async function settle<T>(promise: Promise<T>): Promise<Settled<T>> {
  try {
    return { ok: true, value: await promise }
  } catch (error) {
    return { ok: false, error }
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

const currentStageByStatus: Record<DeliveryStatus, keyof StageTimestamps | 'createdAt'> = {
  pending_fee_payment: 'createdAt',
  payment_failed: 'createdAt',
  ready_for_claim: 'readyForClaimAt',
  claimed: 'claimedAt',
  to_pickup: 'toPickupAt',
  picked_up: 'pickedUpAt',
  delivering: 'deliveringAt',
  delivered: 'deliveredAt',
  cancelled: 'cancelledAt',
}

function formatTime(value?: string | null) {
  return value ? value.replace('T', ' ').slice(0, 16) : ''
}

export function buildProgressSteps(order: MerchantDeliveryOrder): ProgressStep[] {
  const currentIndex = stepTemplates.findIndex((step) => step.key === currentStageByStatus[order.status])
  const steps: ProgressStep[] = stepTemplates.map((step, index) => {
    const timestamp = step.key === 'createdAt' ? order.createdAt : order[step.key]
    const current = index === currentIndex && order.status !== 'delivered'
    return {
      ...step,
      time: formatTime(timestamp),
      note: current
        ? statusCopy[order.status].sub
        : timestamp
          ? step.note
          : index < currentIndex ? '暂未记录此阶段时间' : '等待进入此阶段',
      state: current ? 'current' : timestamp ? 'done' : 'todo',
    }
  })
  if (order.status === 'cancelled') {
    return order.cancelledAt
      ? [
          ...steps.filter((step) => step.time),
          { key: 'cancelledAt', title: '配送单已取消', note: '本单配送已结束', time: formatTime(order.cancelledAt), state: 'done' },
        ]
      : steps.filter((step) => step.time)
  }
  if (order.status === 'delivered') {
    return steps
      .filter((step) => step.time || step.key === 'deliveredAt')
      .map((step) => step.key === 'deliveredAt' ? { ...step, note: '本单配送完成', state: 'done' } : step)
  }
  return steps
}

export function deliveryFundsLabel(funds: DeliveryOrderFunds | null): string {
  if (!funds) return '支付状态暂不可用'
  if (funds.settlement?.status === 'refunded' || funds.refund?.status === 'succeeded') return '配送费已退还'
  if (funds.refund?.status === 'failed') return '退款失败，请查看账单'
  if (funds.settlement?.status === 'refund_pending' || funds.refund?.status === 'pending' || funds.refund?.status === 'refund_pending') return '退款处理中'
  if (funds.settlement?.status === 'paid') return '配送费已支付'
  if (funds.payment?.status === 'paid') return '支付结果待确认'
  if (funds.payment?.status === 'failed') return '配送费支付失败'
  if (funds.payment?.status === 'closed') return '支付已关闭'
  if (funds.payment) return '等待支付配送费'
  return '暂无支付记录'
}

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback
}

function proofStatusLabel(proof: DeliveryProofImage): string {
  if (proof.status === 'binding') return '照片处理中'
  if (proof.status === 'deadletter') return '照片处理失败'
  if (proof.status === 'revoked') return '照片已撤回'
  return proof.url ? '已提交' : '照片暂不可用'
}

function MerchantOrderDetailContent() {
  const identityAllowed = useIdentityGuard('merchant')
  const orderId = String(Taro.getCurrentInstance().router?.params?.orderId || '')
  const [order, setOrder] = useState<MerchantDeliveryOrder | null>(null)
  const [funds, setFunds] = useState<DeliveryOrderFunds | null>(null)
  const [proofs, setProofs] = useState<DeliveryProofImage[]>([])
  const [loading, setLoading] = useState(Boolean(orderId))
  const [loadError, setLoadError] = useState('')
  const [fundsLoaded, setFundsLoaded] = useState(false)
  const [proofsLoaded, setProofsLoaded] = useState(false)
  const [fundsError, setFundsError] = useState('')
  const [proofsError, setProofsError] = useState('')
  const requestVersion = useRef(0)
  const loadingOrderId = useRef('')

  const loadDetail = useCallback(async () => {
    if (!identityAllowed || !orderId || loadingOrderId.current === orderId) return
    loadingOrderId.current = orderId
    const version = ++requestVersion.current
    setLoading(true)
    setLoadError('')
    try {
      const orderValue = await getMerchantDeliveryOrder(orderId)
      if (requestVersion.current !== version) return
      setOrder(orderValue)
      setFundsError('')
      setProofsError('')
      const [fundsResult, proofsResult] = await Promise.all([
        deliveryPaymentEnabled ? settle(getDeliveryOrderFunds(orderId)) : null,
        settle(loadPrivateDeliveryProofs(orderId)),
      ])
      if (requestVersion.current !== version) return
      if (fundsResult?.ok) {
        setFunds(fundsResult.value)
        setFundsLoaded(true)
      } else if (fundsResult && !fundsResult.ok) {
        setFundsError(errorMessage(fundsResult.error, '支付状态加载失败'))
      }
      if (proofsResult.ok) {
        setProofs(proofsResult.value)
        setProofsLoaded(true)
      } else {
        setProofsError(errorMessage(proofsResult.error, '配送照片加载失败'))
      }
    } catch (error) {
      if (requestVersion.current === version) {
        setLoadError(errorMessage(error, '配送单详情加载失败'))
      }
    } finally {
      if (requestVersion.current === version) {
        loadingOrderId.current = ''
        setLoading(false)
      }
    }
  }, [identityAllowed, orderId])

  useEffect(() => {
    setOrder(null)
    setFunds(null)
    setProofs([])
    setFundsLoaded(false)
    setProofsLoaded(false)
    setLoading(false)
    if (identityAllowed) void loadDetail()
    return () => {
      requestVersion.current += 1
      loadingOrderId.current = ''
    }
  }, [identityAllowed, orderId, loadDetail])

  const progressSteps = useMemo(() => order ? buildProgressSteps(order) : [], [order])
  const paymentLabel = !deliveryPaymentEnabled
    ? '支付功能未开放'
    : fundsError
      ? '支付状态暂未更新'
      : !fundsLoaded ? '正在加载支付状态…' : deliveryFundsLabel(funds)

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
          {loading ? <View className='merchant-detail-notice'><Text>{order ? '正在更新配送单…' : '正在加载配送单…'}</Text></View> : null}
          {!loading && loadError ? (
            <View className='merchant-detail-notice merchant-detail-notice--error' ariaRole='button' ariaLabel='重新加载配送单' hoverClass='merchant-detail-control--pressed' onClick={() => void loadDetail()}>
              <Text>{loadError}{order ? '，当前保留上次读取的内容' : ''}</Text>
              <Text className='merchant-detail-notice__retry'>点击重试</Text>
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
                    {paymentLabel}
                  </Text>
                </View>
                {!loading && fundsError ? (
                  <View className='merchant-detail-status-card__funds-error' ariaRole='button' ariaLabel='重新加载支付状态' hoverClass='merchant-detail-control--pressed' onClick={() => void loadDetail()}>
                    <Text>{fundsError}{fundsLoaded ? `，上次状态：${deliveryFundsLabel(funds)}` : ''}</Text>
                    <Text className='merchant-detail-notice__retry'>点击重试</Text>
                  </View>
                ) : null}
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
                    <View
                      className='merchant-detail-rider-card__circle'
                      hoverClass='merchant-detail-rider-card__circle--pressed'
                      ariaRole='button'
                      ariaLabel='联系骑手'
                      onClick={openConversation}
                    >
                      <MerchantHomeIcon name='message' className='merchant-detail-rider-card__message-icon' />
                    </View>
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
                {loading ? <Text className='merchant-detail-timeline__note'>正在更新配送照片…</Text> : null}
                {!loading && proofsError ? (
                  <View className='merchant-detail-photo-card__error' ariaRole='button' ariaLabel='重新加载配送照片' hoverClass='merchant-detail-control--pressed' onClick={() => void loadDetail()}>
                    <Text>{proofsError}{proofsLoaded && proofs.length > 0 ? '，下方保留上次读取的照片' : ''}</Text>
                    <Text className='merchant-detail-notice__retry'>点击重试</Text>
                  </View>
                ) : null}
                {!loading && !proofsError && proofsLoaded && proofs.length === 0 ? <Text className='merchant-detail-timeline__note'>骑手尚未提交取货或送达凭证</Text> : null}
                <View className='merchant-detail-photo-grid'>
                  {proofs.map((proof) => (
                    <View key={proof.proofPublicId} className='merchant-detail-photo-item'>
                      <View className={`merchant-detail-photo-item__thumb ${proof.url ? 'merchant-detail-photo-item__thumb--ready' : 'merchant-detail-photo-item__thumb--pending'}`}>
                        {proof.url ? (
                          <Image className='merchant-detail-photo-item__image' src={proof.url} mode='aspectFill' />
                        ) : (
                          <Text>{proofStatusLabel(proof)}</Text>
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
