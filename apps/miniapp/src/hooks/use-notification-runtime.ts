import { useEffect, useState } from 'react'
import { useDidHide, useDidShow } from '@tarojs/taro'
import { notificationRuntime } from '../services/notification-runtime'
import { useSessionStore } from '../stores/session'

export function useNotificationRuntime() {
  const status = useSessionStore(state => state.status)
  const token = useSessionStore(state => state.token)
  const userId = useSessionStore(state => state.profile.id)
  const role = useSessionStore(state => state.identity?.currentIdentity)
  useEffect(() => {
    if (status !== 'authenticated' || !token || !userId || !role) { notificationRuntime.stop(); return }
    notificationRuntime.start({ key: `${token}:${userId}:${role}`, userId, role })
    return () => notificationRuntime.stop()
  }, [status, token, userId, role])
  useDidHide(() => notificationRuntime.foreground(false))
  useDidShow(() => notificationRuntime.foreground(true))
}

export function useNotificationState() {
  const status = useSessionStore(value => value.status)
  const userId = useSessionStore(value => value.profile.id)
  const role = useSessionStore(value => value.identity?.currentIdentity)
  const [state, setState] = useState(notificationRuntime.getState)
  useEffect(() => {
    setState(notificationRuntime.getState())
    return notificationRuntime.subscribe(() => setState(notificationRuntime.getState()))
  }, [])
  const valid = status === 'authenticated' && state.userId === userId && state.role === role
  return valid ? state : { ...state, ownerKey: '', inboxUnread: null, inboxRevision: '', messageRevision: 0,
    unread: { ...state.unread, confirmed: false, total: null, conversations: {} } }
}

export function notificationBadgeLabel(state: ReturnType<typeof useNotificationState>): string {
  const chat = state.unread.confirmed ? state.unread.total : null
  const inbox = state.inboxUnread
  if (chat == null || inbox == null) return (chat || 0) + (inbox || 0) > 0 ? '•' : ''
  const total = chat + inbox
  return total > 99 ? '99+' : total > 0 ? String(total) : ''
}
