import React, { useEffect, useState } from 'react'
import { Text, View } from '@tarojs/components'
import { listCommunityNotifications, markCommunityNotificationRead } from '../../services/b-community'
import Taro from '../../services/taro'
import type { CommunityNotification } from '../../types'
import './index.scss'

const labels:Record<CommunityNotification['type'],string>={post_commented:'评论了你的帖子',post_liked:'赞了你的帖子',user_followed:'关注了你',product_commented:'留言了你的商品',product_comment_replied:'回复了你的商品留言'}

export default function NotificationsPage(){
  const[items,setItems]=useState<CommunityNotification[]>([])
  const[unread,setUnread]=useState(0)
  const load=async()=>{const result=await listCommunityNotifications();setItems(result.items);setUnread(result.unread)}
  useEffect(()=>{void load().catch(showError)},[])
  const open=async(item:CommunityNotification)=>{try{if(!item.read)await markCommunityNotificationRead(item.id);if(item.productId)await Taro.navigateTo({url:`/pages/secondhand-detail/index?id=${encodeURIComponent(item.productId)}&commentId=${encodeURIComponent(item.productCommentId||'')}`});else if(item.postId)await Taro.navigateTo({url:`/pages/post-detail/index?id=${item.postId}`});else await load()}catch(error){showError(error)}}
  return <View className='page notifications-page'>
    <View className='notifications-head'><Text>社区通知</Text><Text>{unread} 条未读</Text></View>
    {items.length===0&&<View className='card notifications-empty'>暂无通知</View>}
    {items.map(item=><View key={item.id} className={`card notification ${item.read?'':'notification--unread'}`} onClick={()=>void open(item)}>
      <Text className='notification__title'>{item.actor} {labels[item.type]}</Text>
      <Text className='notification__time'>{item.createdAt}</Text>
    </View>)}
  </View>
}

function showError(error:unknown){void Taro.showToast({title:error instanceof Error?error.message:'加载失败',icon:'none'})}
