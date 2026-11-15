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
import { useCallback, useEffect, useMemo, useState } from 'react'
import type { PreviewSection } from './data'
import { Modal, ToastProvider, usePreviewToast } from './ui'
import { DashboardView } from './views/DashboardView'
import { DeliveryView } from './views/DeliveryView'
import { NotificationsView } from './views/NotificationsView'
import { OperationsView } from './views/OperationsView'
import { PeopleView } from './views/PeopleView'
import { SettingsView } from './views/SettingsView'
import { LegacyTakeoutView } from './views/LegacyTakeoutView'
import { Login } from '../Login'
import { AccountPanel } from '../AccountPanel'
import { useAdminStore } from '../store'
import { adminAuthAPI } from '../services/admin-auth'
import { adminAPI } from '../services/f-admin'
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

  const navigate = (next: PreviewSection) => {
    setSection(next)
    setSearchOpen(false)
    setQuery('')
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
            <Input
              className="preview-topbar__search"
              prefix={<span>⌕</span>}
              placeholder="搜索页面"
              onFocus={() => setSearchOpen(true)}
              readOnly
              aria-label="搜索页面"
            />
            {narrow && <AntButton type="text" aria-label="搜索页面" onClick={() => setSearchOpen(true)}>⌕</AntButton>}
            <Badge count={attentionCount} size="small" overflowCount={99}>
              <AntButton type="text" icon={<BellOutlined />} aria-label="查看待处理任务" onClick={() => navigate('moderation')} />
            </Badge>
            <AccountPanel />
          </div>
        </header>
        <Layout.Content className="preview-content">
          {content}
        </Layout.Content>
      </Layout>
    </Layout>

    <Modal
      open={searchOpen}
      title="搜索页面"
      onClose={() => setSearchOpen(false)}
      width="sm"
    >
      <Input
        autoFocus
        value={query}
        prefix={<span>⌕</span>}
        placeholder="输入模块名称"
        onChange={event => setQuery(event.target.value)}
      />
      <div className="preview-search-results">
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
