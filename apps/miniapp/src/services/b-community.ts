import type { CommunityComment, CommunityNotification, CommunityPost, CommunityThreadComment } from '../types'
import { request } from './api'
import { createClientRequestId } from './client-request-id'

export type CommunitySort = 'recent' | 'popular'

export interface PageResult<T> {
  items: T[]
  page: number
  pageSize: number
  total: number
  publicCount?: number
}

export interface CreateCommunityPostInput {
  title: string
  content: string
  topic?: string
  tags?: string[]
  imageAssetIds?: string[]
  clientRequestId?: string
}

export interface CommunityViewerState {
  liked: boolean
  likedCount: number
  favorited: boolean
  followingAuthor: boolean
}

export interface CommunityNotificationResult {
  items: CommunityNotification[]
  unread: number
}

export interface HotSearchKeyword {
  keyword: string
  count: number
  score: number
}

export interface HotCommunityTag {
  tag: string
  usageScore: number
  interactionScore: number
}

export interface MyCommunitySummary {
	favorites: number
	posts: number
	comments: number
}

export interface MyCommunityComment {
	id: string
	postId: string
	postTitle: string
	content: string
	reviewStatus: string
	createdAt: string
}

export interface PublicCommunityProfile {
	profile: {
		id: number
		nickname: string
		avatar: string
		school: string
		campus: string
	}
	followingCount: number
	followerCount: number
	likedCount: number
	isFollowing: boolean
}

type UnknownRecord = Record<string, unknown>

const isRecord = (value: unknown): value is UnknownRecord =>
  Boolean(value) && typeof value === 'object' && !Array.isArray(value)

const asString = (value: unknown, fallback = ''): string => {
  if (typeof value === 'string') return value
  if (typeof value === 'number') return String(value)
  return fallback
}

const asNumber = (value: unknown, fallback = 0): number => {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string') {
    const parsed = Number(value)
    return Number.isFinite(parsed) ? parsed : fallback
  }
  return fallback
}

const asBoolean = (value: unknown): boolean => value === true || value === 'true' || value === 1 || value === '1'

const first = (record: UnknownRecord, keys: string[], fallback: unknown = ''): unknown => {
  for (const key of keys) {
    if (record[key] !== undefined && record[key] !== null) return record[key]
  }
  return fallback
}

