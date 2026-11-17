import zhCN from 'antd/lib/locale/zh_CN'
import {
  Badge,
  Button as AntButton,
  ConfigProvider,
  Drawer,
  Input,
  Layout,
  Menu,
  Tag
} from 'antd'
import {
  AppstoreOutlined,
  AuditOutlined,
  BellOutlined,
  CommentOutlined,
  DashboardOutlined,
  FileImageOutlined,
  IdcardOutlined,
  MessageOutlined,
  SettingOutlined,
  ShareAltOutlined,
  ShopOutlined,
  ShoppingOutlined,
  UserOutlined,
  WarningOutlined
} from '@ant-design/icons'
import { Component, lazy, Suspense, useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import type { PreviewSection } from './data'
import { Modal, ToastProvider, usePreviewToast } from './ui'
// 懒加载分区的错误边界：发版后旧页面引用已删除的 chunk，或网络故障时，
// 未捕获的渲染错误会让 React 卸载整棵树（白屏）。这里兜住并给出一键恢复。
const CHUNK_RELOAD_KEY = 'preview-chunk-reload-at'

class ContentErrorBoundary extends Component<{ children: ReactNode }, { error: Error | null }> {
  state: { error: Error | null } = { error: null }

  static getDerivedStateFromError(error: Error) {
    return { error }
  }

  componentDidCatch(error: Error) {
    const chunkFailure = /dynamically imported module|Importing a module script failed|Loading chunk/i.test(String(error?.message || ''))
    if (!chunkFailure) return
    // 版本更新后旧 chunk 已不存在：自动刷新一次拿新资源；30 秒节流避免死循环。
    try {
      const last = Number(window.sessionStorage.getItem(CHUNK_RELOAD_KEY) || 0)
      if (Date.now() - last > 30_000) {
        window.sessionStorage.setItem(CHUNK_RELOAD_KEY, String(Date.now()))
        window.location.reload()
      }
    } catch {
      // sessionStorage 不可用时仅展示恢复按钮。
    }
  }

  render() {
    if (this.state.error) {
      return <div className="preview-form-error preview-form-error--action" role="alert">
        <span>页面加载失败，可能是版本已更新或网络中断。</span>
        <Button size="sm" onClick={() => window.location.reload()}>刷新页面</Button>
      </div>
    }
    return this.props.children
  }
}

// 各业务分区按需加载：首次进入分区时才下载对应代码块，减小首屏 bundle。
const DashboardView = lazy(() => import('./views/DashboardView').then(module => ({ default: module.DashboardView })))
const DeliveryView = lazy(() => import('./views/DeliveryView').then(module => ({ default: module.DeliveryView })))
const NotificationsView = lazy(() => import('./views/NotificationsView').then(module => ({ default: module.NotificationsView })))
const OperationsView = lazy(() => import('./views/OperationsView').then(module => ({ default: module.OperationsView })))
const PeopleView = lazy(() => import('./views/PeopleView').then(module => ({ default: module.PeopleView })))
const SettingsView = lazy(() => import('./views/SettingsView').then(module => ({ default: module.SettingsView })))
const LegacyTakeoutView = lazy(() => import('./views/LegacyTakeoutView').then(module => ({ default: module.LegacyTakeoutView })))
import { Login } from '../Login'
import { AccountPanel } from '../AccountPanel'
import { useAdminStore, type AdminSearchTarget } from '../store'
import type { AdminSection } from '../types'
import { Button } from './ui'
import { adminAuthAPI } from '../services/admin-auth'
import { adminAPI, type AdminSearchHit } from '../services/f-admin'
import { legacyTakeoutEnabled } from '../feature-flags'

interface NavigationItem {
  key: PreviewSection
  label: string
  icon: React.ReactNode
  group: '工作台' | '业务运营' | '内容与用户' | '平台设置'
}

const navigation: NavigationItem[] = [
  { key: 'dashboard', label: '经营总览', icon: <DashboardOutlined />, group: '工作台' },
  { key: 'delivery', label: '配送与结算', icon: <ShoppingOutlined />, group: '业务运营' },
  { key: 'notifications', label: '消息通知', icon: <BellOutlined />, group: '业务运营' },
  { key: 'media', label: '素材中心', icon: <FileImageOutlined />, group: '业务运营' },
  { key: 'moderation', label: '审核中心', icon: <AuditOutlined />, group: '内容与用户' },
  { key: 'community', label: '社区内容', icon: <CommentOutlined />, group: '内容与用户' },
  { key: 'trade', label: '二手商品', icon: <AppstoreOutlined />, group: '内容与用户' },
  { key: 'reports', label: '举报处理', icon: <WarningOutlined />, group: '内容与用户' },
  { key: 'identities', label: '商家骑手认证', icon: <IdcardOutlined />, group: '内容与用户' },
  { key: 'feedbacks', label: '意见反馈', icon: <MessageOutlined />, group: '内容与用户' },
  { key: 'users', label: '用户与权限', icon: <UserOutlined />, group: '平台设置' },
  { key: 'navigation', label: '服务入口', icon: <AppstoreOutlined />, group: '平台设置' },
  { key: 'share-settings', label: '分享设置', icon: <ShareAltOutlined />, group: '平台设置' },
  { key: 'feature-policy', label: '功能开关', icon: <SettingOutlined />, group: '平台设置' },
  { key: 'takeout', label: '旧版外卖', icon: <ShopOutlined />, group: '平台设置' }
]

const groups: NavigationItem['group'][] = ['工作台', '业务运营', '内容与用户', '平台设置']

const SEARCH_TYPE_LABELS: Record<string, string> = {
  order: '订单',
  user: '用户',
  post: '帖子',
  product: '商品',
  rider: '骑手'
}

function ConsoleShell() {
  const [section, setSection] = useState<PreviewSection>('dashboard')
  const [collapsed, setCollapsed] = useState(false)
  const [navOpen, setNavOpen] = useState(false)
  const [narrow, setNarrow] = useState(() => typeof window !== 'undefined' && window.matchMedia('(max-width: 900px)').matches)
  useEffect(() => {
    const query = window.matchMedia('(max-width: 900px)')
    const onChange = () => setNarrow(query.matches)
    query.addEventListener('change', onChange)
    return () => query.removeEventListener('change', onChange)
  }, [])
  const [searchOpen, setSearchOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [hits, setHits] = useState<AdminSearchHit[]>([])
  const [hitsLoading, setHitsLoading] = useState(false)
  const [hitsError, setHitsError] = useState('')
  const [attentionCount, setAttentionCount] = useState(0)
  const { notify } = usePreviewToast()

  // 待办徽章与导航可见性都来自真实接口；失败时保持全部可见并静默降级。
  useEffect(() => {
    let active = true
    adminAPI.stats()
      .then(stats => { if (active) setAttentionCount(stats.totals.pendingReview) })
      .catch(() => {})
    return () => { active = false }
  }, [])

  // 后台入口与用户端功能开关解耦：开关只影响小程序端展示，
  // 关闭某个用户功能不应连后台的履约、资金与通知管理入口一起隐藏。
  const availableNavigation = useMemo(() => navigation.filter(item => {
    if (item.key === 'takeout') return legacyTakeoutEnabled
    return true
  }), [])

  const active = navigation.find(item => item.key === section)
  const filteredNavigation = useMemo(() => {
    const normalized = query.trim().toLowerCase()
    if (!normalized) return availableNavigation
    return availableNavigation.filter(item => item.label.toLowerCase().includes(normalized))
  }, [availableNavigation, query])

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault()
        setSearchOpen(true)
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [])

  // 全局内容搜索：≥2 个字符，防抖 300ms，带竞态守卫。
  useEffect(() => {
    const keyword = query.trim()
    if (keyword.length < 2) {
      setHits([])
      setHitsError('')
      setHitsLoading(false)
      return
    }
    let active = true
    setHitsLoading(true)
    setHitsError('')
    const timer = window.setTimeout(() => {
      adminAPI.search(keyword)
        .then(result => { if (active) setHits(result.hits ?? []) })
        .catch(reason => { if (active) setHitsError(reason instanceof Error ? reason.message : String(reason)) })
        .finally(() => { if (active) setHitsLoading(false) })
    }, 300)
    return () => { active = false; window.clearTimeout(timer) }
  }, [query])

  const setSearchTarget = useAdminStore(state => state.setSearchTarget)
  // 关闭搜索时一并清空关键字与旧结果，避免下次打开显示过期数据。
  const closeSearch = () => {
    setSearchOpen(false)
    setQuery('')
    setHits([])
    setHitsError('')
  }
  const navigate = (next: PreviewSection) => {
    setSection(next)
    closeSearch()
  }
  const openSearchHit = (hit: AdminSearchHit) => {
    const target: AdminSearchTarget = { section: hit.section as AdminSection }
    if (hit.type === 'order') target.orderId = hit.id
    else if (hit.type === 'rider') target.keyword = hit.title
    setSearchTarget(target)
    navigate(hit.section as PreviewSection)
  }

  const content = (() => {
    switch (section) {
      case 'dashboard':
        return <DashboardView navigate={navigate} />
      case 'delivery':
        return <DeliveryView />
      case 'notifications':
        return <NotificationsView />
      case 'media':
      case 'moderation':
      case 'community':
      case 'trade':
      case 'reports':
        return <OperationsView section={section} />
      case 'identities':
      case 'feedbacks':
      case 'users':
        return <PeopleView section={section} />
      case 'navigation':
      case 'share-settings':
      case 'feature-policy':
        return <SettingsView section={section} />
      case 'takeout':
        return <LegacyTakeoutView />
    }
  })()

  const menuItems = groups.map(group => ({
    type: 'submenu' as const,
    key: `group-${group}`,
    label: group,
    children: availableNavigation
      .filter(item => item.group === group)
      .map(item => ({ key: item.key, icon: item.icon, label: item.label }))
  })).filter(item => item.children.length > 0)

  return <ConfigProvider componentSize="middle" locale={zhCN}>
    <Layout className="preview-shell">
      {!narrow && <Layout.Sider
        className="preview-sider"
        width={216}
        collapsedWidth={64}
        collapsed={collapsed}
        theme="light"
        collapsible
        trigger={null}
      >
        <div className="preview-logo">
          <span className="preview-logo__mark"><img className="brand-logo-img" src="/images/brand-logo.png" alt="湛科万事通" /></span>
          {!collapsed && <span>湛科运营云</span>}
        </div>
        <Menu
          mode="inline"
          theme="light"
          selectedKeys={[section]}
          defaultOpenKeys={[`group-${active?.group || '工作台'}`]}
          items={menuItems}
          onClick={({ key }) => navigate(key as PreviewSection)}
        />
      </Layout.Sider>}
      {narrow && <Drawer
        placement="left"
        width={248}
        open={navOpen}
        onClose={() => setNavOpen(false)}
        closable={false}
        bodyStyle={{ padding: 0 }}
      >
        <div className="preview-logo">
          <span className="preview-logo__mark"><img className="brand-logo-img" src="/images/brand-logo.png" alt="湛科万事通" /></span>
          <span>湛科运营云</span>
        </div>
        <Menu
          mode="inline"
          theme="light"
          selectedKeys={[section]}
          defaultOpenKeys={[`group-${active?.group || '工作台'}`]}
          items={menuItems}
          onClick={({ key }) => { navigate(key as PreviewSection); setNavOpen(false) }}
        />
      </Drawer>}
      <Layout className="preview-main-shell">
        <header className="preview-topbar">
          <div className="preview-topbar__left">
            <AntButton
              type="text"
              icon={collapsed || narrow ? <span>☰</span> : <span>◧</span>}
              onClick={() => (narrow ? setNavOpen(true) : setCollapsed(!collapsed))}
              aria-label={narrow ? '打开导航' : '切换导航'}
            />
            <span className="preview-breadcrumb"><strong>{active?.label || ''}</strong></span>
          </div>
          <div className="preview-topbar__right">
            {/* 用点击打开而不是 focus：Modal 关闭会把焦点还给触发元素，
                onFocus 会导致「关闭后立刻重开」的死循环。 */}
            <Input
              className="preview-topbar__search"
              prefix={<span>⌕</span>}
              placeholder="搜索订单、用户、帖子或页面"
              onClick={() => setSearchOpen(true)}
              onKeyDown={event => { if (event.key === 'Enter' || event.key === ' ') setSearchOpen(true) }}
              readOnly
              aria-label="搜索"
            />
            {narrow && <AntButton type="text" aria-label="搜索页面" onClick={() => setSearchOpen(true)}>⌕</AntButton>}
            <Badge count={attentionCount} size="small" overflowCount={99}>
              <AntButton type="text" icon={<BellOutlined />} aria-label="查看待处理任务" onClick={() => navigate('moderation')} />
            </Badge>
            <AccountPanel />
          </div>
        </header>
        <Layout.Content className="preview-content">
          <ContentErrorBoundary>
            <Suspense fallback={<div className="preview-toolbar__hint">正在加载…</div>}>
              {content}
            </Suspense>
          </ContentErrorBoundary>
        </Layout.Content>
      </Layout>
    </Layout>

    <Modal
      open={searchOpen}
      title="搜索"
      onClose={closeSearch}
      width="md"
    >
      <Input
        autoFocus
        value={query}
        prefix={<span>⌕</span>}
        placeholder="搜索订单号、收件人、用户、帖子、商品，或输入页面名称"
        onChange={event => setQuery(event.target.value)}
      />
      <div className="preview-search-results">
        {query.trim().length >= 2 && <div className="preview-search-group">
          <span className="preview-search-group__title">内容搜索结果{hitsLoading ? ' · 搜索中…' : ` · ${hits.length} 条`}</span>
          {hitsError && <p className="preview-form-error">{hitsError}</p>}
          {!hitsError && !hitsLoading && hits.length === 0 && <p>没有匹配的订单、用户、帖子、商品或骑手</p>}
          {hits.map(hit => (
            <button
              key={`${hit.type}-${hit.id}`}
              type="button"
              className="preview-search-hit"
              onClick={() => openSearchHit(hit)}
            >
              <span className={`preview-search-hit__type is-${hit.type}`}>{SEARCH_TYPE_LABELS[hit.type] || hit.type}</span>
              <span className="preview-search-hit__copy">
                <strong>{hit.title}</strong>
                <small>{hit.subtitle}</small>
              </span>
            </button>
          ))}
        </div>}
        <div className="preview-search-group">
          <span className="preview-search-group__title">页面模块</span>
          {filteredNavigation.map(item => (
            <AntButton
              key={item.key}
              type="text"
              block
              onClick={() => navigate(item.key)}
            >
              {item.icon} {item.label}
            </AntButton>
          ))}
          {filteredNavigation.length === 0 && <p>没有匹配的模块</p>}
        </div>
      </div>
    </Modal>
  </ConfigProvider>
}

export default function App() {
  const authStatus = useAdminStore(state => state.authStatus)
  const setAuthenticated = useAdminStore(state => state.setAuthenticated)
  const setUnauthenticated = useAdminStore(state => state.setUnauthenticated)
  useEffect(() => {
    if (authStatus !== 'checking') return
    let active = true
    adminAuthAPI.session()
      .then(session => { if (active) setAuthenticated(session) })
      .catch(() => { if (active) setUnauthenticated() })
    return () => { active = false }
  }, [authStatus, setAuthenticated, setUnauthenticated])
  if (authStatus === 'checking') return <main className="auth-loading"><div className="auth-loading__mark"><img className="brand-logo-img" src="/images/brand-logo.png" alt="湛科万事通" /></div><p>正在验证管理员会话…</p></main>
  if (authStatus === 'unauthenticated') return <Login />
  return <ToastProvider><ConsoleShell /></ToastProvider>
}
