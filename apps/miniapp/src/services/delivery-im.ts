import TencentCloudChat, { type Message } from '@tencentcloud/chat'
import { request } from './api'
import { ensureTencentIMReady, hasUnrenderedTencentMessages, listenTencentMessageEvents, withTimeout, type IMCredentials, type TencentTextAttempt } from './e-im'

function readyForDeliveryIM(expectedUserId = '') {
  return ensureTencentIMReady(expectedUserId, () => request<IMCredentials>('/delivery/im-credentials', { url: '', timeout: 10000 }))
}

export type DeliveryConversationStatus =
  | 'provisioning'
  | 'active'
  | 'read_only'
  | 'revoking'
  | 'revoked'
  | 'failed'

export interface DeliveryConversation {
  conversationPublicId: string
  deliveryOrderPublicId: string
  assignmentVersion: number
  tencentConversationId: string
  status: DeliveryConversationStatus
  canSend: boolean
  imagesEnabled: boolean
  readReceiptEnabled: boolean
  createdAt: string
  updatedAt: string
}

export interface DeliveryConversationPage {
  items: DeliveryConversation[]
  page: number
  pageSize: number
  total: number
}

export interface DeliveryIMImagePayload {
  type: 'delivery_im_image'
  conversationId: string
  imageId: string
}

export interface DeliveryIMPreparedImage {
  imageId: string
  customPayload: DeliveryIMImagePayload
}

export interface DeliveryIMResolvedImage {
  url: string
  expiresInSeconds: number
}

export interface DeliveryIMUnreadSnapshot {
  conversationId: string
  unreadCount: number
}

interface ConversationProfileResult {
  data?: {
    conversation?: {
      conversationID?: string
      unreadCount?: number
    }
  }
}

interface MessageListResult {
  data?: {
    messageList?: Message[]
  }
}

interface SendMessageResult {
  data?: {
    message?: Message
  }
}

interface ConversationListEvent {
  data?: Array<{
    conversationID?: string
    unreadCount?: number
  }>
}

export function getDeliveryOrderConversation(orderId: string): Promise<DeliveryConversation> {
  return request<DeliveryConversation>(
    `/delivery/orders/${encodeURIComponent(orderId)}/conversation`,
    { url: '', timeout: 10000 }
  )
}

export function listDeliveryConversations(page = 1, pageSize = 20): Promise<DeliveryConversationPage> {
  return request<DeliveryConversationPage>(
    `/delivery/conversations?page=${page}&pageSize=${pageSize}`,
    { url: '', timeout: 10000 }
  )
}

export function prepareDeliveryConversationImage(
  conversationId: string,
  assetPublicId: string
): Promise<DeliveryIMPreparedImage> {
  return request<DeliveryIMPreparedImage>(
    `/delivery/conversations/${encodeURIComponent(conversationId)}/images/prepare`,
    {
      url: '',
      method: 'POST',
      data: { assetPublicId },
      timeout: 20000
    }
  )
}

export function resolveDeliveryConversationImage(
  conversationId: string,
  imageId: string
): Promise<DeliveryIMResolvedImage> {
  return request<DeliveryIMResolvedImage>(
    `/delivery/conversations/${encodeURIComponent(conversationId)}/images/${encodeURIComponent(imageId)}/resolve`,
    { url: '', timeout: 10000 }
  )
}

export async function loadDeliveryTencentMessages(
  conversation: DeliveryConversation,
  expectedUserId = ''
): Promise<Message[]> {
  const chat = await readyForDeliveryIM(expectedUserId)
  const result = await withTimeout(chat.getMessageList({
    conversationID: conversation.tencentConversationId
  }), 15000, '历史消息读取失败，请稍后重试') as MessageListResult
  return result.data?.messageList || []
}

export async function markDeliveryConversationRead(conversation: DeliveryConversation, expectedUserId: string, isVisible: () => boolean, displayedIDs: string[]): Promise<void> {
  if (!isVisible()) return
  const current = await getDeliveryOrderConversation(conversation.deliveryOrderPublicId)
  if (!isVisible() || !['active', 'read_only'].includes(current.status) || current.conversationPublicId !== conversation.conversationPublicId ||
      current.assignmentVersion !== conversation.assignmentVersion || current.tencentConversationId !== conversation.tencentConversationId) return
  const chat = await readyForDeliveryIM(expectedUserId)
  if (!isVisible() || hasUnrenderedTencentMessages(chat, conversation.tencentConversationId, displayedIDs)) return
  await withTimeout(chat.setMessageRead({ conversationID: conversation.tencentConversationId }), 5000, '更新已读状态超时')
}

async function authorizeDeliverySend(conversation: DeliveryConversation) {
  const current = await getDeliveryOrderConversation(conversation.deliveryOrderPublicId)
  if (!deliveryConversationCanSend(current) || current.conversationPublicId !== conversation.conversationPublicId ||
      current.assignmentVersion !== conversation.assignmentVersion || current.tencentConversationId !== conversation.tencentConversationId) {
    throw new Error('订单状态或配送人员已变化，请返回订单查看')
  }
}

