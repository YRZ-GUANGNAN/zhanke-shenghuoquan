// Domain E adapter: official Tencent Cloud Chat SDK only. No message is persisted locally or by this backend.
import TencentCloudChat, { type ChatSDK, type Message } from '@tencentcloud/chat'
import type { IMConversationContext } from './c-trade'
import { request } from './api'

export interface IMCredentials {
  sdkAppId: string
  userId: string
  userSig: string
}

interface ConversationPage {
  items: IMConversationContext[]
  page: number
  pageSize: number
  total: number
}

interface TencentLoginTask {
  userId: string
  generation: number
  promise: Promise<ChatSDK>
}

interface TencentMessagePage {
  data?: {
    messageList?: Message[]
    nextReqMessageID?: string
    isCompleted?: boolean
  }
}

let sdk: ChatSDK | null = null
let sdkAppId = ''
let loginTask: TencentLoginTask | null = null
let sessionGeneration = 0
let resetTask: Promise<void> = Promise.resolve()

const TENCENT_IM_TIMEOUT_MS = 30000
const TENCENT_IM_READY_TIMEOUT_MS = 20000
const TENCENT_IM_HISTORY_PAGE_LIMIT = 20

function withTimeout<T>(promise: Promise<T>, timeoutMs: number, message: string): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error(message)), timeoutMs)
    promise.then(
      (value) => {
        clearTimeout(timer)
        resolve(value)
      },
      (error: unknown) => {
        clearTimeout(timer)
        reject(error)
      }
    )
  })
}

function tencentErrorMessage(event: unknown): string {
  const value = (event && typeof event === 'object' ? event : {}) as Record<string, unknown>
  const nested = (value.data && typeof value.data === 'object' ? value.data : {}) as Record<string, unknown>
  const code = nested.code ?? nested.errorCode ?? value.code ?? value.errorCode
  const message = nested.message ?? nested.errorMessage ?? value.message ?? value.errorMessage
  const detail = typeof message === 'string' && message.trim() ? message.trim() : '连接失败'
  return code === undefined ? `腾讯云 IM：${detail}` : `腾讯云 IM 错误 ${String(code)}：${detail}`
}

function assertCurrentGeneration(generation: number, chat?: ChatSDK): void {
  if (generation !== sessionGeneration || (chat && sdk !== chat)) {
    throw new Error('腾讯云 IM 登录会话已切换，请重试')
  }
}

function isParticipantMessage(
  message: Message,
  context: IMConversationContext,
  currentUserId: string
): boolean {
  if (message.conversationID !== context.conversationId) return false
  const from = String(message.from || '')
  const to = String(message.to || '')
  const peerUserId = String(context.peerUserId || '')
  return (
    (from === currentUserId && to === peerUserId) ||
    (from === peerUserId && to === currentUserId)
  )
}

export async function getIMCredentials(): Promise<IMCredentials> {
  const credentials = await request<IMCredentials>('/im/usersig', { url: '', timeout: 10000 })
  const numericAppId = Number(credentials.sdkAppId)
  if (
    !Number.isSafeInteger(numericAppId) ||
    numericAppId <= 0 ||
    !String(credentials.userId || '').trim() ||
    !String(credentials.userSig || '').trim()
  ) {
    throw new Error('腾讯云 IM 登录凭证无效，请重新登录')
  }
  return credentials
}

export async function listIMConversations(page = 1, pageSize = 20): Promise<IMConversationContext[]> {
  return (await request<ConversationPage>(`/im/conversations?page=${page}&pageSize=${pageSize}`)).items
}

export function getIMConversation(relationId: number): Promise<IMConversationContext> {
  return request(`/im/conversations/${relationId}`)
}

