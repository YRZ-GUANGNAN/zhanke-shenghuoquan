import { useRef } from 'react'
import { Text, View } from '@tarojs/components'
import { useDidShow } from '@tarojs/taro'
import Taro from '../services/taro'
import { useFeatureVisibility } from '../hooks/use-feature-visibility'
import AppIcon from './app-icon'
import './student-bottom-nav.scss'

export type StudentNavKey = 'home' | 'takeout' | 'market' | 'profile'

const navItems = [
  { key: 'home', label: '首页', icon: 'home', url: '/pages/home/index' },
  { key: 'takeout', label: '服务', icon: 'takeaway', url: '/pages/student-takeout/index' },
  { key: 'market', label: '二手', icon: 'market', url: '/pages/student-secondhand/index' },
  { key: 'profile', label: '我的', icon: 'profile', url: '/pages/profile/index' }
] as const

interface StudentBottomNavProps {
  active: StudentNavKey
  className?: string
}

export default function StudentBottomNav({ active, className = '' }: StudentBottomNavProps) {
  const visibility = useFeatureVisibility()
  const navigationLockRef = useRef<string | null>(null)

  useDidShow(() => {
    navigationLockRef.current = null
  })

  const visibleItems = navItems.filter((item) => {
    if (item.key === 'takeout') return visibility.takeout || visibility.campus_tools
    if (item.key === 'market') return visibility.secondhand
    return true
  })
  const canPublish = visibility.publish && (visibility.post || visibility.secondhand_publish)
  const runNavigation = (lockKey: string, navigate: () => Promise<unknown>) => {
    if (navigationLockRef.current) return
    navigationLockRef.current = lockKey
    try {
      void navigate().catch(() => {
        if (navigationLockRef.current === lockKey) navigationLockRef.current = null
      })
    } catch {
      if (navigationLockRef.current === lockKey) navigationLockRef.current = null
    }
  }
  const openPage = (key: StudentNavKey, url: string) => {
    if (key === active) return
    runNavigation(`tab:${key}`, () => Taro.redirectTo({ url }))
  }

  const renderItem = (item: typeof navItems[number]) => (
    <View
      key={item.key}
      className={`student-bottom-nav__item ${item.key === active ? 'student-bottom-nav__item--active' : ''}`}
      onClick={() => openPage(item.key, item.url)}
    >
      <View className='student-bottom-nav__item-visual'>
        <View className='student-bottom-nav__icon-asset'>
          <AppIcon name={item.icon} active={item.key === active} />
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
      {visibleItems.filter((item) => item.key === 'market' || item.key === 'profile').map(renderItem)}
    </View>
  )
}
