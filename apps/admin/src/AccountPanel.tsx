import { useState } from 'react'
import { adminAuthAPI } from './services/admin-auth'
import { useAdminStore } from './store'
import { AdminButton, AdminField, AdminNotice, useAdminDialog } from './components'

export function AccountPanel() {
  const dialog = useAdminDialog()
  const user = useAdminStore(state => state.user)
  const setUnauthenticated = useAdminStore(state => state.setUnauthenticated)
  const [changing, setChanging] = useState(false)
  const [changeOpen, setChangeOpen] = useState(false)
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmation, setConfirmation] = useState('')
  const [error, setError] = useState('')

  const logout = async () => {
    setError('')
    try {
      await adminAuthAPI.logout()
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '退出失败')
      return
    }
    setUnauthenticated()
  }

  const changePassword = async () => {
    if (newPassword !== confirmation) {
      setError('两次输入的新密码不一致')
      return
    }
    setChanging(true)
    setError('')
    try {
      await adminAuthAPI.changePassword(oldPassword, newPassword)
      await dialog.notify({ title: '密码已修改', description: '所有管理网页会话已退出，请使用新密码重新登录。', confirmLabel: '重新登录' })
      setOldPassword('')
      setNewPassword('')
      setConfirmation('')
      setChangeOpen(false)
      setUnauthenticated()
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '修改密码失败')
    } finally {
      setChanging(false)
    }
  }

  return <>
    <div className="account-panel">
      <div className="account-panel__identity">
        <span className="account-panel__avatar" aria-hidden="true">{(user?.nickname || '管').slice(0, 1)}</span>
        <div>
        <strong>{user?.nickname || '超级管理员'}</strong>
        <span>administrator · #{user?.id}</span>
        {error && <small role="alert">{error}</small>}
        </div>
      </div>
      <div className="account-panel__actions">
        <button type="button" className="ghost-button" onClick={() => { setError(''); setChangeOpen(true) }}>修改密码</button>
        <button type="button" className="ghost-button account-panel__logout" onClick={() => void logout()}>退出</button>
      </div>
    </div>
    {changeOpen && <div className="password-dialog" role="dialog" aria-modal="true" aria-labelledby="password-dialog-title">
      <form onSubmit={event => { event.preventDefault(); void changePassword() }}>
        <h2 id="password-dialog-title">修改管理员密码</h2>
        <p>新密码需为 14–72 个字符。修改成功后，所有管理网页会话都会退出。</p>
        <AdminField label="当前密码" type="password" value={oldPassword} onChange={event => setOldPassword(event.target.value)} autoComplete="current-password" required autoFocus />
        <AdminField label="新密码" type="password" value={newPassword} onChange={event => setNewPassword(event.target.value)} minLength={14} maxLength={72} autoComplete="new-password" description="14–72 个字符" required />
        <AdminField label="确认新密码" type="password" value={confirmation} onChange={event => setConfirmation(event.target.value)} minLength={14} maxLength={72} autoComplete="new-password" required />
        {error && <AdminNotice tone="danger">{error}</AdminNotice>}
        <div className="password-dialog__actions">
          <AdminButton tone="secondary" disabled={changing} onClick={() => setChangeOpen(false)}>取消</AdminButton>
          <AdminButton type="submit" tone="primary" loading={changing}>确认修改</AdminButton>
        </div>
      </form>
    </div>}
  </>
}
