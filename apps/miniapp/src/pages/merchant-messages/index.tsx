import React, { useMemo, useState } from 'react'
import { ScrollView, Text, View } from '@tarojs/components'
import Taro from '../../services/taro'
import { getCapsuleSafeStyle } from '../../utils/capsule-safe'
import './index.scss'

type MerchantMessageKind = 'rider' | 'order' | 'wallet' | 'ops'

interface MerchantMessage {
  id: string
  kind: MerchantMessageKind
  title: string
  time: string
  preview: string
  avatar?: string
  tag?: string
  unread?: boolean
}

const messages: MerchantMessage[] = [
  {
    id: 'rider-li',
    kind: 'rider',
    title: '李伟',
    tag: '骑手',
    time: '12:41',
    preview: '#B1079 已取货,正在配送中',
    avatar: '李',
    unread: true
  },
  {
    id: 'rider-zhang',
    kind: 'rider',
    title: '张强',
    tag: '骑手',
    time: '12:28',
    preview: '我到店了,B1077 在哪个取餐口?',
    avatar: '张',
    unread: true
  },
  {
    id: 'order',
    kind: 'order',
    title: '订单通知',
    time: '12:47',
    preview: '#B1085 已进入抢单池,等待骑手接单',
    unread: true
  },
  {
    id: 'wallet',
    kind: 'wallet',
    title: '钱包通知',
    time: '09:20',
    preview: '充值 ¥100.00 已到账,当前余额 ¥86.50'
  },
  {
    id: 'ops',
    kind: 'ops',
    title: '平台运营',
    time: '昨天',
    preview: '七月配送高峰提示:午间 12:00–13:00 建议提...'
  }
]

function handleBack() {
  void Taro.navigateBack()
}

function messageToast(kind: MerchantMessageKind) {
  const title = kind === 'rider' ? '私聊静态占位' : '通知详情静态占位'
  void Taro.showToast({ title, icon: 'none', duration: 1400 })
}

function openMerchantConversation(message: MerchantMessage) {
  if (message.kind !== 'rider') {
    messageToast(message.kind)
    return
  }
  const orderId = message.id === 'rider-zhang' ? 'B1077' : 'B1079'
  void Taro.navigateTo({
    url: `/pages/delivery-conversation/index?orderId=${encodeURIComponent(orderId)}&role=merchant`
  })
}

export default function MerchantMessagesPage() {
  const [readIds, setReadIds] = useState<string[]>([])
  const unreadIds = useMemo(
    () => messages.filter((item) => item.unread && !readIds.includes(item.id)).map((item) => item.id),
    [readIds]
  )

  function markAllRead() {
    setReadIds(messages.map((item) => item.id))
    void Taro.showToast({ title: '已全部标为已读', icon: 'none', duration: 1200 })
  }

  return (
    <View className='merchant-messages-page' style={getCapsuleSafeStyle()}>
      <View className='merchant-messages-nav'>
        <View className='merchant-messages-nav__back' onClick={handleBack}>
          <Text>‹</Text>
        </View>
        <Text className='merchant-messages-nav__title'>消息</Text>
        <Text
          className={`merchant-messages-nav__read ${unreadIds.length === 0 ? 'merchant-messages-nav__read--muted' : ''}`}
          onClick={markAllRead}
        >
          全部已读
        </Text>
      </View>

      <ScrollView className='merchant-messages-scroll' scrollY enhanced showScrollbar={false}>
        <View className='merchant-messages-card'>
          {messages.map((item) => {
            const hasUnread = item.unread && !readIds.includes(item.id)
            return (
              <View key={item.id} className='merchant-message-item' onClick={() => openMerchantConversation(item)}>
                <View className={`merchant-message-item__avatar merchant-message-item__avatar--${item.kind}`}>
                  {item.kind === 'rider' ? <Text>{item.avatar}</Text> : <Text>▵</Text>}
                  {hasUnread && <View className='merchant-message-item__dot' />}
                </View>
                <View className='merchant-message-item__body'>
                  <View className='merchant-message-item__top'>
                    <View className='merchant-message-item__title-row'>
                      <Text className='merchant-message-item__title'>{item.title}</Text>
                      {item.tag && <Text className='merchant-message-item__tag'>{item.tag}</Text>}
                    </View>
                    <Text className='merchant-message-item__time'>{item.time}</Text>
                  </View>
                  <Text className={`merchant-message-item__preview ${hasUnread ? 'merchant-message-item__preview--unread' : ''}`}>
                    {item.preview}
                  </Text>
                </View>
              </View>
            )
          })}
          <Text className='merchant-messages-card__footer'>仅展示最近 30 天消息</Text>
        </View>
      </ScrollView>
    </View>
  )
}
