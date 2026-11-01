import ProductPrice from '../../components/product-price'
import { withFeaturePage } from '../../components/feature-page-gate'
import React, { useEffect, useMemo, useRef, useState } from 'react'
import { Image, ScrollView, Text, Textarea, View } from '@tarojs/components'
import { hideKeyboard, useDidShow } from '@tarojs/taro'
import CapsulePageHeader from '../../components/capsule-page-header'
import AppIcon from '../../components/app-icon'
import ChatAvatar from '../../components/chat-avatar'
import { isApiError } from '../../services/api'
import { contactSeller, getProduct, type IMConversationContext } from '../../services/c-trade'
import {
  getTencentIMLoginUser,
  getIMConversation,
  contactUser,
  loadTencentMessages,
  markTencentConversationRead,
  sendTencentText,
  subscribeTencentMessages,
  type TencentMessage,
  type TencentTextAttempt
} from '../../services/e-im'
import { requestWithSingleSessionRecovery } from '../../services/session-recovery'
import Taro from '../../services/taro'
import { useSessionStore } from '../../stores/session'
import type { Product } from '../../types'
import { useViewportRefresh } from '../../hooks/use-viewport-refresh'
import { useKeyboardInset } from '../../hooks/use-keyboard-inset'
import { useVisibleConversationRead } from '../../hooks/use-visible-conversation-read'
import { conversationPeer } from '../../utils/conversation-presentation'
import './index.scss'

type PageStatus = 'loading' | 'ready' | 'error'
type ChatEntry = { kind: 'product' | 'relation' | 'user'; id: string }
type ViewerRole = 'buyer' | 'seller'
type PendingMessage = { id: string; text: string; time: number; state: 'sending' | 'failed'; error?: string; attempt: TencentTextAttempt }

function parseChatEntry(params: Record<string, string | undefined>): ChatEntry | null {
  const productId = params.productId?.trim() || ''
  const relationId = params.relationId?.trim() || ''
  const userId = params.userId?.trim() || ''
  if ([productId, relationId, userId].filter(Boolean).length !== 1) return null
  const id = productId || relationId || userId
  if (!/^[1-9]\d*$/.test(id)) return null
  return { kind: productId ? 'product' : userId ? 'user' : 'relation', id }
}

function viewerRole(context: IMConversationContext, currentUserId: string): ViewerRole | null {
  if (String(context.buyerId) === currentUserId) return 'buyer'
  if (String(context.sellerId) === currentUserId) return 'seller'
  return null
}

async function requireAuthenticatedUserId(): Promise<string> {
  let session = useSessionStore.getState()
  if (session.status !== 'authenticated') {
    await session.retry()
    session = useSessionStore.getState()
  }
  if (session.status !== 'authenticated' || !session.profile.id) throw new Error('登录恢复失败')
  return session.profile.id
}

function showToast(title: string) {
  void Taro.showToast({ title, icon: 'none', duration: 1800 })
}

function chatErrorMessage(error: unknown): string {
  if (!isApiError(error)) return error instanceof Error ? error.message : '私信服务暂不可用，请稍后重试'
  if (error.status === 0) return '网络连接失败，请检查网络后重试'
  if (error.status === 400) return '私信入口无效，请从用户主页或商品详情重新进入'
  if (error.status === 401) return '登录状态已失效，请重新登录'
  if (error.status === 403) return error.message || '当前无法与该用户私信'
  if (error.status === 404) return '会话不存在或无权访问'
  if (error.status === 409 || error.status === 422) return '当前宝贝无法发起私信'
  return error.message || '私信服务暂不可用，请稍后重试'
}

function mergeMessages(current: TencentMessage[], next: TencentMessage[]): TencentMessage[] {
  const values = new Map(current.map((item) => [item.ID, item]))
  next.forEach((item) => values.set(item.ID, item))
  return [...values.values()].sort((left, right) => left.time - right.time)
}

function messageText(message: TencentMessage): string {
  if (message.type !== 'TIMTextElem') return ''
  return String(message.payload?.text ?? '').trim()
}

function messageDisplayText(message: TencentMessage): string {
  const text = messageText(message)
  return text || '暂不支持的消息类型'
}

function dayKey(value: number): string {
  const date = new Date(value * 1000)
  return `${date.getFullYear()}-${date.getMonth()}-${date.getDate()}`
}

