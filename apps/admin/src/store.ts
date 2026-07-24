import { create } from 'zustand'
import type { AdminSection, AdminSession, AdminUser } from './types'

interface AdminState {
  section: AdminSection
  authStatus: 'checking' | 'authenticated' | 'unauthenticated'
  user: AdminUser | null
  csrfToken: string
  setSection: (section: AdminSection) => void
  setAuthenticated: (session: AdminSession) => void
  setUnauthenticated: () => void
}

export const useAdminStore = create<AdminState>((set) => ({
  section: 'dashboard',
  authStatus: 'checking',
  user: null,
  csrfToken: '',
  setSection: (section) => set({ section }),
  setAuthenticated: (session) => set({
    authStatus: 'authenticated',
    user: session.user,
    csrfToken: session.csrfToken
  }),
  setUnauthenticated: () => set({
    authStatus: 'unauthenticated',
    user: null,
    csrfToken: ''
  })
}))

