import { create } from 'zustand'
import {
  currentIdentity,
  currentUser,
  emptyProfile,
  switchIdentity as requestIdentitySwitch,
  wechatLogin
} from '../services/a-user'
import { isApiError } from '../services/api'
import Taro from '../services/taro'
import type { IdentityRoleKey, IdentitySummary, UserProfile } from '../types'

const TOKEN_KEY = 'domainA.token'

export type SessionStatus = 'idle' | 'booting' | 'authenticated' | 'anonymous' | 'error'

interface SessionState {
  status: SessionStatus
  token: string
  profile: UserProfile
  identity: IdentitySummary | null
  error: string
  bootstrap: (force?: boolean) => Promise<void>
  retry: () => Promise<void>
  logout: () => void
  refreshIdentity: () => Promise<IdentitySummary>
  switchIdentity: (identity: IdentityRoleKey) => Promise<IdentitySummary>
  setToken: (token: string) => void
  updateProfile: (profile: Partial<UserProfile>) => void
}

let bootstrapInFlight: Promise<void> | null = null

function errorMessage(error: unknown): string {
  if (isApiError(error) && error.status === 0) return '网络连接失败，请检查后重试'
  if (error instanceof Error && error.message) return error.message
  return '登录失败，请稍后重试'
}

export const useSessionStore = create<SessionState>((set, get) => {
  const clearExpiredSession = () => {
    Taro.removeStorageSync(TOKEN_KEY)
    set({
      status: 'anonymous',
      token: '',
      profile: emptyProfile,
      identity: null,
      error: '登录已过期，请重新登录'
    })
  }

  const runBootstrap = async (force = false): Promise<void> => {
    if (!force && get().status === 'authenticated') return

    const storedToken = Taro.getStorageSync<string>(TOKEN_KEY) || ''
    set({
      status: 'booting',
      token: storedToken,
      error: ''
    })

    if (storedToken) {
      try {
        // Keep authentication hydration ordered so a stale-token 401 cannot
        // be masked by a concurrent network or server failure.
        const profile = await currentUser()
        const identity = await currentIdentity()
        set({ status: 'authenticated', token: storedToken, profile, identity, error: '' })
        return
      } catch (error) {
        if (!isApiError(error) || error.status !== 401) {
          set({ status: 'error', token: storedToken, error: errorMessage(error) })
          return
        }
        Taro.removeStorageSync(TOKEN_KEY)
        set({ token: '', profile: emptyProfile, identity: null })
      }
    }

    try {
      const loginResult = await Taro.login({ timeout: 10000 })
      if (!loginResult.code) throw new Error('微信登录未返回有效 code')

      const session = await wechatLogin(loginResult.code)
      Taro.setStorageSync(TOKEN_KEY, session.token)
      set({ token: session.token, profile: session.profile })

      const identity = await currentIdentity()
      set({
        status: 'authenticated',
        token: session.token,
        profile: session.profile,
        identity,
        error: ''
      })
    } catch (error) {
      if (isApiError(error) && error.status === 401) {
        Taro.removeStorageSync(TOKEN_KEY)
        set({ token: '', profile: emptyProfile, identity: null })
      }
      set({ status: 'error', error: errorMessage(error) })
    }
  }

  const bootstrap = (force = false): Promise<void> => {
    if (bootstrapInFlight) return bootstrapInFlight
    const current = runBootstrap(force)
    bootstrapInFlight = current
    const clear = () => {
      if (bootstrapInFlight === current) bootstrapInFlight = null
    }
    void current.then(clear, clear)
    return current
  }

  return {
    status: 'idle',
    token: '',
    profile: emptyProfile,
    identity: null,
    error: '',
    bootstrap,
    retry: () => bootstrap(true),
    logout: () => {
      Taro.removeStorageSync(TOKEN_KEY)
      set({
        status: 'anonymous',
        token: '',
        profile: emptyProfile,
        identity: null,
        error: ''
      })
    },
    refreshIdentity: async () => {
      try {
        const identity = await currentIdentity()
        set({ identity })
        return identity
      } catch (error) {
        if (isApiError(error) && error.status === 401) clearExpiredSession()
        throw error
      }
    },
    switchIdentity: async (target) => {
      try {
        const identity = await requestIdentitySwitch(target)
        set({ identity })
        return identity
      } catch (error) {
        if (isApiError(error) && error.status === 401) clearExpiredSession()
        throw error
      }
    },
    setToken: (token) => {
      if (token) Taro.setStorageSync(TOKEN_KEY, token)
      else Taro.removeStorageSync(TOKEN_KEY)
      set({ token })
    },
    updateProfile: (profile) => set((state) => ({
      profile: { ...state.profile, ...profile }
    }))
  }
})
