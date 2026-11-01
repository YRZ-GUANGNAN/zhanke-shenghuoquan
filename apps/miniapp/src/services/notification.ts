import { request } from './api'
import Taro from './taro'

/** A server-owned notification inbox row. Payloads contain public coordinates only. */
export interface NotificationInboxItem {
  id: string
  purpose: string
  title: string
  summary: string
  aggregateType: string
  aggregatePublicId: string
  read: boolean
  createdAt: string
}

export interface NotificationInboxPage {
  items: NotificationInboxItem[]
  page: number
  pageSize: number
  total: number
  unread: number | null
}

export interface NotificationSubscriptionTemplate {
  /** Stable server configuration key; the client never invents a template ID. */
  templateKey: string
  /** Runtime-only template ID returned by the server when configured. */
  templateId?: string
  available: boolean
}

export interface NotificationSubscriptionTemplates {
  items: NotificationSubscriptionTemplate[]
}

export type NotificationSubscriptionStatus =
  | 'accept'
  | 'reject'
  | 'ban'
  | 'close'
  | 'unavailable'
  | 'exception'

export interface NotificationSubscriptionResult {
  templateKey: string
  status: NotificationSubscriptionStatus
  /** The server accepts this nonce for idempotent authorization reporting. */
  nonce: string
  message?: string
}

export interface NotificationUnreadSnapshot {
  /** Session/account generation that owns this snapshot. */
  generation: number
  /** False means the value is unknown and must not be rendered as zero. */
  confirmed: boolean
  total: number | null
  conversations: Record<string, number>
}

export interface NotificationUnreadSnapshotUpdate {
  generation: number
  confirmed?: boolean
  total?: number | null
  conversations?: Record<string, number | null | undefined>
  /** Conversation events carry an absolute unread count, never a delta. */
  conversationId?: string
  unreadCount?: number | null
}

interface UnknownRecord {
  [key: string]: unknown
}

interface TaroSubscribeRuntime {
  requestSubscribeMessage?: (options: { tmplIds: string[] }) => Promise<UnknownRecord>
}

function asRecord(value: unknown): UnknownRecord {
  return value && typeof value === 'object' ? value as UnknownRecord : {}
}

function asString(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

function asNumber(value: unknown): number | null {
  if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) return null
  return Math.floor(value)
}

function first(record: UnknownRecord, keys: string[]): unknown {
  for (const key of keys) {
    if (record[key] !== undefined) return record[key]
  }
  return undefined
}

function normalizeInboxItem(value: unknown): NotificationInboxItem {
  const record = asRecord(value)
  return {
    id: asString(first(record, ['id', 'notificationId', 'notification_id'])),
    purpose: asString(first(record, ['purpose', 'type'])),
    title: asString(record.title),
    // The server returns a redacted summary; do not hydrate arbitrary message正文 fields.
    summary: asString(first(record, ['summary', 'preview'])),
    aggregateType: asString(first(record, ['aggregateType', 'aggregate_type'])),
    aggregatePublicId: asString(first(record, ['aggregatePublicId', 'aggregate_public_id'])),
    read: Boolean(first(record, ['read', 'isRead', 'read_at'])),
    createdAt: asString(first(record, ['createdAt', 'created_at']))
  }
}

function normalizeInboxPage(value: unknown, page: number, pageSize: number): NotificationInboxPage {
  const record = asRecord(value)
  const rawItems = Array.isArray(record.items) ? record.items : Array.isArray(value) ? value : []
  const items = rawItems.map(normalizeInboxItem)
  const rawUnread = first(record, ['unread', 'unreadCount', 'unread_count'])
  return {
    items,
    page: asNumber(first(record, ['page'])) ?? page,
    pageSize: asNumber(first(record, ['pageSize', 'page_size'])) ?? pageSize,
    total: asNumber(first(record, ['total'])) ?? items.length,
    unread: asNumber(rawUnread) ?? (rawUnread === null ? null : items.filter(item => !item.read).length)
  }
}

function normalizeTemplates(value: unknown): NotificationSubscriptionTemplates {
  const record = asRecord(value)
  const raw = Array.isArray(record.items)
    ? record.items
    : Array.isArray(record.templates)
      ? record.templates
      : Array.isArray(value)
        ? value
        : []
  return {
    items: raw.map(item => {
      const row = asRecord(item)
      const templateKey = asString(first(row, ['templateKey', 'template_key', 'key']))
      const templateId = asString(first(row, ['templateId', 'template_id', 'id'])) || undefined
      return { templateKey, templateId, available: row.available !== false && Boolean(templateId) }
    }).filter(item => Boolean(item.templateKey))
  }
}

function createNonce(): string {
  // This is an idempotency nonce, not a credential. Keep it in memory only.
  const random = Math.floor(Math.random() * 0x100000000).toString(16).padStart(8, '0')
  return `${Date.now().toString(36)}-${random}`
}

function classifySubscribeResult(value: UnknownRecord, templateId: string): NotificationSubscriptionStatus {
  const result = asString(value[templateId]).toLowerCase()
  if (result === 'accept' || result === 'accepted') return 'accept'
  if (result === 'reject' || result === 'rejected') return 'reject'
  if (result === 'ban' || result === 'banned') return 'ban'
  const message = asString(value.errMsg).toLowerCase()
  if (message.includes('cancel') || message.includes('close')) return 'close'
  return 'exception'
}