export async function sendDeliveryTencentText(
  conversation: DeliveryConversation,
  text: string,
  expectedUserId = '',
  attempt?: TencentTextAttempt
): Promise<Message> {
  await authorizeDeliverySend(conversation)
  const chat = await readyForDeliveryIM(expectedUserId)
  const sender = chat.getLoginUser()
  if (attempt?.message && (attempt.message.conversationID !== conversation.tencentConversationId || attempt.message.from !== sender || attempt.message.payload?.text !== text)) {
    throw new Error('待发送消息与当前会话不一致')
  }
  const retry = Boolean(attempt?.message)
  const message = attempt?.message || chat.createTextMessage({
    to: groupIDFromConversationID(conversation.tencentConversationId),
    conversationType: TencentCloudChat.TYPES.CONV_GROUP,
    payload: { text },
    needReadReceipt: false
  })
  if (attempt) attempt.message = message
  if (retry && message.status === 'success') return message
  const result = await withTimeout(retry ? chat.resendMessage(message) : chat.sendMessage(message), 30000, '消息发送超时，请重试') as SendMessageResult
  if (chat.getLoginUser() !== sender) throw new Error('登录账号已变化')
  if (!result.data?.message?.ID) throw new Error('消息发送结果未确认，请重试')
  return result.data.message
}

export async function sendDeliveryTencentImage(
  conversation: DeliveryConversation,
  assetPublicId: string,
  expectedUserId = ''
): Promise<Message> {
  await authorizeDeliverySend(conversation)
  const prepared = await prepareDeliveryConversationImage(
    conversation.conversationPublicId,
    assetPublicId
  )
  const chat = await readyForDeliveryIM(expectedUserId)
  const message = chat.createCustomMessage({
    to: groupIDFromConversationID(conversation.tencentConversationId),
    conversationType: TencentCloudChat.TYPES.CONV_GROUP,
    payload: {
      data: JSON.stringify(prepared.customPayload),
      description: 'delivery_im_image',
      extension: ''
    },
    needReadReceipt: false
  })
  const result = await withTimeout(chat.sendMessage(message), 30000, '图片发送超时') as SendMessageResult
  if (!result.data?.message?.ID) throw new Error('图片发送结果未确认')
  return result.data.message
}

export async function getDeliveryConversationUnread(
  conversationID: string
): Promise<number> {
  const chat = await readyForDeliveryIM()
  const result = await chat.getConversationProfile(conversationID) as ConversationProfileResult
  return Number(result.data?.conversation?.unreadCount || 0)
}

export async function getDeliveryConversationUnreadByOrder(
  orderId: string
): Promise<DeliveryIMUnreadSnapshot> {
  const conversation = await getDeliveryOrderConversation(orderId)
  return {
    conversationId: conversation.conversationPublicId,
    unreadCount: await getDeliveryConversationUnread(conversation.tencentConversationId)
  }
}

export async function subscribeDeliveryTencentMessages(
  conversation: DeliveryConversation,
  handler: (messages: Message[]) => void,
  expectedUserId = ''
): Promise<() => void> {
  const chat = await readyForDeliveryIM(expectedUserId)
  return listenTencentMessageEvents(chat, messages => {
    const incoming = messages.filter((item) => item.conversationID === conversation.tencentConversationId)
    if (incoming.length) handler(incoming)
  })
}

export async function subscribeDeliveryConversationUnread(
  conversationID: string,
  handler: (unreadCount: number) => void
): Promise<() => void> {
  const chat = await readyForDeliveryIM()
  const listener = (event: ConversationListEvent) => {
    const value = (event.data || []).find((item) => item.conversationID === conversationID)
    if (value) handler(Number(value.unreadCount || 0))
  }
  chat.on(TencentCloudChat.EVENT.CONVERSATION_LIST_UPDATED, listener)
  return () => chat.off(TencentCloudChat.EVENT.CONVERSATION_LIST_UPDATED, listener)
}

export function parseDeliveryImagePayload(message: Message): DeliveryIMImagePayload | null {
  if (message.type !== 'TIMCustomElem') return null
  const payload = message.payload as { data?: unknown; description?: unknown } | undefined
  if (!payload || payload.description !== 'delivery_im_image' || typeof payload.data !== 'string') return null
  try {
    const value = JSON.parse(payload.data) as Partial<DeliveryIMImagePayload>
    if (value.type === 'delivery_im_image' &&
      typeof value.conversationId === 'string' &&
      typeof value.imageId === 'string') {
      return {
        type: 'delivery_im_image',
        conversationId: value.conversationId,
        imageId: value.imageId
      }
    }
  } catch {
    return null
  }
  return null
}

export function deliveryConversationCanSend(conversation: DeliveryConversation): boolean {
  return conversation.canSend && conversation.status === 'active'
}

function groupIDFromConversationID(conversationID: string): string {
  return conversationID.startsWith('GROUP') ? conversationID.slice(5) : conversationID
}

export type { Message as DeliveryTencentMessage }
