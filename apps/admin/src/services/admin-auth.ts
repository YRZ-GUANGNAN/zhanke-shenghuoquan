import type { AdminSession } from '../types'
import { adminRequest } from './http'

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
  credentials: () => adminRequest<{ items: AdminCredential[] }>('/admin-auth/credentials'),
  saveCredential: (userId: number, username: string, password: string) =>
    adminRequest<{ saved: boolean }>(`/admin-auth/credentials/${userId}`, {
      method: 'PUT',
      body: JSON.stringify({ username, password })
    }),
  removeCredential: (userId: number) =>
    adminRequest<{ removed: boolean }>(`/admin-auth/credentials/${userId}`, { method: 'DELETE' })
}

export interface AdminCredential {
  credentialId: number
  userId: number
  username: string
  nickname: string
  role: 'admin' | 'superadmin'
  locked: boolean
  passwordSetAt: string
}
