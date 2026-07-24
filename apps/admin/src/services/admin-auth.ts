import type { AdminSession } from '../types'
import { adminRequest } from './http'

export const adminAuthAPI = {
  login: (password: string) => adminRequest<AdminSession>('/admin-auth/login', {
    method: 'POST',
    body: JSON.stringify({ username: 'administrator', password })
  }),
  session: () => adminRequest<AdminSession>('/admin-auth/session'),
  logout: () => adminRequest<{ loggedOut: boolean }>('/admin-auth/logout', { method: 'POST' }),
  changePassword: (oldPassword: string, newPassword: string) =>
    adminRequest<{ passwordChanged: boolean }>('/admin-auth/password/change', {
      method: 'POST',
      body: JSON.stringify({ oldPassword, newPassword })
    })
}