const dateLabel = (value: unknown): string => {
  const raw = asString(value)
  if (!raw) return ''
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return raw

  const now = new Date()
  if (date.toDateString() === now.toDateString()) {
    return `今天 ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
  }

  return `${date.getMonth() + 1}月${date.getDate()}日`
}

const normalizeImages = (value: unknown): string[] => {
  if (!Array.isArray(value)) return []
  return value
    .map((item) => {
      if (typeof item === 'string') return item
      if (!isRecord(item)) return ''
      return asString(first(item, ['url', 'displayUrl', 'display_url', 'assetUrl', 'asset_url', 'assetPublicId', 'asset_public_id']))
    })
    .filter(Boolean)
}

const normalizeStrings = (value: unknown): string[] => {
  if (!Array.isArray(value)) return []
  return value.map((item) => asString(item).trim()).filter(Boolean)
}

const normalizePost = (raw: unknown): CommunityPost => {
  const record = isRecord(raw) ? raw : {}
  const authorRecord = isRecord(record.author) ? record.author : {}
  const id = asString(first(record, ['id', 'postId', 'post_id', 'postPublicId', 'post_public_id']))
  const authorId = asString(first(record, ['authorId', 'author_id', 'userId', 'user_id'], first(authorRecord, ['id', 'userId', 'user_id'])))
  const author = asString(first(record, ['authorName', 'author_name', 'author'], first(authorRecord, ['name', 'nickname'])), '校园用户')

  return {
    id,
    authorId,
    author,
    avatar: asString(first(record, ['avatar', 'avatarUrl', 'avatar_url'], first(authorRecord, ['avatar', 'avatarUrl', 'avatar_url']))),
    school: asString(first(record, ['school', 'schoolName', 'school_name'], first(authorRecord, ['school', 'schoolName', 'school_name']))),
    topic: asString(first(record, ['topic', 'tag'])),
    tags: normalizeStrings(first(record, ['tags', 'topics'], [])),
    title: asString(first(record, ['title', 'headline'])),
    content: asString(first(record, ['content', 'body', 'text']), ''),
    images: normalizeImages(first(record, ['images', 'imageUrls', 'image_urls', 'media'])),
    likedCount: asNumber(first(record, ['likedCount', 'liked_count', 'likeCount', 'like_count']), 0),
    commentCount: asNumber(first(record, ['commentCount', 'comment_count']), 0),
    favoriteCount: asNumber(first(record, ['favoriteCount', 'favorite_count']), 0),
		reviewStatus: asString(first(record, ['reviewStatus', 'review_status']), 'submitted') as CommunityPost['reviewStatus'],
    createdAt: dateLabel(first(record, ['createdAt', 'created_at'])),
    pinned: asBoolean(first(record, ['pinned', 'isPinned', 'is_pinned'])),
  }
}

const normalizeComment = (raw: unknown): CommunityComment => {
  const record = isRecord(raw) ? raw : {}
  const authorRecord = isRecord(record.author) ? record.author : {}
  return {
    id: asString(first(record, ['id', 'commentId', 'comment_id'])),
    author: asString(first(record, ['authorName', 'author_name'], first(authorRecord, ['name', 'nickname'], typeof record.author === 'string' ? record.author : '同学')), '同学'),
    avatar: asString(first(record, ['avatar', 'avatarUrl', 'avatar_url'], first(authorRecord, ['avatar', 'avatarUrl', 'avatar_url']))),
    content: asString(first(record, ['content', 'body', 'text']), ''),
    createdAt: dateLabel(first(record, ['createdAt', 'created_at'])),
  }
}

const normalizeThreadComment = (raw: unknown): CommunityThreadComment => {
  const record = isRecord(raw) ? raw : {}
  const replyTo = isRecord(record.replyTo) ? record.replyTo : {}
  const status = asString(record.reviewStatus)
  return {
    ...normalizeComment(raw),
    authorId: asString(record.userId),
    parentCommentId: asString(record.parentCommentId, '0'),
    replyToCommentId: asString(record.replyToCommentId, '0'),
    replyToUserId: asString(record.replyToUserId, '0'),
    replyToAuthor: asString(replyTo.nickname, '同学'),
    reviewStatus: ['approved', 'reviewing', 'rejected', 'taken_down'].includes(status) ? status as CommunityThreadComment['reviewStatus'] : 'submitted',
    liked: typeof record.liked === 'boolean' ? record.liked : undefined,
    likedCount: Math.max(0, asNumber(record.likedCount)),
    replyCount: Math.max(0, asNumber(record.replyCount)),
  }
}

export interface LikeState { liked: boolean; likedCount: number }

const normalizeLikeState = (raw: unknown): LikeState => {
  if (!isRecord(raw) || typeof raw.liked !== 'boolean' || typeof raw.likedCount !== 'number' || !Number.isSafeInteger(raw.likedCount) || raw.likedCount < 0) {
    throw new Error('点赞状态暂未确认，请重新读取')
  }
  return { liked: raw.liked, likedCount: raw.likedCount }
}

export async function getCommentThreads(postId: string, page = 1): Promise<PageResult<CommunityThreadComment>> {
  return normalizePage(await request(`/community/posts/${encodeURIComponent(postId)}/comments/threads?page=${page}&pageSize=20`), normalizeThreadComment)
}

export async function getCommentReplies(postId: string, rootId: string, page = 1): Promise<PageResult<CommunityThreadComment>> {
  return normalizePage(await request(`/community/posts/${encodeURIComponent(postId)}/comments/${encodeURIComponent(rootId)}/replies?page=${page}&pageSize=20`), normalizeThreadComment)
}

export async function submitThreadComment(postId: string, content: string, clientRequestId: string, targetId?: string): Promise<CommunityThreadComment> {
  const suffix = targetId ? `/${encodeURIComponent(targetId)}/replies` : ''
  return normalizeThreadComment(await request(`/community/posts/${encodeURIComponent(postId)}/comments${suffix}`, {
    url: '', method: 'POST', data: { content, clientRequestId }, header: { 'Idempotency-Key': clientRequestId },
  }))
}

export async function setCommentLiked(postId: string, commentId: string, liked: boolean): Promise<LikeState> {
  return normalizeLikeState(await request(`/community/posts/${encodeURIComponent(postId)}/comments/${encodeURIComponent(commentId)}/like`, {
    url: '', method: liked ? 'PUT' : 'DELETE',
  }))
}

const normalizeNotification = (raw: unknown): CommunityNotification => {
  const record = isRecord(raw) ? raw : {}
  const actorRecord = isRecord(record.actor) ? record.actor : {}
  const rawType = asString(first(record, ['type', 'eventType', 'event_type']), 'post_commented')
  const type: CommunityNotification['type'] =
    rawType === 'post_liked'
    || rawType === 'user_followed'
    || rawType === 'product_commented'
    || rawType === 'product_comment_replied'
      ? rawType
      : 'post_commented'

  return {
    id: asString(first(record, ['id', 'notificationId', 'notification_id'])),
    type,
    actor: asString(first(record, ['actorName', 'actor_name', 'actor'], first(actorRecord, ['name', 'nickname'])), '同学'),
    postId: asString(first(record, ['postId', 'post_id', 'postPublicId', 'post_public_id'])),
    productId: asString(first(record, ['productId', 'product_id'])) || undefined,
    productCommentId: asString(first(record, ['productCommentId', 'product_comment_id'])) || undefined,
    read: asBoolean(first(record, ['read', 'isRead', 'is_read'])),
    createdAt: dateLabel(first(record, ['createdAt', 'created_at'])),
  }
}

const normalizePage = <T>(raw: unknown, mapper: (item: unknown) => T): PageResult<T> => {
  const record = isRecord(raw) ? raw : {}
  const list = first(record, ['items', 'list', 'records', 'data'], Array.isArray(raw) ? raw : [])
  const items = Array.isArray(list) ? list.map(mapper) : []
  return {
    items,
    page: asNumber(first(record, ['page', 'pageNo', 'page_no']), 1),
    pageSize: asNumber(first(record, ['pageSize', 'page_size', 'limit']), items.length || 20),
    total: asNumber(first(record, ['total', 'count']), items.length),
    ...(typeof record.publicCount === 'number' ? { publicCount: Math.max(0, record.publicCount) } : {}),
  }
}

const unwrapPost = (raw: unknown): CommunityPost => {
  if (isRecord(raw)) {
    return normalizePost(first(raw, ['post', 'item', 'detail'], raw))
  }
  return normalizePost(raw)
}

export function createCommunityClientRequestId(): string {
  return createClientRequestId()
}

export async function getCommunityPostsPage(params: { page?: number; pageSize?: number; sort?: CommunitySort } = {}): Promise<PageResult<CommunityPost>> {
  const query = new URLSearchParams()
  if (params.page) query.set('page', String(params.page))
  if (params.pageSize) query.set('pageSize', String(params.pageSize))
  if (params.sort) query.set('sort', params.sort)
  const suffix = query.toString() ? `?${query.toString()}` : ''
  const data = await request<unknown>(`/community/posts${suffix}`)
  return normalizePage(data, normalizePost)
}

export async function listCommunityPosts(sort: CommunitySort = 'recent'): Promise<CommunityPost[]> {
  const page = await getCommunityPostsPage({ page: 1, pageSize: 20, sort })
  return page.items
}

export async function listFollowingPosts(): Promise<CommunityPost[]> {
  const data = await request<unknown>('/community/following-feed?page=1&pageSize=20')
  return normalizePage(data, normalizePost).items
}

export async function getCommunityPost(postId: string): Promise<CommunityPost> {
  const data = await request<unknown>(`/community/posts/${encodeURIComponent(postId)}`)
  return unwrapPost(data)
}

export async function getMyCommunityPost(postId: string): Promise<CommunityPost> {
  const data = await request<unknown>(`/community/me/posts/${encodeURIComponent(postId)}`)
  return unwrapPost(data)
}

export async function createCommunityPost(input: CreateCommunityPostInput): Promise<CommunityPost> {
  const clientRequestId = input.clientRequestId ?? createCommunityClientRequestId()
  const data = await request<unknown>('/community/posts', {
    url: '',
    method: 'POST',
    data: {
      title: input.title,
      content: input.content,
      topic: input.topic || '湛科日常',
      tags: input.tags ?? [],
      imageAssetIds: input.imageAssetIds ?? [],
      clientRequestId,
    },
    header: { 'Idempotency-Key': clientRequestId },
    timeout: 12000,
  })
  return unwrapPost(data)
}

export function getMyCommunitySummary(): Promise<MyCommunitySummary> {
	return request('/community/me/summary')
}

export async function getMyFavoritePosts(page = 1, pageSize = 20): Promise<PageResult<CommunityPost>> {
	const data = await request<unknown>(`/community/me/favorites?page=${page}&pageSize=${pageSize}`)
	return normalizePage(data, normalizePost)
}

export async function getMyPosts(page = 1, pageSize = 20): Promise<PageResult<CommunityPost>> {
	const data = await request<unknown>(`/community/me/posts?page=${page}&pageSize=${pageSize}`)
	return normalizePage(data, normalizePost)
}

export async function getMyComments(page = 1, pageSize = 20): Promise<PageResult<MyCommunityComment>> {
	const data = await request<unknown>(`/community/me/comments?page=${page}&pageSize=${pageSize}`)
	return normalizePage(data, (raw) => {
		const record = isRecord(raw) ? raw : {}
		return {
			id: asString(record.id),
			postId: asString(first(record, ['postId', 'post_id'])),
			postTitle: asString(first(record, ['postTitle', 'post_title']), '原帖子'),
			content: asString(record.content),
			reviewStatus: asString(first(record, ['reviewStatus', 'review_status'])),
			createdAt: dateLabel(first(record, ['createdAt', 'created_at']))
		}
	})
}

export function getPublicCommunityProfile(userId: string): Promise<PublicCommunityProfile> {
	return request(`/community/users/${encodeURIComponent(userId)}/profile`)
}

export async function getPublicUserPosts(userId: string, page = 1, pageSize = 20): Promise<PageResult<CommunityPost>> {
	const data = await request<unknown>(`/community/users/${encodeURIComponent(userId)}/posts?page=${page}&pageSize=${pageSize}`)
	return normalizePage(data, normalizePost)
}

export interface CommunitySearchFilters {
  sort?: 'relevance' | 'recent' | 'likes' | 'comments' | 'favorites'
  contentType?: 'all' | 'image' | 'text'
  withinDays?: 0 | 1 | 7 | 180
}

export async function searchCommunityPosts(keyword: string, page = 1, pageSize = 20, filters: CommunitySearchFilters = {}): Promise<PageResult<CommunityPost>> {
  const filtered = Boolean((filters.sort && filters.sort !== 'relevance') || (filters.contentType && filters.contentType !== 'all') || filters.withinDays)
  const data = await request<unknown>('/community/searches', {
    url: '', method: 'POST', data: { keyword, page, pageSize, ...(filtered ? { filters } : {}) }, timeout: 10000,
  })
  if (filtered && (!isRecord(data) || data.filterVersion !== 1)) throw new Error('当前服务暂不支持筛选，请清除筛选后重试')
  return normalizePage(data, normalizePost)
}

export function listCommunitySearchHistory(): Promise<string[]> {
  return request<string[]>('/community/search-history')
}

export async function clearCommunitySearchHistory(): Promise<void> {
  await request('/community/search-history', { url: '', method: 'DELETE' })
}

export function listCommunityHotSearches(): Promise<HotSearchKeyword[]> {
  return request<HotSearchKeyword[]>('/community/hot-searches')
}

export function listHotCommunityTags(): Promise<HotCommunityTag[]> {
  return request<HotCommunityTag[]>('/community/hot-tags')
}

export async function getPostCommentsPage(postId: string, params: { page?: number; pageSize?: number } = {}): Promise<PageResult<CommunityComment>> {
  const query = new URLSearchParams()
  query.set('page', String(params.page ?? 1))
  query.set('pageSize', String(params.pageSize ?? 20))
  const data = await request<unknown>(`/community/posts/${encodeURIComponent(postId)}/comments?${query.toString()}`)
  return normalizePage(data, normalizeComment)
}

export async function listPostComments(postId: string): Promise<CommunityComment[]> {
  const page = await getPostCommentsPage(postId, { page: 1, pageSize: 20 })
  return page.items
}

export async function addPostComment(
  postId: string,
  content: string,
  clientRequestId = createCommunityClientRequestId()
): Promise<CommunityComment> {
  const data = await request<unknown>(`/community/posts/${encodeURIComponent(postId)}/comments`, {
    url: '',
    method: 'POST',
    data: { content, clientRequestId },
    header: { 'Idempotency-Key': clientRequestId },
  })
  if (isRecord(data)) return normalizeComment(first(data, ['comment', 'item'], data))
  return normalizeComment(data)
}

export async function getCommunityViewerState(postId: string): Promise<CommunityViewerState> {
  const data = await request<unknown>(`/community/posts/${encodeURIComponent(postId)}/viewer-state`)
  const record = isRecord(data) ? data : {}
  return {
    ...normalizeLikeState(record),
    favorited: asBoolean(first(record, ['favorited', 'favorite', 'isFavorited', 'is_favorited'])),
    followingAuthor: asBoolean(first(record, ['followingAuthor', 'following_author', 'isFollowingAuthor'])),
  }
}

export async function setPostLiked(postId: string, liked: boolean): Promise<LikeState> {
  return normalizeLikeState(await request(`/community/posts/${encodeURIComponent(postId)}/like`, {
    url: '',
    method: liked ? 'PUT' : 'DELETE',
  }))
}

export async function setPostFavorited(postId: string, favorited: boolean): Promise<void> {
  await request(`/community/posts/${encodeURIComponent(postId)}/favorite`, {
    url: '',
    method: favorited ? 'PUT' : 'DELETE',
  })
}

export async function setUserFollowing(userId: string, following: boolean): Promise<void> {
  await request(`/community/users/${encodeURIComponent(userId)}/follow`, {
    url: '',
    method: following ? 'PUT' : 'DELETE',
  })
}

export async function togglePostLike(postId: string, liked: boolean): Promise<void> {
  await setPostLiked(postId, liked)
}

export async function togglePostFavorite(postId: string, favorited: boolean): Promise<void> {
  await setPostFavorited(postId, favorited)
}

export async function toggleUserFollow(userId: string, following: boolean): Promise<void> {
  await setUserFollowing(userId, following)
}

export async function listCommunityNotifications(): Promise<CommunityNotificationResult> {
  const data = await request<unknown>('/community/notifications?page=1&pageSize=20')
  const page = normalizePage(data, normalizeNotification)
  const record = isRecord(data) ? data : {}
  return {
    items: page.items,
    unread: asNumber(first(record, ['unread', 'unreadCount', 'unread_count']), page.items.filter((item) => !item.read).length),
  }
}

export async function markCommunityNotificationRead(notificationId: string): Promise<void> {
  await request(`/community/notifications/${encodeURIComponent(notificationId)}/read`, {
    url: '',
    method: 'POST',
  })
}
