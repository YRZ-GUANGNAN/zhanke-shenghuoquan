import { cloneElement, isValidElement, type ReactNode } from 'react'
import { Text, View } from '@tarojs/components'
import { useViewportRefresh } from '../hooks/use-viewport-refresh'
import './capsule-page-header.scss'

declare const TARO_APP_VISUAL_DEMO: boolean
declare const require: (path: string) => any
const DemoIndicator = typeof TARO_APP_VISUAL_DEMO !== 'undefined' && TARO_APP_VISUAL_DEMO
  ? require('../demo/indicator').default : null

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

function addSlotContentClass(content: ReactNode) {
  if (!isValidElement<{ className?: string }>(content)) return content
  const className = `${content.props.className ?? ''} capsule-page-header__slot-content`.trim()
  return cloneElement(content, { className })
}

export function CapsuleSystemRow({ className = '' }: CapsuleSystemRowProps) {
  const safeStyle = useViewportRefresh()
  return (
    <View
      className={`capsule-system-row ${className}`.trim()}
      style={safeStyle}
    >{DemoIndicator ? <DemoIndicator /> : null}</View>
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
  const safeStyle = useViewportRefresh()
  return (
    <View
      className={`capsule-page-header ${divider ? 'capsule-page-header--divider' : ''} ${className}`.trim()}
      style={safeStyle}
    >
      <View className='capsule-page-header__system-row'>{DemoIndicator ? <DemoIndicator /> : null}</View>
      <View className={`capsule-page-header__business-row capsule-page-header__business-row--${variant}`}>
        {left ? <View className='capsule-page-header__slot capsule-page-header__slot--left'>{addSlotContentClass(left)}</View> : null}
        {title ? <Text className='capsule-page-header__title'>{title}</Text> : null}
        {center ? <View className='capsule-page-header__center'>{center}</View> : null}
        {right ? <View className='capsule-page-header__slot capsule-page-header__slot--right'>{addSlotContentClass(right)}</View> : null}
      </View>
    </View>
  )
}
