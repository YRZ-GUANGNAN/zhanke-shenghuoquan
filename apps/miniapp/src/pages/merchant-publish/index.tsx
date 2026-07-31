import React, { useEffect, useState } from 'react'
import { Input, ScrollView, Text, Textarea, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import { deliveryMockFeeEnabled, deliveryPaymentEnabled } from '../../feature-flags'
import { useIdentityGuard } from '../../hooks/use-identity-guard'
import { getIdentityApplication } from '../../services/a-user'
import {
  createDeliveryClientRequestId,
  createMerchantDeliveryOrder,
  confirmMockDeliveryFee,
  type MerchantDeliveryOrder,
} from '../../services/delivery-merchant'
import {
  getDeliveryWallet,
  payDeliveryOrderWithWallet,
  payDeliveryOrderWithWechat,
  pollDeliveryPayment,
} from '../../services/delivery-payment'
import Taro from '../../services/taro'
import { useSessionStore } from '../../stores/session'
import './index.scss'

type PaymentMethod = 'wallet' | 'wechat'

function inputValue(event: { detail: { value?: string } }) {
  return String(event.detail.value ?? '')
}

function MerchantPublishContent() {
  const testPublishEnabled = deliveryMockFeeEnabled && !deliveryPaymentEnabled
  const profile = useSessionStore((state) => state.profile)
  const identityAllowed = useIdentityGuard('merchant')
  const [pickupContact, setPickupContact] = useState(profile.nickname)
  const [pickupPhone, setPickupPhone] = useState('')
  const [pickupAddress, setPickupAddress] = useState(profile.campus || profile.school)
  const [receiverName, setReceiverName] = useState('')
  const [receiverPhone, setReceiverPhone] = useState('')
  const [receiverAddress, setReceiverAddress] = useState('')
  const [remark, setRemark] = useState('')
  const [balanceCents, setBalanceCents] = useState<number | null>(null)
  const [sheetVisible, setSheetVisible] = useState(false)
  const [paymentMethod, setPaymentMethod] = useState<PaymentMethod>('wallet')
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    if (!identityAllowed) return
    void getIdentityApplication('merchant').then((application) => {
      if (application.merchantName) setPickupContact(application.merchantName)
      if (application.merchantAddress) setPickupAddress(application.merchantAddress)
    }).catch(() => {
      // The approved identity remains authoritative; profile values are a safe fallback.
    })
    if (deliveryPaymentEnabled) {
      void getDeliveryWallet().then((wallet) => {
        setBalanceCents(wallet.availableBalanceCents)
      }).catch(() => {
        setBalanceCents(null)
      })
    }
  }, [identityAllowed])

  const validate = () => {
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

  const publishWithMockFee = async () => {
    if (submitting || !validate()) return
    setSubmitting(true)
    let order: MerchantDeliveryOrder | null = null
    try {
      order = await createMerchantDeliveryOrder({
        clientRequestId: createDeliveryClientRequestId(),
        pickupName: pickupContact.trim(),
        pickupPhone: pickupPhone.trim(),
        pickupAddress: pickupAddress.trim(),
        recipientName: receiverName.trim(),
        recipientPhone: receiverPhone.trim(),
        deliveryAddress: receiverAddress.trim(),
        remark: remark.trim(),
      })
      await confirmMockDeliveryFee(order)
      await Taro.showToast({ title: '测试配送单已发布', icon: 'success' })
      await Taro.redirectTo({
        url: `/pages/merchant-order-detail/index?orderId=${encodeURIComponent(order.deliveryOrderPublicId)}`,
      })
    } catch (error) {
      const prefix = order ? '配送单已创建，但测试费用确认失败：' : ''
      void Taro.showToast({
        title: `${prefix}${error instanceof Error ? error.message : '发布失败'}`,
        icon: 'none',
        duration: 2400,
      })
      if (order) {
        await Taro.redirectTo({
          url: `/pages/merchant-order-detail/index?orderId=${encodeURIComponent(order.deliveryOrderPublicId)}`,
        })
      }
    } finally {
      setSubmitting(false)
    }
  }

  const openPaymentSheet = () => {
    if (submitting) return
    if (testPublishEnabled) {
      void publishWithMockFee()
      return
    }
    if (!deliveryPaymentEnabled) {
      void Taro.showToast({ title: '配送支付暂未开通，当前不能发布配送单', icon: 'none' })
      return
    }
    if (!validate()) return
    setSheetVisible(true)
  }

  const finishWechatPayment = async (order: MerchantDeliveryOrder) => {
    const result = await payDeliveryOrderWithWechat(order.deliveryOrderPublicId, order.rowVersion)
    if (!result.payment) throw new Error('后端未返回支付记录')
    if (result.payment.provider === 'mock') {
      await pollDeliveryPayment(result.payment.paymentPublicId)
      return
    }
    if (!result.payment.wechat) throw new Error('后端未返回微信支付参数')
    await Taro.requestPayment(result.payment.wechat)
    const latest = await pollDeliveryPayment(result.payment.paymentPublicId)
    if (latest.status !== 'paid') throw new Error('微信支付尚未完成')
  }

  const confirmPayment = async () => {
    if (!deliveryPaymentEnabled) {
      void Taro.showToast({ title: '配送支付暂未开通', icon: 'none' })
      return
    }
    if (submitting || !validate()) return
    setSubmitting(true)
    setSheetVisible(false)
    let order: MerchantDeliveryOrder | null = null
    try {
      order = await createMerchantDeliveryOrder({
        clientRequestId: createDeliveryClientRequestId(),
        pickupName: pickupContact.trim(),
        pickupPhone: pickupPhone.trim(),
        pickupAddress: pickupAddress.trim(),
        recipientName: receiverName.trim(),
        recipientPhone: receiverPhone.trim(),
        deliveryAddress: receiverAddress.trim(),
        remark: remark.trim(),
      })
      if (paymentMethod === 'wallet') {
        await payDeliveryOrderWithWallet(order.deliveryOrderPublicId, order.rowVersion)
      } else {
        await finishWechatPayment(order)
      }
      await Taro.showToast({ title: '配送单已发布', icon: 'success' })
      await Taro.redirectTo({
        url: `/pages/merchant-order-detail/index?orderId=${encodeURIComponent(order.deliveryOrderPublicId)}`,
      })
    } catch (error) {
      const prefix = order ? '配送单已创建，但支付未完成：' : ''
      void Taro.showToast({
        title: `${prefix}${error instanceof Error ? error.message : '发布失败'}`,
        icon: 'none',
        duration: 2400,
      })
      if (order) {
        await Taro.redirectTo({
          url: `/pages/merchant-order-detail/index?orderId=${encodeURIComponent(order.deliveryOrderPublicId)}`,
        })
      }
    } finally {
      setSubmitting(false)
    }
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
        left={<View className='merchant-publish-nav__back' onClick={() => void Taro.navigateBack()}><Text>‹</Text></View>}
      />

      <ScrollView scrollY className='merchant-publish-scroll' enhanced showScrollbar={false}>
        <View className='merchant-publish-card'>
          <View className='merchant-publish-section-head'>
            <View className='merchant-publish-section-head__left'>
              <View className='merchant-publish-dot merchant-publish-dot--pickup' />
              <Text className='merchant-publish-section-head__title'>取货信息</Text>
              <Text className='merchant-publish-pill'>本店</Text>
            </View>
            <Text className='merchant-publish-section-head__hint'>来自审核资料，可修改</Text>
          </View>
          {[
            { label: '联系人', value: pickupContact, placeholder: '请输入联系人', change: setPickupContact },
            { label: '联系电话', value: pickupPhone, placeholder: '请输入联系电话', change: setPickupPhone },
            { label: '取货地址', value: pickupAddress, placeholder: '请输入取货地址', change: setPickupAddress },
          ].map((field, index) => (
            <View key={field.label} className={`merchant-publish-field ${index === 2 ? 'merchant-publish-field--last' : ''}`}>
              <Text className='merchant-publish-field__label'>{field.label}</Text>
              <Input
                className='merchant-publish-field__input'
                value={field.value}
                placeholder={field.placeholder}
                onInput={(event) => field.change(inputValue(event))}
              />
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
              <Input
                className='merchant-publish-field__input'
                value={field.value}
                placeholder={field.placeholder}
                onInput={(event) => field.change(inputValue(event))}
              />
            </View>
          ))}
          <View className='merchant-publish-textarea-wrap'>
            <Text className='merchant-publish-field__label'>备注</Text>
            <Textarea
              className='merchant-publish-textarea'
              value={remark}
              placeholder='选填配送说明'
              maxlength={500}
              onInput={(event) => setRemark(inputValue(event))}
            />
          </View>
        </View>

        <View className='merchant-publish-fee-card'>
          <View className='merchant-publish-fee-card__main'>
            <Text className='merchant-publish-fee-card__label'>配送费</Text>
            <Text className='merchant-publish-fee-card__hint'>由服务端按平台规则计算，客户端不会修改金额</Text>
          </View>
        </View>
        <View className='merchant-publish-scroll__spacer' />
      </ScrollView>

      <View className='merchant-publish-footer'>
        <View
          className={`merchant-publish-footer__button ${!deliveryPaymentEnabled && !testPublishEnabled ? 'merchant-publish-footer__button--disabled' : ''}`}
          onClick={openPaymentSheet}
        >
          <Text>
            {submitting
              ? '正在提交…'
              : testPublishEnabled
                ? '发布测试配送单'
                : !deliveryPaymentEnabled
                  ? '支付功能暂未开通'
                  : '发布并支付'}
          </Text>
        </View>
      </View>

      {sheetVisible ? (
        <View className='merchant-publish-modal'>
          <View className='merchant-publish-modal__mask' onClick={() => setSheetVisible(false)} />
          <View className='merchant-publish-sheet'>
            <View className='merchant-publish-sheet__header'>
              <View>
                <Text className='merchant-publish-sheet__title'>选择支付方式</Text>
                <Text className='merchant-publish-sheet__subtitle'>实际配送费以服务端返回为准</Text>
              </View>
              <View className='merchant-publish-sheet__close' onClick={() => setSheetVisible(false)}><Text>×</Text></View>
            </View>
            <View className='merchant-publish-payments'>
              {paymentOptions.map((option) => (
                <View
                  key={option.key}
                  className={`merchant-publish-payment ${paymentMethod === option.key ? 'merchant-publish-payment--active' : ''}`}
                  onClick={() => setPaymentMethod(option.key)}
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
            <View className='merchant-publish-confirm' onClick={() => void confirmPayment()}>
              <Text>确认发布并支付</Text>
            </View>
          </View>
        </View>
      ) : null}
    </View>
  )
}

export default function MerchantPublishPage() {
  return (
    <RuntimeSessionGate preserveAuthenticatedChildrenDuringRetry>
      <MerchantPublishContent />
    </RuntimeSessionGate>
  )
}