function formatDayLabel(value: number): string {
  const date = new Date(value * 1000)
  const now = new Date()
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime()
  const target = new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime()
  const days = Math.round((today - target) / 86400000)
  if (days === 0) return '今天'
  if (days === 1) return '昨天'
  return `${date.getFullYear()}年${date.getMonth() + 1}月${date.getDate()}日`
}

function formatTime(value: number): string {
  const date = new Date(value * 1000)
  return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
}

function SecondhandChatPage() {
  const capsuleStyle = useViewportRefresh()
  const [params] = useState(() => Taro.getCurrentInstance().router?.params ?? {})
  const entry = parseChatEntry(params)
  const selfProfile = useSessionStore((state) => state.profile)
  const sessionToken = useSessionStore(state => state.token)
  const sessionRole = useSessionStore(state => state.identity?.currentIdentity)
  const sessionUserId = selfProfile.id
  const [status, setStatus] = useState<PageStatus>('loading')
  const chatOperationKey = `${sessionUserId}:${entry?.kind || ''}:${entry?.id || ''}`
  const activeChatKeyRef = useRef(chatOperationKey)
  activeChatKeyRef.current = chatOperationKey
  const [product, setProduct] = useState<Product | null>(null)
  const [context, setContext] = useState<IMConversationContext | null>(null)
  const [role, setRole] = useState<ViewerRole | null>(null)
  const [tencentUserId, setTencentUserId] = useState('')
  const [messages, setMessages] = useState<TencentMessage[]>([])
  const [pendingMessages, setPendingMessages] = useState<PendingMessage[]>([])
  const [draft, setDraft] = useState('')
  const draftRef = useRef('')
  const outgoingLocks = useRef(new Set<string>())
  const authorizeRef = useRef<() => Promise<IMConversationContext>>(async () => { throw new Error('会话暂不可用') })
  const reconnectRef = useRef<() => void>(() => undefined)
  const pendingSequence = useRef(0)
  const [historyLoading, setHistoryLoading] = useState(true)
  const [productImageFailed, setProductImageFailed] = useState(false)
  const [inputFocused, setInputFocused] = useState(false)
  const [errorMessage, setErrorMessage] = useState('')
  const { inset: keyboardHeight, onFocus: onKeyboardFocus, onKeyboardHeightChange, reset: resetKeyboardInset } = useKeyboardInset()
  useDidShow(() => reconnectRef.current())
  useVisibleConversationRead(chatOperationKey, messages, isVisible => context
    ? markTencentConversationRead(context, `campus_${sessionUserId}`, () => isVisible() && useSessionStore.getState().status === 'authenticated' && useSessionStore.getState().token === sessionToken && useSessionStore.getState().identity?.currentIdentity === sessionRole && useSessionStore.getState().profile.id === sessionUserId, messages.map(item => item.ID))
    : Promise.resolve(), status === 'ready' && !historyLoading && messages.length > 0)

  useEffect(() => {
    let active = true
    let unsubscribe: (() => void) | undefined
    const operationKey = chatOperationKey
    activeChatKeyRef.current = operationKey
    const isActive = () => active && activeChatKeyRef.current === operationKey

    setStatus('loading')
    setProduct(null)
    setContext(null)
    setRole(null)
    setTencentUserId('')
    setMessages([])
    setPendingMessages([])
    setDraft('')
    draftRef.current = ''
    outgoingLocks.current.clear()
    setHistoryLoading(true)
    setProductImageFailed(false)
    setInputFocused(false)
    setErrorMessage('')
    resetKeyboardInset()

    if (!entry) {
      setStatus('error')
      setErrorMessage('私信入口无效，请从用户主页或商品详情重新进入。')
      return () => {
        active = false
      }
    }

    let authorized: IMConversationContext | null = null
    let authorizing: Promise<IMConversationContext> | null = null
    let connecting = false
    const connect = () => {
      if (!isActive() || !authorized || connecting) return
      connecting = true
      const nextContext = authorized
      const expectedTencentUserId = `campus_${sessionUserId}`
      void (async () => {
        try {
          const loggedIn = await getTencentIMLoginUser(expectedTencentUserId)
          if (loggedIn !== expectedTencentUserId || !isActive()) return
          // The SDK may have been recreated after a disconnected session.
          // Bind to the current instance before dropping the old subscription.
          const stop = await subscribeTencentMessages(nextContext, incoming => {
            const session = useSessionStore.getState()
            if (isActive() && session.status === 'authenticated' && session.profile.id === sessionUserId) setMessages(current => mergeMessages(current, incoming))
          }, expectedTencentUserId)
          if (!isActive()) { stop(); return }
          unsubscribe?.()
          unsubscribe = stop
          const history = await loadTencentMessages(nextContext, expectedTencentUserId)
          if (isActive()) setMessages(current => mergeMessages(current, history))
        } catch {
          // SDK reconnects silently. Foregrounding or a successful send also
          // retries history/subscription after a failed initial connection.
        } finally {
          connecting = false
          if (isActive()) setHistoryLoading(false)
        }
      })()
    }
    reconnectRef.current = connect
    const resolveContext = async (): Promise<IMConversationContext> => {
        const authenticatedId = await requireAuthenticatedUserId()
        if (!isActive() || authenticatedId !== sessionUserId) throw new Error('会话已切换')
        let nextContext = await requestWithSingleSessionRecovery(() => (
          entry.kind === 'product'
            ? contactSeller(entry.id)
            : entry.kind === 'user' ? contactUser(entry.id) : getIMConversation(Number(entry.id))
        ))
        const recoveredSession = useSessionStore.getState()
        if (recoveredSession.status !== 'authenticated' || !recoveredSession.profile.id) {
          throw new Error('登录恢复失败')
        }
        const currentUserId = recoveredSession.profile.id
        const expectedTencentUserId = `campus_${currentUserId}`
        const nextRole = viewerRole(nextContext, currentUserId)
        const validRelation = Number.isInteger(nextContext.relationId) && nextContext.relationId > 0
        const isUserConversation = nextContext.referenceType === 'user'
        const validReference = isUserConversation
          ? nextContext.productId === 0
          : (!nextContext.referenceType || nextContext.referenceType === 'product') && Number.isInteger(nextContext.productId) && nextContext.productId > 0
        const peerID = nextRole === 'buyer' ? nextContext.sellerId : nextContext.buyerId
        const validTencentPeer = Number.isSafeInteger(peerID) && peerID > 0 && String(peerID) !== currentUserId && nextContext.peerUserId === `campus_${peerID}` && nextContext.conversationId === `C2C${nextContext.peerUserId}`
        const validEntry = entry.kind === 'user' ? isUserConversation && String(peerID) === entry.id
          : entry.kind === 'product' ? !isUserConversation && String(nextContext.productId) === entry.id
          : String(nextContext.relationId) === entry.id
        if (!nextRole || !validRelation || !validReference || !validTencentPeer || !validEntry) {
          throw new Error('会话参与者校验失败')
        }
        if (!isActive()) throw new Error('会话已切换')
        // Resolve authorization before optional product/profile data and before
        // IM login. Queued messages can proceed without waiting for either.
        authorized = nextContext
        setContext(nextContext)
        setRole(nextRole)
        setTencentUserId(expectedTencentUserId)
        setStatus('ready')
        // Contact establishes the relation. Fetch its authorized presentation
        // data separately, without changing the relation or blocking old APIs.
        if (entry.kind === 'product' && !nextContext.peer) void getIMConversation(nextContext.relationId).then(detail => {
          if (isActive() && detail.relationId === nextContext.relationId && detail.buyerId === nextContext.buyerId && detail.sellerId === nextContext.sellerId) setContext({ ...nextContext, peer: detail.peer })
        }).catch(() => undefined)
        if (!isUserConversation) {
          void getProduct(String(nextContext.productId)).then(value => { if (isActive()) setProduct(value) }).catch(() => undefined)
        }
        // The relation is ready for composing as soon as the server authorizes
        // it. Tencent IM login/history/subscription continue in the background.
        connect()
        return nextContext
    }
    authorizeRef.current = () => {
      if (!isActive()) return Promise.reject(new Error('会话已切换'))
      if (authorized) return Promise.resolve(authorized)
      if (authorizing) return authorizing
      authorizing = resolveContext().catch(error => { authorizing = null; throw error })
      return authorizing
    }
    void authorizeRef.current().catch(error => {
        unsubscribe?.()
        unsubscribe = undefined
        if (!isActive()) return
        setProduct(null)
        setContext(null)
        setRole(null)
        setTencentUserId('')
        const message = chatErrorMessage(error)
        setErrorMessage(message)
        setStatus('error')
        setHistoryLoading(false)
    })

    return () => {
      active = false
      if (activeChatKeyRef.current === operationKey) activeChatKeyRef.current = ''
      unsubscribe?.()
    }
  }, [chatOperationKey, entry?.id, entry?.kind, resetKeyboardInset, sessionUserId])

  const visibleMessages = useMemo(() => messages.filter(message => !pendingMessages.some(item => item.attempt.message?.ID === message.ID)), [messages, pendingMessages])
  const scrollAnchorId = `secondhand-chat-bottom-${visibleMessages.length + pendingMessages.length}-${keyboardHeight}-${inputFocused ? 1 : 0}`
  const peer = conversationPeer(context, product, sessionUserId)
  const isDirect = context?.referenceType === 'user' || (!context && entry?.kind === 'user')
  const peerState = isDirect ? '私信' : role === 'seller' ? '咨询买家 · 二手交流' : '宝贝卖家 · 二手交流'
  const canOpenPeerProfile = Boolean(peer.id) && status === 'ready'
  const resolvedProductId = isDirect ? '' : context
    ? String(context.productId)
    : entry?.kind === 'product' ? entry.id : ''
  const productImage = product?.images[0]

  const goBack = async () => {
    if (inputFocused || keyboardHeight > 0) {
      try {
        await hideKeyboard()
      } catch {
        // Keyboard state is best-effort on older runtimes.
      }
      setInputFocused(false)
      resetKeyboardInset()
      return
    }
    const fallback = entry?.kind === 'product'
      ? `/pages/secondhand-detail/index?id=${encodeURIComponent(entry.id)}`
      : entry?.kind === 'user' ? `/pages/user-profile/index?userId=${encodeURIComponent(entry.id)}` : '/pages/home/index?tab=messages'
    void Taro.navigateBack().catch(() => Taro.redirectTo({ url: fallback }))
  }

  const openPeerProfile = () => {
    if (!canOpenPeerProfile) return
    void Taro.navigateTo({
      url: `/pages/user-profile/index?userId=${encodeURIComponent(peer.id)}${isDirect ? '' : '&tab=secondhand'}`
    })
  }

  const openProduct = () => {
    if (!resolvedProductId) return
    void Taro.navigateTo({ url: `/pages/secondhand-detail/index?id=${encodeURIComponent(resolvedProductId)}` })
  }

  const deliverText = async (item: PendingMessage) => {
    const operationKey = activeChatKeyRef.current
    const sendingUserId = sessionUserId
    if (!operationKey || outgoingLocks.current.has(item.id)) return
    outgoingLocks.current.add(item.id)
    const isCurrent = () => activeChatKeyRef.current === operationKey && useSessionStore.getState().profile.id === sendingUserId && useSessionStore.getState().status === 'authenticated'
    setPendingMessages(current => current.map(value => value.id === item.id ? { ...value, state: 'sending', error: undefined } : value))
    try {
      const authorized = await authorizeRef.current()
      if (!isCurrent()) return
      const message = await sendTencentText(authorized, item.text, `campus_${sendingUserId}`, item.attempt)
      if (!isCurrent()) return
      setMessages((current) => mergeMessages(current, [message]))
      setPendingMessages((current) => current.filter((value) => value.id !== item.id))
      reconnectRef.current()
    } catch (error) {
      if (!isCurrent()) return
      const reason = chatErrorMessage(error)
      setPendingMessages((current) => current.map((value) => value.id === item.id ? { ...value, state: 'failed', error: reason } : value))
    } finally {
      outgoingLocks.current.delete(item.id)
    }
  }

  const sendText = async () => {
    const value = draftRef.current.trim()
    if (!value || !entry || !sessionUserId || (status === 'error' && !context)) return
    draftRef.current = '' // native confirm + tap in one frame must enqueue once
    const item: PendingMessage = { id: `pending-${Date.now()}-${++pendingSequence.current}`, text: value, time: Math.floor(Date.now() / 1000), state: 'sending', attempt: {} }
    setPendingMessages((current) => [...current, item])
    setDraft('')
    await deliverText(item)
  }

  const retryPending = (item: PendingMessage) => {
    if (item.state === 'failed') void deliverText(item)
  }

  return (
    <View className='secondhand-chat-page' style={{ ...capsuleStyle, paddingBottom: `${keyboardHeight}px` }}>
      <CapsulePageHeader
        className='secondhand-chat-header'
        variant='standard'
        left={<View className='secondhand-chat-header__back' role='button' aria-label='返回' hoverClass='secondhand-chat-header__back--pressed' onClick={() => void goBack()}><AppIcon name='back' /></View>}
        center={
          <View className='secondhand-chat-seller' role={canOpenPeerProfile ? 'button' : undefined} aria-label={canOpenPeerProfile ? `查看${peer.name}的主页` : undefined} hoverClass={canOpenPeerProfile ? 'secondhand-chat-seller--pressed' : ''} onClick={canOpenPeerProfile ? openPeerProfile : undefined}>
            <ChatAvatar className='secondhand-chat-seller__avatar' src={peer.avatar} name={peer.name} />
            <View className='secondhand-chat-seller__meta'>
            <Text className='secondhand-chat-seller__name'>{peer.name}</Text>
              <Text className='secondhand-chat-seller__state'>
                {status === 'error' ? '暂时不可用' : peerState}
              </Text>
            </View>
          </View>
        }
        right={
          canOpenPeerProfile ? <View
            className='secondhand-chat-header__home'
            role='button'
            aria-label={`查看${peer.name}的主页`}
            hoverClass='secondhand-chat-header__home--pressed'
            onClick={openPeerProfile}
          >
            <Text>主页</Text>
            <AppIcon name='chevron-right' className='secondhand-chat-header__home-icon' />
          </View> : null
        }
      />

      {!isDirect && (context || entry?.kind === 'product') && <View className='secondhand-chat-product'>
        <View className='secondhand-chat-product__image'>
          {productImage && !productImageFailed ? (
            <Image className='secondhand-chat-product__image-content' src={productImage} mode='aspectFill' onError={() => setProductImageFailed(true)} />
          ) : (
            <AppIcon name='market' className='secondhand-chat-product__placeholder' />
          )}
        </View>
        <View className='secondhand-chat-product__main'>
          <Text className='secondhand-chat-product__title'>
            {product?.title || (status === 'loading' ? '正在读取宝贝信息…' : '商品信息暂不可用')}
          </Text>
          {product ? <ProductPrice className='secondhand-chat-product__price' value={product.price} size='compact' /> : null}
        </View>
        <View
          className={`secondhand-chat-product__button ${resolvedProductId ? '' : 'secondhand-chat-product__button--disabled'}`}
          hoverClass={resolvedProductId ? 'secondhand-chat-product__button--pressed' : ''}
          role='button'
          aria-label='查看商品详情'
          onClick={resolvedProductId ? openProduct : undefined}
        >
          <Text>查看宝贝</Text>
        </View>
      </View>}

      <ScrollView
        className='secondhand-chat-scroll'
        scrollY
        enhanced
        showScrollbar={false}
        scrollIntoView={scrollAnchorId}
      >
        <View className='secondhand-chat-scroll__inner'>
        <View className='secondhand-chat-safe-tip'>
          <Text>{isDirect ? '友善交流，注意保护个人隐私' : '平台不担保交易，请当面验货后再付款'}</Text>
        </View>

        {status === 'error' && pendingMessages.length === 0 ? (
          <View className='secondhand-chat-state secondhand-chat-state--retry' onClick={() => { void authorizeRef.current().catch(error => setErrorMessage(chatErrorMessage(error))) }}>
            <Text className='secondhand-chat-state__title'>会话暂不可用，点击重试</Text>
            <Text className='secondhand-chat-state__desc'>{errorMessage || '请检查网络或登录状态后再试。'}</Text>
          </View>
        ) : !historyLoading && visibleMessages.length === 0 && pendingMessages.length === 0 ? (
          <View className='secondhand-chat-state'>
            <Text className='secondhand-chat-state__title'>开始交流</Text>
            <Text className='secondhand-chat-state__desc'>
              {isDirect ? '发一句问候，开始交流吧。' : role === 'seller' ? '回复买家确认交易。' : '发送第一条文字消息，与卖家确认宝贝情况。'}
            </Text>
          </View>
        ) : (
          [...visibleMessages.map((message) => ({ kind: 'remote' as const, message })), ...pendingMessages.map((pending) => ({ kind: 'pending' as const, pending }))]
            .sort((left, right) => (left.kind === 'remote' ? left.message.time : left.pending.time) - (right.kind === 'remote' ? right.message.time : right.pending.time))
            .map((item, index, items) => {
            const message = item.kind === 'remote' ? item.message : null
            const pending = item.kind === 'pending' ? item.pending : null
            const mine = tencentUserId
              ? pending ? true : String(message?.from || '') === tencentUserId
              : pending ? true : message?.flow === 'out'
            const time = pending?.time ?? message?.time ?? 0
            const previous = items[index - 1]
            const previousTime = previous ? (previous.kind === 'remote' ? previous.message.time : previous.pending.time) : 0
            const showDay = !previous || dayKey(previousTime) !== dayKey(time)
            return (
              <React.Fragment key={pending?.id ?? message?.ID}>
                {showDay && <View className='secondhand-chat-day-divider'><Text>{formatDayLabel(time)}</Text></View>}
                <View className={`secondhand-chat-row ${mine ? 'secondhand-chat-row--me' : 'secondhand-chat-row--seller'}`}>
                  {!mine && (
                    <View className='secondhand-chat-avatar-touch' role='button' aria-label={`查看${peer.name}的主页`} onClick={openPeerProfile}>
                      <ChatAvatar className='secondhand-chat-avatar' src={peer.avatar} name={peer.name} />
                    </View>
                  )}
                  <View className={`secondhand-chat-bubble ${mine ? 'secondhand-chat-bubble--me' : 'secondhand-chat-bubble--seller'}`}>
                    <Text className='secondhand-chat-bubble__text' selectable>{pending ? pending.text : messageDisplayText(message!)}</Text>
                    {pending?.state === 'failed' ? <View className='secondhand-chat-bubble__retry' role='button' aria-label={`发送失败：${pending.error || '请重试'}`} onClick={() => retryPending(pending)}><Text>发送失败 · 重试</Text><Text className='secondhand-chat-bubble__reason'>{pending.error}</Text></View> : null}
                    <Text className='secondhand-chat-bubble__time'>{pending?.state === 'sending' ? '发送中' : formatTime(time)}</Text>
                  </View>
                  {mine && (
                    <ChatAvatar className='secondhand-chat-avatar secondhand-chat-avatar--me' src={selfProfile.avatar} name={selfProfile.nickname || '我'} />
                  )}
                </View>
              </React.Fragment>
            )
            })
        )}
        <View id={scrollAnchorId} className='secondhand-chat-scroll__anchor' />
        </View>
      </ScrollView>

      <View
        className={`secondhand-chat-inputbar ${inputFocused || keyboardHeight > 0 ? 'secondhand-chat-inputbar--keyboard' : ''}`}
      >
        <View className='secondhand-chat-input-wrap'>
          <Textarea
            className='secondhand-chat-input'
            value={draft}
            maxlength={500}
            autoHeight
            disabled={!entry || !sessionUserId || status === 'error' && !context}
            placeholder='输入消息…'
            placeholderClass='secondhand-chat-input__placeholder'
            confirmType='send'
            adjustPosition={false}
            cursorSpacing={0}
            showConfirmBar={false}
            holdKeyboard
            onInput={(event) => { draftRef.current = String(event.detail.value ?? ''); setDraft(draftRef.current) }}
            onFocus={() => { setInputFocused(true); onKeyboardFocus() }}
            onKeyboardHeightChange={onKeyboardHeightChange}
            onBlur={() => { setInputFocused(false); resetKeyboardInset() }}
            onConfirm={() => void sendText()}
          />
        </View>
        <View
          className={`secondhand-chat-send ${draft.trim() && entry && sessionUserId ? 'secondhand-chat-send--active' : ''}`}
          role='button'
          aria-label='发送消息'
          aria-disabled={!draft.trim() || !entry || !sessionUserId}
          hoverClass={draft.trim() && entry && sessionUserId ? 'secondhand-chat-send--pressed' : ''}
          onClick={() => void sendText()}
        >
          <Text>发送</Text>
        </View>
      </View>
    </View>
  )
}

export default withFeaturePage(SecondhandChatPage)
