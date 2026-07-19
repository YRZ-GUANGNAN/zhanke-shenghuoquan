import type { MenuButtonRect, WindowInfo } from '../services/taro'

export interface CapsuleLayout {
  systemRowHeight: number
  businessRowTop: number
  businessRowHeight: number
  totalHeaderHeight: number
}

export const BUSINESS_ROW_HEIGHT: 48
export function calculateCapsuleLayout(
  windowInfo: WindowInfo,
  menu?: MenuButtonRect,
): CapsuleLayout
