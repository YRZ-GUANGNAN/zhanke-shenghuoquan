// 功能建议墙与作者的话：列表、提交与支持切换。
import { request } from './api'

export type FeatureSuggestionStatus = 'open' | 'accepted' | 'closed'

export interface FeatureSuggestion {
  id: string
  content: string
  status: FeatureSuggestionStatus
  supportCount: number
  supported: boolean
  commentCount: number
  createdAt: string
}

export interface FeatureComment {
  id: string
  content: string
  isMaintainer: boolean
  createdAt: string
}

export interface FeatureCommentPage {
  items: FeatureComment[]
  page: number
  pageSize: number
  total: number
}

export interface FeatureSuggestionPage {
  items: FeatureSuggestion[]
  page: number
  pageSize: number
  total: number
}

export interface AuthorNote {
  id: string
  content: string
  createdAt: string
}

export async function listFeatureSuggestions(
  sort: 'supports' | 'latest',
  page = 1,
  pageSize = 10
): Promise<FeatureSuggestionPage> {
  return request<FeatureSuggestionPage>(`/auth/feature-suggestions?sort=${sort}&page=${page}&pageSize=${pageSize}`, {
    url: '',
    timeout: 12000
  })
}

export async function submitFeatureSuggestion(content: string): Promise<FeatureSuggestion> {
  return request<FeatureSuggestion>('/auth/feature-suggestions', {
    url: '',
    method: 'POST',
    data: { content },
    timeout: 15000
  })
}

export async function setFeatureSuggestionSupport(
  suggestionId: string,
  supported: boolean
): Promise<{ supported: boolean; supportCount: number }> {
  return request<{ supported: boolean; supportCount: number }>(
    `/auth/feature-suggestions/${encodeURIComponent(suggestionId)}/support`,
    { url: '', method: 'POST', data: { supported }, timeout: 12000 }
  )
}

export async function listAuthorNotes(): Promise<AuthorNote[]> {
  return request<AuthorNote[]>('/auth/author-notes', { url: '', timeout: 12000 })
}

export async function listFeatureComments(
  suggestionId: string,
  page = 1,
  pageSize = 50
): Promise<FeatureCommentPage> {
  return request<FeatureCommentPage>(
    `/auth/feature-suggestions/${encodeURIComponent(suggestionId)}/comments?page=${page}&pageSize=${pageSize}`,
    { url: '', timeout: 12000 }
  )
}

export async function submitFeatureComment(suggestionId: string, content: string): Promise<FeatureComment> {
  return request<FeatureComment>(
    `/auth/feature-suggestions/${encodeURIComponent(suggestionId)}/comments`,
    { url: '', method: 'POST', data: { content }, timeout: 15000 }
  )
}
