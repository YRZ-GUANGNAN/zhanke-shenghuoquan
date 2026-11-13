import { useState, type FormEvent } from 'react'
import { adminAuthAPI } from './services/admin-auth'
import { useAdminStore } from './store'
import { AdminButton, AdminField, AdminNotice } from './components'

export function Login() {
  const [username, setUsername] = useState('administrator')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const setAuthenticated = useAdminStore(state => state.setAuthenticated)

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    setBusy(true)
    setError('')
    try {
      const session = await adminAuthAPI.login(username.trim(), password)
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
        <div><p className="login-card__eyebrow">湛科生活圈</p><span>经营与运营工作台</span></div>
      </div>
      <div className="login-card__intro">
        <h1 id="admin-login-title">欢迎回来</h1>
        <p className="login-card__description">查看经营数据，处理订单、钱包、通知与校园内容。</p>
      </div>
      <form onSubmit={event => void submit(event)}>
        <AdminField label="管理员用户名" value={username} onChange={event => setUsername(event.target.value)} autoComplete="username" required />
        <AdminField label="密码" type="password" value={password} onChange={event => setPassword(event.target.value)} minLength={14} maxLength={72} autoComplete="current-password" autoFocus required description={password.length < 14 ? `至少 14 位字符，当前 ${password.length} 位` : undefined} />
        {error && <AdminNotice tone="danger" title="登录失败">{error}</AdminNotice>}
        <AdminButton type="submit" tone="primary" loading={busy} disabled={password.length < 14}>登录</AdminButton>
      </form>
      <p className="login-card__security">管理员会话受独立权限校验保护，请勿在公共设备保存密码。</p>
    </section>
  </main>
}
