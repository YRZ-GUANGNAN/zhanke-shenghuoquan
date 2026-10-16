import { withFeaturePage } from '../../components/feature-page-gate'
import React, { useEffect, useRef, useState } from 'react'
import { Input, ScrollView, Text, Textarea, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import AppIcon from '../../components/app-icon'
import { AppIconButton } from '../../components/ui'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import { AppActionSheet, AppButton } from '../../components/ui'
import { deliveryMockFeeEnabled, deliveryPaymentEnabled } from '../../feature-flags'
import { useIdentityGuard } from '../../hooks/use-identity-guard'
import { getIdentityApplication } from '../../services/a-user'
import { isApiError } from '../../services/api'
import {
  createDeliveryClientRequestId,
  createMerchantDeliveryOrder,
  confirmMockDeliveryFee,
  getMerchantOrderIntake,
  type CreateMerchantDeliveryInput,
  type MerchantDeliveryOrder,
  type MerchantOrderIntake,
} from '../../services/delivery-merchant'
import {
  getDeliveryWallet,
  payDeliveryOrderWithWallet,
  payDeliveryOrderWithWechat,
  pollDeliveryPayment,
} from '../../services/delivery-payment'
import Taro from '../../services/taro'
import { useSessionStore } from '../../stores/session'
import {
  deliveryClipboardTemplate,
  parseDeliveryOrderClipboard,
  type DeliveryClipboardField,
  type DeliveryClipboardParseResult,
} from '../../utils/delivery-order-clipboard'
import { deliverySourceLabel, type SourcePlatform } from '../../utils/delivery-order-source'
import './index.scss'

type PaymentMethod = 'wallet' | 'wechat'
type PublishMethod = PaymentMethod | 'mock'
type PublishFeedback = { title: string; message: string; tone: 'pending' | 'success' | 'error' }
type PublishAttempt = { input: CreateMerchantDeliveryInput; method: PublishMethod; order?: MerchantDeliveryOrder; hasUnknownCreate?: boolean }

function errorMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) return error.message
  if (error && typeof error === 'object' && 'errMsg' in error) return String(error.errMsg || fallback)
  return fallback
}

function isDefiniteRejection(error: unknown): boolean {
  return isApiError(error) && [400, 401, 403, 404, 409, 422, 429].includes(error.status)
}

function paymentFeedback(status: string): PublishFeedback {
  if (status === 'paid') return { title: '配送费已支付', message: '配送单已发布，可查看最新配送进度。', tone: 'success' }
  if (status === 'failed') return { title: '支付失败', message: '配送单已创建，支付未完成。可查看订单核对资金状态。', tone: 'error' }
  if (status === 'closed') return { title: '支付已关闭', message: '配送单已创建，本次支付已关闭。可查看订单核对资金状态。', tone: 'error' }
  return { title: '支付结果待确认', message: '配送单已创建，暂未取得最终支付结果。请查看订单，勿重复发布或付款。', tone: 'pending' }
}

const clipboardFieldCopy: Record<DeliveryClipboardField, string> = {
  sourcePlatform: '来源平台',
  pickupCode: '取餐号',
  pickupContact: '取货联系人',
  pickupPhone: '取货电话',
  pickupAddress: '取货地址',
  receiverName: '收货人',
  receiverPhone: '收货电话',
  receiverAddress: '送货地址',
  remark: '备注',
}

const sourceOptions: SourcePlatform[] = ['local', 'taobao', 'jd', 'meituan']

function inputValue(event: { detail: { value?: string } }) {
  return String(event.detail.value ?? '')
}

