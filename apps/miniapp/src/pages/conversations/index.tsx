import { withFeaturePage } from '../../components/feature-page-gate'
import React, {useEffect, useState} from 'react'
import {Text, View} from '@tarojs/components'
import type {IMConversationContext} from '../../services/c-trade'
import {listIMConversations} from '../../services/e-im'
import Taro from '../../services/taro'
import './index.scss'

function ConversationsPage(){
  const[items,setItems]=useState<IMConversationContext[]>([])
  useEffect(()=>{listIMConversations().then(setItems).catch(showError)},[])
  const open=(item:IMConversationContext)=>Taro.navigateTo({url:`/pages/chat/index?conversationId=${encodeURIComponent(item.conversationId)}&peerUserId=${encodeURIComponent(item.peerUserId)}&productId=${item.productId}&buyerId=${item.buyerId}&sellerId=${item.sellerId}&relationId=${item.relationId}`})
  return <View className='page conversation-page'>{items.length===0&&<View className='card empty'>暂无交易会话</View>}{items.map(item=><View key={item.relationId} className='card conversation-row' onClick={()=>void open(item)}><View><Text className='conversation-row__title'>商品 #{item.productId}</Text><Text className='conversation-row__meta'>腾讯云 C2C · {item.peerUserId}</Text></View><Text>进入 ›</Text></View>)}</View>
}
function showError(error:unknown){void Taro.showToast({title:error instanceof Error?error.message:'会话加载失败',icon:'none'})}

export default withFeaturePage(ConversationsPage)
