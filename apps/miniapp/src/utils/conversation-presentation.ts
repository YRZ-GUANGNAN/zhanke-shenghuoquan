import type { IMConversationContext } from '../services/c-trade'
import type { Product } from '../types'

export function conversationPeer(context: IMConversationContext | null, product: Product | null, currentUserId: string) {
  const buyer = Boolean(currentUserId) && String(context?.buyerId) === currentUserId
  const seller = Boolean(currentUserId) && String(context?.sellerId) === currentUserId
  const id = buyer ? String(context!.sellerId) : seller ? String(context!.buyerId) : ''
  const fallback = buyer ? '校园卖家' : seller ? '咨询买家' : '交易对方'
  if (!/^[1-9]\d*$/.test(id)) return { id: '', name: fallback, avatar: '' }

  // Profile data is optional on older servers. Never attach the product seller's
  // identity to a buyer, or trust a peer that does not match this conversation.
  if (context?.peer && String(context.peer.id) === id) {
    return { id, name: context.peer.nickname.trim() || fallback, avatar: context.peer.avatar || '' }
  }
  if (buyer && product?.sellerId === id && product.id === String(context?.productId)) {
    return { id, name: product.sellerName.trim() || fallback, avatar: product.sellerAvatar || '' }
  }
  return { id, name: fallback, avatar: '' }
}
