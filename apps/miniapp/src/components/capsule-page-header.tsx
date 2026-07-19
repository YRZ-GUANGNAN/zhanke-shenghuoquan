import type { ReactNode } from 'react'
import { Text, View } from '@tarojs/components'
import { getCapsuleSafeStyle } from '../utils/capsule-safe'
import './capsule-page-header.scss'

export type CapsuleHeaderVariant = 'standard' | 'search' | 'tabs'

interface CapsuleSystemRowProps {
  className?: string
}

interface CapsulePageHeaderProps {
  className?: string
  variant?: CapsuleHeaderVariant
  title?: string
  left?: ReactNode
  center?: ReactNode
  right?: ReactNode
  divider?: boolean
}

export function CapsuleSystemRow({ className = '' }: CapsuleSystemRowProps) {
  return (
    <View
      className={`capsule-system-row ${className}`.trim()}
      style={getCapsuleSafeStyle()}
    />
  )
}

export default function CapsulePageHeader({
  className = '',
  variant = 'standard',
  title,
  left,
  center,
  right,
  divider = true,
}: CapsulePageHeaderProps) {
  return (
    <View
      className={`capsule-page-header ${divider ? 'capsule-page-header--divider' : ''} ${className}`.trim()}
      style={getCapsuleSafeStyle()}
    >
      <View className='capsule-page-header__system-row' />
      <View className={`capsule-page-header__business-row capsule-page-header__business-row--${variant}`}>
        {left ? <View className='capsule-page-header__slot capsule-page-header__slot--left'>{left}</View> : null}
        {title ? <Text className='capsule-page-header__title'>{title}</Text> : null}
        {center ? <View className='capsule-page-header__center'>{center}</View> : null}
        {right ? <View className='capsule-page-header__slot capsule-page-header__slot--right'>{right}</View> : null}
      </View>
    </View>
  )
}
