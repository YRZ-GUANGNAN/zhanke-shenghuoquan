import React, { useEffect, useMemo, useState } from 'react'
import { Button, Image, Input, ScrollView, Text, View } from '@tarojs/components'
import AppIcon from '../../components/app-icon'
import { isApiError } from '../../services/api'
import {
  deliveryConversationCanSend,
  getDeliveryOrderConversation,
  loadDeliveryTencentMessages,
  parseDeliveryImagePayload,
  resolveDeliveryConversationImage,
  sendDeliveryTencentImage,
  sendDeliveryTencentText,
  subscribeDeliveryTencentMessages,
  type DeliveryConversation,
  type DeliveryTencentMessage
} from '../../services/delivery-im'
import { chooseAndUploadImages, waitForMediaApproval } from '../../services/media'
import Taro from '../../services/taro'
import { useSessionStore } from '../../stores/session'
import { useKeyboardInset } from '../../hooks/use-keyboard-inset'
import './index.scss'

type PageStatus = 'loading' | 'ready' | 'error'

function mergeMessages(current: DeliveryTencentMessage[], next: DeliveryTencentMessage[]): DeliveryTencentMessage[] {
  const values = new Map(current.map((item) => [item.ID, item]))
  next.forEach((item) => values.set(item.ID, item))
  return [...values.values()].sort((left, right) => left.time - right.time)
}

function messageText(message: DeliveryTencentMessage): string {
  if (message.type !== 'TIMTextElem') return ''
  return String((message.payload as { text?: unknown } | undefined)?.text ?? '').trim()
}

function messageError(error: unknown, fallback = '配送沟通暂时不可用'): string {
  if (isApiError(error)) {
    if (error.status === 404) return '配送会话不存在或你无权访问'
    if (error.status === 409) return '会话状态已变化，请返回订单页刷新'
    return error.message || fallback
  }
  return error instanceof Error ? error.message : fallback
}

function formatTime(value: number): string {
  const date = new Date(value * 1000)
  return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
}

