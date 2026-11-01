import { withFeaturePage } from '../../components/feature-page-gate'
import React, { useCallback, useEffect, useRef, useState } from 'react'
import { useDidHide, useDidShow } from '@tarojs/taro'
import { Text, View } from '@tarojs/components'
import { listCommunityNotifications, markCommunityNotificationRead } from '../../services/b-community'
import { loadNotificationInbox, markNotificationRead, notificationTargetPath, type NotificationInboxItem } from '../../services/notification'
import { notificationRuntime } from '../../services/notification-runtime'
import { useSessionStore } from '../../stores/session'
import Taro from '../../services/taro'
import type { CommunityNotification } from '../../types'
import './index.scss'

const labels:Record<CommunityNotification['type'],string>={post_commented:'评论了你的帖子',post_liked:'赞了你的帖子',user_followed:'关注了你',product_commented:'留言了你的商品',product_comment_replied:'回复了你的商品留言'}

function NotificationsPage(){
  const token = useSessionStore(state => state.token)
  const identity = useSessionStore(state => state.identity?.currentIdentity)
  const[items,setItems]=useState<CommunityNotification[]>([])
  const[unread,setUnread]=useState<number|null>(null)
  const[deliveryItems,setDeliveryItems]=useState<NotificationInboxItem[]>([])
  const[deliveryUnread,setDeliveryUnread]=useState<number|null>(null)
  const [nextCursor, setNextCursor] = useState('')
  const [error, setError] = useState('')
  const busy = useRef(false)
  const epoch = useRef(0)
  const visible = useRef(true)
  const load = useCallback(async (cursor = '') => {
    if (!visible.current || busy.current) return
    busy.current = true
    const generation = epoch.current
    const current = () => visible.current && epoch.current === generation && useSessionStore.getState().token === token && useSessionStore.getState().identity?.currentIdentity === identity
    const settle = <T,>(promise: Promise<T>) => promise.then(value => ({ value, error: '' }), () => ({ value: null, error: '通知暂未同步，请重试' }))
    const [community, delivery] = await Promise.all([
      cursor ? Promise.resolve({ value: null, error: '' }) : settle(listCommunityNotifications()), settle(loadNotificationInbox(cursor))
    ])
    if (!current()) return
    if (community.value) { setItems(community.value.items); setUnread(community.value.unread) }
    if (delivery.value) {
      setDeliveryItems(previous => cursor ? [...previous, ...delivery.value!.items.filter(item => !previous.some(old => old.id === item.id))] : delivery.value!.items)
      setDeliveryUnread(delivery.value.unread); setNextCursor(delivery.value.nextCursor)
    }
    setError(community.error || delivery.error)
    busy.current = false
  }, [token, identity])
  useEffect(() => {
    epoch.current += 1; busy.current = false
    setItems([]); setDeliveryItems([]); setUnread(null); setDeliveryUnread(null); setNextCursor(''); setError('')
    void load()
    return () => { epoch.current += 1; busy.current = false }
  }, [load])
  useDidHide(() => { visible.current = false; epoch.current += 1; busy.current = false })
  useDidShow(() => { visible.current = true; void load() })
  const open=async(item:CommunityNotification)=>{try{if(!item.read)await markCommunityNotificationRead(item.id);if(useSessionStore.getState().token!==token)return;if(item.productId)await Taro.navigateTo({url:`/pages/secondhand-detail/index?id=${encodeURIComponent(item.productId)}&commentId=${encodeURIComponent(item.productCommentId||'')}`});else if(item.postId)await Taro.navigateTo({url:`/pages/post-detail/index?id=${item.postId}`});else await load()}catch(value){showError(value)}}
  const openDelivery = async (item: NotificationInboxItem) => {
    try {
      const path = notificationTargetPath(item)
      if (!path) throw new Error('通知入口已失效，请返回工作台查看')
      if (!item.read) await markNotificationRead(item.id)
      if (useSessionStore.getState().token !== token || useSessionStore.getState().identity?.currentIdentity !== identity) return
      setDeliveryItems(previous => previous.map(row => row.id === item.id ? { ...row, read: true } : row))
      void notificationRuntime.refresh()
      await Taro.navigateTo({ url: path })
    } catch (value) { showError(value) }
  }
  return <View className='page notifications-page'>
    <View className='notifications-head'><Text>社区通知</Text><Text>{unread == null ? '未读待同步' : `${unread} 条未读`}</Text></View>
    {items.length===0&&<View className='card notifications-empty'>暂无社区通知</View>}
    {items.map(item=><View key={item.id} className={`card notification ${item.read?'':'notification--unread'}`} onClick={()=>void open(item)}>
      <Text className='notification__title'>{item.actor} {labels[item.type]}</Text>
      <Text className='notification__time'>{item.createdAt}</Text>
    </View>)}
    <View className='notifications-head'><Text>订单与服务通知</Text><Text>{deliveryUnread == null ? '未读待同步' : `${deliveryUnread} 条未读`}</Text></View>
    {deliveryItems.length===0&&<View className='card notifications-empty'>暂无订单与服务通知</View>}
    {deliveryItems.map(item=><View key={`delivery-${item.id}`} className={`card notification ${item.read?'':'notification--unread'}`} onClick={()=>void openDelivery(item)}>
      <Text className='notification__title'>{item.title}</Text>
      <Text className='notification__time'>{item.summary}</Text>
    </View>)}
    {nextCursor && <View className='card notifications-empty' onClick={() => void load(nextCursor)}>加载更多通知</View>}
    {error && <View className='card notifications-empty' onClick={() => void load()}>{error}</View>}
  </View>
}

function showError(error:unknown){void Taro.showToast({title:error instanceof Error?error.message:'加载失败',icon:'none'})}

export default withFeaturePage(NotificationsPage)
