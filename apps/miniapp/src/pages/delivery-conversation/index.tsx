import React, { useEffect, useMemo, useRef, useState } from 'react'
import { Button, Image, Input, ScrollView, Text, View } from '@tarojs/components'
import { useDidShow } from '@tarojs/taro'
import type { TencentTextAttempt } from '../../services/e-im'
import AppIcon from '../../components/app-icon'
import { isApiError } from '../../services/api'
import {
  deliveryConversationCanSend,
  getDeliveryOrderConversation,
  loadDeliveryTencentMessages,
  markDeliveryConversationRead,
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
import { useVisibleConversationRead } from '../../hooks/use-visible-conversation-read'
import './index.scss'

type PageStatus = 'loading' | 'ready' | 'error'
type PendingText = { id: string; text: string; time: number; state: 'sending' | 'failed'; error?: string; attempt: TencentTextAttempt }

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
  const selfId = useSessionStore(state => state.profile.id)
  const sessionToken = useSessionStore(state => state.token)
  const operationKey = `${selfId}:${identity}:${orderId}`
  const activeRef = useRef(operationKey)
  activeRef.current = operationKey
  const draftRef = useRef('')
  const sequence = useRef(0)
  const locks = useRef(new Set<string>())
  const imageLock = useRef(false)
  const authorizeRef = useRef<() => Promise<DeliveryConversation>>(async () => { throw new Error('会话不可用') })
  const reconnectRef = useRef<() => void>(() => undefined)
  const [historyLoading, setHistoryLoading] = useState(true)
  useDidShow(() => reconnectRef.current())
  const [conversation, setConversation] = useState<DeliveryConversation | null>(null)
  const [messages, setMessages] = useState<DeliveryTencentMessage[]>([])
  const [pendingMessages, setPendingMessages] = useState<PendingText[]>([])
  const [imageUrls, setImageUrls] = useState<Record<string, string>>({})
  const [draft, setDraft] = useState('')
  const [sending, setSending] = useState(false)
  const [error, setError] = useState('')
  const { inset: keyboardInset, onFocus: onKeyboardFocus, onKeyboardHeightChange, reset: resetKeyboardInset } = useKeyboardInset()
  useVisibleConversationRead(operationKey, messages, isVisible => conversation
    ? markDeliveryConversationRead(conversation, `campus_${selfId}`, () => isVisible() && useSessionStore.getState().status === 'authenticated' && useSessionStore.getState().token === sessionToken && useSessionStore.getState().profile.id === selfId && useSessionStore.getState().identity?.currentIdentity === identity, messages.map(item => item.ID))
    : Promise.resolve(), status === 'ready' && !historyLoading && messages.length > 0)

  useEffect(() => {
    let active = true
    let unsubscribe: (() => void) | undefined
    let authorized: DeliveryConversation | null = null
    let authorizing: Promise<DeliveryConversation> | null = null
    let connecting = false
    activeRef.current = operationKey
    const current = () => active && activeRef.current === operationKey
    const expectedUser = `campus_${selfId}`
    setStatus('loading')
    setConversation(null)
    setMessages([])
    setPendingMessages([])
    setImageUrls({})
    setDraft('')
    draftRef.current = ''
    locks.current.clear()
    setSending(false)
    setError('')
    setHistoryLoading(true)
    resetKeyboardInset()
    const connect = () => {
      if (!current() || !authorized || connecting) return
      connecting = true
      const target = authorized
      void (async () => {
        try {
          const stop = await subscribeDeliveryTencentMessages(target, incoming => {
            const session = useSessionStore.getState()
            if (current() && session.status === 'authenticated' && session.profile.id === selfId && session.identity?.currentIdentity === identity) setMessages(values => mergeMessages(values, incoming))
          }, expectedUser)
          if (!current()) { stop(); return }
          unsubscribe?.()
          unsubscribe = stop
          const history = await loadDeliveryTencentMessages(target, expectedUser)
          if (current()) setMessages(values => mergeMessages(values, history))
        } catch {
          // A background connection failure never blocks composing.
        } finally {
          connecting = false
          if (current()) setHistoryLoading(false)
        }
      })()
    }
    reconnectRef.current = connect
    authorizeRef.current = () => {
      if (!current() || !orderId || !selfId) return Promise.reject(new Error('配送会话入口无效'))
      if (authorized) return Promise.resolve(authorized)
      if (authorizing) return authorizing
      authorizing = getDeliveryOrderConversation(orderId).then(value => {
        if (!current()) throw new Error('账号或会话已切换')
        if (value.deliveryOrderPublicId !== orderId || !value.conversationPublicId || !value.tencentConversationId.startsWith('GROUP')) throw new Error('配送会话校验失败')
        authorized = value
        setConversation(value)
        setStatus('ready')
        setError('')
        connect()
        return value
      }).catch(value => {
        authorizing = null
        if (current()) { setStatus('error'); setError(messageError(value)); setHistoryLoading(false) }
        throw value
      })
      return authorizing
    }
    void authorizeRef.current().catch(() => undefined)
    return () => {
      active = false
      if (activeRef.current === operationKey) activeRef.current = ''
      unsubscribe?.()
    }
  }, [operationKey, orderId, selfId, resetKeyboardInset])

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

  const canCompose = Boolean(orderId && selfId && status !== 'error' && (!conversation || canSend))
  const deliverText = async (item: PendingText) => {
    const key = activeRef.current
    if (!key || locks.current.has(item.id)) return
    locks.current.add(item.id)
    const current = () => {
      const session = useSessionStore.getState()
      return activeRef.current === key && session.status === 'authenticated' && session.profile.id === selfId && session.identity?.currentIdentity === identity
    }
    setPendingMessages(values => values.map(value => value.id === item.id ? { ...value, state: 'sending', error: undefined } : value))
    try {
      const target = await authorizeRef.current()
      if (!current()) return
      if (!deliveryConversationCanSend(target)) throw new Error('订单已结束或会话已只读')
      const sent = await sendDeliveryTencentText(target, item.text, `campus_${selfId}`, item.attempt)
      if (!current()) return
      setMessages(values => mergeMessages(values, [sent]))
      setPendingMessages(values => values.filter(value => value.id !== item.id))
      reconnectRef.current()
    } catch (value) {
      if (current()) setPendingMessages(values => values.map(valueItem => valueItem.id === item.id ? { ...valueItem, state: 'failed', error: messageError(value, '消息发送失败') } : valueItem))
    } finally {
      locks.current.delete(item.id)
    }
  }

  const sendText = async () => {
    const value = draftRef.current.trim()
    if (!value || !canCompose) return
    draftRef.current = ''
    setDraft('')
    const item: PendingText = { id: `pending-${Date.now()}-${++sequence.current}`, text: value, time: Math.floor(Date.now() / 1000), state: 'sending', attempt: {} }
    setPendingMessages(values => [...values, item])
    await deliverText(item)
  }

  const retryText = (item: PendingText) => {
    if (item.state === 'failed') void deliverText(item)
  }

  const sendImage = async () => {
    if (!conversation || !canSend || !conversation.imagesEnabled || imageLock.current) return
    const key = activeRef.current
    const current = () => {
      const session = useSessionStore.getState()
      return activeRef.current === key && session.status === 'authenticated' && session.profile.id === selfId && session.identity?.currentIdentity === identity
    }
    imageLock.current = true
    setSending(true)
    try {
      const [uploaded] = await chooseAndUploadImages('delivery_im', 1, (progress) => {
        void Taro.showLoading({ title: `${progress.message} ${progress.percent}%`, mask: true })
      })
      if (!uploaded || !current()) return
      await waitForMediaApproval(uploaded.assetId, (mediaStatus) => {
        void Taro.showLoading({ title: mediaStatus.message || '图片审核中', mask: true })
      })
      if (!current()) return
      const sent = await sendDeliveryTencentImage(conversation, uploaded.assetId, `campus_${selfId}`)
      if (!current()) return
      setImageUrls((current) => ({ ...current, [sent.ID]: uploaded.previewPath }))
      setMessages((current) => mergeMessages(current, [sent]))
    } catch (value) {
      if (current()) await Taro.showToast({ title: messageError(value, '图片发送失败'), icon: 'none' })
    } finally {
      Taro.hideLoading()
      imageLock.current = false
      if (current()) setSending(false)
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
        {status === 'error' ? (
          <View className='delivery-chat-state delivery-chat-state--error' onClick={() => void authorizeRef.current().catch(() => undefined)}>
            <Text>{error}</Text>
            <Text className='delivery-chat-state__retry'>点击重试</Text>
          </View>
        ) : null}
        {status === 'ready' && !historyLoading && visibleMessages.length === 0 && pendingMessages.length === 0 ? <View className='delivery-chat-state'>会话已建立，发送第一条消息吧</View> : null}
        {[
          ...visibleMessages.filter(message => !pendingMessages.some(item => item.attempt.message?.ID === message.ID)).map(message => ({ message, pending: undefined as PendingText | undefined, time: message.time })),
          ...pendingMessages.map(pending => ({ message: undefined as DeliveryTencentMessage | undefined, pending, time: pending.time }))
        ].sort((a, b) => a.time - b.time).map(({ message, pending }) => {
          if (pending) return <View key={pending.id} className='delivery-chat-row delivery-chat-row--mine'>
            <View className='delivery-chat-bubble delivery-chat-bubble--mine'>
              <Text>{pending.text}</Text>
              {pending.state === 'failed' ? <View className='delivery-chat-retry' role='button' aria-label='重试发送消息' onClick={() => retryText(pending)}><Text>发送失败 · 重试</Text><Text className='delivery-chat-retry__reason'>{pending.error}</Text></View> : null}
              <Text className='delivery-chat-time'>{pending.state === 'sending' ? '发送中' : formatTime(pending.time)}</Text>
            </View>
          </View>
          if (!message) return null
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
          disabled={!canCompose}
          placeholder={canCompose ? '输入消息' : '当前会话不可发送'}
          confirmType='send'
          adjustPosition={false}
          onInput={(event) => { draftRef.current = String(event.detail.value ?? ''); setDraft(draftRef.current) }}
          onFocus={onKeyboardFocus}
          onKeyboardHeightChange={onKeyboardHeightChange}
          onBlur={resetKeyboardInset}
          onConfirm={() => void sendText()}
        />
        <Button className='delivery-chat-send' disabled={!canCompose || !draft.trim()} onClick={() => void sendText()}>发送</Button>
      </View>
    </View>
  )
}
