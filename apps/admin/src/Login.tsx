import { useState, type FormEvent } from 'react'
import { adminAuthAPI } from './services/admin-auth'
import { useAdminStore } from './store'

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
    <section className="login-card">
      <div className="login-card__mark">湛</div>
      <p className="login-card__eyebrow">湛科生活圈</p>
      <h1>管理后台登录</h1>
      <p className="login-card__description">使用独立管理员密码登录，无需将当前微信账号添加为小程序项目成员。</p>
      <form onSubmit={event => void submit(event)}>
        <label>
          管理员用户名
          <input value="administrator" disabled autoComplete="username" />
        </label>
        <label>
          密码
          <input
            type="password"
            value={password}
            onChange={event => setPassword(event.target.value)}
            minLength={14}
            maxLength={72}
            autoComplete="current-password"
            autoFocus
            required
          />
        </label>
        {error && <div className="login-error" role="alert">{error}</div>}
        <button type="submit" disabled={busy || password.length < 14}>
          {busy ? '正在登录…' : '登录'}
        </button>
      </form>
    </section>
  </main>
}
