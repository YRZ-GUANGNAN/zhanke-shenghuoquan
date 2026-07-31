import type { ReactNode } from 'react'
import { Text, View } from '@tarojs/components'
import './app-notice.scss'

export interface AppNoticeProps {
  tone?: 'info' | 'success' | 'warning' | 'danger'
  title?: string
  children: ReactNode
}

export function AppNotice({ tone = 'info', title, children }: AppNoticeProps) {
  return (
    <View className={`app-notice app-notice--${tone}`}>
      <View className='app-notice__mark' />
      <View className='app-notice__body'>
        {title ? <Text className='app-notice__title'>{title}</Text> : null}
        <Text className='app-notice__content'>{children}</Text>
      </View>
    </View>
  )
}
