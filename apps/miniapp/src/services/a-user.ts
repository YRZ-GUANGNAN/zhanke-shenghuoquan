// Domain A client: WeChat login, profile and runtime identity.
import type {
  IdentityApplicationRole,
  IdentityApplicationSubmitInput,
  IdentityApplicationView,
  IdentityRoleKey,
  IdentitySummary,
  UserProfile
} from '../types'
import { request } from './api'

interface BackendUser {
  id: number
  nickname: string
  avatar: string
  cover?: string
  school: string
  campus: string
  role?: 'user' | 'admin' | 'superadmin'
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
	wechatBound: boolean
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
  cover: '',
  school: '',
  campus: ''
}

export async function wechatLogin(code: string, loginAvatarPreview = ''): Promise<LoginSession> {
  const result = await request<LoginResult>('/auth/wechat-login', {
    url: '',
    method: 'POST',
    data: { code },
    timeout: 12000
  })
  const profile = toProfile(result.user)
  return {
    token: result.token,
    profile: profile.avatar ? profile : { ...profile, avatar: loginAvatarPreview }
  }
}

export async function currentUser(): Promise<UserProfile> {
  return toProfile(await request<BackendUser>('/auth/me', { url: '', timeout: 12000 }))
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
  return request('/auth/identity', { url: '', timeout: 12000 })
}

// Only the cover changes; never resend editable nickname/avatar form fields.
export async function updateProfileCover(assetId: string): Promise<UserProfile> {
  return toProfile(await request<BackendUser>('/auth/me/cover', {
    url: '', method: 'PUT', data: { assetId }, timeout: 20000
  }))
}

export async function switchIdentity(identity: IdentityRoleKey): Promise<IdentitySummary> {
  return request('/auth/identity', { url: '', method: 'POST', data: { identity } })
}

export async function getIdentityApplication(role: IdentityApplicationRole): Promise<IdentityApplicationView> {
  return request(`/auth/identity-applications/${role}`)
}

export async function submitIdentityApplication(
  role: IdentityApplicationRole,
  input: IdentityApplicationSubmitInput
): Promise<IdentityApplicationView> {
  return request(`/auth/identity-applications/${role}`, {
    url: '',
    method: 'PUT',
    data: input,
    timeout: 20000
  })
}

export async function getUserSettings(): Promise<UserSettingsView> {
	return normalizeUserSettings(await request('/auth/settings') as UserSettingsView)
}

export async function updateUserSettings(value: Pick<UserSettingsView, 'orderNotice' | 'socialNotice' | 'quietHours' | 'personalized'>): Promise<UserSettingsView> {
	return normalizeUserSettings(await request<UserSettingsView>('/auth/settings', { url: '', method: 'PATCH', data: value }))
}

export async function bindWechatPhone(code: string): Promise<UserSettingsView> {
	return normalizeUserSettings(await request<UserSettingsView>('/auth/me/phone', { url: '', method: 'POST', data: { code } }))
}

function normalizeUserSettings(value: UserSettingsView): UserSettingsView {
	// Older servers do not return the derived binding flag. A valid authenticated
	// session is still a WeChat-backed account, so keep the UI truthful while the
	// server rolls forward.
	return { ...value, wechatBound: value.wechatBound !== false }
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
    cover: value.cover || '',
    school: value.school,
    campus: value.campus,
    role: value.role
  }
}
