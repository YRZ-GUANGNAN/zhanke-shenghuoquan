import { useCallback, useEffect, useRef, useState } from 'react'
import type { CommonEvent, ITouch, ITouchEvent } from '@tarojs/components'
import { useDidHide, useDidShow, useResize } from '@tarojs/taro'
import Taro from '../services/taro'
import { navigateBackOrHome } from '../utils/navigation'
import { createEdgeBack, type EdgeBackState } from '../utils/edge-back'

const IDLE: EdgeBackState = { offset: 0, tracking: false }
const CLICK_GUARD_MS = 350

function touches(event: CommonEvent): ITouch[] {
  const value = event as ITouchEvent
  return Array.from(value.touches || [])
}

function changedTouches(event: CommonEvent): ITouch[] {
  const value = event as ITouchEvent
  return Array.from(value.changedTouches || [])
}

function hasNativeBack() {
  // iOS owns its interactive page transition. Running navigateBack as well can
  // pop a second page after the system finishes its gesture.
  let platform = ''
  try { platform = Taro.getDeviceInfo().platform } catch {
    try { platform = Taro.getSystemInfoSync().platform } catch { return false }
  }
  if (platform !== 'ios') return false
  try { return Taro.getCurrentPages().length > 1 } catch { return true }
}

function viewportWidth() {
  try {
    const width = Taro.getWindowInfo().windowWidth
    return Number.isFinite(width) && width > 0 ? width : 390
  } catch {
    return 390
  }
}

export function useEdgeBack(enabled: boolean) {
  const tracker = useRef<ReturnType<typeof createEdgeBack> | null>(null)
  if (!tracker.current) tracker.current = createEdgeBack()
  const [state, setState] = useState<EdgeBackState>(IDLE)
  const [nativeBack, setNativeBack] = useState(hasNativeBack)
  const foreground = useRef(true)
  const mounted = useRef(true)
  const returning = useRef(false)
  const allowed = useRef(false)
  allowed.current = enabled && !nativeBack
  const touchOrigin = useRef<ITouch | null>(null)
  const moved = useRef(false)
  const clickBlockedUntil = useRef(0)

  const clear = useCallback(() => {
    if (moved.current) clickBlockedUntil.current = Date.now() + CLICK_GUARD_MS
    moved.current = false
    touchOrigin.current = null
    return tracker.current!.cancel()
  }, [])
  const reset = useCallback(() => {
    const next = clear()
    if (mounted.current) setState(next)
  }, [clear])

  useEffect(() => {
    mounted.current = true
    foreground.current = true
    return () => { mounted.current = false; foreground.current = false; clear() }
  }, [clear])
  useEffect(() => { if (!enabled) reset() }, [enabled, reset])
  useDidHide(() => { foreground.current = false; reset() })
  useDidShow(() => {
    foreground.current = true
    setNativeBack(hasNativeBack())
    reset()
  })
  useResize(reset)

  const canTrack = () => allowed.current && foreground.current && mounted.current && !returning.current

  return {
    available: enabled && !nativeBack,
    offset: enabled && !nativeBack ? state.offset : 0,
    tracking: enabled && !nativeBack && state.tracking,
    canActivate: () => enabled && !moved.current && !returning.current && Date.now() >= clickBlockedUntil.current,
    handlers: {
      onTouchStart: (event: CommonEvent) => {
        reset()
        if (!canTrack()) return
        const points = touches(event)
        touchOrigin.current = points.length === 1 ? points[0] : null
        setState(tracker.current!.start(points))
      },
      onTouchMove: (event: CommonEvent) => {
        if (!canTrack()) { reset(); return }
        const points = touches(event)
        const origin = touchOrigin.current
        if (origin && (points.length !== 1 || points[0].identifier !== origin.identifier
          || Math.max(Math.abs(points[0].clientX - origin.clientX), Math.abs(points[0].clientY - origin.clientY)) >= 8)) moved.current = true
        setState(tracker.current!.move(points))
      },
      onTouchEnd: (event: CommonEvent) => {
        const shouldReturn = canTrack() && tracker.current!.end(changedTouches(event), touches(event), viewportWidth())
        reset()
        if (shouldReturn) {
          returning.current = true
          // The shared navigation utility also protects the native transition
          // and falls back home when this profile is the first shared page.
          const release = () => { returning.current = false }
          void navigateBackOrHome().then(release, release)
        }
      },
      onTouchCancel: reset,
    },
  }
}