export default function DeliveryConversationPage() {
  const params = Taro.getCurrentInstance().router?.params ?? {}
  const orderId = String(params.orderId || '').trim()
  const identity = useSessionStore((state) => state.identity?.currentIdentity)
  const [status, setStatus] = useState<PageStatus>('loading')
  const [reloadKey, setReloadKey] = useState(0)
  const [conversation, setConversation] = useState<DeliveryConversation | null>(null)
  const [messages, setMessages] = useState<DeliveryTencentMessage[]>([])
  const [imageUrls, setImageUrls] = useState<Record<string, string>>({})
  const [draft, setDraft] = useState('')
  const [sending, setSending] = useState(false)
  const [error, setError] = useState('')
  const { inset: keyboardInset, onFocus: onKeyboardFocus, onKeyboardHeightChange, reset: resetKeyboardInset } = useKeyboardInset()

  useEffect(() => {
    let active = true
    let unsubscribe: (() => void) | undefined
    setStatus('loading')
    setConversation(null)
    setMessages([])
    setImageUrls({})
    setError('')
    resetKeyboardInset()

    if (!orderId) {
      setStatus('error')
      setError('配送会话入口无效，请从订单详情重新进入')
      return () => { active = false }
    }

    void (async () => {
      try {
        const nextConversation = await getDeliveryOrderConversation(orderId)
        if (!active) return
        setConversation(nextConversation)
        unsubscribe = await subscribeDeliveryTencentMessages(nextConversation, (incoming) => {
          if (!active) return
          setMessages((current) => mergeMessages(current, incoming))
        })
        if (!active) {
          unsubscribe()
          unsubscribe = undefined
          return
        }
        const history = await loadDeliveryTencentMessages(nextConversation)
        if (!active) return
        setMessages((current) => mergeMessages(current, history))
        setStatus('ready')
      } catch (value) {
        unsubscribe?.()
        unsubscribe = undefined
        if (!active) return
        const nextError = messageError(value)
        setError(nextError)
        setStatus('error')
      }
    })()

    return () => {
      active = false
      unsubscribe?.()
    }
  }, [orderId, reloadKey, resetKeyboardInset])

  useEffect(() => {
    if (!conversation) return
    let active = true
    const pending = messages.reduce<Array<{ messageId: string; imageId: string }>>((result, message) => {
      const payload = parseDeliveryImagePayload(message)
      if (payload && payload.conversationId === conversation.conversationPublicId && !Object.prototype.hasOwnProperty.call(imageUrls, message.ID)) {
        result.push({ messageId: message.ID, imageId: payload.imageId })
      }
      return result
    }, [])
    if (pending.length === 0) return () => { active = false }

    void Promise.all(pending.map(async (item) => {
      try {
        const resolved = await resolveDeliveryConversationImage(conversation.conversationPublicId, item.imageId)
        return { messageId: item.messageId, url: resolved.url }
      } catch {
        return { messageId: item.messageId, url: '' }
      }
    })).then((values) => {
      if (!active) return
      setImageUrls((current) => {
        const next = { ...current }
        values.forEach((value) => { next[value.messageId] = value.url })
        return next
      })
    })
    return () => { active = false }
  }, [conversation, imageUrls, messages])

  const canSend = Boolean(conversation && deliveryConversationCanSend(conversation))
  const visibleMessages = useMemo(
    () => messages.filter((message) => Boolean(messageText(message) || parseDeliveryImagePayload(message))),
    [messages]
  )

  const sendText = async () => {
    const text = draft.trim()
    if (!text) return
    if (!conversation || !canSend || sending) return
    setSending(true)
    try {
      const sent = await sendDeliveryTencentText(conversation, text)
      setMessages((current) => mergeMessages(current, [sent]))
      setDraft('')
    } catch (value) {
      await Taro.showToast({ title: messageError(value, '消息发送失败'), icon: 'none' })
    } finally {
      setSending(false)
    }
  }

  const sendImage = async () => {
    if (!conversation || !canSend || !conversation.imagesEnabled || sending) return
    setSending(true)
    try {
      const [uploaded] = await chooseAndUploadImages('delivery_im', 1, (progress) => {
        void Taro.showLoading({ title: `${progress.message} ${progress.percent}%`, mask: true })
      })
      if (!uploaded) return
      await waitForMediaApproval(uploaded.assetId, (mediaStatus) => {
        void Taro.showLoading({ title: mediaStatus.message || '图片审核中', mask: true })
      })
      const sent = await sendDeliveryTencentImage(conversation, uploaded.assetId)
      setImageUrls((current) => ({ ...current, [sent.ID]: uploaded.previewPath }))
      setMessages((current) => mergeMessages(current, [sent]))
    } catch (value) {
      await Taro.showToast({ title: messageError(value, '图片发送失败'), icon: 'none' })
    } finally {
      Taro.hideLoading()
      setSending(false)
    }
  }

  const fallback = identity === 'merchant' ? '/pages/merchant-home/index' : '/pages/delivery-rider/index'
  const goBack = () => {
    void Taro.navigateBack().catch(() => Taro.redirectTo({ url: fallback }))
  }

  return (
    <View className='delivery-chat-page'>
      <View className='delivery-chat-header'>
        <View className='delivery-chat-order'>
          <View className='delivery-chat-order__main'>
            <Text className='delivery-chat-order__title'>配送单沟通</Text>
            <Text className='delivery-chat-order__id'>{orderId ? `单号 ${orderId}` : '缺少配送单号'}</Text>
          </View>
          <Button className='cc-button cc-button--small delivery-chat-order__back' onClick={goBack}>返回订单</Button>
        </View>
      </View>

      <View className='delivery-chat-safe'>仅限本配送单的商家与骑手沟通；图片会先经过内容安全审核。</View>

      <ScrollView
        className={`delivery-chat-scroll${keyboardInset > 0 ? ' delivery-chat-scroll--keyboard' : ''}`}
        style={{ paddingBottom: `calc(64px + ${keyboardInset}px + env(safe-area-inset-bottom))` }}
        scrollY
        enhanced
        showScrollbar={false}
      >
        {status === 'loading' ? <View className='delivery-chat-state'>正在连接配送会话…</View> : null}
        {status === 'error' ? (
          <View className='delivery-chat-state delivery-chat-state--error' onClick={() => setReloadKey((value) => value + 1)}>
            <Text>{error}</Text>
            <Text className='delivery-chat-state__retry'>点击重试</Text>
          </View>
        ) : null}
        {status === 'ready' && visibleMessages.length === 0 ? <View className='delivery-chat-state'>会话已建立，发送第一条消息吧</View> : null}
        {visibleMessages.map((message) => {
          const mine = message.flow === 'out'
          const text = messageText(message)
          const imagePayload = parseDeliveryImagePayload(message)
          const imageUrl = imageUrls[message.ID]
          return (
            <View key={message.ID} className={`delivery-chat-row ${mine ? 'delivery-chat-row--mine' : ''}`}>
              <View className={`delivery-chat-bubble ${mine ? 'delivery-chat-bubble--mine' : ''}`}>
                {text ? <Text>{text}</Text> : null}
                {imagePayload ? (
                  imageUrl ? <Image className='delivery-chat-image' src={imageUrl} mode='widthFix' /> : <Text className='delivery-chat-image-loading'>图片读取中…</Text>
                ) : null}
                <Text className='delivery-chat-time'>{formatTime(message.time)}</Text>
              </View>
            </View>
          )
        })}
        {status === 'ready' && !canSend ? <View className='delivery-chat-state'>订单已结束或会话已只读，可查看历史消息但不能继续发送。</View> : null}
      </ScrollView>

      <View
        className={`delivery-chat-inputbar${keyboardInset > 0 ? ' delivery-chat-inputbar--keyboard' : ''}`}
        style={keyboardInset > 0 ? { bottom: `${keyboardInset}px` } : undefined}
      >
        <Button className='delivery-chat-image-button' disabled={!canSend || !conversation?.imagesEnabled || sending} onClick={() => void sendImage()}>
          <AppIcon name='image' className='delivery-chat-image-button__icon' />
        </Button>
        <Input
          className='delivery-chat-input'
          value={draft}
          maxlength={500}
          disabled={!canSend || sending}
          placeholder={canSend ? '输入消息' : '当前会话不可发送'}
          confirmType='send'
          adjustPosition={false}
          onInput={(event) => setDraft(String(event.detail.value ?? ''))}
          onFocus={onKeyboardFocus}
          onKeyboardHeightChange={onKeyboardHeightChange}
          onBlur={resetKeyboardInset}
          onConfirm={() => void sendText()}
        />
        <Button className='delivery-chat-send' disabled={!canSend || !draft.trim() || sending} onClick={() => void sendText()}>{sending ? '发送中' : '发送'}</Button>
      </View>
    </View>
  )
}
