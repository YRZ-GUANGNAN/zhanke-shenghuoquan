import type { ReactNode } from 'react'
import { Text, View } from '@tarojs/components'
import { useLayerPresence } from '../../hooks/use-layer-presence'
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
  showCancel?: boolean
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
  showCancel = true,
  onConfirm,
  onCancel,
}: AppDialogProps) {
  const { mounted, phase } = useLayerPresence(visible)
  if (!mounted) return null
  const interactive = phase !== 'exit'
  const handleCancel = () => {
    if (interactive && !loading) onCancel()
  }
  const handleConfirm = () => {
    if (interactive) onConfirm()
  }
  return (
    <View className={`app-dialog-layer app-dialog-layer--${phase}`}>
      <View className='app-dialog-layer__mask' onClick={handleCancel} />
      <View className={`app-dialog app-dialog--${phase}`} role='dialog' aria-modal>
        <Text className='app-dialog__title'>{title}</Text>
        {description ? <Text className='app-dialog__description'>{description}</Text> : null}
        {children ? <View className='app-dialog__content'>{children}</View> : null}
        {error ? <Text className='app-dialog__error'>{error}</Text> : null}
        <View className={`app-dialog__actions ${showCancel ? '' : 'app-dialog__actions--single'}`}>
          {showCancel ? <AppButton variant='secondary' disabled={loading || !interactive} onClick={handleCancel}>{cancelText}</AppButton> : null}
          <AppButton variant={tone === 'danger' ? 'danger' : 'primary'} loading={loading} disabled={!interactive} onClick={handleConfirm}>{confirmText}</AppButton>
        </View>
      </View>
    </View>
  )
}
