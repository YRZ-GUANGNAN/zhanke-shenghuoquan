import { useEffect, useState, type FormEvent } from 'react'
import { adminAuthAPI } from './services/admin-auth'
import { useAdminStore } from './store'
import { AdminButton, AdminField, AdminNotice } from './components'

export function Login() {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const setAuthenticated = useAdminStore(state => state.setAuthenticated)

  // 后台控制台整体按桌面宽度设计（body min-width），登录页允许窄屏使用。
  useEffect(() => {
    document.body.classList.add('login-mode')
    return () => document.body.classList.remove('login-mode')
  }, [])

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    // 浏览器密码管理器自动填充不会触发 onChange；以表单 DOM 值为准，避免按钮永远禁用。
    const data = new FormData(event.currentTarget)
    const submittedUsername = String(data.get('username') ?? username).trim()
    const submittedPassword = String(data.get('password') ?? password)
    if (!submittedUsername || !submittedPassword) {
      setError('请输入管理员用户名和密码')
      return
    }
    setBusy(true)
    setError('')
    try {
      const session = await adminAuthAPI.login(submittedUsername, submittedPassword)
      setPassword('')
      setAuthenticated(session)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '登录失败')
    } finally {
      setBusy(false)
    }
  }

  return <main className="login-page">
    <div className="login-stage">
      <header className="login-hero">
        <span className="login-hero__mark"><img src="/images/brand-logo.png" alt="湛科万事通" /></span>
        <div className="login-hero__name">
          <strong>湛科生活圈</strong>
          <span>经营与运营工作台</span>
          <p className="login-hero__desc">校园社区 · 二手交易 · 校园配送 · 服务入口</p>
        </div>
      </header>

      <section className="login-form" aria-labelledby="admin-login-title">
        <div className="login-form__head">
          <p className="login-form__eyebrow">湛科生活圈 · 运营后台</p>
          <h1 id="admin-login-title">管理员登录</h1>
          <p className="login-form__description">请使用分配给你的管理员账号继续。</p>
        </div>
        <form onSubmit={event => void submit(event)}>
          <AdminField label="管理员用户名" name="username" value={username} onChange={event => setUsername(event.target.value)} autoComplete="username" required />
          <AdminField label="密码" name="password" type="password" value={password} onChange={event => setPassword(event.target.value)} autoComplete="current-password" autoFocus required />
          {error && <AdminNotice tone="danger" title="登录失败">{error}</AdminNotice>}
          <AdminButton type="submit" tone="primary" loading={busy} disabled={busy}>登录</AdminButton>
        </form>
        <p className="login-form__security">管理员会话受独立权限校验保护，请勿在公共设备保存密码。</p>
      </section>

      <p className="login-page__foot">湛科生活圈 · 校园生活服务平台</p>
    </div>
  </main>
}
