import { create } from 'zustand'
import { emptyProfile } from '../services/a-user'
import Taro from '../services/taro'
import type { UserProfile } from '../types'

interface SessionState {
  token: string
  profile: UserProfile
  setToken: (token: string) => void
  updateProfile: (profile: Partial<UserProfile>) => void
}

export const useSessionStore = create<SessionState>((set) => ({
  token: '',
	profile: emptyProfile,
	setToken: (token) => {
	  Taro.setStorageSync('domainA.token', token)
	  set({ token })
	},
  updateProfile: (profile) => set((state) => ({ profile: { ...state.profile, ...profile } }))
}))
