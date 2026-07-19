import type { MenuButtonRect, WindowInfo } from '../../services/taro'

export interface HomeHeaderLayout {
  top: number
  minHeight: number
  right: number
}

export function calculateHomeHeaderLayout(windowInfo: WindowInfo, menu?: MenuButtonRect): HomeHeaderLayout {
  const fallbackTop = Math.max(20, windowInfo.statusBarHeight || 20)
  if (!menu || menu.width <= 0 || menu.height <= 0 || menu.left <= 0) {
    return { top: fallbackTop, minHeight: fallbackTop + 44, right: 96 }
  }
  const capsuleGap = Math.max(8, windowInfo.windowWidth - menu.right)
  return {
    top: Math.max(fallbackTop, menu.top),
    minHeight: Math.max(menu.bottom + 8, fallbackTop + 44),
    right: Math.max(96, windowInfo.windowWidth - menu.left + capsuleGap)
  }
}
