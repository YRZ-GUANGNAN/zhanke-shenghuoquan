import { useCallback, useEffect, useRef, useState } from 'react'
import type { CommonEvent, ITouchEvent } from '@tarojs/components'
import { useDidHide, useDidShow } from '@tarojs/taro'
import { createSheetDrag, type SheetDragState } from '../utils/sheet-drag'

export function useSheetDismiss(enabled: boolean, onDismiss: () => void) {
  const tracker = useRef<ReturnType<typeof createSheetDrag> | null>(null)
  if (!tracker.current) tracker.current = createSheetDrag()
  const [drag, setDrag] = useState<SheetDragState>({ offset: 0, dragging: false })
  const foreground = useRef(true)
  const mounted = useRef(true)
  const allowed = useRef(enabled)
  allowed.current = enabled
  const update = useCallback((next: SheetDragState) => {
    if (mounted.current) setDrag(current => current.offset === next.offset && current.dragging === next.dragging ? current : next)
  }, [])
  const cancel = useCallback(() => update(tracker.current!.cancel()), [update])
  useEffect(() => {
    mounted.current = true
    return () => { mounted.current = false; tracker.current!.cancel() }
  }, [])
  useEffect(cancel, [enabled, cancel])
  useDidHide(() => { foreground.current = false; cancel() })
  useDidShow(() => { foreground.current = true; cancel() })

  const canDrag = () => mounted.current && foreground.current && allowed.current
  return {
    offset: enabled ? drag.offset : 0,
    dragging: enabled && drag.dragging,
    handlers: {
      onTouchStart: (event: CommonEvent) => {
        const touchEvent = event as ITouchEvent
        touchEvent.stopPropagation()
        update(canDrag() ? tracker.current!.start(touchEvent.touches) : tracker.current!.cancel())
      },
      onTouchMove: (event: CommonEvent) => {
        const touchEvent = event as ITouchEvent
        touchEvent.stopPropagation()
        update(canDrag() ? tracker.current!.move(touchEvent.touches) : tracker.current!.cancel())
      },
      onTouchEnd: (event: CommonEvent) => {
        const touchEvent = event as ITouchEvent
        touchEvent.stopPropagation()
        const dismiss = canDrag() && tracker.current!.end(touchEvent.changedTouches, touchEvent.touches)
        cancel()
        if (dismiss) onDismiss()
      },
      onTouchCancel: cancel,
    },
  }
}
