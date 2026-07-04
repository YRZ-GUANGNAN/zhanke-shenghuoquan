import React from 'react'
import { Image, Text, View } from '@tarojs/components'
import Taro from './services/taro'
import type { TabKey } from './types'

const tabs: Array<{ key: TabKey; label: string; icon: string; url: string; primary?: boolean }> = [
  { key: 'home', label: '首页', icon: '⌂', url: '/pages/home/index' },
  { key: 'food', label: '外卖', icon: '▱', url: '/pages/food/index' },
  { key: 'publish', label: '发布', icon: '+', url: '/pages/publish/index', primary: true },
  { key: 'market', label: '二手', icon: '▣', url: '/pages/market/index' },
  { key: 'profile', label: '我的', icon: '○', url: '/pages/profile/index' }
]

export function usePageRefresh(title = '已刷新') {
  Taro.usePullDownRefresh(() => {
    Taro.showToast({ title, icon: 'success', duration: 700 })
    setTimeout(() => {
      Taro.stopPullDownRefresh()
    }, 360)
  })
}

export function PageShell({
  children,
  active,
  plain = false,
  refreshTitle
}: {
  children: React.ReactNode
  active?: TabKey
  plain?: boolean
  refreshTitle?: string
}) {
  usePageRefresh(refreshTitle)

  return (
    <View className={`zk-page ${plain ? 'zk-page--plain' : ''}`}>
      {children}
      {active && <BottomNav active={active} />}
    </View>
  )
}

export function PageHeader({
  title,
  subtitle,
  right,
  back = true
}: {
  title: string
  subtitle?: string
  right?: React.ReactNode
  back?: boolean
}) {
  return (
    <View className='zk-header'>
      <Text className='zk-header__back' onClick={() => back && Taro.navigateBack()}>
        {back ? '‹' : ''}
      </Text>
      <View className='zk-header__main'>
        <Text className='zk-header__title'>{title}</Text>
        {subtitle && <Text className='zk-header__subtitle'>{subtitle}</Text>}
      </View>
      <View className='zk-header__right'>{right}</View>
    </View>
  )
}

export function BottomNav({ active }: { active: TabKey }) {
  return (
    <View className='bottom-nav'>
      {tabs.map((tab) => (
        <View
          key={tab.key}
          className={[
            'bottom-nav__item',
            active === tab.key ? 'bottom-nav__item--active' : '',
            tab.primary ? 'bottom-nav__item--primary' : ''
          ].join(' ')}
          onClick={() => active !== tab.key && Taro.redirectTo({ url: tab.url })}
        >
          <View className='bottom-nav__icon'>{tab.icon}</View>
          <Text>{tab.label}</Text>
        </View>
      ))}
    </View>
  )
}

export function AvatarMark({
  label,
  size = 'normal',
  src
}: {
  label: string
  size?: 'small' | 'normal' | 'large'
  src?: string
}) {
  const initial = label.trim().slice(0, 1) || '湛'
  const className = `cc-avatar cc-avatar--${size}`

  if (src) {
    return <Image className={className} src={src} mode='aspectFill' />
  }

  return (
    <View className={className}>
      <Text>{initial}</Text>
    </View>
  )
}

export function ImageTile({ src, tone = 0, label = '图' }: { src?: string; tone?: number; label?: string }) {
  if (src) return <Image className='zk-image-tile' src={src} mode='aspectFill' />
  return (
    <View className={`zk-image-tile zk-image-tile--tone-${tone % 5}`}>
      <Text>{label}</Text>
    </View>
  )
}

export function EmptyState({ title, hint }: { title: string; hint?: string }) {
  return (
    <View className='zk-empty'>
      <Text className='zk-empty__title'>{title}</Text>
      {hint && <Text className='zk-empty__hint'>{hint}</Text>}
    </View>
  )
}

export function SectionTitle({ title, sub }: { title: string; sub?: string }) {
  return (
    <View className='zk-section-title'>
      <Text className='zk-section-title__main'>{title}</Text>
      {sub && <Text className='zk-section-title__sub'>{sub}</Text>}
    </View>
  )
}

export function SegmentedTabs<T extends string>({
  items,
  active,
  onChange
}: {
  items: Array<{ key: T; label: string; count?: number }>
  active: T
  onChange: (key: T) => void
}) {
  return (
    <View className='zk-tabs'>
      {items.map((item) => (
        <Text key={item.key} className={active === item.key ? 'zk-tabs__active' : ''} onClick={() => onChange(item.key)}>
          {item.label}
          {typeof item.count === 'number' ? ` ${item.count}` : ''}
        </Text>
      ))}
    </View>
  )
}
