import { useEffect, useRef, type ReactNode } from 'react'
import { Text, View } from '@tarojs/components'
import { useLayerPresence } from '../../hooks/use-layer-presence'
import { useSheetDismiss } from '../../hooks/use-sheet-dismiss'
import { AppIconButton } from './app-icon-button'
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
  const { mounted, phase } = useLayerPresence(visible)
  const closeRequested = useRef(false)
  useEffect(() => { if (visible) closeRequested.current = false }, [visible])
  const interactive = visible && phase !== 'exit'
  const handleClose = () => {
    if (interactive && !loading && !closeRequested.current) {
      closeRequested.current = true
      onClose()
    }
  }
  const drag = useSheetDismiss(visible && phase === 'open' && !loading, handleClose)
  if (!mounted) return null
  return (
    <View className={`app-action-sheet-layer app-action-sheet-layer--${phase}`} onClick={(event) => event.stopPropagation()}>
      <View className='app-action-sheet-layer__mask' onClick={handleClose} />
      <View
        className={`app-action-sheet app-action-sheet--${phase}${drag.dragging ? ' app-action-sheet--dragging' : ''}`}
        style={drag.dragging ? { transform: `translateY(${drag.offset}px)` } : undefined}
        role='dialog' aria-modal
      >
        <View className='app-action-sheet__drag-handle' {...drag.handlers} catchMove ariaLabel='向下拖动关闭面板'>
          <View className='app-action-sheet__handle' />
        </View>
        <View className='app-action-sheet__header'>
          <View className='app-action-sheet__heading'>
            <Text className='app-action-sheet__title'>{title}</Text>
            {description ? <Text className='app-action-sheet__description'>{description}</Text> : null}
          </View>
          <AppIconButton
            name='close'
            label='关闭'
            size='small'
            tone='surface'
            disabled={loading}
            className='app-action-sheet__close'
            onClick={handleClose}
          />
        </View>
        <View className='app-action-sheet__content'>{children}</View>
      </View>
    </View>
  )
}
