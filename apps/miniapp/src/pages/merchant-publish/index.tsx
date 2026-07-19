import React, { useState } from 'react'
import { Input, ScrollView, Text, Textarea, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import Taro from '../../services/taro'
import './index.scss'

type PaymentMethod = 'wallet' | 'wechat'

const paymentOptions: Array<{
  key: PaymentMethod
  title: string
  desc: string
  icon: string
}> = [
  {
    key: 'wallet',
    title: '钱包余额',
    desc: '余额 ¥86.50，优先使用钱包支付',
    icon: '钱'
  },
  {
    key: 'wechat',
    title: '微信支付',
    desc: '从微信支付本次配送费',
    icon: '微'
  }
]

function inputValue(event: { detail: { value?: string } }) {
  return String(event.detail.value ?? '')
}

export default function MerchantPublishPage() {
  const [pickupContact, setPickupContact] = useState('王老板')
  const [pickupPhone, setPickupPhone] = useState('139 2280 6612')
  const [pickupAddress, setPickupAddress] = useState('茶知味·大学城店（二食堂一楼）')
  const [receiverName, setReceiverName] = useState('')
  const [receiverPhone, setReceiverPhone] = useState('')
  const [receiverAddress, setReceiverAddress] = useState('')
  const [remark, setRemark] = useState('')
  const [sheetVisible, setSheetVisible] = useState(false)
  const [paymentMethod, setPaymentMethod] = useState<PaymentMethod>('wallet')

  const openPaymentSheet = () => {
    if (!receiverName.trim() || !receiverPhone.trim() || !receiverAddress.trim()) {
      void Taro.showToast({ title: '请先填写收货信息', icon: 'none', duration: 1400 })
      return
    }
    setSheetVisible(true)
  }

  const confirmPayment = () => {
    setSheetVisible(false)
    void Taro.showToast({ title: '发布成功,等待骑手接单', icon: 'none', duration: 1600 })
  }

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
            <Text className='merchant-publish-section-head__hint'>默认店铺信息,可修改</Text>
          </View>

          <View className='merchant-publish-field'>
            <Text className='merchant-publish-field__label'>联系人</Text>
            <Input
              className='merchant-publish-field__input'
              value={pickupContact}
              placeholder='请输入联系人'
              onInput={(event) => setPickupContact(inputValue(event))}
            />
          </View>
          <View className='merchant-publish-field'>
            <Text className='merchant-publish-field__label'>联系电话</Text>
            <Input
              className='merchant-publish-field__input'
              value={pickupPhone}
              placeholder='请输入联系电话'
              onInput={(event) => setPickupPhone(inputValue(event))}
            />
          </View>
          <View className='merchant-publish-field merchant-publish-field--last'>
            <Text className='merchant-publish-field__label'>取货地址</Text>
            <Input
              className='merchant-publish-field__input'
              value={pickupAddress}
              placeholder='请输入取货地址'
              onInput={(event) => setPickupAddress(inputValue(event))}
            />
          </View>
        </View>

        <View className='merchant-publish-card'>
          <View className='merchant-publish-section-head'>
            <View className='merchant-publish-section-head__left'>
              <View className='merchant-publish-dot merchant-publish-dot--dropoff' />
              <Text className='merchant-publish-section-head__title'>收货信息</Text>
            </View>
          </View>

          <View className='merchant-publish-field'>
            <Text className='merchant-publish-field__label'>收货人</Text>
            <Input
              className='merchant-publish-field__input'
              value={receiverName}
              placeholder='请输入收货人姓名'
              onInput={(event) => setReceiverName(inputValue(event))}
            />
          </View>
          <View className='merchant-publish-field'>
            <Text className='merchant-publish-field__label'>联系电话</Text>
            <Input
              className='merchant-publish-field__input'
              value={receiverPhone}
              placeholder='请输入收货人电话'
              onInput={(event) => setReceiverPhone(inputValue(event))}
            />
          </View>
          <View className='merchant-publish-field'>
            <Text className='merchant-publish-field__label'>送达地址</Text>
            <Input
              className='merchant-publish-field__input'
              value={receiverAddress}
              placeholder='如 3栋 4楼 412室'
              onInput={(event) => setReceiverAddress(inputValue(event))}
            />
          </View>
          <View className='merchant-publish-textarea-wrap'>
            <Text className='merchant-publish-field__label'>备注</Text>
            <Textarea
              className='merchant-publish-textarea'
              value={remark}
              placeholder='如 放门口即可、到楼下打电话'
              maxlength={80}
              onInput={(event) => setRemark(inputValue(event))}
            />
          </View>
        </View>

        <View className='merchant-publish-fee-card'>
          <View className='merchant-publish-fee-card__main'>
            <Text className='merchant-publish-fee-card__label'>配送费</Text>
            <Text className='merchant-publish-fee-card__hint'>平台统一定价,无需填写,发布时支付</Text>
          </View>
          <View className='merchant-publish-fee-card__amount'>
            <Text className='merchant-publish-fee-card__currency'>¥</Text>
            <Text>1.5</Text>
          </View>
        </View>

        <View className='merchant-publish-scroll__spacer' />
      </ScrollView>

      <View className='merchant-publish-footer'>
        <View className='merchant-publish-footer__button' onClick={openPaymentSheet}>
          <Text>发布并支付</Text>
        </View>
      </View>

      {sheetVisible && (
        <View className='merchant-publish-modal'>
          <View className='merchant-publish-modal__mask' onClick={() => setSheetVisible(false)} />
          <View className='merchant-publish-sheet'>
            <View className='merchant-publish-sheet__header'>
              <View>
                <Text className='merchant-publish-sheet__title'>选择支付方式</Text>
                <Text className='merchant-publish-sheet__subtitle'>配送费 · 平台统一定价</Text>
              </View>
              <View className='merchant-publish-sheet__close' onClick={() => setSheetVisible(false)}>
                <Text>×</Text>
              </View>
            </View>

            <View className='merchant-publish-payments'>
              {paymentOptions.map((option) => (
                <View
                  key={option.key}
                  className={`merchant-publish-payment ${
                    paymentMethod === option.key ? 'merchant-publish-payment--active' : ''
                  }`}
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
                    {paymentMethod === option.key && <View className='merchant-publish-payment__radio-dot' />}
                  </View>
                </View>
              ))}
            </View>

            <View className='merchant-publish-confirm' onClick={confirmPayment}>
              <Text>确认支付 ¥1.50</Text>
            </View>
          </View>
        </View>
      )}
    </View>
  )
}
