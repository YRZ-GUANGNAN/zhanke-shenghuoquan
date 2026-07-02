import React, { useEffect, useMemo, useState } from 'react'
import { Button, Image, Input, ScrollView, Text, View } from '@tarojs/components'
import {
  deliveryConversationCanSend,
  getDeliveryConversationUnread,
  getDeliveryOrderConversation,
  loadDeliveryTencentMessages,
  parseDeliveryImagePayload,
  resolveDeliveryConversationImage,
  sendDeliveryTencentImage,
  sendDeliveryTencentText,
  subscribeDeliveryConversationUnread,
  subscribeDeliveryTencentMessages,
  type DeliveryConversation,
  type DeliveryTencentMessage
} from '../../services/delivery-im'
import { chooseAndUploadImages, waitForMediaApproval } from '../../services/media'
import Taro from '../../services/taro'
import './index.scss'

interface ResolvedImage {
  url: string
  expiresAt: number
  error?: string
}

export default function DeliveryConversationPage() {
  const params = Taro.getCurrentInstance().router?.params || {}
  const orderId = useMemo(() => String(params.orderId || ''), [])
  const [conversation, setConversation] = useState<DeliveryConversation | null>(null)
  const [messages, setMessages] = useState<DeliveryTencentMessage[]>([])
  const [images, setImages] = useState<Record<string, ResolvedImage>>({})
  const [text, setText] = useState('')
  const [loading, setLoading] = useState(true)
  const [sending, setSending] = useState(false)
  const [progress, setProgress] = useState('')
  const [unread, setUnread] = useState(0)

  useEffect(() => {
    let active = true
    let unsubscribeMessages: (() => void) | undefined
    let unsubscribeUnread: (() => void) | undefined
    ;(async () => {
      try {
        if (!orderId) throw new Error('配送单不存在或无权访问')
        const value = await getDeliveryOrderConversation(orderId)
        if (!active) return
        setConversation(value)
        setUnread(await getDeliveryConversationUnread(value.tencentConversationId))
        const initial = await loadDeliveryTencentMessages(value)
        if (!active) return
        setUnread(0)
        setMessages(mergeMessages([], initial))
        void resolveImageMessages(value, initial)
        unsubscribeMessages = await subscribeDeliveryTencentMessages(value, (next) => {
          setMessages((current) => {
            const merged = mergeMessages(current, next)
            void resolveImageMessages(value, next)
            return merged
          })
        })
        unsubscribeUnread = await subscribeDeliveryConversationUnread(value.tencentConversationId, setUnread)
      } catch (error) {
        showConversationError(error)
      } finally {
        if (active) setLoading(false)
      }
    })()
    return () => {
      active = false
      unsubscribeMessages?.()
      unsubscribeUnread?.()
    }
  }, [orderId])

  const canSend = Boolean(conversation && deliveryConversationCanSend(conversation))

  const sendText = async () => {
    const value = text.trim()
    if (!conversation || !canSend || !value || sending) return
    setSending(true)
    try {
      const message = await sendDeliveryTencentText(conversation, value)
      setMessages((current) => mergeMessages(current, [message]))
      setText('')
    } catch (error) {
      showConversationError(error)
    } finally {
      setSending(false)
    }
  }

  const sendImage = async () => {
    if (!conversation || !canSend || sending) return
    if (!conversation.imagesEnabled) {
      await Taro.showToast({ title: '当前环境未开启会话图片发送', icon: 'none' })
      return
    }
    setSending(true)
    setProgress('请选择会话图片')
    try {
      const [uploaded] = await chooseAndUploadImages(
        'delivery_im',
        1,
        (value) => setProgress(`${value.percent}% · ${value.message}`)
      )
      if (!uploaded) throw new Error('未选择图片')
      const approved = await waitForMediaApproval(
        uploaded.assetId,
        (value) => setProgress(value.canUse ? '审核已通过，正在发送图片' : `处理中 · ${value.message}`)
      )
      if (!approved.canUse) throw new Error('图片尚未审核通过')
      const message = await sendDeliveryTencentImage(conversation, approved.assetId)
      setMessages((current) => {
        const merged = mergeMessages(current, [message])
        void resolveImageMessages(conversation, [message])
        return merged
      })
      await Taro.showToast({ title: '图片已发送', icon: 'success' })
    } catch (error) {
      showConversationError(error)
    } finally {
      setProgress('')
      setSending(false)
    }
  }

  const resolveImageMessages = async (
    value: DeliveryConversation,
    values: DeliveryTencentMessage[]
  ) => {
    for (const message of values) {
      const payload = parseDeliveryImagePayload(message)
      if (!payload || payload.conversationId !== value.conversationPublicId) continue
      const key = messageKey(message)
      const cached = images[key]
      if (cached && cached.expiresAt > Date.now() + 5000) continue
      try {
        const resolved = await resolveDeliveryConversationImage(value.conversationPublicId, payload.imageId)
        setImages((current) => ({
          ...current,
          [key]: {
            url: resolved.url,
            expiresAt: Date.now() + resolved.expiresInSeconds * 1000
          }
        }))
      } catch (error) {
        setImages((current) => ({
          ...current,
          [key]: {
            url: '',
            expiresAt: Date.now() + 10000,
            error: error instanceof Error ? error.message : '图片暂不可见'
          }
        }))
      }
    }
  }

  return (
    <View className='delivery-conversation-page'>
      <View className='delivery-conversation-status'>
        <Text className='delivery-conversation-title'>配送会话</Text>
        <Text className='delivery-conversation-hint'>
          {conversation
            ? `${statusText(conversation)} · 未读 ${unread}`
            : loading ? '正在进入会话...' : '会话不可用'}
        </Text>
        {conversation?.readReceiptEnabled
          ? <Text className='delivery-conversation-hint'>已读回执由后端开启后才展示；当前页面仍不展示对方已读。</Text>
          : <Text className='delivery-conversation-hint'>本阶段不展示“对方已读”，打开会话后仅清除自己的未读。</Text>}
        {progress && <Text className='delivery-conversation-hint'>{progress}</Text>}
      </View>

      <ScrollView className='delivery-conversation-list' scrollY>
        {messages.map((message) => renderMessage(message))}
        {!loading && messages.length === 0 && (
          <View className='delivery-message delivery-message--in'>
            <Text className='delivery-message__text'>暂无消息，可以从这里联系对方。</Text>
          </View>
        )}
      </ScrollView>

      <View className='delivery-compose'>
        <Input
          className='delivery-compose__input'
          value={text}
          maxlength={500}
          placeholder={canSend ? '输入消息' : '会话只读或不可用'}
          disabled={!canSend || sending}
          onInput={(event) => setText(String(event.detail.value))}
          onConfirm={() => void sendText()}
        />
        {conversation?.imagesEnabled && (
          <Button
            className='delivery-compose__button delivery-compose__button--plain'
            disabled={!canSend || sending}
            onClick={() => void sendImage()}
          >
            图片
          </Button>
        )}
        <Button
          className='delivery-compose__button'
          disabled={!canSend || sending || !text.trim()}
          onClick={() => void sendText()}
        >
          发送
        </Button>
      </View>
    </View>
  )

  function renderMessage(message: DeliveryTencentMessage) {
    const key = messageKey(message)
    const imagePayload = parseDeliveryImagePayload(message)
    const outgoing = message.flow === 'out'
    return (
      <View key={key} className={`delivery-message delivery-message--${outgoing ? 'out' : 'in'}`}>
        {message.type === 'TIMTextElem' && (
          <Text className='delivery-message__text'>{String(message.payload?.text || '')}</Text>
        )}
        {imagePayload && renderImageMessage(key)}
        {message.type !== 'TIMTextElem' && !imagePayload && (
          <Text className='delivery-message__text'>暂不支持的消息类型</Text>
        )}
        <Text className='delivery-message__time'>{formatMessageTime(message.time)}</Text>
      </View>
    )
  }

  function renderImageMessage(key: string) {
    const image = images[key]
    if (image?.url) {
      return <Image className='delivery-message__image' src={image.url} mode='aspectFill' />
    }
    return (
      <Text className='delivery-message__placeholder'>
        {image?.error || '正在获取私有图片地址...'}
      </Text>
    )
  }
}

