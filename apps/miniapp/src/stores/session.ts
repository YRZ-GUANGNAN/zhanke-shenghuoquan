import { create } from 'zustand'
import {
  currentIdentity,
  currentUser,
  emptyProfile,
  switchIdentity as requestIdentitySwitch,
  wechatLogin
} from '../services/a-user'
import { isApiError } from '../services/api'
import { resetTencentIMSession } from '../services/e-im'
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
let bootstrapInFlightEpoch = 0
let sessionEpoch = 0

function errorMessage(error: unknown): string {
  if (isApiError(error) && error.status === 0) return '网络连接失败，请检查后重试'
  if (error instanceof Error && error.message) return error.message
  return '登录失败，请稍后重试'
}

export const useSessionStore = create<SessionState>((set, get) => {
  const invalidateBootstrap = () => {
    sessionEpoch += 1
    bootstrapInFlight = null
    bootstrapInFlightEpoch = 0
  }

  const clearExpiredSession = () => {
    invalidateBootstrap()
    void resetTencentIMSession()
    Taro.removeStorageSync(TOKEN_KEY)
    set({
      status: 'anonymous',
      token: '',
      profile: emptyProfile,
      identity: null,
      error: '登录已过期，请重新登录'
    })
  }

  const runBootstrap = async (force: boolean, epoch: number): Promise<void> => {
    if (!force && get().status === 'authenticated') return
    const isCurrent = () => sessionEpoch === epoch

    const storedToken = Taro.getStorageSync<string>(TOKEN_KEY) || ''
    if (!isCurrent()) return
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
        if (!isCurrent()) return
        const identity = await currentIdentity()
        if (!isCurrent()) return
        set({ status: 'authenticated', token: storedToken, profile, identity, error: '' })
        return
      } catch (error) {
        if (!isCurrent()) return
        if (!isApiError(error) || error.status !== 401) {
          set({ status: 'error', token: storedToken, error: errorMessage(error) })
          return
        }
        await resetTencentIMSession()
        if (!isCurrent()) return
        Taro.removeStorageSync(TOKEN_KEY)
        set({ token: '', profile: emptyProfile, identity: null })
      }
    }

    try {
      const loginResult = await Taro.login({ timeout: 10000 })
      if (!isCurrent()) return
      if (!loginResult.code) throw new Error('微信登录未返回有效 code')

      const session = await wechatLogin(loginResult.code)
      if (!isCurrent()) return
      await resetTencentIMSession()
      if (!isCurrent()) return
      Taro.setStorageSync(TOKEN_KEY, session.token)
      set({ token: session.token, profile: session.profile })

      const identity = await currentIdentity()
      if (!isCurrent()) return
      set({
        status: 'authenticated',
        token: session.token,
        profile: session.profile,
        identity,
        error: ''
      })
    } catch (error) {
      if (!isCurrent()) return
      if (isApiError(error) && error.status === 401) {
        await resetTencentIMSession()
        if (!isCurrent()) return
        Taro.removeStorageSync(TOKEN_KEY)
        set({ token: '', profile: emptyProfile, identity: null })
      }
      set({ status: 'error', error: errorMessage(error) })
    }
  }

  const bootstrap = (force = false): Promise<void> => {
    if (bootstrapInFlight) return bootstrapInFlight
    if (!force && get().status === 'authenticated') return Promise.resolve()
    const epoch = sessionEpoch + 1
    sessionEpoch = epoch
    const current = runBootstrap(force, epoch)
    bootstrapInFlight = current
    bootstrapInFlightEpoch = epoch
    const clear = () => {
      if (bootstrapInFlight === current && bootstrapInFlightEpoch === epoch) {
        bootstrapInFlight = null
        bootstrapInFlightEpoch = 0
      }
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
      invalidateBootstrap()
      void resetTencentIMSession()
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
      const epoch = sessionEpoch
      const token = get().token
      try {
        const identity = await currentIdentity()
        if (sessionEpoch !== epoch || get().token !== token) {
          throw new Error('登录会话已切换，请重试')
        }
        set({ identity })
        return identity
      } catch (error) {
        if (isApiError(error) && error.status === 401) clearExpiredSession()
        throw error
      }
    },
    switchIdentity: async (target) => {
      const epoch = sessionEpoch
      const token = get().token
      try {
        const identity = await requestIdentitySwitch(target)
        if (sessionEpoch !== epoch || get().token !== token) {
          throw new Error('登录会话已切换，请重试')
        }
        set({ identity })
        return identity
      } catch (error) {
        if (isApiError(error) && error.status === 401) clearExpiredSession()
        throw error
      }
    },
    setToken: (token) => {
      if (token === get().token) return
      invalidateBootstrap()
      void resetTencentIMSession()
      if (token) Taro.setStorageSync(TOKEN_KEY, token)
      else Taro.removeStorageSync(TOKEN_KEY)
      set({
        status: token ? 'idle' : 'anonymous',
        token,
        profile: emptyProfile,
        identity: null,
        error: ''
      })
    },
    updateProfile: (profile) => set((state) => ({
      profile: { ...state.profile, ...profile }
    }))
  }
})
