import { useCallback, useEffect, useRef, useState } from 'react'
import { useDidHide, useDidShow } from '@tarojs/taro'
import { Text, View } from '@tarojs/components'
import { useNotificationState } from '../hooks/use-notification-runtime'
import { loadNotificationInbox, markNotificationRead, notificationTargetPath, type NotificationInboxItem } from '../services/notification'
import { notificationRuntime } from '../services/notification-runtime'
import { useSessionStore } from '../stores/session'
import Taro from '../services/taro'

/** Delivery messages expose their own inbox without opening the community feature gate. */
export default function NotificationInbox({ enabled }: { enabled: boolean }) {
  const token = useSessionStore(state => state.token)
  const role = useSessionStore(state => state.identity?.currentIdentity)
  const notifications = useNotificationState()
  const [items, setItems] = useState<NotificationInboxItem[]>([])
  const [cursor, setCursor] = useState('')
  const [error, setError] = useState('')
  const busy = useRef(false)
  const visible = useRef(true)
  const epoch = useRef(0)
  const pages = useRef(1)
  const load = useCallback(async (next = '') => {
    if (!enabled || !visible.current || busy.current) return
    const version = epoch.current
    const current = () => version === epoch.current && visible.current && token === useSessionStore.getState().token && role === useSessionStore.getState().identity?.currentIdentity
    busy.current = true
    try {
      const result = await loadNotificationInbox(next, 10)
      if (!current()) return
      setItems(previous => next ? [...previous, ...result.items.filter(item => !previous.some(old => old.id === item.id))] : result.items)
      setCursor(result.nextCursor); setError(''); pages.current = next ? pages.current + 1 : 1
    } catch { if (current()) setError('通知暂未同步，点击重试') }
    finally { if (current()) busy.current = false }
  }, [enabled, token, role])
  useEffect(() => {
    epoch.current += 1; busy.current = false; pages.current = 1
    setItems([]); setCursor(''); setError(''); void load()
    return () => { epoch.current += 1; busy.current = false }
  }, [load])
  useEffect(() => { if (pages.current === 1) void load() }, [notifications.inboxRevision, load])
  useDidHide(() => { visible.current = false; epoch.current += 1; busy.current = false })
  useDidShow(() => { visible.current = true; void load() })
  const open = async (item: NotificationInboxItem) => {
    try {
      const path = notificationTargetPath(item)
      if (!path) throw new Error('通知入口已失效，请返回工作台查看')
      if (!item.read) await markNotificationRead(item.id)
      if (token !== useSessionStore.getState().token || role !== useSessionStore.getState().identity?.currentIdentity) return
      setItems(previous => previous.map(row => row.id === item.id ? { ...row, read: true } : row))
      void notificationRuntime.refresh()
      await Taro.navigateTo({ url: path })
    } catch (value) { void Taro.showToast({ title: value instanceof Error ? value.message : '通知打开失败', icon: 'none' }) }
  }
  if (!enabled || (!items.length && !error)) return null
  return <View className='merchant-messages-card'>
    <Text className='merchant-messages-card__footer'>订单与服务通知{notifications.inboxUnread != null && notifications.inboxUnread > 0 ? ` · ${notifications.inboxUnread} 条未读` : ''}</Text>
    {items.map(item => <View key={item.id} className='merchant-message-item' onClick={() => void open(item)}>
      <View className='merchant-message-item__body'>
        <Text className='merchant-message-item__title'>{item.title}</Text>
        <Text className={`merchant-message-item__preview ${item.read ? '' : 'merchant-message-item__preview--unread'}`}>{item.summary}</Text>
      </View>
      <Text className='merchant-message-item__time'>{item.createdAt.replace('T', ' ').slice(5, 16)}</Text>
    </View>)}
    {error && <Text className='merchant-messages-card__footer' onClick={() => void load()}>{error}</Text>}
    {cursor && <Text className='merchant-messages-card__footer' onClick={() => void load(cursor)}>加载更多通知</Text>}
  </View>
}
