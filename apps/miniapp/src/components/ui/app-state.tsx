import { Text, View } from '@tarojs/components'
import { AppButton } from './app-button'
import { StatusGlyph } from './status-glyph'
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
}

export function AppState({ kind, title, description, actionText, onAction, compact = false, scene }: AppStateProps) {
  return (
    <View className={`app-state app-state--${kind} ${compact ? 'app-state--compact' : ''}`}>
      <StatusGlyph kind={kind === 'loading' && scene === 'login' ? 'login' : kind} />
      <Text className='app-state__title'>{title}</Text>
      {description ? <Text className='app-state__description'>{description}</Text> : null}
      {actionText && onAction ? <AppButton variant='secondary' onClick={onAction}>{actionText}</AppButton> : null}
    </View>
  )
}
