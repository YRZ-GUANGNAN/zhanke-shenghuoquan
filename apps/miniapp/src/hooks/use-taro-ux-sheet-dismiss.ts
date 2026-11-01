import { useCallback, useEffect, useRef, useState } from 'react'
import type { CommonEvent, ITouchEvent } from '@tarojs/components'
import { useDidHide, useDidShow } from '@tarojs/taro'
import { createTaroUxSheetDrag, type TaroUxSheetDragState } from '../utils/taro-ux-sheet-drag'

/** Upstream drag math, attached only to the existing handle with app lifecycle locks. */
export function useTaroUxSheetDismiss(enabled: boolean, onDismiss: () => void) {
  const tracker = useRef<ReturnType<typeof createTaroUxSheetDrag> | null>(null)
  if (!tracker.current) tracker.current = createTaroUxSheetDrag()
  const [drag, setDrag] = useState<TaroUxSheetDragState>({ offset: 0, dragging: false })
  const foreground = useRef(true)
  const mounted = useRef(true)
  const allowed = useRef(enabled)
  const resetTimer = useRef<ReturnType<typeof setTimeout>>()
  allowed.current = enabled
  const update = useCallback((next: TaroUxSheetDragState) => {
    if (mounted.current) setDrag(current => current.offset === next.offset && current.dragging === next.dragging ? current : next)
  }, [])
  const cancel = useCallback(() => {
    clearTimeout(resetTimer.current)
    update(tracker.current!.cancel())
  }, [update])
  useEffect(() => {
    mounted.current = true
    return () => { mounted.current = false; clearTimeout(resetTimer.current); tracker.current!.cancel() }
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
        const touch = event as ITouchEvent
        touch.stopPropagation()
        clearTimeout(resetTimer.current)
        update(canDrag() ? tracker.current!.start(touch.touches) : tracker.current!.cancel())
      },
      onTouchMove: (event: CommonEvent) => {
        const touch = event as ITouchEvent
        touch.stopPropagation()
        update(canDrag() ? tracker.current!.move(touch.touches) : tracker.current!.cancel())
      },
      onTouchEnd: (event: CommonEvent) => {
        const touch = event as ITouchEvent
        touch.stopPropagation()
        const dismiss = canDrag() && tracker.current!.end(touch.changedTouches, touch.touches)
        if (!dismiss) { cancel(); return }
        // Modal.jsx clears its drag offset after the original 200ms transition.
        resetTimer.current = setTimeout(cancel, 200)
        onDismiss()
      },
      onTouchCancel: (event: CommonEvent) => { event.stopPropagation(); cancel() },
    },
  }
}
