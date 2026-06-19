import { create } from 'zustand'
import type { AdminSection } from './types'

interface AdminState {
  section: AdminSection
  setSection: (section: AdminSection) => void
}

export const useAdminStore = create<AdminState>((set) => ({
  section: 'dashboard',
  setSection: (section) => set({ section })
}))

