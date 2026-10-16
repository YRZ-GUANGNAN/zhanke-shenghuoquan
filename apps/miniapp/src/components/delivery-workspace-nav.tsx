import React, { useRef } from 'react'
import { Text, View } from '@tarojs/components'
import { useDidShow } from '@tarojs/taro'
import AppIcon, { type AppIconName } from './app-icon'
import Taro from '../services/taro'
import { useFeatureVisibility } from '../hooks/use-feature-visibility'
import './delivery-workspace-nav.scss'

type NavKey = 'orders' | 'publish' | 'messages' | 'mine' | 'grab' | 'tasks'
type Role = 'merchant' | 'rider'
interface Props { role: Role; active: NavKey; onSelect?: (key: string) => void }
interface Item { key: NavKey; label: string; icon: AppIconName; url: string }

export function deliveryWorkspaceItems(role: Role): Item[] {
  return role === 'merchant' ? [
    { key: 'orders', label: '订单', icon: 'order', url: '/pages/merchant-home/index' },
    { key: 'publish', label: '发单', icon: 'document', url: '/pages/merchant-publish/index' },
    { key: 'messages', label: '消息', icon: 'messages', url: '/pages/merchant-messages/index' },
    { key: 'mine', label: '我的', icon: 'profile', url: '/pages/delivery-profile/index?role=merchant' },
  ] : [
    { key: 'grab', label: '接单', icon: 'takeaway', url: '/pages/delivery-rider/index' },
    { key: 'tasks', label: '配送', icon: 'order', url: '/pages/delivery-rider/index?tab=tasks' },
    { key: 'messages', label: '消息', icon: 'messages', url: '/pages/rider-messages/index' },
    { key: 'mine', label: '我的', icon: 'profile', url: '/pages/delivery-profile/index?role=rider' },
  ]
}

export default function DeliveryWorkspaceNav({ role, active, onSelect }: Props) {
  const visibility = useFeatureVisibility()
  const lock = useRef(false)
  useDidShow(() => { lock.current = false })
  const items = deliveryWorkspaceItems(role).filter(item => {
    if (item.key === 'publish') return visibility.delivery && visibility.merchant
    if (item.key === 'grab') return visibility.delivery && visibility.rider
    return true // Existing orders and delivery conversations remain accessible.
  })
  const select = (item: Item) => {
    if (item.key === active || lock.current) return
    if (onSelect && (item.key === 'grab' || item.key === 'tasks')) { onSelect(item.key); return }
    lock.current = true
    // Publishing preserves the originating workspace so the back button keeps working.
    const navigation = item.key === 'publish' ? Taro.navigateTo({ url: item.url }) : Taro.redirectTo({ url: item.url })
    void navigation.catch(() => {
      lock.current = false
      void Taro.showToast({ title: '页面打开失败，请重试', icon: 'none' }).catch(() => undefined)
    })
  }
  return <View className='delivery-workspace-nav'>
    {items.map(item => <View key={item.key} className={`delivery-workspace-nav__item ${active === item.key ? 'delivery-workspace-nav__item--active' : ''}`}
      ariaRole='button' ariaLabel={`${item.label}${active === item.key ? '，当前页面' : ''}`}
      hoverClass='delivery-workspace-nav__item--pressed' onClick={() => select(item)}>
      <AppIcon name={item.icon} active={active === item.key} className='delivery-workspace-nav__icon' />
      <Text>{item.label}</Text>
    </View>)}
  </View>
}
