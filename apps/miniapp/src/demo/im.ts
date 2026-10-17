import type { Message, ChatSDK } from '@tencentcloud/chat'
import type { IMConversationContext } from '../services/c-trade'
import { DEMO_SELF, demoUsers } from './data'
import { demoConversation, demoRelation } from './api'
import { ensureDemo } from './runtime'
export interface IMCredentials { sdkAppId: string; userId: string; userSig: string }
export type { Message as TencentMessage }
const histories = new Map<string, Message[]>()
let sequence = 0
export async function listIMConversations() { ensureDemo(); return demoUsers.slice(1).map(user => demoConversation(user.id)) }
export async function contactUser(userId: string) { ensureDemo(); return demoConversation(Number(userId)) }
export async function getIMConversation(id: number) { ensureDemo(); return demoRelation(id) }
export async function getTencentIMLoginUser(expected = '') {
  ensureDemo()
  const id = `campus_${DEMO_SELF}`
  if (expected && expected !== id) throw new Error('模拟身份不匹配')
  return id
}
function check(context: IMConversationContext) {
  ensureDemo()
  const canonical = demoRelation(context.relationId)
  if (canonical.peerUserId !== context.peerUserId || canonical.buyerId !== context.buyerId || canonical.sellerId !== context.sellerId) throw new Error('模拟会话不匹配')
}
function message(context: IMConversationContext, from: string, text: string, time: number): Message {
  return { ID: `demo-${++sequence}`, type: 'TIMTextElem', from, to: from === `campus_${DEMO_SELF}` ? context.peerUserId : `campus_${DEMO_SELF}`,
    conversationID: context.conversationId, time, payload: { text } } as Message
}
export async function loadTencentMessages(context: IMConversationContext) {
  check(context)
  if (!histories.has(context.conversationId)) {
    const now = Math.floor(Date.now()/1000)
    histories.set(context.conversationId, [
      message(context, context.peerUserId, '你好，这是模拟聊天。可以试试头像、主页和消息输入的显示效果。', now-680),
      message(context, `campus_${DEMO_SELF}`, '看到了，头像和主页背景都是同一个用户的资料。', now-570),
      message(context, context.peerUserId, '点头像可以打开主页；输入的消息只留在本次体验里，不会发给真实用户。', now-480),
    ])
  }
  return [...histories.get(context.conversationId)!]
}
export async function sendTencentText(context: IMConversationContext, text: string) {
  check(context)
  if (!text.trim()) throw new Error('请输入内容')
  await loadTencentMessages(context)
  await new Promise(resolve=>setTimeout(resolve,180))
  const item=message(context,`campus_${DEMO_SELF}`,text,Math.floor(Date.now()/1000))
  histories.get(context.conversationId)!.push(item)
  return item
}
export async function subscribeTencentMessages(context: IMConversationContext, _callback: (messages: Message[]) => void, _expected = '') { check(context); return () => {} }
export async function resetTencentIMSession() { histories.clear() }
export async function getIMCredentials(): Promise<IMCredentials> { throw new Error('模拟体验不申请腾讯 IM 凭据') }
export async function ensureTencentIMReady(): Promise<ChatSDK> { throw new Error('模拟体验不连接腾讯 IM') }
