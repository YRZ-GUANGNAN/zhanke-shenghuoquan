import type { CommunityComment, CommunityNotification, CommunityPost } from '../types'
import { request } from './api'
import { createClientRequestId } from './client-request-id'

export type CommunitySort = 'recent' | 'popular'

export interface PageResult<T> {
  items: T[]
  page: number
  pageSize: number
  total: number
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
  const id = asString(first(record, ['id', 'postId', 'post_id', 'postPublicId', 'post_public_id']), 'demo-post')
  const authorId = asString(first(record, ['authorId', 'author_id', 'userId', 'user_id'], first(authorRecord, ['id', 'userId', 'user_id'])), 'demo-author')
  const author = asString(first(record, ['authorName', 'author_name', 'author'], first(authorRecord, ['name', 'nickname'])), '奶茶脑袋')

  return {
    id,
    authorId,
    author,
    avatar: asString(first(record, ['avatar', 'avatarUrl', 'avatar_url'], first(authorRecord, ['avatar', 'avatarUrl', 'avatar_url']))),
    school: asString(first(record, ['school', 'schoolName', 'school_name'], first(authorRecord, ['school', 'schoolName', 'school_name'])), '湛江科技学院'),
    topic: asString(first(record, ['topic', 'tag']), '#湛科日常'),
    tags: normalizeStrings(first(record, ['tags', 'topics'], [])),
    title: asString(first(record, ['title', 'headline']), '三食堂新出的芋泥波波真的绝，一天一杯根本停不下来'),
    content: asString(first(record, ['content', 'body', 'text']), ''),
    images: normalizeImages(first(record, ['images', 'imageUrls', 'image_urls', 'media'])),
    likedCount: asNumber(first(record, ['likedCount', 'liked_count', 'likeCount', 'like_count']), 0),
    commentCount: asNumber(first(record, ['commentCount', 'comment_count']), 0),
    favoriteCount: asNumber(first(record, ['favoriteCount', 'favorite_count']), 0),
    createdAt: dateLabel(first(record, ['createdAt', 'created_at'])),
    pinned: asBoolean(first(record, ['pinned', 'isPinned', 'is_pinned'])),
  }
}

const normalizeComment = (raw: unknown): CommunityComment => {
  const record = isRecord(raw) ? raw : {}
  const authorRecord = isRecord(record.author) ? record.author : {}
  return {
    id: asString(first(record, ['id', 'commentId', 'comment_id']), `comment-${Date.now()}`),
    author: asString(first(record, ['authorName', 'author_name', 'author'], first(authorRecord, ['name', 'nickname'])), '同学'),
    avatar: asString(first(record, ['avatar', 'avatarUrl', 'avatar_url'], first(authorRecord, ['avatar', 'avatarUrl', 'avatar_url']))),
    content: asString(first(record, ['content', 'body', 'text']), ''),
    createdAt: dateLabel(first(record, ['createdAt', 'created_at'])),
  }
}

const normalizeNotification = (raw: unknown): CommunityNotification => {
  const record = isRecord(raw) ? raw : {}
  const actorRecord = isRecord(record.actor) ? record.actor : {}
  const rawType = asString(first(record, ['type', 'eventType', 'event_type']), 'post_commented')
  const type: CommunityNotification['type'] =
    rawType === 'post_liked' || rawType === 'user_followed' ? rawType : 'post_commented'

  return {
    id: asString(first(record, ['id', 'notificationId', 'notification_id']), `notification-${Date.now()}`),
    type,
    actor: asString(first(record, ['actorName', 'actor_name', 'actor'], first(actorRecord, ['name', 'nickname'])), '同学'),
    postId: asString(first(record, ['postId', 'post_id', 'postPublicId', 'post_public_id'])),
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
  })
  return unwrapPost(data)
}

export async function searchCommunityPosts(keyword: string, page = 1, pageSize = 20): Promise<PageResult<CommunityPost>> {
  const data = await request<unknown>('/community/searches', {
    url: '', method: 'POST', data: { keyword, page, pageSize }, timeout: 10000,
  })
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

export async function addPostComment(postId: string, content: string): Promise<CommunityComment> {
  const data = await request<unknown>(`/community/posts/${encodeURIComponent(postId)}/comments`, {
    url: '',
    method: 'POST',
    data: { content },
  })
  if (isRecord(data)) return normalizeComment(first(data, ['comment', 'item'], data))
  return normalizeComment(data)
}

export async function getCommunityViewerState(postId: string): Promise<CommunityViewerState> {
  const data = await request<unknown>(`/community/posts/${encodeURIComponent(postId)}/viewer-state`)
  const record = isRecord(data) ? data : {}
  return {
    liked: asBoolean(first(record, ['liked', 'isLiked', 'is_liked'])),
    favorited: asBoolean(first(record, ['favorited', 'favorite', 'isFavorited', 'is_favorited'])),
    followingAuthor: asBoolean(first(record, ['followingAuthor', 'following_author', 'isFollowingAuthor'])),
  }
}

export async function setPostLiked(postId: string, liked: boolean): Promise<void> {
  await request(`/community/posts/${encodeURIComponent(postId)}/likes`, {
    url: '',
    method: liked ? 'POST' : 'DELETE',
  })
}

export async function setPostFavorited(postId: string, favorited: boolean): Promise<void> {
  await request(`/community/posts/${encodeURIComponent(postId)}/favorites`, {
    url: '',
    method: favorited ? 'POST' : 'DELETE',
  })
}

export async function setUserFollowing(userId: string, following: boolean): Promise<void> {
  await request(`/community/users/${encodeURIComponent(userId)}/follow`, {
    url: '',
    method: following ? 'POST' : 'DELETE',
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
