import type { AdminSession } from '../types'
import { adminRequest } from './http'

export interface AdminAccount {
  id: number
  username: string
  displayName: string
  role: 'admin' | 'superadmin'
  status: 'active' | 'disabled'
  locked: boolean
  passwordSetAt: string
  legacyUserId?: number
}

export interface CreateAdminAccountInput {
  username: string
  displayName: string
  password: string
  role: 'admin' | 'superadmin'
}

export const adminAuthAPI = {
  login: (username: string, password: string) => adminRequest<AdminSession>('/admin-auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password })
  }),
  session: () => adminRequest<AdminSession>('/admin-auth/session'),
  logout: () => adminRequest<{ loggedOut: boolean }>('/admin-auth/logout', { method: 'POST' }),
  changePassword: (oldPassword: string, newPassword: string) =>
    adminRequest<{ passwordChanged: boolean }>('/admin-auth/password/change', {
      method: 'POST',
      body: JSON.stringify({ oldPassword, newPassword })
    }),
  accounts: () => adminRequest<{ items: AdminAccount[] }>('/admin-auth/accounts'),
  createAccount: (input: CreateAdminAccountInput) =>
    adminRequest<AdminAccount>('/admin-auth/accounts', {
      method: 'POST',
      body: JSON.stringify(input)
    }),
  updateAccount: (id: number, displayName: string, role: 'admin' | 'superadmin') =>
    adminRequest<AdminAccount>(`/admin-auth/accounts/${id}`, {
      method: 'PATCH',
      body: JSON.stringify({ displayName, role })
    }),
  setAccountStatus: (id: number, status: 'active' | 'disabled') =>
    adminRequest<AdminAccount>(`/admin-auth/accounts/${id}/status`, {
      method: 'POST',
      body: JSON.stringify({ status })
    }),
  resetAccountPassword: (id: number, password: string) =>
    adminRequest<{ reset: boolean }>(`/admin-auth/accounts/${id}/password`, {
      method: 'POST',
      body: JSON.stringify({ password })
    })
}
