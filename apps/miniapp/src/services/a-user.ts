// Domain A client: WeChat login, token and profile only.
import type { IdentityRoleKey, IdentitySummary, UserProfile } from '../types'
import Taro from './taro'
import { request } from './api'

interface BackendUser {
  id: number
  nickname: string
  avatar: string
  school: string
  campus: string
  currentIdentity?: IdentityRoleKey
}
interface LoginResult { token: string; user: BackendUser }

export const emptyProfile: UserProfile = { id: '', nickname: '未登录', avatar: '', school: '', campus: '' }

export async function wechatLogin(code: string, loginAvatarPreview = ''): Promise<UserProfile> {
	const result = await request<LoginResult>('/auth/wechat-login', { url: '', method: 'POST', data: { code } })
	Taro.setStorageSync('domainA.token', result.token)
	if (loginAvatarPreview) Taro.setStorageSync('domainA.loginAvatarPreview', loginAvatarPreview)
	const profile = toProfile(result.user)
	return profile.avatar ? profile : { ...profile, avatar: loginAvatarPreview }
}
export async function currentUser(): Promise<UserProfile> { return toProfile(await request<BackendUser>('/auth/me')) }
export async function updateProfile(profile:UserProfile,avatarAssetId?:string):Promise<UserProfile>{return toProfile(await request<BackendUser>('/auth/me',{url:'',method:'PATCH',data:{nickname:profile.nickname,school:profile.school,campus:profile.campus,avatarAssetId}}))}
export async function currentIdentity(): Promise<IdentitySummary> { return request<IdentitySummary>('/auth/identity') }
export async function switchIdentity(identity: IdentityRoleKey): Promise<IdentitySummary> { return request<IdentitySummary>('/auth/identity', { url: '', method: 'POST', data: { identity } }) }
function toProfile(value: BackendUser): UserProfile { return { ...emptyProfile, id: String(value.id), nickname: value.nickname, avatar: value.avatar, school: value.school, campus: value.campus } }
