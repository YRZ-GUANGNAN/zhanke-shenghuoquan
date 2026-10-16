import React, { useCallback, useEffect, useRef, useState } from 'react'
import { ScrollView, Text, View } from '@tarojs/components'
import { useDidHide, useDidShow } from '@tarojs/taro'
import CapsulePageHeader from '../../components/capsule-page-header'
import { AppIconButton } from '../../components/ui'
import DeliveryWorkspaceNav from '../../components/delivery-workspace-nav'
import { useSessionStore } from '../../stores/session'
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

export function DeliveryMessagesContent({ role, identityAllowed }: { role: 'merchant' | 'rider'; identityAllowed: boolean }) {
  const accountId = useSessionStore(state => state.profile.id)
  const token = useSessionStore(state => state.token)
  const [items, setItems] = useState<DeliveryConversation[]>([])
  const [unreadById, setUnreadById] = useState<Record<string, number>>({})
  const [loading, setLoading] = useState(false)
  const [loadError, setLoadError] = useState('')
  const [total, setTotal] = useState(0)
  const pageRef = useRef(0)
  const epoch = useRef(0)
  const busy = useRef(false)
  const visible = useRef(true)
  const allowedRef = useRef(identityAllowed)
  allowedRef.current = identityAllowed

  const loadMessages = useCallback(async (append = false) => {
    if (!allowedRef.current || !visible.current || busy.current) return
    busy.current = true
    const version = epoch.current
    const session = useSessionStore.getState()
    const current = () => version === epoch.current && allowedRef.current && visible.current
      && useSessionStore.getState().token === session.token && useSessionStore.getState().profile.id === session.profile.id
    const nextPage = append ? pageRef.current + 1 : 1
    setLoading(true)
    setLoadError('')
    try {
      const result = await listDeliveryConversations(nextPage, 30)
      if (!current()) return
      setItems(previous => append ? Array.from(new Map([...previous, ...result.items].map(item => [item.conversationPublicId, item])).values()) : result.items)
      if (!append) setUnreadById({})
      pageRef.current = nextPage
      setTotal(result.total)
      const unreadEntries = await Promise.all(result.items.map(async (item) => {
        try {
          const count = await getDeliveryConversationUnread(item.tencentConversationId)
          return [item.conversationPublicId, count] as const
        } catch {
          return [item.conversationPublicId, -1] as const
        }
      }))
      if (!current()) return
      const unread = unreadEntries.reduce<Record<string, number>>((result, [id, count]) => {
        result[id] = count
        return result
      }, {})
      setUnreadById(previous => append ? { ...previous, ...unread } : unread)
    } catch (error) {
      if (!current()) return
      setLoadError(error instanceof Error ? error.message : '消息加载失败')
    } finally {
      if (current()) { busy.current = false; setLoading(false) }
    }
  }, [])

  useDidHide(() => { visible.current = false; epoch.current += 1; busy.current = false })
  useDidShow(() => { visible.current = true; void loadMessages() })

  useEffect(() => {
    epoch.current += 1
    busy.current = false
    setItems([])
    setUnreadById({})
    setTotal(0)
    setLoading(false)
    setLoadError('')
    pageRef.current = 0
    if (identityAllowed) void loadMessages()
    return () => { epoch.current += 1; busy.current = false }
  }, [identityAllowed, accountId, token, role, loadMessages])

  const openConversation = (item: DeliveryConversation) => {
    void Taro.navigateTo({
      url: `/pages/delivery-conversation/index?orderId=${encodeURIComponent(item.deliveryOrderPublicId)}&role=${role}`,
    }).catch(() => Taro.showToast({ title: '会话打开失败，请重试', icon: 'none' }))
  }

  if (!identityAllowed) return <View className='merchant-messages-page' />

  return (
    <View className='merchant-messages-page'>
      <CapsulePageHeader
        className='merchant-messages-nav'
        title='消息'
        left={<AppIconButton name='back' label='返回工作台' className='merchant-messages-nav__back' onClick={() => void Taro.redirectTo({ url: role === 'merchant' ? '/pages/merchant-home/index' : '/pages/delivery-rider/index' })} />}
        right={(
          <Text className='merchant-messages-nav__read' onClick={() => void loadMessages()}>
            刷新
          </Text>
        )}
      />

      <ScrollView className='merchant-messages-scroll' scrollY enhanced showScrollbar={false} lowerThreshold={100}
        onScrollToLower={() => { if (items.length < total) void loadMessages(true) }}>
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
            const unread = unreadById[item.conversationPublicId]
            return (
              <View key={item.conversationPublicId} className='merchant-message-item' onClick={() => openConversation(item)}>
                <View className='merchant-message-item__avatar merchant-message-item__avatar--rider'>
                  <Text>聊</Text>
                  {unread != null && unread > 0 ? <View className='merchant-message-item__dot' /> : null}
                </View>
                <View className='merchant-message-item__body'>
                  <View className='merchant-message-item__top'>
                    <View className='merchant-message-item__title-row'>
                      <Text className='merchant-message-item__title'>配送单 #{item.deliveryOrderPublicId}</Text>
                      <Text className='merchant-message-item__tag'>{item.canSend ? '可私聊' : '只读'}</Text>
                    </View>
                    <Text className='merchant-message-item__time'>{formatTime(item.updatedAt)}</Text>
                  </View>
                  <Text className={`merchant-message-item__preview ${unread != null && unread > 0 ? 'merchant-message-item__preview--unread' : ''}`}>
                    {unread == null ? '未读状态同步中…' : unread > 0 ? `${unread} 条未读消息` : unread === -1 ? '未读状态暂未同步' : statusCopy[item.status]}
                  </Text>
                </View>
              </View>
            )
          })}
          {!loading && items.length < total ? <Text className='merchant-messages-card__footer' onClick={() => void loadMessages(true)}>加载更多会话</Text> : null}
        </View>
      </ScrollView>
      <DeliveryWorkspaceNav role={role} active='messages' />
    </View>
  )
}
