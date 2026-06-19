import React, {useEffect, useMemo, useState} from 'react'
import {Button, Input, ScrollView, Text, View} from '@tarojs/components'
import type {IMConversationContext} from '../../services/c-trade'
import {loadTencentMessages, sendTencentText, subscribeTencentMessages, type TencentMessage} from '../../services/e-im'
import Taro from '../../services/taro'
import './index.scss'

export default function ChatPage(){
  const params=Taro.getCurrentInstance().router?.params||{}
  const context=useMemo<IMConversationContext>(()=>({relationId:Number(params.relationId||0),conversationId:params.conversationId||'',peerUserId:params.peerUserId||'',productId:Number(params.productId||0),buyerId:Number(params.buyerId||0),sellerId:Number(params.sellerId||0)}),[])
  const[messages,setMessages]=useState<TencentMessage[]>([])
  const[text,setText]=useState('')
  const[ready,setReady]=useState(false)
  useEffect(()=>{let active=true;let unsubscribe:(()=>void)|undefined;(async()=>{const initial=await loadTencentMessages(context);if(active){setMessages(initial);setReady(true)}unsubscribe=await subscribeTencentMessages(context,next=>setMessages(current=>mergeMessages(current,next)))})().catch(showError);return()=>{active=false;unsubscribe?.()}},[])
  const send=async()=>{const value=text.trim();if(!value||!ready)return;try{const message=await sendTencentText(context,value);setMessages(current=>mergeMessages(current,[message]));setText('')}catch(error){showError(error)}}
  return <View className='chat-page'><View className='chat-product'>正在沟通商品 #{context.productId}</View><ScrollView className='chat-list' scrollY>{messages.map(message=><View key={message.ID} className={`chat-message chat-message--${message.flow}`}><Text>{message.type==='TIMTextElem'?String(message.payload?.text||''):'[非文本消息]'}</Text><Text className='chat-message__time'>{formatTime(message.time)}</Text></View>)}</ScrollView><View className='chat-compose'><Input value={text} maxlength={500} placeholder={ready?'输入消息':'腾讯云 IM 登录中'} disabled={!ready} onInput={event=>setText(String(event.detail.value))} onConfirm={()=>void send()}/><Button className='cc-button cc-button--primary' disabled={!ready||!text.trim()} onClick={()=>void send()}>发送</Button></View></View>
}
function mergeMessages(current:TencentMessage[],next:TencentMessage[]){const values=new Map(current.map(item=>[item.ID,item]));next.forEach(item=>values.set(item.ID,item));return[...values.values()].sort((a,b)=>a.time-b.time)}
function formatTime(value:number){const date=new Date(value*1000);return`${String(date.getHours()).padStart(2,'0')}:${String(date.getMinutes()).padStart(2,'0')}`}
function showError(error:unknown){void Taro.showToast({title:error instanceof Error?error.message:'腾讯云 IM 操作失败',icon:'none'})}