function createTencentReadyWaiter(chat: ChatSDK): { promise: Promise<void>; cancel: () => void } {
  if (chat.isReady()) return { promise: Promise.resolve(), cancel: () => undefined }
  let cleanup = () => undefined
  const promise = new Promise<void>((resolve, reject) => {
    let timer: ReturnType<typeof setTimeout> | undefined
    let lastNetState = 'connecting'
    cleanup = () => {
      if (timer) clearTimeout(timer)
      chat.off(TencentCloudChat.EVENT.SDK_READY, onReady)
      chat.off(TencentCloudChat.EVENT.ERROR, onError)
      chat.off(TencentCloudChat.EVENT.NET_STATE_CHANGE, onNetStateChange)
    }
    const onReady = () => {
      cleanup()
      resolve()
    }
    const onError = (event: unknown) => {
      cleanup()
      reject(new Error(tencentErrorMessage(event)))
    }
    const onNetStateChange = (event: unknown) => {
      const value = (event && typeof event === 'object' ? event : {}) as Record<string, unknown>
      const nested = (value.data && typeof value.data === 'object' ? value.data : {}) as Record<string, unknown>
      lastNetState = String(nested.state ?? value.state ?? lastNetState)
    }
    timer = setTimeout(() => {
      cleanup()
      reject(new Error(`腾讯云 IM 初始化超时（网络状态：${lastNetState}；请检查小程序 socket 合法域名）`))
    }, TENCENT_IM_READY_TIMEOUT_MS)
    chat.on(TencentCloudChat.EVENT.SDK_READY, onReady)
    chat.on(TencentCloudChat.EVENT.ERROR, onError)
    chat.on(TencentCloudChat.EVENT.NET_STATE_CHANGE, onNetStateChange)
  })
  return { promise, cancel: cleanup }
}

async function loginTencentIM(credentials: IMCredentials, generation: number): Promise<ChatSDK> {
  assertCurrentGeneration(generation)

  let chat = sdk
  if (!chat || sdkAppId !== credentials.sdkAppId) {
    const previous = chat
    sdk = null
    sdkAppId = ''
    if (previous) {
      await withTimeout(previous.destroy(), TENCENT_IM_TIMEOUT_MS, '腾讯云 IM 释放旧连接超时')
      assertCurrentGeneration(generation)
    }
    chat = TencentCloudChat.create({ SDKAppID: Number(credentials.sdkAppId) })
    chat.setLogLevel(1)
    assertCurrentGeneration(generation)
    sdk = chat
    sdkAppId = credentials.sdkAppId
  }

  assertCurrentGeneration(generation, chat)
  const currentUser = String(chat.getLoginUser() || '')
  if (currentUser === credentials.userId && chat.isReady()) return chat
  if (currentUser === credentials.userId) {
    const ready = createTencentReadyWaiter(chat)
    try {
      await ready.promise
      assertCurrentGeneration(generation, chat)
      return chat
    } finally {
      ready.cancel()
    }
  }

  if (currentUser && currentUser !== credentials.userId) {
    await withTimeout(chat.logout(), TENCENT_IM_TIMEOUT_MS, '腾讯云 IM 退出旧账号超时')
    assertCurrentGeneration(generation, chat)
  }

  const ready = createTencentReadyWaiter(chat)
  const readyOutcome = ready.promise.then(
    () => ({ error: null as unknown }),
    (error: unknown) => ({ error })
  )
  try {
    await chat.login({ userID: credentials.userId, userSig: credentials.userSig })
    assertCurrentGeneration(generation, chat)
    if (!chat.isReady()) {
      const outcome = await readyOutcome
      if (outcome.error) throw outcome.error
    }
    assertCurrentGeneration(generation, chat)
  } catch (error) {
    throw error
  } finally {
    ready.cancel()
  }
  return chat
}

export function resetTencentIMSession(): Promise<void> {
  sessionGeneration += 1
  const previous = sdk
  sdk = null
  sdkAppId = ''
  loginTask = null
  const pendingReset = resetTask
  resetTask = (async () => {
    await pendingReset
    if (!previous) return
    await withTimeout(previous.destroy(), 5000, '腾讯云 IM 重置超时').catch(() => undefined)
  })()
  return resetTask
}

async function ensureTencentIMReadyAttempt(expectedUserId: string, retried: boolean): Promise<ChatSDK> {
  await resetTask
  const generation = sessionGeneration
  const credentials = await getIMCredentials()
  assertCurrentGeneration(generation)

  if (expectedUserId && credentials.userId !== expectedUserId) {
    throw new Error(`腾讯云 IM 账号尚未切换：期望 ${expectedUserId}，实际 ${credentials.userId}`)
  }

  if (loginTask) {
    if (loginTask.generation === generation && loginTask.userId === credentials.userId) {
      return loginTask.promise
    }
    await resetTencentIMSession()
    if (retried) throw new Error('腾讯云 IM 登录任务与当前账号不一致，请重试')
    return ensureTencentIMReadyAttempt(expectedUserId, true)
  }

  let promise: Promise<ChatSDK>
  promise = withTimeout(
    loginTencentIM(credentials, generation),
    TENCENT_IM_TIMEOUT_MS,
    '腾讯云 IM 登录超时'
  ).then(
    (chat) => {
      if (loginTask?.promise === promise) loginTask = null
      return chat
    },
    async (error: unknown) => {
      if (generation === sessionGeneration) await resetTencentIMSession()
      if (loginTask?.promise === promise) loginTask = null
      throw error
    }
  )
  loginTask = { userId: credentials.userId, generation, promise }
  return promise
}

