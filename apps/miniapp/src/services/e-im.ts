// Domain E adapter: official Tencent Cloud Chat SDK only. No message is persisted locally or by this backend.
import TencentCloudChat, {type ChatSDK, type Message} from '@tencentcloud/chat'
import type { IMConversationContext } from './c-trade'
import { request } from './api'

export interface IMCredentials{sdkAppId:string;userId:string;userSig:string}
interface ConversationPage{items:IMConversationContext[];page:number;pageSize:number;total:number}

let sdk:ChatSDK|null=null
let sdkAppId=''
let loginPromise:Promise<ChatSDK>|null=null

export function getIMCredentials():Promise<IMCredentials>{return request('/im/usersig',{url:'',timeout:10000})}
export async function listIMConversations():Promise<IMConversationContext[]>{return(await request<ConversationPage>('/im/conversations')).items}

export async function ensureTencentIMReady():Promise<ChatSDK>{
  if(loginPromise)return loginPromise
  loginPromise=(async()=>{
    const credentials=await getIMCredentials()
    if(!sdk||sdkAppId!==credentials.sdkAppId){
      if(sdk)await sdk.destroy()
      sdk=TencentCloudChat.create({SDKAppID:Number(credentials.sdkAppId)})
      sdk.setLogLevel(1)
      sdkAppId=credentials.sdkAppId
    }
    if(sdk.getLoginUser()===credentials.userId&&sdk.isReady())return sdk
    if(sdk.getLoginUser()&&sdk.getLoginUser()!==credentials.userId)await sdk.logout()
    let timer:ReturnType<typeof setTimeout>|undefined
    let onReady:()=>void=()=>undefined
    const cleanup=()=>{if(timer)clearTimeout(timer);sdk?.off(TencentCloudChat.EVENT.SDK_READY,onReady)}
    const ready=new Promise<void>((resolve,reject)=>{
      onReady=()=>{cleanup();resolve()}
      timer=setTimeout(()=>{cleanup();reject(new Error('腾讯云 IM 初始化超时'))},10000)
      sdk?.on(TencentCloudChat.EVENT.SDK_READY,onReady)
    })
    try{await sdk.login({userID:credentials.userId,userSig:credentials.userSig});if(!sdk.isReady())await ready;else cleanup()}catch(error){cleanup();throw error}
    return sdk
  })()
  try{return await loginPromise}finally{loginPromise=null}
}

export async function loadTencentMessages(context:IMConversationContext):Promise<Message[]>{
  const chat=await ensureTencentIMReady()
  await chat.getConversationProfile(context.conversationId)
  const result=await chat.getMessageList({conversationID:context.conversationId}) as {data?:{messageList?:Message[]}}
  await chat.setMessageRead({conversationID:context.conversationId})
  return result.data?.messageList||[]
}

export async function sendTencentText(context:IMConversationContext,text:string):Promise<Message>{
  const chat=await ensureTencentIMReady()
  const message=chat.createTextMessage({to:context.peerUserId,conversationType:TencentCloudChat.TYPES.CONV_C2C,payload:{text}})
  const result=await chat.sendMessage(message) as {data?:{message?:Message}}
  return result.data?.message||message
}

export async function subscribeTencentMessages(context:IMConversationContext,handler:(messages:Message[])=>void):Promise<()=>void>{
  const chat=await ensureTencentIMReady()
  const listener=(event:{data?:Message[]})=>handler((event.data||[]).filter(item=>item.conversationID===context.conversationId))
  chat.on(TencentCloudChat.EVENT.MESSAGE_RECEIVED,listener)
  return()=>chat.off(TencentCloudChat.EVENT.MESSAGE_RECEIVED,listener)
}

export type {Message as TencentMessage}
