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
  targetPath: string
  read: boolean
  createdAt: string
}

export interface NotificationInboxPage {
  items: NotificationInboxItem[]
  nextCursor: string
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
  reportStatus: 'recorded' | 'failed' | 'not_sent'
}

export interface PreparedNotificationSubscription {
  templateKey: string
  templateId: string
  nonce: string
  expiresAt: number
}

// Account binding and one-shot state stay in memory, never in API payloads.
const preparedOwners = new WeakMap<PreparedNotificationSubscription, string>()
const consumed = new WeakSet<PreparedNotificationSubscription>()

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
    targetPath: asString(record.targetPath),
    read: Boolean(first(record, ['read', 'isRead', 'read_at'])),
    createdAt: asString(first(record, ['createdAt', 'created_at']))
  }
}

function normalizeInboxPage(value: unknown): NotificationInboxPage {
  const record = asRecord(value)
  const rawItems = Array.isArray(record.items) ? record.items : Array.isArray(value) ? value : []
  const items = rawItems.map(normalizeInboxItem)
  const rawUnread = first(record, ['unread', 'unreadCount', 'unread_count'])
  return {
    items,
    nextCursor: asString(record.nextCursor),
    // The current page is not the whole inbox. Missing totals remain unknown.
    unread: asNumber(rawUnread)
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
export async function loadNotificationInbox(cursor = '', limit = 20): Promise<NotificationInboxPage> {
  const safeLimit = Number.isSafeInteger(limit) && limit > 0 ? Math.min(limit, 100) : 20
  const value = await request<unknown>(`/notification/inbox?limit=${safeLimit}${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ''}`)
  return normalizeInboxPage(value)
}

export async function markNotificationRead(notificationId: string): Promise<void> {
  const id = String(notificationId || '').trim()
  if (!id) throw new Error('通知编号不能为空')
  await request(`/notification/inbox/${encodeURIComponent(id)}/read`, { url: '', method: 'POST' })
}

/** Only established protected routes may be opened from notification metadata. */
export function notificationTargetPath(item: NotificationInboxItem): string | null {
  const path = item.targetPath || ''
  if (!/^\/pages\/(merchant-order-detail|delivery-order-detail|delivery-conversation|secondhand-chat|rider-wallet|merchant-wallet|delivery-rider|merchant-home)\/index(?:\?[^#\r\n]*)?$/.test(path)) return null
  if (/[\\\u0000-\u0020]/.test(path)) return null
  return path
}

export async function loadNotificationSubscriptionTemplates(): Promise<NotificationSubscriptionTemplates> {
  return normalizeTemplates(await request<unknown>('/notification/subscribe/templates'))
}

/** Prepare before the user clicks. Network awaits must not consume the click gesture. */
export async function prepareNotificationSubscription(templateKey: string): Promise<PreparedNotificationSubscription> {
  const key = String(templateKey || '').trim()
  const owner = Taro.getStorageSync<string>('domainA.token') || ''
  if (!key || !owner) throw new Error('请先登录后订阅通知')
  const template = (await loadNotificationSubscriptionTemplates()).items.find(item => item.templateKey === key)
  const templateId = template?.available ? template.templateId : undefined
  if (!templateId) throw new Error('通知模板暂不可用')
  if (Taro.getStorageSync<string>('domainA.token') !== owner) throw new Error('登录账号已变化')
  const grant = await request<{ nonce: string; expiresAt: string }>('/notification/subscribe/nonce', {
    url: '', method: 'POST', data: { templateKey: key }
  })
  const expiresAt = Date.parse(grant.expiresAt)
  if (!grant.nonce || !Number.isFinite(expiresAt) || expiresAt <= Date.now() ||
    Taro.getStorageSync<string>('domainA.token') !== owner) throw new Error('订阅授权准备已失效，请重试')
  const prepared = { templateKey: key, templateId, nonce: grant.nonce, expiresAt }
  preparedOwners.set(prepared, owner)
  return prepared
}

/** Call directly inside a user click handler with a pre-fetched, one-shot grant. */
export async function requestNotificationSubscription(prepared: PreparedNotificationSubscription): Promise<NotificationSubscriptionResult> {
  const { templateKey: key, templateId, nonce, expiresAt } = prepared
  const owner = preparedOwners.get(prepared)
  const isCurrent = () => Boolean(owner) && Taro.getStorageSync<string>('domainA.token') === owner
  if (!isCurrent() || consumed.has(prepared) || expiresAt <= Date.now() || typeof Taro.requestSubscribeMessage !== 'function') {
    return { templateKey: key, status: 'unavailable', nonce, reportStatus: 'not_sent', message: '请重新准备订阅授权' }
  }
  consumed.add(prepared)

  let status: NotificationSubscriptionStatus
  let message: string | undefined
  try {
    // This is the first asynchronous call: preserve the native user gesture.
    const response = await Taro.requestSubscribeMessage({ tmplIds: [templateId] })
    status = classifySubscribeResult(response, templateId)
    message = asString(response.errMsg) || undefined
  } catch (error) {
    const detail = asString(asRecord(error).errMsg) || (error instanceof Error ? error.message : '')
    status = /cancel|close/i.test(detail) ? 'close' : 'exception'
    message = '订阅未完成，可再次尝试'
  }

  // A delayed authorization response must never be reported under a new account.
  if (!isCurrent()) return { templateKey: key, status, nonce, reportStatus: 'not_sent', message: '登录账号已变化' }
  let reportStatus: NotificationSubscriptionResult['reportStatus'] = 'recorded'
  try {
    await request('/notification/subscribe/authorize', {
      url: '',
      method: 'POST',
      data: { templateKey: key, result: status, nonce }
    })
  } catch {
    reportStatus = 'failed'
    message = '订阅结果未同步，请重新尝试'
  }
  return { templateKey: key, status, nonce, message, reportStatus }
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
  const fullSnapshot = update.confirmed === true && update.conversations !== undefined
  const conversations = fullSnapshot ? {} as Record<string, number> : { ...previous.conversations }
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
  const confirmed = update.confirmed === false ? false : previous.confirmed || fullSnapshot
  const total = !confirmed ? null : update.total !== undefined ? asNumber(update.total)
    : (hasConversationUpdate ? Object.values(conversations).reduce((sum, count) => sum + count, 0) : previous.total)
  return { generation: previous.generation, confirmed, total, conversations }
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
