import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Image, ScrollView, Text, View } from '@tarojs/components'
import { useDidHide, useDidShow } from '@tarojs/taro'
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
import { useSessionStore } from '../../stores/session'
import { deliveryPickupLabel, deliverySourceLabel } from '../../utils/delivery-order-source'
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
  const accountId = useSessionStore((state) => state.profile.id)
  const token = useSessionStore((state) => state.token)
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
  const [progressExpanded, setProgressExpanded] = useState(false)
  const [previewLoading, setPreviewLoading] = useState(false)
  const [previewError, setPreviewError] = useState('')
  const requestVersion = useRef(0)
  const loadingOrderId = useRef('')
  const previewVersion = useRef(0)
  const previewLock = useRef(false)
  const pageActive = useRef(true)
  const context = `${accountId}:${token}:${identityAllowed}:${orderId}`
  const contextRef = useRef(context)
  contextRef.current = context
  const isSessionCurrent = () => {
    const session = useSessionStore.getState()
    return session.profile.id === accountId && session.token === token
  }

  useDidShow(() => { pageActive.current = true })
  useDidHide(() => {
    pageActive.current = false
    previewVersion.current += 1
    previewLock.current = false
    setPreviewLoading(false)
  })

  const loadDetail = useCallback(async () => {
    if (!identityAllowed || !orderId || contextRef.current !== context || !isSessionCurrent() || loadingOrderId.current === orderId || previewLock.current) return
    loadingOrderId.current = orderId
    const version = ++requestVersion.current
    const isCurrent = () => requestVersion.current === version && contextRef.current === context && isSessionCurrent()
    setLoading(true)
    setLoadError('')
    try {
      const orderValue = await getMerchantDeliveryOrder(orderId)
      if (!isCurrent()) return
      setOrder(orderValue)
      setFundsError('')
      setProofsError('')
      const [fundsResult, proofsResult] = await Promise.all([
        deliveryPaymentEnabled ? settle(getDeliveryOrderFunds(orderId)) : null,
        settle(loadPrivateDeliveryProofs(orderId)),
      ])
      if (!isCurrent()) return
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
      if (isCurrent()) {
        setLoadError(errorMessage(error, '配送单详情加载失败'))
      }
    } finally {
      if (isCurrent()) {
        loadingOrderId.current = ''
        setLoading(false)
      }
    }
  }, [identityAllowed, orderId, context])

  useEffect(() => {
    setOrder(null)
    setFunds(null)
    setProofs([])
    setFundsLoaded(false)
    setProofsLoaded(false)
    setLoadError('')
    setFundsError('')
    setProofsError('')
    setPreviewError('')
    setPreviewLoading(false)
    setProgressExpanded(false)
    previewLock.current = false
    setLoading(false)
    if (identityAllowed) void loadDetail()
    return () => {
      requestVersion.current += 1
      previewVersion.current += 1
      previewLock.current = false
      loadingOrderId.current = ''
    }
  }, [identityAllowed, orderId, loadDetail])

  const progressSteps = useMemo(() => order ? buildProgressSteps(order) : [], [order])
  const currentStep = progressSteps.find((step) => step.state === 'current') || progressSteps[progressSteps.length - 1]
  const visibleSteps = progressExpanded ? progressSteps : currentStep ? [currentStep] : []
  const availableProofs = proofs.filter((proof) => proof.status === 'active' && proof.url)

  const previewProofs = async (proofId?: string) => {
    if (!identityAllowed || !order || contextRef.current !== context || !isSessionCurrent() || !pageActive.current || previewLock.current || loadingOrderId.current) return
    const version = ++previewVersion.current
    const activeContext = contextRef.current
    const isCurrent = () => pageActive.current && previewVersion.current === version && contextRef.current === activeContext && isSessionCurrent()
    previewLock.current = true
    setPreviewLoading(true)
    setPreviewError('')
    try {
      // Resolve privately on every view; cached signed links may have expired or been revoked.
      const latest = await loadPrivateDeliveryProofs(order.deliveryOrderPublicId)
      if (!isCurrent()) return
      setProofs(latest)
      setProofsLoaded(true)
      setProofsError('')
      const available = latest.filter((proof) => proof.status === 'active' && proof.url && (!proof.expiresAt || proof.expiresAt > Date.now()))
      const selected = proofId ? available.find((proof) => proof.proofPublicId === proofId) : available[0]
      if (!selected?.url) {
        setPreviewError('这张凭证暂不可查看，照片状态已更新')
        return
      }
      await Taro.previewImage({ urls: available.map((proof) => proof.url!), current: selected.url })
    } catch (error) {
      if (isCurrent()) setPreviewError(errorMessage(error, '照片打开失败，请重试'))
    } finally {
      if (isCurrent()) {
        previewLock.current = false
        setPreviewLoading(false)
      }
    }
  }
  const paymentLabel = !deliveryPaymentEnabled
    ? '支付功能未开放'
    : fundsError
      ? '支付状态暂未更新'
      : !fundsLoaded ? '正在加载支付状态…' : deliveryFundsLabel(funds)

  const openConversation = () => {
    if (!identityAllowed || !pageActive.current || contextRef.current !== context || !isSessionCurrent()) return
    if (!order?.riderUserId) {
      void Taro.showToast({ title: '骑手接单后才能进入私聊', icon: 'none' })
      return
    }
    void Taro.navigateTo({
      url: `/pages/delivery-conversation/index?orderId=${encodeURIComponent(order.deliveryOrderPublicId)}&role=merchant`,
    }).catch(() => { void Taro.showToast({ title: '会话打开失败，请重试', icon: 'none' }).catch(() => undefined) })
  }

  if (!identityAllowed) return <View className='merchant-order-detail-page' />

  return (
    <View className='merchant-order-detail-page'>
      <CapsulePageHeader
        className='merchant-detail-nav'
        title='配送单详情'
        left={<AppIconButton name='back' label='返回' className='merchant-detail-nav__back' onClick={() => void Taro.navigateBack()} />}
        right={<View className='merchant-detail-nav__refresh' ariaRole='button' ariaLabel='刷新配送单详情' hoverClass='merchant-detail-control--pressed' onClick={() => void loadDetail()}><Text>刷新</Text></View>}
      />

      <ScrollView scrollY className='merchant-detail-scroll' enhanced showScrollbar={false}>
        <View className={`merchant-detail-content${order?.riderUserId ? ' merchant-detail-content--with-actions' : ''}`}>
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
              <View className={`merchant-detail-status-card merchant-detail-status-card--${order.status}`}>
                <View className='merchant-detail-order-identity'>
                  <Text className='merchant-detail-order-identity__source'>{deliverySourceLabel(order.sourcePlatform)}</Text>
                  <Text className={`merchant-detail-order-identity__code${order.pickupCode ? '' : ' merchant-detail-order-identity__code--fallback'}${(order.pickupCode?.length || 0) > 7 ? ' merchant-detail-order-identity__code--long' : ''}`}>
                    {order.pickupCode?.trim() ? <>{deliveryPickupLabel(order).slice(0, -1)}<Text className='merchant-detail-order-identity__unit'>号</Text></> : deliveryPickupLabel(order)}
                  </Text>
                  {order.pickupCode ? <Text className='merchant-detail-order-identity__caption'>取货时核对平台与取餐号</Text> : null}
                </View>
                <View className='merchant-detail-status-card__main'>
                  <Text className='merchant-detail-status-card__eyebrow'>当前配送状态</Text>
                  <Text className='merchant-detail-status-card__state'>{statusCopy[order.status].label}</Text>
                  <Text className='merchant-detail-status-card__sub'>{statusCopy[order.status].sub}</Text>
                </View>
                <View className='merchant-detail-status-card__meta'>
                  <Text>配送单 #{order.deliveryOrderPublicId}</Text>
                  <Text>创建于 {formatTime(order.createdAt)}</Text>
                </View>
              </View>

              <View className='merchant-detail-route-card'>
                <Text className='merchant-detail-section-title'>取送信息</Text>
                <View className='merchant-detail-route__row'>
                  <Text className='merchant-detail-route__marker merchant-detail-route__marker--pickup'>取</Text>
                  <View className='merchant-detail-route__body'>
                    <Text className='merchant-detail-route__address'>{order.pickupAddress}</Text>
                    <Text className='merchant-detail-route__contact'>{[order.pickupContactName, order.pickupContactPhone].filter(Boolean).join(' · ')}</Text>
                  </View>
                </View>
                <View className='merchant-detail-route__row'>
                  <Text className='merchant-detail-route__marker merchant-detail-route__marker--dropoff'>送</Text>
                  <View className='merchant-detail-route__body'>
                    <Text className='merchant-detail-route__address'>{order.deliveryAddress}</Text>
                    <Text className='merchant-detail-route__contact'>{[order.recipientName, order.recipientPhone].filter(Boolean).join(' · ')}</Text>
                  </View>
                </View>
                {order.remark ? (
                  <View className='merchant-detail-route__remark'>
                    <Text className='merchant-detail-route__remark-label'>备注</Text>
                    <Text>{order.remark}</Text>
                  </View>
                ) : null}
              </View>

              <View className='merchant-detail-fee-card'>
                <View className='merchant-detail-status-card__fee'>
                  <Text className='merchant-detail-section-title'>配送费用</Text>
                  <Text className='merchant-detail-status-card__amount'>¥{centsToYuan(order.deliveryFeeCents)}</Text>
                </View>
                <Text className='merchant-detail-status-card__pay'>{paymentLabel}</Text>
                {!loading && fundsError ? (
                  <View className='merchant-detail-status-card__funds-error' ariaRole='button' ariaLabel='重新加载支付状态' hoverClass='merchant-detail-control--pressed' onClick={() => void loadDetail()}>
                    <Text>{fundsError}{fundsLoaded ? `，上次状态：${deliveryFundsLabel(funds)}` : ''}</Text>
                    <Text className='merchant-detail-notice__retry'>点击重试</Text>
                  </View>
                ) : null}
              </View>

              <View className='merchant-detail-progress-card'>
                <View className='merchant-detail-section-head'>
                  <Text className='merchant-detail-section-title'>配送进度</Text>
                  {progressSteps.length > 1 ? <View className='merchant-detail-section-toggle' ariaRole='button'
                    ariaLabel={progressExpanded ? '收起配送进度' : '展开全部配送进度'} hoverClass='merchant-detail-control--pressed'
                    onClick={() => setProgressExpanded((expanded) => !expanded)}>
                    <Text>{progressExpanded ? '收起' : '全部记录'}</Text><View className={`merchant-detail-chevron${progressExpanded ? ' merchant-detail-chevron--up' : ''}`} />
                  </View> : null}
                </View>
                <View className={`merchant-detail-timeline${progressExpanded ? ' merchant-detail-timeline--expanded' : ''}`}>
                  {visibleSteps.map((step, index) => (
                    <View key={step.key} className='merchant-detail-timeline__row'>
                      <View className='merchant-detail-timeline__rail'>
                        <View className={`merchant-detail-timeline__marker merchant-detail-timeline__marker--${step.state}`}>
                          {step.state === 'done' ? <Text>✓</Text> : null}
                        </View>
                        {index < visibleSteps.length - 1 ? (
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
                  {availableProofs.length ? <View className='merchant-detail-photo-card__view merchant-detail-section-toggle' ariaRole='button'
                    ariaLabel='查看配送照片' hoverClass='merchant-detail-control--pressed' onClick={() => void previewProofs()}>
                    <Text>{previewLoading ? '正在打开…' : `查看 ${availableProofs.length} 张`}</Text>
                  </View> : null}
                </View>
                <Text className='merchant-detail-photo-card__privacy'>仅本单商家与骑手可见</Text>
                {previewError ? <Text className='merchant-detail-photo-card__preview-error'>{previewError}</Text> : null}
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
                      <View className={`merchant-detail-photo-item__thumb ${proof.status === 'active' && proof.url ? 'merchant-detail-photo-item__thumb--ready' : 'merchant-detail-photo-item__thumb--pending'}`}
                        ariaRole={proof.status === 'active' && proof.url ? 'button' : undefined}
                        ariaLabel={proof.status === 'active' && proof.url ? `查看${proof.proofType === 'pickup' ? '取货' : '送达'}凭证` : undefined}
                        hoverClass={proof.status === 'active' && proof.url ? 'merchant-detail-control--pressed' : 'none'}
                        onClick={proof.status === 'active' && proof.url ? () => void previewProofs(proof.proofPublicId) : undefined}>
                        {proof.status === 'active' && proof.url ? (
                          <Image className='merchant-detail-photo-item__image' src={proof.url} mode='aspectFill' />
                        ) : (
                          <Text>{proofStatusLabel(proof)}</Text>
                        )}
                      </View>
                      <Text className={`merchant-detail-photo-item__caption ${proof.status === 'active' && proof.url ? 'merchant-detail-photo-item__caption--ready' : 'merchant-detail-photo-item__caption--pending'}`}>
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
      {order?.riderUserId ? <View className='merchant-detail-action-bar'>
        <View className='merchant-detail-action-bar__identity'>
          <Text className='merchant-detail-action-bar__name'>骑手 #{order.riderUserId}</Text>
          <Text className='merchant-detail-action-bar__hint'>本单配送联系</Text>
        </View>
        <View className='merchant-detail-action-bar__contact' hoverClass='merchant-detail-control--pressed'
          ariaRole='button' ariaLabel='联系骑手' onClick={openConversation}>
          <MerchantHomeIcon name='message' className='merchant-detail-rider-card__message-icon' /><Text>联系骑手</Text>
        </View>
      </View> : null}
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
