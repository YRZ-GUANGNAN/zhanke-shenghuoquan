import React, { useState } from 'react'
import { Image, Input, ScrollView, Text, View } from '@tarojs/components'
import Taro from '../../services/taro'
import { resolveStaticDeliveryOrder } from '../delivery-static-data'
import './index.scss'

type ConversationRole = 'merchant' | 'rider'

interface ConversationRoleCopy {
  fallbackUrl: string
}

const copyByRole: Record<ConversationRole, ConversationRoleCopy> = {
  merchant: {
    fallbackUrl: '/pages/merchant-home/index'
  },
  rider: {
    fallbackUrl: '/pages/delivery-rider/index'
  }
}

function showToast(title: string) {
  void Taro.showToast({ title, icon: 'none', duration: 1400 })
}

export default function DeliveryConversationPage() {
  const params = Taro.getCurrentInstance().router?.params || {}
  const role: ConversationRole = params.role === 'rider' ? 'rider' : 'merchant'
  const roleCopy = copyByRole[role]
  const { order, found } = resolveStaticDeliveryOrder(params.orderId)
  const assignedRider = order.rider || { name: '骑手待接单', phone: '暂无电话', avatar: '骑' }
  const peer = role === 'merchant' ? assignedRider : order.merchant
  const selfAvatar = role === 'merchant' ? order.merchant.avatar : assignedRider.avatar
  const riderQuestion = `老板，${order.id} 我到 ${order.pickupName} 了，在几号取货口拿？`
  const merchantReply = `请在 ${order.pickupName} 的取货架拿，送往 ${order.destination}`
  const copy = {
    peerAvatar: peer.avatar,
    selfAvatar,
    peerName: peer.name,
    peerMeta: `${order.statusLabel} · ${peer.phone}`,
    orderTitle: `#${order.id} · ${order.pickupName} → ${order.destination}`,
    orderMeta: `${order.statusLabel} · 配送费 ¥${order.fee}`,
    leftFirst: role === 'merchant' ? riderQuestion : merchantReply,
    rightReply: role === 'merchant' ? merchantReply : riderQuestion,
    leftSecond: found ? '好的，信息核对无误，保持本单沟通' : '静态订单数据待补充'
  }
  const [draft, setDraft] = useState('')
  const [pickedImage, setPickedImage] = useState('')

  const goBack = () => {
    void Taro.navigateBack().catch(() => Taro.redirectTo({ url: roleCopy.fallbackUrl }))
  }

  const openOrder = () => {
    void Taro.navigateTo({
      url: `/pages/merchant-order-detail/index?orderId=${encodeURIComponent(order.id)}&role=${role}`
    }).catch(() => showToast('详情页稍后接入'))
  }

  const pickImage = async () => {
    try {
      const result = await Taro.chooseMedia({
        count: 1,
        mediaType: ['image'],
        sourceType: ['album', 'camera'],
        sizeType: ['compressed']
      })
      const imagePath = result.tempFiles[0]?.tempFilePath
      if (imagePath) {
        setPickedImage(imagePath)
        showToast('图片已加入本单沟通')
      }
    } catch {
      showToast('已取消选择图片')
    }
  }

  const sendText = () => {
    if (!draft.trim()) {
      showToast('先输入消息')
      return
    }
    setDraft('')
    showToast('静态演示：消息已发送')
  }

  return (
    <View className='delivery-chat-page'>
      <View className='delivery-chat-header'>
        <View className='delivery-chat-back' onClick={goBack} />
        <View className={`delivery-chat-peer delivery-chat-peer--${role}`}>
          <View className='delivery-chat-peer__avatar'>
            <Text>{copy.peerAvatar}</Text>
          </View>
          <View className='delivery-chat-peer__body'>
            <Text className='delivery-chat-peer__name'>{copy.peerName}</Text>
            <View className='delivery-chat-peer__meta'>
              <View className='delivery-chat-peer__dot' />
              <Text>{copy.peerMeta}</Text>
            </View>
          </View>
        </View>
        <View className='delivery-chat-phone' onClick={() => showToast('电话联系稍后接入')}>
          <Text>⌕</Text>
        </View>
      </View>

      <View className='delivery-chat-order' onClick={openOrder}>
        <View className='delivery-chat-order__route'>
          <View className='delivery-chat-order__dot' />
          <View className='delivery-chat-order__line' />
          <View className='delivery-chat-order__square' />
        </View>
        <View className='delivery-chat-order__main'>
          <Text className='delivery-chat-order__title'>{copy.orderTitle}</Text>
          <Text className='delivery-chat-order__meta'>{copy.orderMeta}</Text>
        </View>
        <Text className='delivery-chat-order__link'>查看</Text>
      </View>

      <ScrollView className='delivery-chat-scroll' scrollY enhanced showScrollbar={false}>
        <View className='delivery-chat-tip'>
          <Text>仅本单沟通，请勿透露与订单无关的隐私</Text>
        </View>

        <View className='delivery-chat-row delivery-chat-row--peer'>
          <View className='delivery-chat-avatar delivery-chat-avatar--peer'>
            <Text>{copy.peerAvatar}</Text>
          </View>
          <View className='delivery-chat-bubble delivery-chat-bubble--peer'>
            <Text>{copy.leftFirst}</Text>
          </View>
        </View>

        <View className='delivery-chat-row delivery-chat-row--self'>
          <View className='delivery-chat-bubble delivery-chat-bubble--self'>
            <Text>{copy.rightReply}</Text>
          </View>
          <View className='delivery-chat-avatar delivery-chat-avatar--self'>
            <Text>{copy.selfAvatar}</Text>
          </View>
        </View>

        <View className='delivery-chat-row delivery-chat-row--peer'>
          <View className='delivery-chat-avatar delivery-chat-avatar--peer'>
            <Text>{copy.peerAvatar}</Text>
          </View>
          <View className='delivery-chat-bubble delivery-chat-bubble--peer delivery-chat-bubble--short'>
            <Text>{copy.leftSecond}</Text>
          </View>
        </View>

        {pickedImage && (
          <View className='delivery-chat-row delivery-chat-row--self delivery-chat-row--image'>
            <View className='delivery-chat-image-message'>
              <Image className='delivery-chat-image-message__img' src={pickedImage} mode='aspectFill' />
            </View>
            <View className='delivery-chat-avatar delivery-chat-avatar--self'>
              <Text>{copy.selfAvatar}</Text>
            </View>
          </View>
        )}
      </ScrollView>

      <View className='delivery-chat-inputbar'>
        <View className='delivery-chat-image-button' onClick={pickImage}>
          <View className='delivery-chat-image-button__icon' />
        </View>
        <View className='delivery-chat-input-wrap'>
          <Input
            className='delivery-chat-input'
            value={draft}
            placeholder='发消息...'
            placeholderClass='delivery-chat-input__placeholder'
            confirmType='send'
            onInput={(event) => setDraft(String(event.detail.value ?? ''))}
            onConfirm={sendText}
          />
        </View>
        <View className={`delivery-chat-send ${draft.trim() ? 'delivery-chat-send--active' : ''}`} onClick={sendText}>
          <Text>发送</Text>
        </View>
      </View>
    </View>
  )
}
