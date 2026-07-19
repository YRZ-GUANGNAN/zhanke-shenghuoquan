import type { CSSProperties } from 'react'
import Taro from '../services/taro'
import { calculateCapsuleLayout, type CapsuleLayout } from './capsule-layout'

export type CapsuleSafeStyle = CSSProperties & {
  '--capsule-system-row-height': string
  '--capsule-business-row-top': string
  '--capsule-business-row-height': string
  '--capsule-total-header-height': string
}

export function getCapsuleLayout(): CapsuleLayout {
  try {
    return calculateCapsuleLayout(
      Taro.getWindowInfo(),
      Taro.getMenuButtonBoundingClientRect(),
    )
  } catch {
    return calculateCapsuleLayout({
      windowWidth: 375,
      statusBarHeight: 20,
      pixelRatio: 1,
    })
  }
}

export function getCapsuleSafeStyle(): CapsuleSafeStyle {
  const layout = getCapsuleLayout()
  return {
    '--capsule-system-row-height': `${layout.systemRowHeight}px`,
    '--capsule-business-row-top': `${layout.businessRowTop}px`,
    '--capsule-business-row-height': `${layout.businessRowHeight}px`,
    '--capsule-total-header-height': `${layout.totalHeaderHeight}px`,
  }
}
