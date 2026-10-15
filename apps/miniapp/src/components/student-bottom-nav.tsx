import { useRef } from 'react'
import { Text, View } from '@tarojs/components'
import { useDidShow } from '@tarojs/taro'
import Taro from '../services/taro'
import { useFeatureVisibility } from '../hooks/use-feature-visibility'
import AppIcon from './app-icon'
import './student-bottom-nav.scss'

// market remains a compatibility alias for direct links to the old market route.
export type StudentNavKey = 'home' | 'takeout' | 'messages' | 'profile' | 'market'

const navItems = [
  { key: 'home', label: '首页', icon: 'home', url: '/pages/home/index' },
  { key: 'takeout', label: '服务', icon: 'takeaway', url: '/pages/student-takeout/index' },
  { key: 'messages', label: '私信', icon: 'messages', url: '/pages/home/index?tab=messages' },
  { key: 'profile', label: '我的', icon: 'profile', url: '/pages/profile/index' }
] as const

interface StudentBottomNavProps {
  active: StudentNavKey
  className?: string
}

export default function StudentBottomNav({ active, className = '' }: StudentBottomNavProps) {
  const visibility = useFeatureVisibility()
  const visualActive = active === 'market' ? 'home' : active
  const navigationLockRef = useRef<{ key: string } | null>(null)

  useDidShow(() => {
    navigationLockRef.current = null
  })

  const visibleItems = navItems.filter((item) => {
    if (item.key === 'takeout') return visibility.takeout || visibility.campus_tools
    if (item.key === 'messages') return visibility.messages && visibility.secondhand_chat
    return true
  })
  const canPublish = visibility.publish && (visibility.post || visibility.secondhand_publish)
  const runNavigation = (lockKey: string, navigate: () => Promise<unknown>) => {
    if (navigationLockRef.current) return
    const request = { key: lockKey }
    navigationLockRef.current = request
    try {
      void navigate().catch(() => {
        if (navigationLockRef.current === request) navigationLockRef.current = null
      })
    } catch {
      if (navigationLockRef.current === request) navigationLockRef.current = null
    }
  }
  const openPage = (key: StudentNavKey, url: string) => {
    if (key === active) return
    runNavigation(`tab:${key}`, () => Taro.redirectTo({ url }))
  }

  const renderItem = (item: typeof navItems[number]) => (
    <View
      key={item.key}
      className={`student-bottom-nav__item ${item.key === visualActive ? 'student-bottom-nav__item--active' : ''}`}
      onClick={() => openPage(item.key, item.url)}
    >
      <View className='student-bottom-nav__item-visual'>
        <View className='student-bottom-nav__icon-asset'>
          <AppIcon name={item.icon} active={item.key === visualActive} />
        </View>
        <Text>{item.label}</Text>
      </View>
    </View>
  )

  return (
    <View className={`student-bottom-nav ${className}`.trim()} style={{ gridTemplateColumns: `repeat(${visibleItems.length + (canPublish ? 1 : 0)}, 1fr)` }}>
      {visibleItems.filter((item) => item.key === 'home' || item.key === 'takeout').map(renderItem)}
      {canPublish && <View
          className='student-bottom-nav__publish'
          onClick={() => runNavigation('publish', () => Taro.navigateTo({ url: '/pages/publish-entry/index' }))}
        >
          <View className='student-bottom-nav__publish-visual'>
            <Text className='student-bottom-nav__publish-label'>+</Text>
          </View>
        </View>}
      {visibleItems.filter((item) => item.key === 'messages' || item.key === 'profile').map(renderItem)}
    </View>
  )
}
