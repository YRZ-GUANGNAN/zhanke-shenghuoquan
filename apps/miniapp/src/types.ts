export type TabKey = 'home' | 'food' | 'publish' | 'market' | 'profile'

export interface UserProfile {
  id: string
  nickname: string
  avatar: string
  school: string
  campus: string
}

export interface CommunityPost {
  id: string
  authorId: string
  author: string
  avatar: string
  school: string
  topic: string
  title: string
  content: string
  images: string[]
  likedCount: number
  commentCount: number
  favoriteCount: number
  createdAt: string
  pinned?: boolean
}

export interface CommunityComment {
  id: string
  author: string
  avatar: string
  content: string
  createdAt: string
}

export interface CommunityNotification {
  id: string
  type: 'post_commented' | 'post_liked' | 'user_followed'
  actor: string
  postId: string
  read: boolean
  createdAt: string
}

export interface Product {
  id: string
  sellerId: string
  title: string
  description: string
  price: number
  originalPrice?: number
  category: string
  images: string[]
  sellerName: string
  sellerAvatar: string
  campus: string
  condition: string
  createdAt: string
  status: 'selling' | 'reserved' | 'sold'
  reviewStatus: 'submitted' | 'reviewing' | 'approved' | 'rejected'
  favoriteCount: number
}

export interface ToolLink {
  id: string
  title: string
  subtitle: string
  iconText: string
  appId?: string
  path?: string
  color: string
}
