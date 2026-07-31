import { Text, View } from '@tarojs/components'
import { AppButton } from './app-button'
import './app-state.scss'

export type AppStateKind = 'loading' | 'empty' | 'error' | 'offline' | 'forbidden'

export interface AppStateProps {
  kind: AppStateKind
  title: string
  description?: string
  actionText?: string
  onAction?: () => void
  compact?: boolean
}

export function AppState({ kind, title, description, actionText, onAction, compact = false }: AppStateProps) {
  return (
    <View className={`app-state app-state--${kind} ${compact ? 'app-state--compact' : ''}`}>
      <View className='app-state__symbol'><View className='app-state__symbol-core' /></View>
      <Text className='app-state__title'>{title}</Text>
      {description ? <Text className='app-state__description'>{description}</Text> : null}
      {actionText && onAction ? <AppButton variant='secondary' size='small' onClick={onAction}>{actionText}</AppButton> : null}
    </View>
  )
}
