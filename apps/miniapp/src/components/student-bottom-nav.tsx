import React from 'react'
import { Text, View } from '@tarojs/components'
import Taro from '../services/taro'

export type StudentNavKey = 'home' | 'takeout' | 'market' | 'profile'

const navItems = [
  { key: 'home', label: '首页', icon: 'home', url: '/pages/home/index' },
  { key: 'takeout', label: '外卖', icon: 'takeaway', url: '/pages/student-takeout/index' },
  { key: 'market', label: '二手', icon: 'market', url: '/pages/student-secondhand/index' },
  { key: 'profile', label: '我的', icon: 'profile', url: '/pages/profile/index' }
] as const

interface StudentBottomNavProps {
  active: StudentNavKey
  className?: string
}

export default function StudentBottomNav({ active, className = '' }: StudentBottomNavProps) {
  const openPage = (key: StudentNavKey, url: string) => {
    if (key === active) return
    void Taro.redirectTo({ url })
  }

  const renderItem = (item: typeof navItems[number]) => (
    <View
      key={item.key}
      className={`student-bottom-nav__item ${item.key === active ? 'student-bottom-nav__item--active' : ''}`}
      onClick={() => openPage(item.key, item.url)}
    >
      <View className={`student-bottom-nav__icon student-bottom-nav__icon--${item.icon}`} />
      <Text>{item.label}</Text>
    </View>
  )

  return (
    <View className={`student-bottom-nav ${className}`.trim()}>
      {navItems.slice(0, 2).map(renderItem)}
      <View
        className='student-bottom-nav__fab'
        onClick={() => void Taro.navigateTo({ url: '/pages/publish-entry/index' })}
      >
        <Text>+</Text>
      </View>
      {navItems.slice(2).map(renderItem)}
    </View>
  )
}
