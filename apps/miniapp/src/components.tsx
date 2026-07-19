import React from 'react'
import { Image, View, Text } from '@tarojs/components'
import Taro from './services/taro'
import type { TabKey } from './types'

const tabs: Array<{ key: TabKey; label: string; icon: string; url: string }> = [
  { key: 'home', label: '首页', icon: 'S', url: '/pages/home/index' },
  { key: 'takeout', label: '外卖', icon: 'F', url: '/pages/student-takeout/index' },
  { key: 'market', label: '二手', icon: 'M', url: '/pages/student-secondhand/index' },
  { key: 'profile', label: '我的', icon: 'U', url: '/pages/profile/index' }
]

export function BottomNav({ active }: { active: TabKey }) {
  return (
    <View className='bottom-nav'>
      {tabs.map((tab) => (
        <View
          key={tab.key}
          className={`bottom-nav__item ${active === tab.key ? 'bottom-nav__item--active' : ''}`}
          onClick={() => active !== tab.key && Taro.redirectTo({ url: tab.url })}
        >
          <View className='bottom-nav__icon'>{tab.icon}</View>
          <Text>{tab.label}</Text>
        </View>
      ))}
    </View>
  )
}

export function AvatarMark({ label, size = 'normal', src }: { label: string; size?: 'small' | 'normal' | 'large'; src?: string }) {
  const initial = label.trim().slice(0, 1) || '广'
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
