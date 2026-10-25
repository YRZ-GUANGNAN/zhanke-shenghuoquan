import { Text, View } from '@tarojs/components'
import { AppButton } from './app-button'
import { StatusGlyph } from './status-glyph'
import ChatAvatar from '../chat-avatar'
import './app-state.scss'

export type AppStateKind = 'loading' | 'empty' | 'error' | 'offline' | 'forbidden'

export interface AppStateProps {
  kind: AppStateKind
  title: string
  description?: string
  actionText?: string
  onAction?: () => void
  compact?: boolean
  scene?: 'login'
  loginProfile?: { name: string; avatar?: string }
}

export function AppState({ kind, title, description, actionText, onAction, compact = false, scene, loginProfile }: AppStateProps) {
  const welcoming = kind === 'loading' && scene === 'login'
  return (
    <View className={`app-state app-state--${kind} ${compact ? 'app-state--compact' : ''} ${welcoming ? 'app-state--welcome' : ''}`}>
      {welcoming ? <View className='app-state__identity'>
        {loginProfile ? <ChatAvatar src={loginProfile.avatar} name={loginProfile.name} className='app-state__portrait' /> : null}
      </View> : <StatusGlyph kind={kind} />}
      <Text className='app-state__title'>{title}</Text>
      {description ? <Text className='app-state__description'>{description}</Text> : null}
      {welcoming ? <View className='app-state__connection' role='status' aria-label='正在登录'>
        <View className='app-state__connection-dot' /><Text>微信登录</Text>
      </View> : null}
      {actionText && onAction ? <AppButton variant='secondary' onClick={onAction}>{actionText}</AppButton> : null}
    </View>
  )
}
