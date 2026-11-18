import { create } from 'zustand'
import type { AdminSection, AdminSession, AdminUser } from './types'

export interface AdminSearchTarget {
  section: AdminSection
  orderId?: string
  keyword?: string
}

interface AdminState {
  section: AdminSection
  authStatus: 'checking' | 'authenticated' | 'unauthenticated'
  user: AdminUser | null
  csrfToken: string
  // 全局搜索结果跳转目标：目标视图消费后清除（订单直达详情等）。
  searchTarget: AdminSearchTarget | null
  setSection: (section: AdminSection) => void
  setSearchTarget: (target: AdminSearchTarget | null) => void
  setAuthenticated: (session: AdminSession) => void
  setUnauthenticated: () => void
}

export const useAdminStore = create<AdminState>((set) => ({
  section: 'dashboard',
  authStatus: 'checking',
  searchTarget: null,
  setSearchTarget: (target) => set({ searchTarget: target }),
  user: null,
  csrfToken: '',
  setSection: (section) => set({ section }),
  setAuthenticated: (session) => set({
    authStatus: 'authenticated',
    user: session.account,
    csrfToken: session.csrfToken
  }),
  setUnauthenticated: () => set({
    authStatus: 'unauthenticated',
    user: null,
    csrfToken: ''
  })
}))

