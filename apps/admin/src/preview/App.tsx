import {
  Badge,
  Button as AntButton,
  ConfigProvider,
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
  ProfileOutlined,
  SettingOutlined,
  ShareAltOutlined,
  ShopOutlined,
  ShoppingOutlined,
  UserOutlined,
  WarningOutlined
} from '@ant-design/icons'
import { useEffect, useMemo, useState } from 'react'
import type { PreviewSection } from './data'
import { usePreviewStore } from './store'
import { Modal, ToastProvider, usePreviewToast } from './ui'
import { DashboardView } from './views/DashboardView'
import { DeliveryView } from './views/DeliveryView'
import { NotificationsView } from './views/NotificationsView'
import { OperationsView } from './views/OperationsView'
import { PeopleView } from './views/PeopleView'
import { SettingsView } from './views/SettingsView'
import { LegacyTakeoutView } from './views/LegacyTakeoutView'

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

function PreviewApp() {
  const section = usePreviewStore(state => state.section)
  const setSection = usePreviewStore(state => state.setSection)
  const deliveries = usePreviewStore(state => state.deliveries)
  const reviews = usePreviewStore(state => state.reviews)
  const features = usePreviewStore(state => state.settings.features)
  const resetPreview = usePreviewStore(state => state.resetPreview)
  const [collapsed, setCollapsed] = useState(false)
  const [searchOpen, setSearchOpen] = useState(false)
  const [resetOpen, setResetOpen] = useState(false)
  const [query, setQuery] = useState('')
  const { notify } = usePreviewToast()

  const availableNavigation = useMemo(() => navigation.filter(item => {
    if (item.key === 'delivery') return features.delivery
    if (item.key === 'notifications') return features.notifications
    if (item.key === 'community') return features.community
    if (item.key === 'trade') return features.community
    if (item.key === 'takeout') return features.takeout
    return true
  }), [features])

  const active = navigation.find(item => item.key === section)
  const attentionCount = deliveries.filter(item => item.status === '失败').length
    + reviews.filter(item => item.status === '待审核' && item.risk === '高风险').length

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

  return <ConfigProvider componentSize="middle">
    <Layout className="preview-shell">
      <Layout.Sider
        className="preview-sider"
        width={216}
        collapsedWidth={64}
        collapsed={collapsed}
        theme="light"
        collapsible
        trigger={null}
      >
        <div className="preview-logo">
          <span className="preview-logo__mark">湛</span>
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
      </Layout.Sider>
      <Layout className="preview-main-shell">
        <header className="preview-topbar">
          <div className="preview-topbar__left">
            <AntButton
              type="text"
              icon={collapsed ? <span>☰</span> : <span>◧</span>}
              onClick={() => setCollapsed(!collapsed)}
              aria-label="切换导航"
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
            />
            <Badge count={attentionCount} size="small">
              <AntButton type="text" icon={<BellOutlined />} onClick={() => navigate('notifications')} />
            </Badge>
            <Tag color="blue">本地预览</Tag>
            <AntButton type="link" onClick={() => setResetOpen(true)}>重置数据</AntButton>
            <span className="preview-topbar__avatar">运</span>
          </div>
        </header>
        <Layout.Content className="preview-content">
          <div className="preview-mock-banner" role="note">
            <span>演示数据</span>以下数字为本地模拟数据，仅用于界面评审，不代表真实经营情况。
          </div>
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

    <Modal
      open={resetOpen}
      title="恢复本地演示数据"
      onClose={() => setResetOpen(false)}
      width="sm"
      footer={<>
        <AntButton onClick={() => setResetOpen(false)}>取消</AntButton>
        <AntButton type="primary" onClick={() => {
          resetPreview()
          setResetOpen(false)
          notify('本地预览数据已恢复', 'info')
        }}>确认恢复</AntButton>
      </>}
    >
      <p>将清除当前浏览器中新增和修改的本地预览数据。</p>
    </Modal>
  </ConfigProvider>
}

export default function App() {
  return <ToastProvider><PreviewApp /></ToastProvider>
}
