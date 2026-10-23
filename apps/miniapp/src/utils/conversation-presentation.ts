import type { IMConversationContext } from '../services/c-trade'
import type { Product } from '../types'

export function conversationPeer(context: IMConversationContext | null, product: Product | null, currentUserId: string) {
  const buyer = Boolean(currentUserId) && String(context?.buyerId) === currentUserId
  const seller = Boolean(currentUserId) && String(context?.sellerId) === currentUserId
  const id = buyer ? String(context!.sellerId) : seller ? String(context!.buyerId) : ''
  const fallback = context?.referenceType === 'user' ? '校园用户' : buyer ? '校园卖家' : seller ? '咨询买家' : '交易对方'
  if (!/^[1-9]\d*$/.test(id)) return { id: '', name: fallback, avatar: '' }

  // Profile data is optional on older servers. Never attach the product seller's
  // identity to a buyer, or trust a peer that does not match this conversation.
  if (context?.peer && String(context.peer.id) === id) {
    return { id, name: context.peer.nickname.trim() || fallback, avatar: context.peer.avatar || '' }
  }
  if (context?.referenceType !== 'user' && buyer && product?.sellerId === id && product.id === String(context?.productId)) {
    return { id, name: product.sellerName.trim() || fallback, avatar: product.sellerAvatar || '' }
  }
  return { id, name: fallback, avatar: '' }
}

// Product relations and profile relations share one Tencent C2C history.
export function uniquePeerConversations(items: IMConversationContext[]): IMConversationContext[] {
  const peers = new Map<string, IMConversationContext>()
  for (const item of items) {
    if (!item.peerUserId || item.conversationId !== `C2C${item.peerUserId}`) continue
    const previous = peers.get(item.conversationId)
    if (!previous || (item.referenceType === 'user' && previous.referenceType !== 'user')) peers.set(item.conversationId, item)
  }
  return [...peers.values()]
}