function mergeMessages(
  current: DeliveryTencentMessage[],
  next: DeliveryTencentMessage[]
): DeliveryTencentMessage[] {
  const values = new Map(current.map((item) => [messageKey(item), item]))
  next.forEach((item) => values.set(messageKey(item), item))
  return [...values.values()].sort((a, b) => a.time - b.time)
}

function messageKey(message: DeliveryTencentMessage): string {
  return message.ID || `${message.conversationID}:${message.time}:${String(message.payload?.data || '')}`
}

function formatMessageTime(value: number): string {
  const date = new Date(value * 1000)
  return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
}

function statusText(conversation: DeliveryConversation): string {
  if (conversation.status === 'active' && conversation.canSend) return '可发送消息'
  if (conversation.status === 'read_only') return '会话只读'
  if (conversation.status === 'provisioning') return '会话开通中'
  if (conversation.status === 'failed') return '会话暂不可用'
  if (conversation.status === 'revoked' || conversation.status === 'revoking') return '会话已结束'
  return '会话不可发送'
}

function showConversationError(error: unknown) {
  const message = error instanceof Error ? error.message : '配送会话操作失败'
  let friendly = message
  if (/not found|404|forbidden/i.test(message)) {
    friendly = '会话已结束或你已不再参与该配送单'
  } else if (/read-only|read only/i.test(message)) {
    friendly = '当前会话只读，不能继续发送'
  } else if (/unavailable|service unavailable/i.test(message)) {
    friendly = '会话服务暂不可用，请稍后重试'
  }
  void Taro.showToast({ title: friendly, icon: 'none' })
}
