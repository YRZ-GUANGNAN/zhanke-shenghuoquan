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
