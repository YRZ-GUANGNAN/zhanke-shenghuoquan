import React, { useCallback, useEffect, useState } from 'react'
import { ScrollView, Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import { AppIconButton } from '../../components/ui'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import { useIdentityGuard } from '../../hooks/use-identity-guard'
import {
  getDeliveryConversationUnread,
  listDeliveryConversations,
  type DeliveryConversation,
} from '../../services/delivery-im'
import Taro from '../../services/taro'
import './index.scss'

const statusCopy: Record<DeliveryConversation['status'], string> = {
  provisioning: '会话准备中',
  active: '可发送消息',
  read_only: '只读会话',
  revoking: '会话正在关闭',
  revoked: '会话已关闭',
  failed: '会话创建失败',
}

function formatTime(value: string) {
  return value.replace('T', ' ').slice(0, 16)
}

function MerchantMessagesContent() {
  const identityAllowed = useIdentityGuard('merchant')
  const [items, setItems] = useState<DeliveryConversation[]>([])
  const [unreadById, setUnreadById] = useState<Record<string, number>>({})
  const [loading, setLoading] = useState(false)
  const [loadError, setLoadError] = useState('')

  const loadMessages = useCallback(async () => {
    if (!identityAllowed || loading) return
    setLoading(true)
    setLoadError('')
    try {
      const result = await listDeliveryConversations(1, 30)
      setItems(result.items)
      const unreadEntries = await Promise.all(result.items.map(async (item) => {
        try {
          const count = await getDeliveryConversationUnread(item.tencentConversationId)
          return [item.conversationPublicId, count] as const
        } catch {
          return [item.conversationPublicId, 0] as const
        }
      }))
      setUnreadById(unreadEntries.reduce<Record<string, number>>((result, [id, count]) => {
        result[id] = count
        return result
      }, {}))
    } catch (error) {
      setItems([])
      setUnreadById({})
      setLoadError(error instanceof Error ? error.message : '消息加载失败')
    } finally {
      setLoading(false)
    }
  }, [identityAllowed, loading])

  useEffect(() => {
    if (identityAllowed) void loadMessages()
  }, [identityAllowed]) // eslint-disable-line react-hooks/exhaustive-deps

  const openConversation = (item: DeliveryConversation) => {
    void Taro.navigateTo({
      url: `/pages/delivery-conversation/index?orderId=${encodeURIComponent(item.deliveryOrderPublicId)}&role=merchant`,
    })
  }

  if (!identityAllowed) return <View className='merchant-messages-page' />

  return (
    <View className='merchant-messages-page'>
      <CapsulePageHeader
        className='merchant-messages-nav'
        title='消息'
        left={<AppIconButton name='back' label='返回' className='merchant-messages-nav__back' onClick={() => void Taro.navigateBack()} />}
        right={(
          <Text className='merchant-messages-nav__read' onClick={() => void loadMessages()}>
            刷新
          </Text>
        )}
      />

      <ScrollView className='merchant-messages-scroll' scrollY enhanced showScrollbar={false}>
        <View className='merchant-messages-card'>
          {loading ? <Text className='merchant-messages-card__footer'>正在加载真实配送会话…</Text> : null}
          {!loading && loadError ? (
            <Text className='merchant-messages-card__footer' onClick={() => void loadMessages()}>
              {loadError}，点击重试
            </Text>
          ) : null}
          {!loading && !loadError && items.length === 0 ? (
            <Text className='merchant-messages-card__footer'>当前没有配送会话</Text>
          ) : null}
          {items.map((item) => {
            const unread = unreadById[item.conversationPublicId] || 0
            return (
              <View key={item.conversationPublicId} className='merchant-message-item' onClick={() => openConversation(item)}>
                <View className='merchant-message-item__avatar merchant-message-item__avatar--rider'>
                  <Text>聊</Text>
                  {unread > 0 ? <View className='merchant-message-item__dot' /> : null}
                </View>
                <View className='merchant-message-item__body'>
                  <View className='merchant-message-item__top'>
                    <View className='merchant-message-item__title-row'>
                      <Text className='merchant-message-item__title'>配送单 #{item.deliveryOrderPublicId}</Text>
                      <Text className='merchant-message-item__tag'>{item.canSend ? '可私聊' : '只读'}</Text>
                    </View>
                    <Text className='merchant-message-item__time'>{formatTime(item.updatedAt)}</Text>
                  </View>
                  <Text className={`merchant-message-item__preview ${unread > 0 ? 'merchant-message-item__preview--unread' : ''}`}>
                    {unread > 0 ? `${unread} 条未读消息` : statusCopy[item.status]}
                  </Text>
                </View>
              </View>
            )
          })}
        </View>
      </ScrollView>
    </View>
  )
}

export default function MerchantMessagesPage() {
  return (
    <RuntimeSessionGate preserveAuthenticatedChildrenDuringRetry>
      <MerchantMessagesContent />
    </RuntimeSessionGate>
  )
}
