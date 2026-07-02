import TencentCloudChat, { type Message } from '@tencentcloud/chat'
import { request } from './api'
import { ensureTencentIMReady } from './e-im'

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

interface MessageEvent {
  data?: Message[]
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
  conversation: DeliveryConversation
): Promise<Message[]> {
  const chat = await ensureTencentIMReady()
  await chat.getConversationProfile(conversation.tencentConversationId)
  const result = await chat.getMessageList({
    conversationID: conversation.tencentConversationId
  }) as MessageListResult
  await chat.setMessageRead({ conversationID: conversation.tencentConversationId })
  return result.data?.messageList || []
}

export async function sendDeliveryTencentText(
  conversation: DeliveryConversation,
  text: string
): Promise<Message> {
  const chat = await ensureTencentIMReady()
  const message = chat.createTextMessage({
    to: groupIDFromConversationID(conversation.tencentConversationId),
    conversationType: TencentCloudChat.TYPES.CONV_GROUP,
    payload: { text },
    needReadReceipt: false
  })
  const result = await chat.sendMessage(message) as SendMessageResult
  return result.data?.message || message
}

export async function sendDeliveryTencentImage(
  conversation: DeliveryConversation,
  assetPublicId: string
): Promise<Message> {
  const prepared = await prepareDeliveryConversationImage(
    conversation.conversationPublicId,
    assetPublicId
  )
  const chat = await ensureTencentIMReady()
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
  const result = await chat.sendMessage(message) as SendMessageResult
  return result.data?.message || message
}

export async function getDeliveryConversationUnread(
  conversationID: string
): Promise<number> {
  const chat = await ensureTencentIMReady()
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
  handler: (messages: Message[]) => void
): Promise<() => void> {
  const chat = await ensureTencentIMReady()
  const listener = (event: MessageEvent) => {
    handler((event.data || []).filter((item) => item.conversationID === conversation.tencentConversationId))
  }
  chat.on(TencentCloudChat.EVENT.MESSAGE_RECEIVED, listener)
  return () => chat.off(TencentCloudChat.EVENT.MESSAGE_RECEIVED, listener)
}

export async function subscribeDeliveryConversationUnread(
  conversationID: string,
  handler: (unreadCount: number) => void
): Promise<() => void> {
  const chat = await ensureTencentIMReady()
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
