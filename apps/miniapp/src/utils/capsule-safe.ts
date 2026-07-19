import type { CSSProperties } from 'react'
import Taro, { type MenuButtonRect, type WindowInfo } from '../services/taro'

export interface CapsuleSafeLayout {
  top: number
  bottom: number
  right: number
}

export type CapsuleSafeStyle = CSSProperties & {
  '--capsule-safe-top': string
  '--capsule-safe-bottom': string
  '--capsule-safe-right': string
}

const FALLBACK_RIGHT = 96

export function calculateCapsuleSafeLayout(
  windowInfo: WindowInfo,
  menu?: MenuButtonRect,
): CapsuleSafeLayout {
  const statusBarTop = Math.max(20, windowInfo.statusBarHeight || 20)
  if (!menu || menu.width <= 0 || menu.height <= 0 || menu.left <= 0) {
    return {
      top: statusBarTop,
      bottom: statusBarTop + 52,
      right: FALLBACK_RIGHT,
    }
  }

  const outerGap = Math.max(8, windowInfo.windowWidth - menu.right)
  return {
    top: Math.max(statusBarTop, menu.top),
    bottom: Math.max(menu.bottom + 8, statusBarTop + 52),
    right: Math.max(FALLBACK_RIGHT, windowInfo.windowWidth - menu.left + outerGap),
  }
}

export function getCapsuleSafeStyle(): CapsuleSafeStyle {
  let layout: CapsuleSafeLayout
  try {
    layout = calculateCapsuleSafeLayout(
      Taro.getWindowInfo(),
      Taro.getMenuButtonBoundingClientRect(),
    )
  } catch {
    layout = { top: 20, bottom: 72, right: FALLBACK_RIGHT }
  }

  return {
    '--capsule-safe-top': `${layout.top}px`,
    '--capsule-safe-bottom': `${layout.bottom}px`,
    '--capsule-safe-right': `${layout.right}px`,
  }
}
