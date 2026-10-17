import ProductPrice from '../../components/product-price'
import { withFeaturePage } from '../../components/feature-page-gate'
import React, { useEffect, useMemo, useRef, useState } from 'react'
import { Image, ScrollView, Text, Textarea, View } from '@tarojs/components'
import { hideKeyboard } from '@tarojs/taro'
import CapsulePageHeader from '../../components/capsule-page-header'
import AppIcon from '../../components/app-icon'
import ChatAvatar from '../../components/chat-avatar'
import { isApiError } from '../../services/api'
import { contactSeller, getProduct, type IMConversationContext } from '../../services/c-trade'
import {
  getTencentIMLoginUser,
  getIMConversation,
  loadTencentMessages,
  sendTencentText,
  subscribeTencentMessages,
  type TencentMessage
} from '../../services/e-im'
import { requestWithSingleSessionRecovery } from '../../services/session-recovery'
import Taro from '../../services/taro'
import { useSessionStore } from '../../stores/session'
import type { Product } from '../../types'
import { useViewportRefresh } from '../../hooks/use-viewport-refresh'
import { useKeyboardInset } from '../../hooks/use-keyboard-inset'
import { conversationPeer } from '../../utils/conversation-presentation'
import './index.scss'

type PageStatus = 'loading' | 'ready' | 'error'
type ChatEntry = { kind: 'product' | 'relation'; id: string }
type ViewerRole = 'buyer' | 'seller'