/** Read the independent delivery notification inbox; community notifications stay on b-community. */
export async function loadNotificationInbox(page = 1, pageSize = 20): Promise<NotificationInboxPage> {
  const safePage = Number.isSafeInteger(page) && page > 0 ? page : 1
  const safePageSize = Number.isSafeInteger(pageSize) && pageSize > 0 ? Math.min(pageSize, 100) : 20
  const value = await request<unknown>(`/notification/inbox?page=${safePage}&pageSize=${safePageSize}`)
  return normalizeInboxPage(value, safePage, safePageSize)
}

export async function markNotificationRead(notificationId: string): Promise<void> {
  const id = String(notificationId || '').trim()
  if (!id) throw new Error('通知编号不能为空')
  await request(`/notification/inbox/${encodeURIComponent(id)}/read`, { url: '', method: 'POST' })
}

export async function loadNotificationSubscriptionTemplates(): Promise<NotificationSubscriptionTemplates> {
  return normalizeTemplates(await request<unknown>('/notification/subscribe/templates'))
}

/**
 * Request a one-time subscription. This function is intentionally explicit: no caller
 * should invoke it from startup, lifecycle or a background refresh.
 */
export async function requestNotificationSubscription(templateKey: string): Promise<NotificationSubscriptionResult> {
  const key = String(templateKey || '').trim()
  const nonce = createNonce()
  if (!key) return { templateKey: key, status: 'unavailable', nonce, message: '模板键不能为空' }

  let template: NotificationSubscriptionTemplate | undefined
  try {
    template = (await loadNotificationSubscriptionTemplates()).items.find(item => item.templateKey === key)
  } catch (error) {
    return { templateKey: key, status: 'unavailable', nonce, message: error instanceof Error ? error.message : '模板暂不可用' }
  }
  const templateId = template?.available ? template.templateId : undefined
  if (!templateId) return { templateKey: key, status: 'unavailable', nonce, message: '模板暂不可用' }

  const taroRuntime = Taro as unknown as TaroSubscribeRuntime
  const globalRuntime = (globalThis as unknown as { wx?: TaroSubscribeRuntime }).wx
  const runtime = typeof taroRuntime?.requestSubscribeMessage === 'function' ? taroRuntime : globalRuntime
  const requestSubscribeMessage = runtime?.requestSubscribeMessage
  if (typeof requestSubscribeMessage !== 'function') {
    return { templateKey: key, status: 'unavailable', nonce, message: '当前平台不支持订阅消息' }
  }

  let status: NotificationSubscriptionStatus
  let message: string | undefined
  try {
    const response = await requestSubscribeMessage({ tmplIds: [templateId] })
    status = classifySubscribeResult(response, templateId)
    message = asString(response.errMsg) || undefined
  } catch (error) {
    status = 'exception'
    message = error instanceof Error ? error.message : '订阅授权失败'
  }

  // Reporting is best effort. Authorization failures must never block the main business flow.
  try {
    await request('/notification/subscribe/authorize', {
      url: '',
      method: 'POST',
      data: { templateKey: key, result: status, nonce }
    })
  } catch {
    // The server can retry/reconcile the missing report; no credential is retained client-side.
  }
  return { templateKey: key, status, nonce, message }
}

export function createUnreadSnapshot(generation = 0): NotificationUnreadSnapshot {
  return { generation, confirmed: false, total: null, conversations: {} }
}

/**
 * Merge absolute unread snapshots/events. A stale generation is ignored, and repeated
 * events overwrite the same conversation count instead of incrementing it.
 */
export function mergeUnreadSnapshot(
  previous: NotificationUnreadSnapshot,
  update: NotificationUnreadSnapshotUpdate
): NotificationUnreadSnapshot {
  if (update.generation !== previous.generation) return previous
  const conversations = { ...previous.conversations }
  if (update.conversations) {
    for (const [id, count] of Object.entries(update.conversations)) {
      if (!id) continue
      const safe = asNumber(count)
      if (safe === null) delete conversations[id]
      else conversations[id] = safe
    }
  }
  if (update.conversationId) {
    const count = asNumber(update.unreadCount)
    if (count === null) delete conversations[update.conversationId]
    else conversations[update.conversationId] = count
  }
  const hasConversationUpdate = Boolean(update.conversations || update.conversationId)
  const confirmed = update.confirmed === true || (update.confirmed === undefined && (update.total !== undefined || hasConversationUpdate))
  const total = update.total === undefined
    ? (hasConversationUpdate ? Object.values(conversations).reduce((sum, count) => sum + count, 0) : previous.total)
    : asNumber(update.total)
  return { generation: previous.generation, confirmed: previous.confirmed || confirmed, total, conversations }
}

export function unreadCountForDisplay(snapshot: NotificationUnreadSnapshot): number | null {
  return snapshot.confirmed ? snapshot.total : null
}

/** Alias used by badge presenters; unknown remains null until a confirmed snapshot arrives. */
export const getUnreadBadgeCount = unreadCountForDisplay

export const resetUnreadSnapshot = createUnreadSnapshot

export function nextUnreadGeneration(generation: number): NotificationUnreadSnapshot {
  return createUnreadSnapshot(generation + 1)
}
