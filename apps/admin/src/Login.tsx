import { useState, type FormEvent } from 'react'
import { adminAuthAPI } from './services/admin-auth'
import { useAdminStore } from './store'
import { AdminButton, AdminField, AdminNotice } from './components'

export function Login() {
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const setAuthenticated = useAdminStore(state => state.setAuthenticated)

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    setBusy(true)
    setError('')
    try {
      const session = await adminAuthAPI.login(password)
      setPassword('')
      setAuthenticated(session)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '登录失败')
    } finally {
      setBusy(false)
    }
  }

  return <main className="login-page">
    <section className="login-card" aria-labelledby="admin-login-title">
      <div className="login-card__identity">
        <div className="login-card__mark" aria-hidden="true">湛</div>
        <div><p className="login-card__eyebrow">湛科生活圈</p><span>运营管理系统</span></div>
      </div>
      <div className="login-card__intro">
        <h1 id="admin-login-title">欢迎回来</h1>
        <p className="login-card__description">使用独立管理员密码进入后台，管理校园内容与服务运营。</p>
      </div>
      <form onSubmit={event => void submit(event)}>
        <AdminField label="管理员用户名" value="administrator" disabled autoComplete="username" />
        <AdminField label="密码" type="password" value={password} onChange={event => setPassword(event.target.value)} minLength={14} maxLength={72} autoComplete="current-password" autoFocus required />
        {error && <AdminNotice tone="danger" title="登录失败">{error}</AdminNotice>}
        <AdminButton type="submit" tone="primary" loading={busy} disabled={password.length < 14}>登录</AdminButton>
      </form>
      <p className="login-card__security">管理员会话受独立权限校验保护，请勿在公共设备保存密码。</p>
    </section>
  </main>
}
