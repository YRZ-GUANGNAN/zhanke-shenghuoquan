import React, { useRef } from 'react'
import { Text, View } from '@tarojs/components'
import { useDidHide, useDidShow } from '@tarojs/taro'
import { useHorizontalNavigation } from '../hooks/use-horizontal-navigation'
import AppIcon, { type AppIconName } from './app-icon'
import Taro from '../services/taro'
import { useFeatureVisibility } from '../hooks/use-feature-visibility'
import './delivery-workspace-nav.scss'

type NavKey = 'orders' | 'publish' | 'messages' | 'mine' | 'grab' | 'tasks'
type Role = 'merchant' | 'rider'
interface Props { role: Role; active: NavKey; onSelect?: (key: string) => void }
interface WorkspaceNavProps extends Props {
  /** 页面根手势的点击保护，避免横滑结束后误触导航项。 */
  gestureGuard?: () => boolean
}
interface Item { key: NavKey; label: string; icon: AppIconName; url: string; swipe?: boolean }

export function deliveryWorkspaceItems(role: Role): Item[] {
  return role === 'merchant' ? [
    { key: 'orders', label: '订单', icon: 'order', url: '/pages/merchant-home/index' },
    // 发单是独立操作页，不进入工作台横向频道，避免把表单夹在订单和消息之间。
    { key: 'publish', label: '发单', icon: 'document', url: '/pages/merchant-publish/index', swipe: false },
    { key: 'messages', label: '消息', icon: 'messages', url: '/pages/merchant-messages/index' },
    { key: 'mine', label: '我的', icon: 'profile', url: '/pages/delivery-profile/index?role=merchant' },
  ] : [
    { key: 'grab', label: '接单', icon: 'takeaway', url: '/pages/delivery-rider/index' },
    { key: 'tasks', label: '配送', icon: 'order', url: '/pages/delivery-rider/index?tab=tasks' },
    { key: 'messages', label: '消息', icon: 'messages', url: '/pages/rider-messages/index' },
    { key: 'mine', label: '我的', icon: 'profile', url: '/pages/delivery-profile/index?role=rider' },
  ]
}

export function useDeliveryWorkspaceNavigation({ role, active, onSelect }: Props, enabled = true) {
  const visibility = useFeatureVisibility()
  const lock = useRef(false)
  const visible = useRef(true)
  useDidHide(() => { visible.current = false; lock.current = false })
  useDidShow(() => { visible.current = true })
  useDidShow(() => { lock.current = false })
  const items = deliveryWorkspaceItems(role).filter(item => {
    if (item.key === 'publish') return visibility.delivery && visibility.merchant
    if (item.key === 'grab') return visibility.delivery && visibility.rider
    return true // Existing orders and delivery conversations remain accessible.
  })
  const select = (item: Item) => {
    if (!enabled || !visible.current || item.key === active || lock.current) return
    if (onSelect && (item.key === 'grab' || item.key === 'tasks')) { onSelect(item.key); return }
    lock.current = true
    // Publishing preserves the originating workspace so the back button keeps working.
    const navigation = item.key === 'publish' ? Taro.navigateTo({ url: item.url }) : Taro.redirectTo({ url: item.url })
    void navigation.catch(() => {
      lock.current = false
      void Taro.showToast({ title: '页面打开失败，请重试', icon: 'none' }).catch(() => undefined)
    })
  }
  const swipeItems = items.filter(item => item.swipe !== false)
  const swipe = useHorizontalNavigation(direction => {
    const index = swipeItems.findIndex(item => item.key === active)
    if (index < 0) return
    const target = swipeItems[index + direction]
    if (target) select(target)
  }, enabled && swipeItems.length > 1)
  return { items, select, swipe }
}

export default function DeliveryWorkspaceNav(props: WorkspaceNavProps) {
  const { items, select, swipe } = useDeliveryWorkspaceNavigation(props)
  const { active, gestureGuard } = props
  const canActivate = gestureGuard ?? swipe.canActivate
  return <View className='delivery-workspace-nav'>
    {items.map(item => <View key={item.key} className={`delivery-workspace-nav__item ${active === item.key ? 'delivery-workspace-nav__item--active' : ''}`}
      ariaRole='button' ariaLabel={`${item.label}${active === item.key ? '，当前页面' : ''}`}
      hoverClass='delivery-workspace-nav__item--pressed' onClick={() => { if (canActivate()) select(item) }}>
      <AppIcon name={item.icon} active={active === item.key} className='delivery-workspace-nav__icon' />
      <Text>{item.label}</Text>
    </View>)}
  </View>
}
