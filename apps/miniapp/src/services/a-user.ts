// Domain A client: WeChat login, profile and runtime identity.
import type { IdentityRoleKey, IdentitySummary, UserProfile } from '../types'
import { request } from './api'

interface BackendUser {
  id: number
  nickname: string
  avatar: string
  school: string
  campus: string
}

interface LoginResult {
  token: string
  user: BackendUser
}

export interface LoginSession {
  token: string
  profile: UserProfile
}

export interface UserSettingsView {
	orderNotice: boolean
	socialNotice: boolean
	quietHours: boolean
	personalized: boolean
	phoneBound: boolean
	maskedPhone: string
	blacklistCount: number
}

export interface BlockedUserView {
	userId: number
	nickname: string
	avatar: string
	createdAt: string
}

export type FeedbackCategory = 'bug' | 'suggestion' | 'report' | 'other'

export interface FeedbackView {
	id: string
	category: FeedbackCategory
	content: string
	status: string
	createdAt: string
}

export interface AccountClosureBlocker {
	code: string
	message: string
}

export const emptyProfile: UserProfile = {
  id: '',
  nickname: '未登录',
  avatar: '',
  school: '',
  campus: ''
}

export async function wechatLogin(code: string, loginAvatarPreview = ''): Promise<LoginSession> {
  const result = await request<LoginResult>('/auth/wechat-login', {
    url: '',
    method: 'POST',
    data: { code }
  })
  const profile = toProfile(result.user)
  return {
    token: result.token,
    profile: profile.avatar ? profile : { ...profile, avatar: loginAvatarPreview }
  }
}

export async function currentUser(): Promise<UserProfile> {
  return toProfile(await request<BackendUser>('/auth/me'))
}

export async function updateProfile(profile: UserProfile, avatarAssetId?: string): Promise<UserProfile> {
  return toProfile(await request<BackendUser>('/auth/me', {
    url: '',
    method: 'PATCH',
    data: {
      nickname: profile.nickname,
      school: profile.school,
      campus: profile.campus,
      avatarAssetId
    }
  }))
}

export async function currentIdentity(): Promise<IdentitySummary> {
  return request('/auth/identity')
}

export async function switchIdentity(identity: IdentityRoleKey): Promise<IdentitySummary> {
  return request('/auth/identity', { url: '', method: 'POST', data: { identity } })
}

export async function getUserSettings(): Promise<UserSettingsView> {
	return request('/auth/settings')
}

export async function updateUserSettings(value: Pick<UserSettingsView, 'orderNotice' | 'socialNotice' | 'quietHours' | 'personalized'>): Promise<UserSettingsView> {
	return request('/auth/settings', { url: '', method: 'PATCH', data: value })
}

export async function bindWechatPhone(code: string): Promise<UserSettingsView> {
	return request('/auth/me/phone', { url: '', method: 'POST', data: { code } })
}

export async function listBlockedUsers(): Promise<BlockedUserView[]> {
	const value = await request<{ items: BlockedUserView[] }>('/auth/blocks')
	return value.items
}

export async function removeBlockedUser(userId: number): Promise<void> {
	await request(`/auth/blocks/${userId}`, { url: '', method: 'DELETE' })
}

export async function blockUser(userId: number): Promise<void> {
	await request(`/auth/blocks/${userId}`, { url: '', method: 'POST' })
}

export async function submitFeedback(category: FeedbackCategory, content: string, imageAssetIds: string[]): Promise<FeedbackView> {
	return request('/auth/feedbacks', { url: '', method: 'POST', data: { category, content, imageAssetIds }, timeout: 15000 })
}

export async function closeAccount(): Promise<void> {
	await request('/auth/me', { url: '', method: 'DELETE', timeout: 15000 })
}

function toProfile(value: BackendUser): UserProfile {
  return {
    ...emptyProfile,
    id: String(value.id),
    nickname: value.nickname,
    avatar: value.avatar,
    school: value.school,
    campus: value.campus
  }
}
