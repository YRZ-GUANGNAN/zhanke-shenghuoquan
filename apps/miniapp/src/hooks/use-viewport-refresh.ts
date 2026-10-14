import { useCallback, useState } from 'react'
import { useDidShow, useResize } from '@tarojs/taro'
import { getCapsuleSafeStyle, type CapsuleSafeStyle } from '../utils/capsule-safe'

function sameStyle(left: CapsuleSafeStyle, right: CapsuleSafeStyle) {
  return left['--capsule-system-row-height'] === right['--capsule-system-row-height']
    && left['--capsule-business-row-top'] === right['--capsule-business-row-top']
    && left['--capsule-business-row-height'] === right['--capsule-business-row-height']
    && left['--capsule-total-header-height'] === right['--capsule-total-header-height']
}

/**
 * Refreshes only the geometry tokens used by page shells when the window or
 * foreground state changes. It does not remount the page or reload business
 * data, so scroll position, drafts and in-flight requests remain untouched.
 */
export function useViewportRefresh(): CapsuleSafeStyle {
  const [style, setStyle] = useState<CapsuleSafeStyle>(() => getCapsuleSafeStyle())
  const refresh = useCallback(() => {
    const next = getCapsuleSafeStyle()
    setStyle((current) => sameStyle(current, next) ? current : next)
  }, [])

  useResize(refresh)
  useDidShow(refresh)
  return style
}
