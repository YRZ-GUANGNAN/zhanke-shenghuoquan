import type { ReactNode } from 'react'
import { Text, View } from '@tarojs/components'
import './app-action-sheet.scss'

export interface AppActionSheetProps {
  visible: boolean
  title: string
  description?: string
  children: ReactNode
  loading?: boolean
  onClose: () => void
}

export function AppActionSheet({ visible, title, description, children, loading = false, onClose }: AppActionSheetProps) {
  if (!visible) return null
  return (
    <View className='app-action-sheet-layer'>
      <View className='app-action-sheet-layer__mask' onClick={() => !loading && onClose()} />
      <View className='app-action-sheet' role='dialog' aria-modal>
        <View className='app-action-sheet__handle' />
        <View className='app-action-sheet__header'>
          <View className='app-action-sheet__heading'>
            <Text className='app-action-sheet__title'>{title}</Text>
            {description ? <Text className='app-action-sheet__description'>{description}</Text> : null}
          </View>
          <View className={`app-action-sheet__close ${loading ? 'app-action-sheet__close--disabled' : ''}`} role='button' onClick={() => !loading && onClose()}>
            <Text>×</Text>
          </View>
        </View>
        <View className='app-action-sheet__content'>{children}</View>
      </View>
    </View>
  )
}
