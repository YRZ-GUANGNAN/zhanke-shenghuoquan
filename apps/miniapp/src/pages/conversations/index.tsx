import { withFeaturePage } from '../../components/feature-page-gate'
import React, {useEffect, useState} from 'react'
import {Text, View} from '@tarojs/components'
import type {IMConversationContext} from '../../services/c-trade'
import {listIMConversations} from '../../services/e-im'
import ChatAvatar from '../../components/chat-avatar'
import {conversationPeer, uniquePeerConversations} from '../../utils/conversation-presentation'
import {useSessionStore} from '../../stores/session'
import Taro from '../../services/taro'
import './index.scss'

function ConversationsPage(){
  const[items,setItems]=useState<IMConversationContext[]>([])
  const currentUserId=useSessionStore(state=>state.profile.id)
  useEffect(()=>{let active=true;setItems([]);listIMConversations().then(value=>{if(active)setItems(uniquePeerConversations(value))}).catch(error=>{if(active)showError(error)});return()=>{active=false}},[currentUserId])
  const open=(item:IMConversationContext)=>Taro.navigateTo({url:`/pages/secondhand-chat/index?relationId=${encodeURIComponent(String(item.relationId))}`})
  return <View className='page conversation-page'>{items.length===0&&<View className='card empty'>暂无私信会话</View>}{items.map(item=>{const peer=conversationPeer(item,null,currentUserId);return <View key={item.relationId} className='card conversation-row' onClick={()=>void open(item)}><ChatAvatar src={peer.avatar} name={peer.name}/><View><Text className='conversation-row__title'>{peer.name}</Text><Text className='conversation-row__meta'>{item.referenceType==='user'?'用户私信':'二手交流'}</Text></View><Text>进入 ›</Text></View>})}</View>
}
function showError(error:unknown){void Taro.showToast({title:error instanceof Error?error.message:'会话加载失败',icon:'none'})}

export default withFeaturePage(ConversationsPage)
