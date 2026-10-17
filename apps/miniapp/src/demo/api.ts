import { FEATURE_KEYS } from '../services/feature-policy-model'
import { DEMO_SELF, demoComments, demoPosts, demoProducts, demoUser, demoUsers } from './data'
import type { IMConversationContext } from '../services/c-trade'

export class DemoRequestError extends Error { constructor(message: string, readonly status = 400) { super(message) } }
const following = new Set<number>()
const likes = new Set<string>()
const favorites = new Set<string>()
const history: string[] = []
const page = <T,>(items: T[], q: Record<string, unknown>) => {
  const size = Math.max(1, Math.min(50, Number(q.pageSize) || 20))
  const current = Math.max(1, Number(q.page) || 1)
  return { items: items.slice((current - 1) * size, current * size), total: items.length, publicCount: items.length, page: current, pageSize: size, filterVersion: 1 }
}
export const demoIdentity = { currentIdentity: 'student', identities: [{ identity: 'student', available: true, status: 'active' }, { identity: 'rider', available: false, status: 'missing' }, { identity: 'merchant', available: false, status: 'missing' }] }
export function demoConversation(peerId: number, productId = 0): IMConversationContext {
  if (peerId === DEMO_SELF) throw new DemoRequestError('不能与自己私信')
  const peer = demoUser(peerId)
  return { relationId: productId ? 960000 + productId - 930000 : 950000 + peerId - 910000,
    referenceType: productId ? 'product' : 'user', productId, buyerId: DEMO_SELF, sellerId: peerId,
    peerUserId: `campus_${peerId}`, conversationId: `C2Ccampus_${peerId}`,
    peer: { id: peer.id, nickname: peer.nickname, avatar: peer.avatar } }
}
export function demoRelation(id: number) {
  if (id > 960000 && id < 970000) {
    const product = demoProducts.find(p => p.id === 930000 + id - 960000)
    if (product) return demoConversation(product.sellerId, product.id)
  }
  return demoConversation(910000 + id - 950000)
}
function toggle(set: Set<string>, key: string, method: string) { method === 'DELETE' ? set.delete(key) : set.add(key); return set.has(key) }

