import React, { useEffect, useMemo, useState } from 'react'
import { Image, Input, ScrollView, Text, View } from '@tarojs/components'
import { isApiError } from '../../services/api'
import { contactSeller, getProduct, type IMConversationContext } from '../../services/c-trade'
import {
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

function formatTime(value: number): string {
  const date = new Date(value * 1000)
  return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
}

export default function SecondhandChatPage() {
  const params = Taro.getCurrentInstance().router?.params ?? {}
  const entry = parseChatEntry(params)
  const [status, setStatus] = useState<PageStatus>('loading')
  const [reloadKey, setReloadKey] = useState(0)
  const [product, setProduct] = useState<Product | null>(null)
  const [context, setContext] = useState<IMConversationContext | null>(null)
  const [role, setRole] = useState<ViewerRole | null>(null)
  const [messages, setMessages] = useState<TencentMessage[]>([])
  const [draft, setDraft] = useState('')
  const [sending, setSending] = useState(false)
  const [errorMessage, setErrorMessage] = useState('')

  useEffect(() => {
    let active = true
    let unsubscribe: (() => void) | undefined

    setStatus('loading')
    setProduct(null)
    setContext(null)
    setRole(null)
    setMessages([])
    setErrorMessage('')

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
        const nextContext = await requestWithSingleSessionRecovery(() => (
          entry.kind === 'product'
            ? contactSeller(entry.id)
            : getIMConversation(Number(entry.id))
        ))
        const recoveredSession = useSessionStore.getState()
        if (recoveredSession.status !== 'authenticated' || !recoveredSession.profile.id) {
          throw new Error('登录恢复失败')
        }
        const nextRole = viewerRole(nextContext, recoveredSession.profile.id)
        const validRelation = Number.isInteger(nextContext.relationId) && nextContext.relationId > 0
        const validProduct = Number.isInteger(nextContext.productId) && nextContext.productId > 0
        const validTencentPeer = Boolean(nextContext.peerUserId) && nextContext.conversationId === `C2C${nextContext.peerUserId}`
        if (!nextRole || !validRelation || !validProduct || !validTencentPeer) {
          throw new Error('会话参与者校验失败')
        }
        let nextProduct: Product | null = null
        try {
          nextProduct = await getProduct(String(nextContext.productId))
        } catch {
          nextProduct = null
        }
        if (!active) return

        setContext(nextContext)
        setProduct(nextProduct)
        setRole(nextRole)
        unsubscribe = await subscribeTencentMessages(nextContext, (incoming) => {
          if (!active) return
          setMessages((current) => mergeMessages(current, incoming.filter((message) => Boolean(messageText(message)))))
        })
        if (!active) {
          unsubscribe()
          unsubscribe = undefined
          return
        }

        const history = await loadTencentMessages(nextContext)
        if (!active) return
        setMessages((current) => mergeMessages(current, history.filter((message) => Boolean(messageText(message)))))
        setStatus('ready')
      } catch (error) {
        unsubscribe?.()
        unsubscribe = undefined
        if (!active) return
        setProduct(null)
        setContext(null)
        setRole(null)
        const message = chatErrorMessage(error)
        setErrorMessage(message)
        setStatus('error')
        showToast(message)
      }
    })()

    return () => {
      active = false
      unsubscribe?.()
    }
  }, [entry?.id, entry?.kind, reloadKey])

  const visibleMessages = useMemo(
    () => messages.filter((message) => Boolean(messageText(message))),
    [messages]
  )
  const peerName = role === 'seller' ? '交易对方' : product?.sellerName || '校园卖家'
  const peerAvatar = role === 'seller' ? '买' : peerName.trim().charAt(0) || '校'
  const peerAvatarImage = role === 'buyer' ? product?.sellerAvatar : ''
  const peerState = role === 'seller' ? '买家 · 腾讯云 IM' : `${product?.campus || '校内'} · 腾讯云 IM`
  const canOpenSellerProfile = role === 'buyer' && Boolean(product?.sellerId)
  const resolvedProductId = context
    ? String(context.productId)
    : entry?.kind === 'product' ? entry.id : ''
  const productImage = product?.images[0]

  const goBack = () => {
    const fallback = entry?.kind === 'product'
      ? `/pages/secondhand-detail/index?id=${encodeURIComponent(entry.id)}`
      : '/pages/home/index'
    void Taro.navigateBack().catch(() => Taro.redirectTo({ url: fallback }))
  }

  const openSellerProfile = () => {
    if (!canOpenSellerProfile || !product?.sellerId) return
    void Taro.navigateTo({
      url: `/pages/user-profile/index?userId=${encodeURIComponent(product.sellerId)}&tab=secondhand`
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
    if (!context || !role || status !== 'ready' || sending) return

    setSending(true)
    try {
      const message = await sendTencentText(context, value)
      setMessages((current) => mergeMessages(current, [message]))
      setDraft('')
    } catch (error) {
      showToast(chatErrorMessage(error))
    } finally {
      setSending(false)
    }
  }

  return (
    <View className='secondhand-chat-page'>
      <View className='secondhand-chat-header'>
        <View className='secondhand-chat-header__back' onClick={goBack} />
        <View className='secondhand-chat-seller' onClick={canOpenSellerProfile ? openSellerProfile : undefined}>
          <View className='secondhand-chat-seller__avatar'>
            {peerAvatarImage ? (
              <Image className='secondhand-chat-seller__avatar-image' src={peerAvatarImage} mode='aspectFill' />
            ) : (
              <Text>{peerAvatar}</Text>
            )}
          </View>
          <View className='secondhand-chat-seller__meta'>
            <Text className='secondhand-chat-seller__name'>{status === 'loading' ? '正在连接私信' : peerName}</Text>
            <Text className='secondhand-chat-seller__state'>
              {status === 'ready' ? peerState : status === 'error' ? '连接失败' : '安全初始化中'}
            </Text>
          </View>
        </View>
        <View
          className={`secondhand-chat-header__home ${canOpenSellerProfile ? '' : 'secondhand-chat-header__home--disabled'}`}
          onClick={canOpenSellerProfile ? openSellerProfile : undefined}
        >
          <Text>{canOpenSellerProfile ? '主页' : '交易中'}</Text>
        </View>
      </View>

      <View className='secondhand-chat-product'>
        <View className='secondhand-chat-product__image'>
          {productImage ? (
            <Image className='secondhand-chat-product__image-content' src={productImage} mode='aspectFill' />
          ) : (
            <Text>图 · 宝贝</Text>
          )}
        </View>
        <View className='secondhand-chat-product__main'>
          <Text className='secondhand-chat-product__title'>
            {product?.title || (resolvedProductId ? `二手商品 #${resolvedProductId}` : '正在读取宝贝信息')}
          </Text>
          {product ? <Text className='secondhand-chat-product__price'>¥{product.price}</Text> : null}
        </View>
        <View
          className={`secondhand-chat-product__button ${resolvedProductId ? '' : 'secondhand-chat-product__button--disabled'}`}
          onClick={resolvedProductId ? openProduct : undefined}
        >
          <Text>查看</Text>
        </View>
      </View>

      <ScrollView className='secondhand-chat-scroll' scrollY enhanced showScrollbar={false}>
        <View className='secondhand-chat-safe-tip'>
          <Text>平台不担保交易，请当面验货后再付款</Text>
        </View>

        {status === 'loading' ? (
          <View className='secondhand-chat-state'>
            <Text className='secondhand-chat-state__title'>正在连接真实私信</Text>
            <Text className='secondhand-chat-state__desc'>正在校验商品与会话权限，请稍候。</Text>
          </View>
        ) : status === 'error' ? (
          <View className='secondhand-chat-state secondhand-chat-state--retry' onClick={() => setReloadKey((value) => value + 1)}>
            <Text className='secondhand-chat-state__title'>私信连接失败，点击重试</Text>
            <Text className='secondhand-chat-state__desc'>{errorMessage || '请检查网络或登录状态后再试。'}</Text>
          </View>
        ) : visibleMessages.length === 0 ? (
          <View className='secondhand-chat-state'>
            <Text className='secondhand-chat-state__title'>会话已建立</Text>
            <Text className='secondhand-chat-state__desc'>
              {role === 'seller' ? '回复买家确认交易。' : '发送第一条文字消息，与卖家确认宝贝情况。'}
            </Text>
          </View>
        ) : (
          visibleMessages.map((message) => {
            const mine = message.flow === 'out'
            return (
              <View
                key={message.ID}
                className={`secondhand-chat-row ${mine ? 'secondhand-chat-row--me' : 'secondhand-chat-row--seller'}`}
              >
                {!mine && (
                  <View className='secondhand-chat-avatar'>
                    <Text>{peerAvatar}</Text>
                  </View>
                )}
                <View className={`secondhand-chat-bubble ${mine ? 'secondhand-chat-bubble--me' : 'secondhand-chat-bubble--seller'}`}>
                  <Text>{messageText(message)}</Text>
                  <Text className='secondhand-chat-bubble__time'>{formatTime(message.time)}</Text>
                </View>
                {mine && (
                  <View className='secondhand-chat-avatar secondhand-chat-avatar--me'>
                    <Text>我</Text>
                  </View>
                )}
              </View>
            )
          })
        )}
      </ScrollView>

      <View className='secondhand-chat-inputbar'>
        <View className='secondhand-chat-image-button' onClick={() => showToast('二手私信图片发送尚未开放')}>
          <View className='secondhand-chat-image-button__icon' />
        </View>
        <View className='secondhand-chat-input-wrap'>
          <Input
            className='secondhand-chat-input'
            value={draft}
            maxlength={500}
            disabled={status !== 'ready' || !role || sending}
            placeholder={status === 'ready' ? '发消息...' : '私信连接中...'}
            placeholderClass='secondhand-chat-input__placeholder'
            confirmType='send'
            onInput={(event) => setDraft(String(event.detail.value ?? ''))}
            onConfirm={() => void sendText()}
          />
        </View>
        <View
          className={`secondhand-chat-send ${draft.trim() && status === 'ready' && role && !sending ? 'secondhand-chat-send--active' : ''}`}
          onClick={() => void sendText()}
        >
          <Text>{sending ? '发送中' : '发送'}</Text>
        </View>
      </View>
    </View>
  )
}
