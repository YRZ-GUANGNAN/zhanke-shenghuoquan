import React, { useState } from 'react'
import { Image, Input, ScrollView, Text, View } from '@tarojs/components'
import Taro from '../../services/taro'
import './index.scss'

type ConversationRole = 'merchant' | 'rider'

interface ConversationCopy {
  peerAvatar: string
  selfAvatar: string
  peerName: string
  peerMeta: string
  orderTitle: string
  orderMeta: string
  leftFirst: string
  rightReply: string
  leftSecond: string
  detailUrl: string
  fallbackUrl: string
}

const copyByRole: Record<ConversationRole, ConversationCopy> = {
  merchant: {
    peerAvatar: '李',
    selfAvatar: '茶',
    peerName: '李伟 · 骑手',
    peerMeta: '配送中 · 137 5560 8814',
    orderTitle: '#B1079 · 本店 → 3栋 4楼 412室',
    orderMeta: '配送中 · 配送费 ¥1.5',
    leftFirst: '老板，B1079 我到店了，在几号取餐口拿？',
    rightReply: '左手边 3 号取餐架，蓝色袋子写了 412',
    leftSecond: '好的看到了，这就出发',
    detailUrl: '/pages/merchant-order-detail/index',
    fallbackUrl: '/pages/merchant-home/index'
  },
  rider: {
    peerAvatar: '茶',
    selfAvatar: '李',
    peerName: '茶知味 · 商家',
    peerMeta: '配送中 · 188 5530 2217',
    orderTitle: '#B1079 · 本店 → 3栋 4楼 412室',
    orderMeta: '配送中 · 配送费 ¥1.5',
    leftFirst: '左手边 3 号取餐架，蓝色袋子写了 412',
    rightReply: '老板，B1079 我到店了，在几号取餐口拿？',
    leftSecond: '好的看到了，这就出发',
    detailUrl: '/pages/delivery-order-detail/index',
    fallbackUrl: '/pages/delivery-rider/index'
  }
}

function showToast(title: string) {
  void Taro.showToast({ title, icon: 'none', duration: 1400 })
}

export default function DeliveryConversationPage() {
  const params = Taro.getCurrentInstance().router?.params || {}
  const role: ConversationRole = params.role === 'rider' ? 'rider' : 'merchant'
  const copy = copyByRole[role]
  const [draft, setDraft] = useState('')
  const [pickedImage, setPickedImage] = useState('')

  const goBack = () => {
    void Taro.navigateBack().catch(() => Taro.redirectTo({ url: copy.fallbackUrl }))
  }

  const openOrder = () => {
    void Taro.navigateTo({ url: copy.detailUrl }).catch(() => showToast('详情页稍后接入'))
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
