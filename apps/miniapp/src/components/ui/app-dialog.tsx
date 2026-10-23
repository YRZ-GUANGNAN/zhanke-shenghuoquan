import { useEffect, useRef, type ReactNode } from 'react'
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
  const confirmLockRef = useRef(false)
  const loadingRef = useRef(loading)
  const previousLoadingRef = useRef(loading)
  const previousErrorRef = useRef(error)
  useEffect(() => {
    if (!visible || (previousLoadingRef.current && !loading) || previousErrorRef.current !== error) {
      confirmLockRef.current = false
    }
    previousLoadingRef.current = loading
    previousErrorRef.current = error
    loadingRef.current = loading
  }, [error, loading, visible])
  if (!mounted) return null
  const interactive = visible && phase !== 'exit'
  const handleCancel = () => {
    if (interactive && !loading) onCancel()
  }
  const handleConfirm = () => {
    if (interactive && !loading && !confirmLockRef.current) {
      confirmLockRef.current = true
      try {
        onConfirm()
      } catch (error) {
        confirmLockRef.current = false
        throw error
      }
      queueMicrotask(() => {
        if (!loadingRef.current) confirmLockRef.current = false
      })
    }
  }
  return (
    <View className={`app-dialog-layer app-dialog-layer--${phase}`} onClick={(event) => event.stopPropagation()}>
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
