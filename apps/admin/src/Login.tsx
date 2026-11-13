import { useState, type FormEvent } from 'react'
import { adminAuthAPI } from './services/admin-auth'
import { useAdminStore } from './store'
import { AdminButton, AdminField, AdminNotice } from './components'

function greeting() {
  const hour = new Date().getHours()
  if (hour < 6) return '夜深了'
  if (hour < 11) return '早上好'
  if (hour < 14) return '中午好'
  if (hour < 18) return '下午好'
  return '晚上好'
}

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
    <div className="login-shell">
      <section className="login-brand" aria-hidden="true">
        <div className="login-brand__top">
          <div className="login-brand__mark">湛</div>
          <div className="login-brand__name"><strong>湛科生活圈</strong><span>经营与运营工作台</span></div>
        </div>
        <div className="login-brand__body">
          <h2 className="login-brand__word">{greeting()}</h2>
        </div>
      </section>

      <section className="login-form" aria-labelledby="admin-login-title">
        <div className="login-form__head">
          <p className="login-form__eyebrow">湛科生活圈 · 运营后台</p>
          <h1 id="admin-login-title">管理员登录</h1>
          <p className="login-form__description">请使用分配给你的管理员账号继续。</p>
        </div>
        <form onSubmit={event => void submit(event)}>
          <AdminField label="管理员用户名" value={username} onChange={event => setUsername(event.target.value)} autoComplete="username" required />
          <AdminField label="密码" type="password" value={password} onChange={event => setPassword(event.target.value)} minLength={14} maxLength={72} autoComplete="current-password" autoFocus required description={password.length < 14 ? `至少 14 位字符，当前 ${password.length} 位` : undefined} />
          {error && <AdminNotice tone="danger" title="登录失败">{error}</AdminNotice>}
          <AdminButton type="submit" tone="primary" loading={busy} disabled={password.length < 14}>登录</AdminButton>
        </form>
        <p className="login-form__security">管理员会话受独立权限校验保护，请勿在公共设备保存密码。</p>
      </section>
    </div>
  </main>
}
