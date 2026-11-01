import TencentCloudChat from '@tencentcloud/chat'
import { ensureTencentIMReady, getIMCredentials, listIMConversationPage, listenTencentMessageEvents, withTimeout, type IMCredentials } from './e-im'
import { listDeliveryConversations } from './delivery-im'
import { getFeatureVisibility } from './feature-visibility'
import { request } from './api'
import { createUnreadSnapshot, loadNotificationInbox, mergeUnreadSnapshot, type NotificationInboxPage, type NotificationUnreadSnapshot } from './notification'

export interface NotificationOwner { key: string; userId: string; role: string }
export interface NotificationRuntimeState {
  ownerKey: string
  userId: string
  role: string
  unread: NotificationUnreadSnapshot
  inboxUnread: number | null
  inboxRevision: string
  messageRevision: number
}
export interface NotificationRuntimePorts {
  loadCounts(owner: NotificationOwner): Promise<Record<string, number>>
  loadInbox(): Promise<NotificationInboxPage>
  listen(owner: NotificationOwner, changed: () => void): Promise<() => void>
}

/** One app-owned lifecycle; account and role generations reject every delayed result. */
export function createNotificationRuntime(ports: NotificationRuntimePorts) {
  let generation = 0
  let owner: NotificationOwner | null = null
  let visible = true
  let busy = false
  let dirty = false
  let requestVersion = 0
  let listening = false
  let stopListener: (() => void) | undefined
  let poll: ReturnType<typeof setInterval> | undefined
  let debounce: ReturnType<typeof setTimeout> | undefined
  let state: NotificationRuntimeState = { ownerKey: '', userId: '', role: '', unread: createUnreadSnapshot(), inboxUnread: null, inboxRevision: '', messageRevision: 0 }
  const listeners = new Set<() => void>()
  const emit = () => { for (const notify of listeners) notify() }
  const current = (epoch: number, key: string) => generation === epoch && owner?.key === key && visible
  const refresh = async () => {
    if (!owner || !visible) return
    if (busy) { dirty = true; return }
    busy = true
    const target = owner
    const epoch = generation
    const version = ++requestVersion
    const [counts, inbox] = await Promise.all([
      ports.loadCounts(target).then(value => ({ value }), () => ({ value: null })),
      ports.loadInbox().then(value => ({ value }), () => ({ value: null }))
    ])
    if (!current(epoch, target.key) || version !== requestVersion) { busy = false; return }
    if (counts.value === null) { stopListener?.(); stopListener = undefined }
    const unread = counts.value === null ? createUnreadSnapshot(epoch)
      : mergeUnreadSnapshot(state.unread, { generation: epoch, confirmed: true, conversations: counts.value })
    const inboxRevision = inbox.value ? inbox.value.items.map(item => `${item.id}:${item.read}`).join('|') : state.inboxRevision
    const changed = JSON.stringify(state.unread.conversations) !== JSON.stringify(unread.conversations)
    state = { ...state, unread, inboxUnread: inbox.value?.unread ?? null, inboxRevision,
      messageRevision: state.messageRevision + (changed ? 1 : 0) }
    busy = false
    emit()
    bindListener(target, epoch)
    if (dirty) { dirty = false; schedule() }
  }
  const schedule = () => {
    if (!visible || !owner || debounce) return
    debounce = setTimeout(() => { debounce = undefined; void refresh() }, 500)
  }
  const stop = () => {
    generation += 1
    requestVersion += 1
    owner = null
    busy = false
    dirty = false
    stopListener?.()
    stopListener = undefined
    listening = false
    if (poll) clearInterval(poll)
    if (debounce) clearTimeout(debounce)
    poll = undefined
    debounce = undefined
    state = { ownerKey: '', userId: '', role: '', unread: createUnreadSnapshot(generation), inboxUnread: null, inboxRevision: '', messageRevision: 0 }
    emit()
  }
  const bindListener = (next: NotificationOwner, epoch: number) => {
    if (stopListener || listening || !visible) return
    listening = true
    void ports.listen(next, () => {
      if (!current(epoch, next.key)) return
      state = { ...state, messageRevision: state.messageRevision + 1 }
      emit()
      schedule()
    }).then(stop => {
      if (generation === epoch && owner?.key === next.key) { stopListener = stop; listening = false }
      else stop()
    }).catch(() => { if (generation === epoch) listening = false })
  }
  const start = (next: NotificationOwner) => {
    if (owner?.key === next.key && owner.userId === next.userId && owner.role === next.role) return
    stop()
    owner = next
    const epoch = generation
    // The authentication token stays inside the lifecycle closure, never presenter state.
    state = { ...state, ownerKey: `${next.userId}:${next.role}:${epoch}`, userId: next.userId, role: next.role }
    emit()
    void refresh()
    bindListener(next, epoch)
    poll = setInterval(() => { void refresh() }, 30000)
  }
  return {
    start, stop, refresh,
    foreground(show: boolean) {
      visible = show
      if (!show) { requestVersion += 1; busy = false; dirty = false; if (debounce) clearTimeout(debounce); debounce = undefined }
      else void refresh()
    },
    getState: () => state,
    subscribe(listener: () => void) { listeners.add(listener); return () => { listeners.delete(listener) } }
  }
}