function MerchantPublishContent() {
  const testPublishEnabled = deliveryMockFeeEnabled && !deliveryPaymentEnabled
  const profile = useSessionStore((state) => state.profile)
  const identityAllowed = useIdentityGuard('merchant')
  const [intakeMode, setIntakeMode] = useState<'manual' | 'external'>('manual')
  const [sourcePlatform, setSourcePlatform] = useState<SourcePlatform>('local')
  const [pickupCode, setPickupCode] = useState('')
  const [intake, setIntake] = useState<MerchantOrderIntake | null>(null)
  const [intakeLoading, setIntakeLoading] = useState(false)
  const [intakeError, setIntakeError] = useState('')
  const intakeRequestRef = useRef(0)
  const intakeModeRef = useRef(intakeMode)
  intakeModeRef.current = intakeMode
  const [pickupContact, setPickupContact] = useState(profile.nickname)
  const [pickupPhone, setPickupPhone] = useState('')
  const [pickupAddress, setPickupAddress] = useState(profile.campus || profile.school)
  const [receiverName, setReceiverName] = useState('')
  const [receiverPhone, setReceiverPhone] = useState('')
  const [receiverAddress, setReceiverAddress] = useState('')
  const [remark, setRemark] = useState('')
  const [balanceCents, setBalanceCents] = useState<number | null>(null)
  const [sheetVisible, setSheetVisible] = useState(false)
  const [clipboardSheetVisible, setClipboardSheetVisible] = useState(false)
  const [clipboardPreview, setClipboardPreview] = useState<DeliveryClipboardParseResult | null>(null)
  const [clipboardReading, setClipboardReading] = useState(false)
  const [paymentMethod, setPaymentMethod] = useState<PaymentMethod>('wallet')
  const [submitting, setSubmitting] = useState(false)
  const [operation, setOperation] = useState<'create' | 'pay' | 'navigate' | ''>('')
  const [attemptPending, setAttemptPending] = useState(false)
  const [createdOrder, setCreatedOrder] = useState<MerchantDeliveryOrder | null>(null)
  const [feedback, setFeedback] = useState<PublishFeedback | null>(null)
  const [navigationError, setNavigationError] = useState('')
  const attemptRef = useRef<PublishAttempt | null>(null)
  const submittingRef = useRef(false)
  const lifecycleRef = useRef(0)
  const activeRef = useRef(identityAllowed)
  const identityRef = useRef(identityAllowed)
  const accountRef = useRef(profile.id)
  const sessionAccountRef = useRef(profile.id)
  identityRef.current = identityAllowed
  accountRef.current = profile.id
  const formLocked = submitting || attemptPending || Boolean(createdOrder)

  const operationGuard = () => {
    const epoch = lifecycleRef.current
    const account = accountRef.current
    return () => activeRef.current && identityRef.current
      && lifecycleRef.current === epoch && accountRef.current === account
  }

  // Native payment can temporarily cover this page. Only leaving the page or
  // losing its account/identity invalidates the in-flight business operation.
  useEffect(() => {
    activeRef.current = identityAllowed
    // Authentication retry can transiently clear the profile/identity while
    // this page is preserved. Keep its idempotency key until a different actual
    // account is known; otherwise a retry can create the same delivery twice.
    const accountChanged = Boolean(profile.id && profile.id !== sessionAccountRef.current)
    if (accountChanged) {
      sessionAccountRef.current = profile.id
      attemptRef.current = null
      setAttemptPending(false)
      setCreatedOrder(null)
      setFeedback(null)
      setNavigationError('')
      setPickupContact('')
      setPickupPhone('')
      setPickupAddress('')
      setReceiverName('')
      setReceiverPhone('')
      setReceiverAddress('')
      setRemark('')
      setSourcePlatform('local')
      setPickupCode('')
      setIntakeMode('manual')
      setBalanceCents(null)
      setPaymentMethod('wallet')
      setClipboardPreview(null)
    } else if (submittingRef.current && attemptRef.current) {
      if (attemptRef.current.order) {
        setFeedback((current) => current?.tone === 'success' ? current : paymentFeedback(''))
      } else {
        attemptRef.current.hasUnknownCreate = true
        setAttemptPending(true)
        setFeedback({ title: '创建结果待确认', message: '身份核验中断了本次确认。已保留原配送信息，可继续确认结果或到工作台核对。', tone: 'pending' })
      }
    }
    setSheetVisible(false)
    setClipboardSheetVisible(false)
    setClipboardReading(false)
    submittingRef.current = false
    setSubmitting(false)
    setOperation('')
    return () => {
      activeRef.current = false
      lifecycleRef.current += 1
    }
  }, [identityAllowed, profile.id])

  const showToast = (title: string, success = false) => {
    // Toasts are optional feedback; an unavailable native toast cannot turn a
    // successful business response into a failed order or a second submission.
    try {
      void Taro.showToast({ title, icon: success ? 'success' : 'none', duration: 2400 }).catch(() => {})
    } catch {
      // Native bridges can also throw synchronously before returning a promise.
    }
  }

  const loadIntake = async () => {
    if (!activeRef.current || !identityRef.current) return
    const current = operationGuard()
    const requestId = ++intakeRequestRef.current
    setIntakeLoading(true)
    setIntakeError('')
    setIntake(null)
    try {
      const result = await getMerchantOrderIntake()
      if (current() && requestId === intakeRequestRef.current) setIntake(result)
    } catch {
      if (current() && requestId === intakeRequestRef.current) setIntakeError('暂时无法确认平台接入状态，请重试。')
    } finally {
      if (current() && requestId === intakeRequestRef.current) setIntakeLoading(false)
    }
  }

  useEffect(() => {
    setIntake(null)
    setIntakeError('')
    if (identityAllowed) void loadIntake()
  }, [identityAllowed, profile.id])

  useEffect(() => {
    if (!identityAllowed) return
    const current = operationGuard()
    void getIdentityApplication('merchant').then((application) => {
      if (!current() || attemptRef.current || submittingRef.current) return
      if (application.merchantName) setPickupContact(application.merchantName)
      if (application.merchantAddress) setPickupAddress(application.merchantAddress)
    }).catch(() => {
      // The approved identity remains authoritative; profile values are a safe fallback.
    })
    if (deliveryPaymentEnabled) {
      void getDeliveryWallet().then((wallet) => {
        if (!current()) return
        setBalanceCents(wallet.availableBalanceCents)
      }).catch(() => {
        if (current()) setBalanceCents(null)
      })
    }
  }, [identityAllowed, profile.id])

  const currentFormValues: Record<DeliveryClipboardField, string> = {
    sourcePlatform,
    pickupCode,
    pickupContact,
    pickupPhone,
    pickupAddress,
    receiverName,
    receiverPhone,
    receiverAddress,
    remark,
  }

  const copyImportTemplate = async () => {
    try {
      await Taro.setClipboardData({ data: deliveryClipboardTemplate })
      void Taro.showToast({ title: '填写模板已复制', icon: 'success' })
    } catch {
      void Taro.showToast({ title: '复制失败，请稍后重试', icon: 'none' })
    }
  }

  const readClipboard = async () => {
    if (intakeMode !== 'manual' || clipboardReading || submittingRef.current || attemptRef.current) return
    const current = operationGuard()
    setClipboardReading(true)
    try {
      const result = await Taro.getClipboardData()
      if (!current() || intakeModeRef.current !== 'manual' || submittingRef.current || attemptRef.current) return
      const source = String(result.data ?? '')
      if (!source.trim()) {
        void Taro.showToast({ title: '剪贴板没有可导入的文字', icon: 'none' })
        return
      }
      const parsed = parseDeliveryOrderClipboard(source)
      if (parsed.recognizedFields.length === 0) {
        void Taro.showToast({ title: '未识别到配送字段，请使用填写模板', icon: 'none', duration: 2400 })
        return
      }
      setClipboardPreview(parsed)
      setClipboardSheetVisible(true)
    } catch {
      if (current()) showToast('读取剪贴板失败，请检查授权后重试')
    } finally {
      if (current()) setClipboardReading(false)
    }
  }

  const applyClipboardPreview = () => {
    if (intakeMode !== 'manual' || !clipboardPreview || submittingRef.current || attemptRef.current) return
    const setters: Record<DeliveryClipboardField, (value: string) => void> = {
      sourcePlatform: (value) => { if (sourceOptions.includes(value as SourcePlatform)) setSourcePlatform(value as SourcePlatform) },
      pickupCode: setPickupCode,
      pickupContact: setPickupContact,
      pickupPhone: setPickupPhone,
      pickupAddress: setPickupAddress,
      receiverName: setReceiverName,
      receiverPhone: setReceiverPhone,
      receiverAddress: setReceiverAddress,
      remark: setRemark,
    }
    clipboardPreview.recognizedFields.forEach((field) => {
      const value = clipboardPreview.values[field]?.trim()
      if (value) setters[field](value)
    })
    setClipboardSheetVisible(false)
    void Taro.showToast({ title: `已导入 ${clipboardPreview.recognizedFields.length} 项信息`, icon: 'success' })
  }

  const validate = () => {
    if (intakeMode !== 'manual') return false
    if (intake?.manualAvailable === false) {
      showToast('自行发单暂未开放')
      return false
    }
    if (sourcePlatform !== 'local' && !pickupCode.trim()) {
      showToast('请填写该平台的取餐号')
      return false
    }
    if (pickupCode.trim().length > 16 || /[\r\n]/.test(pickupCode)) {
      showToast('取餐号请控制在 16 个字符以内')
      return false
    }
    if (!pickupContact.trim() || !pickupPhone.trim() || !pickupAddress.trim()) {
      void Taro.showToast({ title: '请完整填写取货信息', icon: 'none' })
      return false
    }
    if (!receiverName.trim() || !receiverPhone.trim() || !receiverAddress.trim()) {
      void Taro.showToast({ title: '请完整填写收货信息', icon: 'none' })
      return false
    }
    return true
  }

  const navigateToOrder = async (order: MerchantDeliveryOrder, current: () => boolean) => {
    if (!current()) return
    setOperation('navigate')
    setNavigationError('')
    try {
      await Taro.redirectTo({
        url: `/pages/merchant-order-detail/index?orderId=${encodeURIComponent(order.deliveryOrderPublicId)}`,
      })
    } catch (error) {
      if (current()) setNavigationError(errorMessage(error, '暂时无法打开订单，请重试查看'))
    }
  }

  const viewCreatedOrder = async () => {
    const order = attemptRef.current?.order
    if (!order || submittingRef.current || !activeRef.current || !identityRef.current) return
    const current = operationGuard()
    submittingRef.current = true
    setSubmitting(true)
    try {
      await navigateToOrder(order, current)
    } finally {
      if (current()) {
        submittingRef.current = false
        setSubmitting(false)
        setOperation('')
      }
    }
  }

  const openWorkbench = async () => {
    if (submittingRef.current || !activeRef.current || !identityRef.current) return
    const current = operationGuard()
    submittingRef.current = true
    setSubmitting(true)
    setOperation('navigate')
    setNavigationError('')
    try {
      await Taro.redirectTo({ url: '/pages/merchant-home/index' })
    } catch (error) {
      if (current()) setNavigationError(errorMessage(error, '暂时无法打开工作台，请重试'))
    } finally {
      if (current()) {
        submittingRef.current = false
        setSubmitting(false)
        setOperation('')
      }
    }
  }

  const finishWechatPayment = async (order: MerchantDeliveryOrder, current: () => boolean): Promise<PublishFeedback | null> => {
    const result = await payDeliveryOrderWithWechat(order.deliveryOrderPublicId, order.rowVersion)
    if (!current()) return null
    if (!result.payment) throw new Error('后端未返回支付记录')
    if (result.payment.status === 'paid' || result.payment.status === 'failed' || result.payment.status === 'closed') {
      return paymentFeedback(result.payment.status)
    }
    if (result.payment.provider !== 'mock') {
      if (!result.payment.wechat) throw new Error('后端未返回微信支付参数')
      try {
        await Taro.requestPayment(result.payment.wechat)
      } catch (error) {
        if (!current()) return null
        if (/cancel/i.test(errorMessage(error, ''))) {
          return { title: '已取消付款', message: '配送单已创建，本次未完成付款。可查看订单核对支付状态。', tone: 'pending' }
        }
        throw error
      }
      if (!current()) return null
    }
    const latest = await pollDeliveryPayment(result.payment.paymentPublicId)
    return current() ? paymentFeedback(latest.status) : null
  }

  const submitAttempt = async (method: PublishMethod) => {
    if (intakeMode !== 'manual' || submittingRef.current || !activeRef.current || !identityRef.current) return
    if (attemptRef.current?.order) {
      await viewCreatedOrder()
      return
    }
    if (!attemptRef.current && !validate()) return
    const current = operationGuard()
    submittingRef.current = true
    setSubmitting(true)
    setOperation('create')
    setSheetVisible(false)
    setClipboardSheetVisible(false)
    setNavigationError('')
    if (!attemptRef.current) {
      attemptRef.current = {
        method,
        input: {
          clientRequestId: createDeliveryClientRequestId(),
          sourcePlatform,
          pickupCode: pickupCode.trim(),
          pickupName: pickupContact.trim(),
          pickupPhone: pickupPhone.trim(),
          pickupAddress: pickupAddress.trim(),
          recipientName: receiverName.trim(),
          recipientPhone: receiverPhone.trim(),
          deliveryAddress: receiverAddress.trim(),
          remark: remark.trim(),
        },
      }
    }
    const attempt = attemptRef.current
    setAttemptPending(true)
    setFeedback({ title: '正在创建配送单', message: '配送信息已锁定，请稍候。', tone: 'pending' })
    try {
      let order: MerchantDeliveryOrder
      try {
        order = await createMerchantDeliveryOrder(attempt.input)
      } catch (error) {
        if (!current()) return
        if (isDefiniteRejection(error) && !attempt.hasUnknownCreate) {
          attemptRef.current = null
          setAttemptPending(false)
          setFeedback({ title: '配送单未创建', message: errorMessage(error, '请检查配送信息后重试'), tone: 'error' })
        } else {
          attempt.hasUnknownCreate = true
          setFeedback({
            title: '创建结果待确认',
            message: isDefiniteRejection(error)
              ? `${errorMessage(error, '暂时无法确认原请求')}。此前请求可能已创建订单，请到工作台核对，勿重新发布。`
              : '网络未返回明确结果，已保留本次信息。点击确认创建结果会使用原请求，也可到工作台核对。',
            tone: 'pending',
          })
        }
        return
      }
      if (!current()) return
      attempt.order = order
      setCreatedOrder(order)
      setAttemptPending(false)
      setFeedback({ title: '配送单已创建', message: '正在确认配送费，请勿重复发布。', tone: 'pending' })

      // A retry may return a previously accepted order that has already moved
      // forward. Never issue another payment against that returned snapshot.
      if (order.status !== 'pending_fee_payment' && order.status !== 'payment_failed') {
        setFeedback({ title: '配送单已创建', message: '此单已有更新，查看订单确认当前进度与资金状态。', tone: 'pending' })
        await navigateToOrder(order, current)
        return
      }
      setOperation('pay')
      let result: PublishFeedback | null
      try {
        if (attempt.method === 'mock') {
          if (!testPublishEnabled) throw new Error('测试配送费确认暂未开放')
          const confirmedOrder = await confirmMockDeliveryFee(order)
          if (!current()) return
          result = confirmedOrder.status === 'ready_for_claim'
            ? { title: '测试配送单已发布', message: '测试配送费已确认，可查看配送进度。', tone: 'success' }
            : { title: '测试费用结果待确认', message: '配送单已创建，请查看订单核对最新状态。', tone: 'pending' }
        } else if (attempt.method === 'wallet') {
          const payment = await payDeliveryOrderWithWallet(order.deliveryOrderPublicId, order.rowVersion)
          if (!current()) return
          result = paymentFeedback(payment.settlement?.status === 'paid' ? 'paid' : payment.payment?.status || '')
        } else {
          result = await finishWechatPayment(order, current)
        }
      } catch (error) {
        if (!current()) return
        result = isDefiniteRejection(error)
          ? { title: '配送费未完成支付', message: `${errorMessage(error, '支付请求未通过')}。配送单已保留，请查看订单。`, tone: 'error' }
          : { title: '支付结果待确认', message: '配送单已创建，暂未取得明确支付结果。请查看订单核对，勿重复付款。', tone: 'pending' }
      }
      if (!current() || !result) return
      setFeedback(result)
      if (result.tone === 'success') {
        showToast(attempt.method === 'mock' ? '测试配送单已发布' : '配送单已发布', true)
        await navigateToOrder(order, current)
      }
    } finally {
      if (current()) {
        submittingRef.current = false
        setSubmitting(false)
        setOperation('')
      }
    }
  }

  const publishWithMockFee = async () => {
    if (!testPublishEnabled) return
    await submitAttempt('mock')
  }

  const openPaymentSheet = () => {
    if (intakeMode !== 'manual' || submittingRef.current || !activeRef.current || !identityRef.current) return
    if (attemptRef.current?.order) {
      void viewCreatedOrder()
      return
    }
    if (testPublishEnabled) {
      void publishWithMockFee()
      return
    }
    if (!deliveryPaymentEnabled) {
      showToast('配送支付暂未开通，当前不能发布配送单')
      return
    }
    if (attemptRef.current) {
      void submitAttempt(attemptRef.current.method)
      return
    }
    if (!validate()) return
    setSheetVisible(true)
  }

  const confirmPayment = async () => {
    if (!deliveryPaymentEnabled) {
      showToast('配送支付暂未开通')
      return
    }
    await submitAttempt(paymentMethod)
  }

  if (!identityAllowed) return <View className='merchant-publish-page' />

  const paymentOptions: Array<{ key: PaymentMethod; title: string; desc: string; icon: string }> = [
    {
      key: 'wallet',
      title: '钱包余额',
      desc: balanceCents == null ? '余额加载失败，可返回钱包页重试' : `当前余额 ¥${(balanceCents / 100).toFixed(2)}`,
      icon: '钱',
    },
    { key: 'wechat', title: '微信支付', desc: '由微信支付确认本次配送费', icon: '微' },
  ]

  return (
    <View className='merchant-publish-page'>
      <CapsulePageHeader
        className='merchant-publish-nav'
        title='发布配送单'
        left={<AppIconButton name='back' label='返回' className='merchant-publish-nav__back' onClick={() => void Taro.navigateBack()} />}
      />

      <ScrollView scrollY className='merchant-publish-scroll' enhanced showScrollbar={false}>
        <View className='merchant-publish-intake-tabs'>
          {([{ key: 'manual', title: '自行发单' }, { key: 'external', title: '平台接入' }] as const).map((item) => (
            <View key={item.key} className={`merchant-publish-intake-tab ${intakeMode === item.key ? 'merchant-publish-intake-tab--active' : ''} ${formLocked ? 'merchant-publish-control--disabled' : ''}`} ariaRole='button' ariaLabel={item.title}
              onClick={() => { if (!submittingRef.current && !attemptRef.current) { setIntakeMode(item.key); setClipboardSheetVisible(false); setSheetVisible(false) } }}>
              <Text>{item.title}</Text>
            </View>
          ))}
        </View>

        {intakeMode === 'external' ? (
          <View className='merchant-publish-card merchant-publish-intake'>
            <Text className='merchant-publish-intake__title'>外卖订单接入</Text>
            <Text className='merchant-publish-intake__description'>连接已授权店铺后，外卖订单可由接口导入。选择订单来源不代表已经连接平台。</Text>
            {sourceOptions.filter((platform) => platform !== 'local').map((platform) => {
              const platformState = intake?.platforms.find((item) => item.platform === platform)
              return (
                <View key={platform} className='merchant-publish-intake__platform'>
                  <Text>{deliverySourceLabel(platform)}</Text>
                  <Text className='merchant-publish-intake__status'>{intakeLoading ? '正在确认' : !platformState ? '状态未确认' : platformState.connected ? '已接入' : '未接入'}</Text>
                </View>
              )
            })}
            <Text className='merchant-publish-intake__notice'>{intakeError || (intakeLoading ? '正在读取店铺接入能力…' : intake?.externalAvailable ? '已开放外部订单接入，可返回订单页查看平台订单。' : '当前尚未开放自动接单。可先选择自行发单，填写平台与取餐号安排配送。')}</Text>
            {intakeError ? <AppButton variant='secondary' block onClick={() => void loadIntake()}>重新确认接入状态</AppButton> : null}
          </View>
        ) : <>
        <View className='merchant-publish-card merchant-publish-source'>
          <View className='merchant-publish-section-head'>
            <Text className='merchant-publish-section-head__title'>订单来源</Text>
            <Text className='merchant-publish-section-head__hint'>仅标记来源，不会自动接单</Text>
          </View>
          <View className='merchant-publish-source__options'>
            {sourceOptions.map((platform) => (
              <View key={platform} className={`merchant-publish-source__option ${sourcePlatform === platform ? 'merchant-publish-source__option--active' : ''} ${formLocked ? 'merchant-publish-control--disabled' : ''}`} ariaRole='button' ariaLabel={`订单来源${deliverySourceLabel(platform)}`}
                onClick={() => { if (!submittingRef.current && !attemptRef.current) setSourcePlatform(platform) }}>
                <Text>{deliverySourceLabel(platform)}</Text>
              </View>
            ))}
          </View>
          <View className='merchant-publish-field merchant-publish-field--last'>
            <Text className='merchant-publish-field__label'>取餐号</Text>
            <Input className='merchant-publish-source__code' value={pickupCode} maxlength={16} disabled={formLocked}
              placeholder={sourcePlatform === 'local' ? '选填，如 0022 或 A15' : '必填，使用平台取餐号'}
              onInput={(event) => { if (!submittingRef.current && !attemptRef.current) setPickupCode(inputValue(event)) }} />
          </View>
          <Text className='merchant-publish-source__hint'>取餐号用于门店核对，不是平台完整订单编号。</Text>
        </View>
        <View className='merchant-publish-import-card'>
          <View className='merchant-publish-import-card__icon'>
            <AppIcon name='order' />
          </View>
          <View className='merchant-publish-import-card__main'>
            <Text className='merchant-publish-import-card__title'>导入配送信息</Text>
            <Text className='merchant-publish-import-card__description'>{formLocked ? '本次信息已锁定，请先确认订单结果' : '粘贴填写好的信息，确认后导入'}</Text>
          </View>
          <View className='merchant-publish-import-card__actions'>
            <View className='merchant-publish-import-card__secondary' onClick={() => void copyImportTemplate()}>
              <Text>复制模板</Text>
            </View>
            <View className={`merchant-publish-import-card__primary ${formLocked ? 'merchant-publish-control--disabled' : ''}`} ariaRole='button' ariaLabel='从剪贴板导入配送信息' onClick={() => void readClipboard()}>
              <Text>{clipboardReading ? '读取中…' : '从剪贴板导入'}</Text>
            </View>
          </View>
        </View>

        <View className='merchant-publish-card'>
          <View className='merchant-publish-section-head'>
            <View className='merchant-publish-section-head__left'>
              <View className='merchant-publish-dot merchant-publish-dot--pickup' />
              <Text className='merchant-publish-section-head__title'>取货信息</Text>
              <Text className='merchant-publish-pill'>本店</Text>
            </View>
            <Text className='merchant-publish-section-head__hint'>{formLocked ? '本次信息已锁定' : '来自商家资料，可修改'}</Text>
          </View>
          {[
            { label: '联系人', value: pickupContact, placeholder: '请输入联系人', change: setPickupContact },
            { label: '联系电话', value: pickupPhone, placeholder: '请输入联系电话', change: setPickupPhone },
            { label: '取货地址', value: pickupAddress, placeholder: '请输入取货地址', change: setPickupAddress },
          ].map((field, index) => (
            <View key={field.label} className={`merchant-publish-field ${index === 2 ? 'merchant-publish-field--last' : ''}`}>
              <Text className='merchant-publish-field__label'>{field.label}</Text>
              {index === 2 ? <Textarea
                className='merchant-publish-field__input merchant-publish-field__input--address'
                value={field.value}
                placeholder={field.placeholder}
                autoHeight
                maxlength={140}
                disabled={formLocked}
                onInput={(event) => { if (!submittingRef.current && !attemptRef.current) field.change(inputValue(event)) }}
              /> : <Input
                className='merchant-publish-field__input'
                value={field.value}
                placeholder={field.placeholder}
                disabled={formLocked}
                onInput={(event) => { if (!submittingRef.current && !attemptRef.current) field.change(inputValue(event)) }}
              />}
            </View>
          ))}
        </View>

        <View className='merchant-publish-card'>
          <View className='merchant-publish-section-head'>
            <View className='merchant-publish-section-head__left'>
              <View className='merchant-publish-dot merchant-publish-dot--dropoff' />
              <Text className='merchant-publish-section-head__title'>收货信息</Text>
            </View>
          </View>
          {[
            { label: '收货人', value: receiverName, placeholder: '请输入收货人姓名', change: setReceiverName },
            { label: '联系电话', value: receiverPhone, placeholder: '请输入收货人电话', change: setReceiverPhone },
            { label: '送货地址', value: receiverAddress, placeholder: '楼栋 / 房间', change: setReceiverAddress },
          ].map((field, index) => (
            <View key={field.label} className={`merchant-publish-field ${index === 2 ? 'merchant-publish-field--last' : ''}`}>
              <Text className='merchant-publish-field__label'>{field.label}</Text>
              {index === 2 ? <Textarea
                className='merchant-publish-field__input merchant-publish-field__input--address'
                value={field.value}
                placeholder={field.placeholder}
                autoHeight
                maxlength={140}
                disabled={formLocked}
                onInput={(event) => { if (!submittingRef.current && !attemptRef.current) field.change(inputValue(event)) }}
              /> : <Input
                className='merchant-publish-field__input'
                value={field.value}
                placeholder={field.placeholder}
                disabled={formLocked}
                onInput={(event) => { if (!submittingRef.current && !attemptRef.current) field.change(inputValue(event)) }}
              />}
            </View>
          ))}
          <View className='merchant-publish-textarea-wrap'>
            <Text className='merchant-publish-field__label'>备注</Text>
            <Textarea
              className='merchant-publish-textarea'
              value={remark}
              placeholder='选填配送说明'
              maxlength={500}
              disabled={formLocked}
              onInput={(event) => { if (!submittingRef.current && !attemptRef.current) setRemark(inputValue(event)) }}
            />
          </View>
        </View>

        <View className='merchant-publish-fee-card'>
          <View className='merchant-publish-fee-card__main'>
            <Text className='merchant-publish-fee-card__label'>配送费</Text>
            <Text className='merchant-publish-fee-card__hint'>按平台规则计算，以订单实际金额为准</Text>
          </View>
          <Text className='merchant-publish-fee-card__summary'>{createdOrder ? `¥${(createdOrder.deliveryFeeCents / 100).toFixed(2)}` : '创建后确认'}</Text>
        </View>
        </>}
        <View className='merchant-publish-scroll__spacer' />
      </ScrollView>

      <View className='merchant-publish-footer'>
        {intakeMode === 'external' ? <View className='merchant-publish-footer__button' ariaRole='button' ariaLabel='返回自行发单' onClick={() => setIntakeMode('manual')}><Text>返回自行发单</Text></View> : <>
        {feedback ? (
          <View className={`merchant-publish-feedback merchant-publish-feedback--${feedback.tone}`}>
            <Text className='merchant-publish-feedback__title'>{feedback.title}</Text>
            <Text className='merchant-publish-feedback__message'>{feedback.message}</Text>
            {createdOrder ? <Text className='merchant-publish-feedback__order'>订单号 {createdOrder.deliveryOrderPublicId}</Text> : null}
            {navigationError ? <Text className='merchant-publish-feedback__error'>{navigationError}，可点击下方重新查看。</Text> : null}
            {attemptPending ? <View className='merchant-publish-feedback__workbench' ariaRole='button' ariaLabel='到商家工作台核对配送单' onClick={() => void openWorkbench()}><Text>到工作台核对</Text></View> : null}
          </View>
        ) : null}
        <View
          className={`merchant-publish-footer__button ${submitting || (!deliveryPaymentEnabled && !testPublishEnabled && !createdOrder) ? 'merchant-publish-footer__button--disabled' : ''}`}
          ariaRole='button'
          ariaLabel={createdOrder ? '查看配送单' : attemptPending ? '确认创建结果' : '发布配送单'}
          onClick={openPaymentSheet}
        >
          <Text>
            {submitting
              ? operation === 'navigate' ? '正在打开订单…' : operation === 'pay' ? '正在确认付款…' : '正在创建配送单…'
              : createdOrder
                ? '查看配送单'
                : attemptPending
                  ? '确认创建结果'
                  : testPublishEnabled
                    ? '发布测试配送单'
                    : !deliveryPaymentEnabled
                      ? '支付功能暂未开通'
                      : '发布并支付'}
          </Text>
        </View>
        </>}
      </View>

      <AppActionSheet
        visible={clipboardSheetVisible}
        title='确认导入配送信息'
        description={clipboardPreview ? `识别到 ${clipboardPreview.recognizedFields.length} 项，只更新下列字段` : ''}
        onClose={() => setClipboardSheetVisible(false)}
      >
        <View className='merchant-publish-import-sheet'>
          <View className='merchant-publish-import-preview'>
            {clipboardPreview?.recognizedFields.map((field) => {
              const nextValue = clipboardPreview.values[field] ?? ''
              const currentValue = currentFormValues[field].trim()
              const state = currentValue && currentValue !== nextValue ? '更新' : currentValue === nextValue ? '保持' : '新增'
              const stateTone = state === '更新' ? 'update' : state === '保持' ? 'keep' : 'add'
              return (
                <View key={field} className='merchant-publish-import-preview__row'>
                  <View className='merchant-publish-import-preview__copy'>
                    <Text className='merchant-publish-import-preview__label'>{clipboardFieldCopy[field]}</Text>
                    <Text className='merchant-publish-import-preview__value'>{field === 'sourcePlatform' ? deliverySourceLabel(nextValue as SourcePlatform) : nextValue}</Text>
                  </View>
                  <Text className={`merchant-publish-import-preview__state merchant-publish-import-preview__state--${stateTone}`}>{state}</Text>
                </View>
              )
            })}
          </View>
          <Text className='merchant-publish-import-sheet__notice'>未识别字段保持原值，确认后仍可继续修改。</Text>
          <AppButton block onClick={applyClipboardPreview}>确认导入</AppButton>
        </View>
      </AppActionSheet>

      <AppActionSheet
        visible={sheetVisible}
        title='选择支付方式'
        description='实际配送费以服务端返回为准'
        loading={submitting}
        onClose={() => setSheetVisible(false)}
      >
        <View className='merchant-publish-sheet__content'>
            <View className='merchant-publish-payments'>
              {paymentOptions.map((option) => (
                <View
                  key={option.key}
                  className={`merchant-publish-payment ${paymentMethod === option.key ? 'merchant-publish-payment--active' : ''}`}
                  onClick={() => { if (!submittingRef.current && !attemptRef.current) setPaymentMethod(option.key) }}
                >
                  <View className={`merchant-publish-payment__icon merchant-publish-payment__icon--${option.key}`}>
                    <Text>{option.icon}</Text>
                  </View>
                  <View className='merchant-publish-payment__main'>
                    <Text className='merchant-publish-payment__title'>{option.title}</Text>
                    <Text className='merchant-publish-payment__desc'>{option.desc}</Text>
                  </View>
                  <View className='merchant-publish-payment__radio'>
                    {paymentMethod === option.key ? <View className='merchant-publish-payment__radio-dot' /> : null}
                  </View>
                </View>
              ))}
            </View>
          <AppButton block loading={submitting} onClick={() => void confirmPayment()}>确认发布并支付</AppButton>
        </View>
      </AppActionSheet>
    </View>
  )
}

function MerchantPublishPage() {
  return (
    <RuntimeSessionGate preserveAuthenticatedChildrenDuringRetry>
      <MerchantPublishContent />
    </RuntimeSessionGate>
  )
}

export default withFeaturePage(MerchantPublishPage)