export function demoRequest(input: string, method = 'GET', data?: unknown): unknown {
  const [pathname, search = ''] = input.split('?')
  const q: Record<string, unknown> = {}
  // WeChat JS runtimes do not consistently provide the browser URLSearchParams API.
  search.split('&').filter(Boolean).forEach(pair => {
    const separator = pair.indexOf('=')
    const key = separator < 0 ? pair : pair.slice(0, separator)
    const value = separator < 0 ? '' : pair.slice(separator + 1)
    q[decodeURIComponent(key.replace(/\+/g, ' '))] = decodeURIComponent(value.replace(/\+/g, ' '))
  })
  const body = (data && typeof data === 'object' ? data : {}) as Record<string, any>
  const self = demoUser(DEMO_SELF)
  if (pathname === '/features' && method === 'GET') return { enabled: true, presentationMode: 'standard', postingMode: 'all_users', rowVersion: 1, flags: FEATURE_KEYS.reduce((flags,key)=>{flags[key]=true;return flags},{} as Record<string,boolean>) }
  if (pathname === '/auth/wechat-login' && method === 'POST') return { token: 'visual-demo-local-only', user: self }
  if (pathname === '/auth/me' && method === 'GET') return self
  if (pathname === '/auth/identity' && method === 'GET') return demoIdentity
  if (pathname === '/auth/settings' && method === 'GET') return { wechatBound: false, phoneBound: false, maskedPhone: '', blacklistCount: 0, orderNotice: false, socialNotice: false, quietHours: false, personalized: false }
  if (pathname === '/im/conversations' && method === 'GET') return page(demoUsers.slice(1).map(p => demoConversation(p.id)), q)
  let match = pathname.match(/^\/im\/users\/(\d+)\/conversation$/)
  if (match && method === 'PUT') return demoConversation(Number(match[1]))
  match = pathname.match(/^\/im\/conversations\/(\d+)$/)
  if (match && method === 'GET') return demoRelation(Number(match[1]))
  match = pathname.match(/^\/community\/users\/(\d+)\/(profile|posts|follow)$/)
  if (match) {
    const id = Number(match[1]); const profile = demoUser(id)
    if (match[2] === 'profile' && method === 'GET') return { profile, followingCount: 12, followerCount: 26 + (following.has(id) ? 1 : 0), likedCount: 89, isFollowing: following.has(id) }
    if (match[2] === 'posts' && method === 'GET') return page(demoPosts.filter(p => p.authorId === id), q)
    if (match[2] === 'follow' && ['PUT', 'DELETE'].includes(method)) { method === 'PUT' ? following.add(id) : following.delete(id); return { following: following.has(id) } }
  }
  if (pathname === '/community/me/summary' && method === 'GET') return { favorites: favorites.size, posts: demoPosts.filter(p => p.authorId === DEMO_SELF).length, comments: 0 }
  if (['/community/posts', '/community/following-feed', '/community/me/posts', '/community/me/favorites'].includes(pathname) && method === 'GET') {
    let items = [...demoPosts]
    if (pathname.endsWith('/me/posts')) items = items.filter(p => p.authorId === DEMO_SELF)
    if (pathname.endsWith('/favorites')) items = items.filter(p => favorites.has(`post:${p.id}`))
    if (q.sort === 'popular') items.sort((a, b) => b.likedCount - a.likedCount)
    return page(items, q)
  }
  match = pathname.match(/^\/community\/(?:me\/)?posts\/(\d+)(.*)$/)
  if (match) {
    const post = demoPosts.find(p => p.id === Number(match![1])); if (!post) throw new DemoRequestError('演示帖子不存在', 404)
    const tail = match[2]; const key = `post:${post.id}`
    if (!tail && method === 'GET') return post
    if (tail === '/viewer-state' && method === 'GET') return { liked: likes.has(key), likedCount: post.likedCount + Number(likes.has(key)), favorited: favorites.has(key), followingAuthor: following.has(post.authorId) }
    if (tail === '/like' && ['PUT','DELETE'].includes(method)) { const liked = toggle(likes, key, method); return { liked, likedCount: post.likedCount + Number(liked) } }
    if (tail === '/favorite' && ['PUT','DELETE'].includes(method)) return { favorited: toggle(favorites, key, method) }
    if ((tail === '/comments' || tail === '/comments/threads') && method === 'GET') return page(demoComments(post.id), q)
    if (/^\/comments\/\d+\/replies$/.test(tail) && method === 'GET') return page(demoComments(post.id).slice(1).map(c => ({ ...c, id: c.id + 10000000, parentCommentId: post.id * 10 + 1, replyCount: 0, replyToUserId: demoUsers[1].id, replyTo: demoUsers[1], content: '这是用于检查评论展开的模拟回复。' })), q)
    if (/^\/comments\/\d+\/like$/.test(tail) && ['PUT','DELETE'].includes(method)) { const liked = toggle(likes, `${key}${tail}`, method); return { liked, likedCount: 2 + Number(liked) } }
  }
  match = pathname.match(/^\/trade\/users\/(\d+)\/products$/)
  if (match && method === 'GET') return page(demoProducts.filter(p => p.sellerId === Number(match![1])), q)
  if (['/trade/products','/trade/my-products'].includes(pathname) && method === 'GET') {
    let items = [...demoProducts]
    if (pathname === '/trade/my-products') items = items.filter(p => p.sellerId === DEMO_SELF)
    if (q.category) items = items.filter(p => p.category === q.category)
    if (q.keyword) items = items.filter(p => p.title.includes(String(q.keyword)))
    return page(items, q)
  }
  match = pathname.match(/^\/trade\/products\/(\d+)(.*)$/)
  if (match) {
    const product = demoProducts.find(p => p.id === Number(match![1])); if (!product) throw new DemoRequestError('演示商品不存在', 404)
    const tail = match[2]; const key = `product:${product.id}`
    if (!tail && method === 'GET') return product
    if (tail === '/viewer-state' && method === 'GET') return { favorited: favorites.has(key), contactedSeller: false }
    if (tail === '/favorite' && ['PUT','DELETE'].includes(method)) return { favorited: toggle(favorites, key, method) }
    if (tail === '/contact' && method === 'PUT') return demoConversation(product.sellerId, product.id)
    if (tail === '/comments' && method === 'GET') return page(demoComments(product.id).map(c => ({...c, productId: product.id, replies: [], isSeller: false})), q)
  }
  if (['/community/searches','/trade/searches'].includes(pathname) && method === 'POST') {
    const keyword = String(body.keyword || '').trim(); const filters = body.filters || {}
    if (keyword && !history.includes(keyword)) history.unshift(keyword)
    if (pathname.startsWith('/community')) {
      let posts = demoPosts.filter(p => (p.title + p.content).includes(keyword))
      if (filters.contentType === 'image') posts = posts.filter(p => p.images.length > 0)
      if (filters.contentType === 'text') posts = posts.filter(p => !p.images.length)
      if (filters.withinDays) posts = posts.filter(p => Date.now() - Date.parse(p.createdAt) < filters.withinDays * 86400000)
      if (filters.sort === 'likes') posts.sort((a,b)=>b.likedCount-a.likedCount)
      return page(posts, body)
    }
    let products = demoProducts.filter(p => p.title.includes(keyword))
    if (filters.category) products = products.filter(p => p.category === filters.category)
    if (filters.minPrice !== undefined) products = products.filter(p => p.price >= filters.minPrice)
    if (filters.maxPrice !== undefined) products = products.filter(p => p.price <= filters.maxPrice)
    if (filters.sort === 'price_asc') products.sort((a,b)=>a.price-b.price)
    if (filters.sort === 'price_desc') products.sort((a,b)=>b.price-a.price)
    return page(products, body)
  }
  if (/^\/(community|trade)\/search-history$/.test(pathname)) {
    if (method === 'GET') return [...history]
    if (method === 'DELETE') { history.length = 0; return {} }
  }
  if (/^\/(community|trade)\/hot-searches$/.test(pathname) && method === 'GET') return ['书','耳机','台灯'].map((keyword,i)=>({keyword,count:5-i,score:10-i}))
  if (pathname === '/community/hot-tags' && method === 'GET') return ['学习交流','日常记录','校园问答'].map(tag=>({tag,usageScore:8,interactionScore:3}))
  if (pathname === '/community/notifications' && method === 'GET') return { items: [], unread: 0 }
  if (pathname === '/community/me/comments' && method === 'GET') return page([],q)
  if (pathname === '/navigation/items' && method === 'GET') return []
  throw new DemoRequestError(method === 'GET' ? '模拟体验未提供该页面数据' : '模拟体验不执行真实发布、资料修改或交易操作', 403)
}
