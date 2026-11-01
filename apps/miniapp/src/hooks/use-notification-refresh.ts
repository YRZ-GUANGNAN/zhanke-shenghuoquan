import { useEffect, useRef, useState } from 'react'
import { useDidHide, useDidShow } from '@tarojs/taro'
import { useNotificationState } from './use-notification-runtime'

/** Notifications invalidate a list; they never supply order facts or change its selected page. */
export function useNotificationRefresh(enabled: boolean, canRefresh: boolean, refresh: () => Promise<boolean>) {
  const notifications = useNotificationState()
  const seen = useRef({ owner: '', revision: '', initialized: false })
  const visible = useRef(true)
  const running = useRef(false)
  const attempted = useRef('')
  const action = useRef(refresh)
  action.current = refresh
  const [pending, setPending] = useState(false)
  const [foreground, setForeground] = useState(0)
  useDidHide(() => { visible.current = false })
  useDidShow(() => { visible.current = true; setForeground(value => value + 1) })
  useEffect(() => {
    if (seen.current.owner !== notifications.ownerKey) {
      seen.current = { owner: notifications.ownerKey, revision: '', initialized: false }
      setPending(false)
      attempted.current = ''
    }
    if (!notifications.ownerKey || notifications.inboxUnread == null) return
    const previous = seen.current
    if (previous.initialized && previous.revision !== notifications.inboxRevision) setPending(true)
    seen.current = { ...previous, initialized: true, revision: notifications.inboxRevision }
  }, [notifications.ownerKey, notifications.inboxRevision, notifications.inboxUnread])
  useEffect(() => {
    if (!pending || !enabled || !canRefresh || !visible.current || running.current) return
    const owner = notifications.ownerKey
    const revision = notifications.inboxRevision
    const attemptKey = `${owner}:${revision}`
    if (attempted.current === attemptKey) return
    attempted.current = attemptKey
    running.current = true
    void action.current().then(success => {
      if (success && seen.current.owner === owner && seen.current.revision === revision) setPending(false)
    }).catch(() => undefined).then(() => { running.current = false })
  }, [pending, enabled, canRefresh, foreground, notifications.ownerKey, notifications.inboxRevision])
  return { pending, acknowledge: () => setPending(false) }
}
