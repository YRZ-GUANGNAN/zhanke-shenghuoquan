import { View } from '@tarojs/components'
import AppIcon, { type AppIconName } from '../app-icon'
import './app-icon-button.scss'

export interface AppIconButtonProps {
  name: AppIconName
  label: string
  size?: 'small' | 'regular'
  tone?: 'default' | 'surface' | 'brand'
  disabled?: boolean
  className?: string
  onClick?: () => void
}

export function AppIconButton({
  name,
  label,
  size = 'regular',
  tone = 'default',
  disabled = false,
  className = '',
  onClick,
}: AppIconButtonProps) {
  return (
    <View
      className={`app-icon-button app-icon-button--${size} app-icon-button--${tone} ${disabled ? 'app-icon-button--disabled' : ''} ${className}`.trim()}
      role='button'
      aria-label={label}
      onClick={() => !disabled && onClick?.()}
    >
      <AppIcon name={name} className='app-icon-button__icon' />
    </View>
  )
}
