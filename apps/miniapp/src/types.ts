export type TabKey = 'home' | 'takeout' | 'market' | 'tools' | 'profile'

export type IdentityRoleKey = 'student' | 'rider' | 'merchant'

export interface IdentityAvailability {
  identity: IdentityRoleKey
  available: boolean
  reason?: string
  status?: string
}

export interface IdentitySummary {
  currentIdentity: IdentityRoleKey
  identities: IdentityAvailability[]
}

export type IdentityApplicationRole = 'rider' | 'merchant'

export type IdentityApplicationStatus = 'pending' | 'approved' | 'rejected'

export interface IdentityApplicationView {
  applicationId: string
  userId: string
  role: IdentityApplicationRole
  status: IdentityApplicationStatus
  realName: string
  phone: string
  merchantName: string
  merchantAddress: string
  assetIds: string[]
  reviewReason: string
  reviewerUserId?: string
  submittedAt: string
  reviewedAt?: string
  rowVersion: number
}

export interface IdentityApplicationSubmitInput {
  realName: string
  phone: string
  merchantName: string
  merchantAddress: string
  assetIds: string[]
  expectedRowVersion: number
}

// A: UserProfile; B: CommunityPost; C: Product; D: ToolLink.
// E uses SDK-specific types in services/e-im.ts; F exists only in apps/admin.

export interface UserProfile {
  id: string
  nickname: string
  avatar: string
  cover?: string
  school: string
  campus: string
  role?: 'user' | 'admin' | 'superadmin'
}

export interface CommunityPost {
  id: string
  authorId: string
  author: string
  avatar: string
  school: string
  topic: string
  tags?: string[]
  title: string
  content: string
  images: string[]
  likedCount: number
  commentCount: number
  favoriteCount: number
	reviewStatus?: 'submitted' | 'reviewing' | 'approved' | 'rejected' | 'taken_down'
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

export interface CommunityThreadComment extends CommunityComment {
  authorId: string
  parentCommentId: string
  replyToCommentId: string
  replyToUserId: string
  replyToAuthor: string
  reviewStatus: NonNullable<CommunityPost['reviewStatus']>
  liked?: boolean
  likedCount: number
  replyCount: number
}

export interface CommunityNotification {
  id: string
  type:
    | 'post_commented'
    | 'post_liked'
    | 'user_followed'
    | 'product_commented'
    | 'product_comment_replied'
  actor: string
  postId: string
  productId?: string
  productCommentId?: string
  read: boolean
  createdAt: string
}

export type ProductCommentReviewStatus =
  | 'submitted'
  | 'reviewing'
  | 'approved'
  | 'rejected'

export interface ProductComment {
  id: string
  productId: string
  parentCommentId: string
  replyToUserId: string
  content: string
  reviewStatus: ProductCommentReviewStatus
  author: UserProfile
  replyTo?: UserProfile
  isSeller: boolean
  replies: ProductComment[]
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
  categoryId?: string
  categoryName?: string
  id: string
  title: string
  subtitle: string
  iconText: string
  appId?: string
  path?: string
  color: string
}