function parseChatEntry(params: Record<string, string | undefined>): ChatEntry | null {
  const productId = params.productId?.trim() || ''
  const relationId = params.relationId?.trim() || ''
  if (Boolean(productId) === Boolean(relationId)) return null
  const id = productId || relationId
  if (!/^[1-9]\d*$/.test(id)) return null
  return { kind: productId ? 'product' : 'relation', id }
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
  if (error.status === 400) return '私信入口无效，请从商品详情或首页重新进入'
  if (error.status === 401 || error.status === 403) return '登录状态已失效，请重新登录'
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
  const params = Taro.getCurrentInstance().router?.params ?? {}
  const entry = parseChatEntry(params)
  const selfProfile = useSessionStore((state) => state.profile)
  const sessionUserId = selfProfile.id
  const [status, setStatus] = useState<PageStatus>('loading')
  const [reloadKey, setReloadKey] = useState(0)
  const chatOperationKey = `${sessionUserId}:${entry?.kind || ''}:${entry?.id || ''}:${reloadKey}`
  const activeChatKeyRef = useRef(chatOperationKey)
  activeChatKeyRef.current = chatOperationKey
  const [product, setProduct] = useState<Product | null>(null)
  const [context, setContext] = useState<IMConversationContext | null>(null)
  const [role, setRole] = useState<ViewerRole | null>(null)
  const [tencentUserId, setTencentUserId] = useState('')
  const [messages, setMessages] = useState<TencentMessage[]>([])
  const [draft, setDraft] = useState('')
  const [sending, setSending] = useState(false)
  const sendLockRef = useRef<object | null>(null)
  const [historyLoading, setHistoryLoading] = useState(true)
  const [productImageFailed, setProductImageFailed] = useState(false)
  const [inputFocused, setInputFocused] = useState(false)
  const [errorMessage, setErrorMessage] = useState('')
  const { inset: keyboardHeight, onFocus: onKeyboardFocus, onKeyboardHeightChange, reset: resetKeyboardInset } = useKeyboardInset()

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
    setDraft('')
    setSending(false)
    sendLockRef.current = null
    setHistoryLoading(true)
    setProductImageFailed(false)
    setInputFocused(false)
    setErrorMessage('')
    resetKeyboardInset()

    if (!entry) {
      setStatus('error')
      setErrorMessage('私信入口无效，请从商品详情或首页重新进入。')
      return () => {
        active = false
      }
    }

    void (async () => {
      try {
        await requireAuthenticatedUserId()
        if (!isActive()) return
        let nextContext = await requestWithSingleSessionRecovery(() => (
          entry.kind === 'product'
            ? contactSeller(entry.id)
            : getIMConversation(Number(entry.id))
        ))
        const recoveredSession = useSessionStore.getState()
        if (recoveredSession.status !== 'authenticated' || !recoveredSession.profile.id) {
          throw new Error('登录恢复失败')
        }
        const currentUserId = recoveredSession.profile.id
        const expectedTencentUserId = `campus_${currentUserId}`
        const nextRole = viewerRole(nextContext, currentUserId)
        const validRelation = Number.isInteger(nextContext.relationId) && nextContext.relationId > 0
        const validProduct = Number.isInteger(nextContext.productId) && nextContext.productId > 0
        const validTencentPeer = Boolean(nextContext.peerUserId) && nextContext.conversationId === `C2C${nextContext.peerUserId}`
        if (!nextRole || !validRelation || !validProduct || !validTencentPeer) {
          throw new Error('会话参与者校验失败')
        }
        if (!isActive()) return
        // Contact establishes the relation. Fetch its authorized presentation
        // data separately, without changing the relation or blocking old APIs.
        if (entry.kind === 'product' && !nextContext.peer) {
          try {
            const detail = await getIMConversation(nextContext.relationId)
            if (detail.relationId === nextContext.relationId && detail.buyerId === nextContext.buyerId && detail.sellerId === nextContext.sellerId) {
              nextContext = { ...nextContext, peer: detail.peer }
            }
          } catch {
            // Public profile data is optional; authorized text chat still works.
          }
        }
        if (!isActive()) return
        const tencentLoginUser = await getTencentIMLoginUser(expectedTencentUserId)
        if (tencentLoginUser !== expectedTencentUserId) {
          throw new Error('腾讯云 IM 账号与当前登录用户不一致，请返回“我的”重新登录')
        }
        let nextProduct: Product | null = null
        try {
          nextProduct = await getProduct(String(nextContext.productId))
        } catch {
          nextProduct = null
        }
        if (!isActive()) return

        setContext(nextContext)
        setProduct(nextProduct)
        setRole(nextRole)
        setTencentUserId(tencentLoginUser)
        unsubscribe = await subscribeTencentMessages(nextContext, (incoming) => {
          if (!isActive()) return
          setMessages((current) => mergeMessages(current, incoming))
        }, expectedTencentUserId)
        if (!isActive()) {
          unsubscribe()
          unsubscribe = undefined
          return
        }

        setStatus('ready')
        try {
          const history = await loadTencentMessages(nextContext, expectedTencentUserId)
          if (!isActive()) return
          setMessages((current) => mergeMessages(current, history))
        } catch (error) {
          if (isActive()) showToast(`历史消息读取失败：${chatErrorMessage(error)}`)
        } finally {
          if (isActive()) setHistoryLoading(false)
        }
      } catch (error) {
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
        showToast(message)
      }
    })()

    return () => {
      active = false
      if (activeChatKeyRef.current === operationKey) activeChatKeyRef.current = ''
      unsubscribe?.()
    }
  }, [chatOperationKey, entry?.id, entry?.kind, reloadKey, resetKeyboardInset, sessionUserId])

  const visibleMessages = useMemo(() => messages, [messages])
  const scrollAnchorId = `secondhand-chat-bottom-${visibleMessages.length}-${keyboardHeight}-${inputFocused ? 1 : 0}`
  const peer = conversationPeer(context, product, sessionUserId)
  const peerState = role === 'seller' ? '咨询买家 · 二手交流' : '宝贝卖家 · 二手交流'
  const canOpenPeerProfile = Boolean(peer.id) && status === 'ready'
  const resolvedProductId = context
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
      : '/pages/home/index'
    void Taro.navigateBack().catch(() => Taro.redirectTo({ url: fallback }))
  }

  const openPeerProfile = () => {
    if (!canOpenPeerProfile) return
    void Taro.navigateTo({
      url: `/pages/user-profile/index?userId=${encodeURIComponent(peer.id)}&tab=secondhand`
    })
  }

  const openProduct = () => {
    if (!resolvedProductId) return
    void Taro.navigateTo({ url: `/pages/secondhand-detail/index?id=${encodeURIComponent(resolvedProductId)}` })
  }

  const sendText = async () => {
    const value = draft.trim()
    if (!value) {
      showToast('先输入消息')
      return
    }
    if (!context || !role || !tencentUserId || status !== 'ready' || sendLockRef.current) return

    const operationKey = activeChatKeyRef.current
    const sendingUserId = sessionUserId
    if (!operationKey || useSessionStore.getState().profile.id !== sendingUserId) return
    const sendLock = {}
    sendLockRef.current = sendLock
    setSending(true)
    try {
      const message = await sendTencentText(context, value, tencentUserId)
      if (
        activeChatKeyRef.current !== operationKey ||
        useSessionStore.getState().profile.id !== sendingUserId
      ) return
      setMessages((current) => mergeMessages(current, [message]))
      setDraft((current) => current.trim() === value ? '' : current)
    } catch (error) {
      if (activeChatKeyRef.current !== operationKey) return
      showToast(chatErrorMessage(error))
    } finally {
      if (sendLockRef.current === sendLock) {
        sendLockRef.current = null
        if (activeChatKeyRef.current === operationKey) setSending(false)
      }
    }
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
              <Text className='secondhand-chat-seller__name'>{status === 'loading' ? '正在连接…' : peer.name}</Text>
              <Text className='secondhand-chat-seller__state'>
                {status === 'ready' ? peerState : status === 'error' ? '连接失败' : '稍等片刻'}
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

      <View className='secondhand-chat-product'>
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
      </View>

      <ScrollView
        className='secondhand-chat-scroll'
        scrollY
        enhanced
        showScrollbar={false}
        scrollIntoView={scrollAnchorId}
      >
        <View className='secondhand-chat-scroll__inner'>
        <View className='secondhand-chat-safe-tip'>
          <Text>平台不担保交易，请当面验货后再付款</Text>
        </View>

        {status === 'loading' ? (
          <View className='secondhand-chat-state'>
            <Text className='secondhand-chat-state__title'>正在连接私信…</Text>
            <Text className='secondhand-chat-state__desc'>连接后即可继续交流。</Text>
          </View>
        ) : status === 'error' ? (
          <View className='secondhand-chat-state secondhand-chat-state--retry' onClick={() => setReloadKey((value) => value + 1)}>
            <Text className='secondhand-chat-state__title'>私信连接失败，点击重试</Text>
            <Text className='secondhand-chat-state__desc'>{errorMessage || '请检查网络或登录状态后再试。'}</Text>
          </View>
        ) : historyLoading && visibleMessages.length === 0 ? (
          <View className='secondhand-chat-state'><Text className='secondhand-chat-state__desc'>正在读取消息…</Text></View>
        ) : visibleMessages.length === 0 ? (
          <View className='secondhand-chat-state'>
            <Text className='secondhand-chat-state__title'>会话已建立</Text>
            <Text className='secondhand-chat-state__desc'>
              {role === 'seller' ? '回复买家确认交易。' : '发送第一条文字消息，与卖家确认宝贝情况。'}
            </Text>
          </View>
        ) : (
          visibleMessages.map((message, index) => {
            const mine = tencentUserId
              ? String(message.from || '') === tencentUserId
              : message.flow === 'out'
            const previous = visibleMessages[index - 1]
            const showDay = !previous || dayKey(previous.time) !== dayKey(message.time)
            return (
              <React.Fragment key={message.ID}>
                {showDay && <View className='secondhand-chat-day-divider'><Text>{formatDayLabel(message.time)}</Text></View>}
                <View className={`secondhand-chat-row ${mine ? 'secondhand-chat-row--me' : 'secondhand-chat-row--seller'}`}>
                  {!mine && (
                    <View className='secondhand-chat-avatar-touch' role='button' aria-label={`查看${peer.name}的主页`} onClick={openPeerProfile}>
                      <ChatAvatar className='secondhand-chat-avatar' src={peer.avatar} name={peer.name} />
                    </View>
                  )}
                  <View className={`secondhand-chat-bubble ${mine ? 'secondhand-chat-bubble--me' : 'secondhand-chat-bubble--seller'}`}>
                    <Text className='secondhand-chat-bubble__text' selectable>{messageDisplayText(message)}</Text>
                    <Text className='secondhand-chat-bubble__time'>{formatTime(message.time)}</Text>
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
            disabled={status !== 'ready' || !role}
            placeholder={status === 'ready' ? '发送消息…' : status === 'error' ? '重新连接后即可发送' : '私信连接中…'}
            placeholderClass='secondhand-chat-input__placeholder'
            confirmType='send'
            adjustPosition={false}
            cursorSpacing={0}
            showConfirmBar={false}
            holdKeyboard
            onInput={(event) => setDraft(String(event.detail.value ?? ''))}
            onFocus={() => { setInputFocused(true); onKeyboardFocus() }}
            onKeyboardHeightChange={onKeyboardHeightChange}
            onBlur={() => { setInputFocused(false); resetKeyboardInset() }}
            onConfirm={() => void sendText()}
          />
        </View>
        <View
          className={`secondhand-chat-send ${draft.trim() && status === 'ready' && role && !sending ? 'secondhand-chat-send--active' : ''}`}
          role='button'
          aria-label={sending ? '消息发送中' : '发送消息'}
          aria-disabled={!draft.trim() || status !== 'ready' || !role || sending}
          hoverClass={draft.trim() && status === 'ready' && role && !sending ? 'secondhand-chat-send--pressed' : ''}
          onClick={() => void sendText()}
        >
          <Text>{sending ? '发送中' : '发送'}</Text>
        </View>
      </View>
    </View>
  )
}

export default withFeaturePage(SecondhandChatPage)