function credentialsFor(owner: NotificationOwner) {
  return owner.role === 'student' ? getIMCredentials : () => request<IMCredentials>('/delivery/im-credentials', { url: '', timeout: 10000 })
}

async function authorizedConversationIDs(owner: NotificationOwner): Promise<string[]> {
  const ids = new Set<string>()
  if (owner.role === 'student' && !(await getFeatureVisibility()).messages) return []
  for (let page = 1; page <= 50; page++) {
    if (owner.role === 'student') {
      const result = await listIMConversationPage(page, 100)
      for (const item of result.items) ids.add(item.conversationId)
      if (page * result.pageSize >= result.total) return [...ids]
    } else {
      const result = await listDeliveryConversations(page, 100)
      for (const item of result.items) {
        if (item.status === 'active' || item.status === 'read_only') ids.add(item.tencentConversationId)
      }
      if (page * result.pageSize >= result.total) return [...ids]
    }
  }
  // Never label a truncated account list as a complete unread total.
  throw new Error('未读会话范围尚未同步完成')
}

export const notificationRuntimePorts: NotificationRuntimePorts = {
  loadInbox: () => loadNotificationInbox(),
  async loadCounts(owner) {
    const ids = await authorizedConversationIDs(owner)
    if (!ids.length) return {}
    const chat = await ensureTencentIMReady(`campus_${owner.userId}`, credentialsFor(owner))
    const counts: Record<string, number> = {}
    const authorized = new Set(ids)
    for (let start = 0; start < ids.length; start += 100) {
      const result = await withTimeout(chat.getConversationList(ids.slice(start, start + 100)), 10000, '未读消息同步超时') as {
        data?: { conversationList?: Array<{ conversationID: string; unreadCount: number }> }
      }
      if (!Array.isArray(result.data?.conversationList)) throw new Error('未读消息快照无效')
      for (const row of result.data.conversationList) {
        if (!authorized.has(row.conversationID)) continue
        if (!Number.isSafeInteger(row.unreadCount) || row.unreadCount < 0) throw new Error('未读消息快照无效')
        counts[row.conversationID] = row.unreadCount
      }
    }
    if (ids.some(id => !Object.prototype.hasOwnProperty.call(counts, id))) throw new Error('未读消息快照尚未完整')
    return counts
  },
  async listen(owner, changed) {
    if (owner.role === 'student' && !(await getFeatureVisibility()).messages) return () => undefined
    const chat = await ensureTencentIMReady(`campus_${owner.userId}`, credentialsFor(owner))
    const stopMessages = listenTencentMessageEvents(chat, changed)
    const observed = new Map<string, number>()
    const onConversation = (event: { data?: Array<{ conversationID?: string; unreadCount?: number }> }) => {
      let different = false
      for (const row of Array.isArray(event?.data) ? event.data : []) {
        if (!row.conversationID || !Number.isSafeInteger(row.unreadCount) || Number(row.unreadCount) < 0) continue
        if (observed.get(row.conversationID) !== row.unreadCount) different = true
        observed.set(row.conversationID, Number(row.unreadCount))
      }
      if (different) changed()
    }
    const onReady = () => changed()
    chat.on(TencentCloudChat.EVENT.CONVERSATION_LIST_UPDATED, onConversation)
    chat.on(TencentCloudChat.EVENT.SDK_READY, onReady)
    return () => {
      stopMessages()
      chat.off(TencentCloudChat.EVENT.CONVERSATION_LIST_UPDATED, onConversation)
      chat.off(TencentCloudChat.EVENT.SDK_READY, onReady)
    }
  }
}

export const notificationRuntime = createNotificationRuntime(notificationRuntimePorts)
