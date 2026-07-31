import type { ReactNode } from 'react'
import { Text, View } from '@tarojs/components'
import { AppButton } from './app-button'
import './app-dialog.scss'

export interface AppDialogProps {
  visible: boolean
  title: string
  description?: string
  children?: ReactNode
  confirmText?: string
  cancelText?: string
  tone?: 'default' | 'danger'
  loading?: boolean
  error?: string
  onConfirm: () => void
  onCancel: () => void
}

export function AppDialog({
  visible,
  title,
  description,
  children,
  confirmText = '确认',
  cancelText = '取消',
  tone = 'default',
  loading = false,
  error,
  onConfirm,
  onCancel,
}: AppDialogProps) {
  if (!visible) return null
  return (
    <View className='app-dialog-layer'>
      <View className='app-dialog-layer__mask' onClick={() => !loading && onCancel()} />
      <View className='app-dialog' role='dialog' aria-modal>
        <Text className='app-dialog__title'>{title}</Text>
        {description ? <Text className='app-dialog__description'>{description}</Text> : null}
        {children ? <View className='app-dialog__content'>{children}</View> : null}
        {error ? <Text className='app-dialog__error'>{error}</Text> : null}
        <View className='app-dialog__actions'>
          <AppButton variant='secondary' disabled={loading} onClick={onCancel}>{cancelText}</AppButton>
          <AppButton variant={tone === 'danger' ? 'danger' : 'primary'} loading={loading} onClick={onConfirm}>{confirmText}</AppButton>
        </View>
      </View>
    </View>
  )
}
