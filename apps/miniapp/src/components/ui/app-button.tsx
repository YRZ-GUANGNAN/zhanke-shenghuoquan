import type { ReactNode } from 'react'
import { Button, Text } from '@tarojs/components'
import './app-button.scss'

export type AppButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'text'
export type AppButtonSize = 'small' | 'regular'

export interface AppButtonProps {
  children: ReactNode
  variant?: AppButtonVariant
  size?: AppButtonSize
  block?: boolean
  loading?: boolean
  disabled?: boolean
  className?: string
  onClick?: () => void
}

export function AppButton({
  children,
  variant = 'primary',
  size = 'regular',
  block = false,
  loading = false,
  disabled = false,
  className = '',
  onClick,
}: AppButtonProps) {
  const locked = loading || disabled
  return (
    <Button
      className={`app-button app-button--${variant} app-button--${size} ${block ? 'app-button--block' : ''} ${locked ? 'app-button--disabled' : ''} ${className}`.trim()}
      disabled={locked}
      onClick={() => !locked && onClick?.()}
    >
      {loading ? <Text className='app-button__loading-dot' /> : null}
      <Text>{loading ? '处理中…' : children}</Text>
    </Button>
  )
}
