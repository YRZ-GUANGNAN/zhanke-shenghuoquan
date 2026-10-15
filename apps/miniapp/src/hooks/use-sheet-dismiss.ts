import { useEffect, useRef, useState } from 'react'
import type { CommonEvent, ITouchEvent } from '@tarojs/components'
import { createSheetDrag, type SheetDragState } from '../utils/sheet-drag'

export function useSheetDismiss(enabled: boolean, onDismiss: () => void) {
  const tracker = useRef<ReturnType<typeof createSheetDrag> | null>(null)
  if (!tracker.current) tracker.current = createSheetDrag()
  const [drag, setDrag] = useState<SheetDragState>({ offset: 0, dragging: false })
  useEffect(() => {
    setDrag(tracker.current!.cancel())
  }, [enabled])

  const cancel = () => setDrag(tracker.current!.cancel())
  return {
    offset: enabled ? drag.offset : 0,
    dragging: enabled && drag.dragging,
    handlers: {
      onTouchStart: (event: CommonEvent) => {
        const touchEvent = event as ITouchEvent
        touchEvent.stopPropagation()
        setDrag(enabled ? tracker.current!.start(touchEvent.touches) : tracker.current!.cancel())
      },
      onTouchMove: (event: CommonEvent) => {
        const touchEvent = event as ITouchEvent
        touchEvent.stopPropagation()
        setDrag(enabled ? tracker.current!.move(touchEvent.touches) : tracker.current!.cancel())
      },
      onTouchEnd: (event: CommonEvent) => {
        const touchEvent = event as ITouchEvent
        touchEvent.stopPropagation()
        const dismiss = enabled && tracker.current!.end(touchEvent.changedTouches, touchEvent.touches)
        cancel()
        if (dismiss) onDismiss()
      },
      onTouchCancel: cancel,
    },
  }
}