export function ensureTencentIMReady(expectedUserId = ''): Promise<ChatSDK> {
  return ensureTencentIMReadyAttempt(expectedUserId, false)
}

export async function getTencentIMLoginUser(expectedUserId = ''): Promise<string> {
  const chat = await ensureTencentIMReady(expectedUserId)
  return String(chat.getLoginUser() || '')
}

export async function loadTencentMessages(
  context: IMConversationContext,
  expectedUserId = ''
): Promise<Message[]> {
  const chat = await ensureTencentIMReady(expectedUserId)
  const currentUserId = String(chat.getLoginUser() || '')
  if (!currentUserId) throw new Error('腾讯云 IM 当前登录用户为空，请重新登录')

  await withTimeout(
    chat.getConversationProfile(context.conversationId),
    TENCENT_IM_TIMEOUT_MS,
    '读取腾讯云 IM 会话超时'
  )

  const messages: Message[] = []
  const seenMessageIds = new Set<string>()
  const seenRequestIds = new Set<string>()
  let nextReqMessageID = ''
  for (let page = 0; page < TENCENT_IM_HISTORY_PAGE_LIMIT; page += 1) {
    const options: { conversationID: string; nextReqMessageID?: string } = {
      conversationID: context.conversationId
    }
    if (nextReqMessageID) options.nextReqMessageID = nextReqMessageID

    const result = await withTimeout(
      chat.getMessageList(options),
      TENCENT_IM_TIMEOUT_MS,
      '读取腾讯云 IM 历史消息超时'
    ) as TencentMessagePage
    const pageMessages = Array.isArray(result.data?.messageList)
      ? result.data.messageList
      : []
    for (const message of pageMessages) {
      const messageId = String(message.ID || '')
      if (messageId && seenMessageIds.has(messageId)) continue
      if (messageId) seenMessageIds.add(messageId)
      messages.push(message)
    }

    const next = String(result.data?.nextReqMessageID || '')
    if (result.data?.isCompleted === true || !next || seenRequestIds.has(next)) break
    seenRequestIds.add(next)
    nextReqMessageID = next
  }

  await withTimeout(
    chat.setMessageRead({ conversationID: context.conversationId }),
    5000,
    '更新腾讯云 IM 已读状态超时'
  ).catch(() => undefined)

  return messages.filter((message) => isParticipantMessage(message, context, currentUserId))
}

export async function sendTencentText(
  context: IMConversationContext,
  text: string,
  expectedUserId = ''
): Promise<Message> {
  const chat = await ensureTencentIMReady(expectedUserId)
  const message = chat.createTextMessage({
    to: context.peerUserId,
    conversationType: TencentCloudChat.TYPES.CONV_C2C,
    payload: { text }
  })
  const result = await withTimeout(
    chat.sendMessage(message),
    TENCENT_IM_TIMEOUT_MS,
    '发送腾讯云 IM 消息超时'
  ) as { data?: { message?: Message } }
  return result.data?.message || message
}

export async function subscribeTencentMessages(
  context: IMConversationContext,
  handler: (messages: Message[]) => void,
  expectedUserId = ''
): Promise<() => void> {
  const chat = await ensureTencentIMReady(expectedUserId)
  const currentUserId = String(chat.getLoginUser() || '')
  if (!currentUserId) throw new Error('腾讯云 IM 当前登录用户为空，请重新登录')

  const listener = (event: { data?: Message[] }) => {
    const incoming = (event.data || []).filter((message) =>
      isParticipantMessage(message, context, currentUserId)
    )
    if (incoming.length > 0) handler(incoming)
  }
  chat.on(TencentCloudChat.EVENT.MESSAGE_RECEIVED, listener)
  return () => chat.off(TencentCloudChat.EVENT.MESSAGE_RECEIVED, listener)
}

export type { Message as TencentMessage }
