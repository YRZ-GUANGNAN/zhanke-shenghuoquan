export interface SheetTouch {
  identifier: number
  clientX: number
  clientY: number
}

export interface SheetDragState {
  offset: number
  dragging: boolean
}

const IDLE: SheetDragState = { offset: 0, dragging: false }
const DIRECTION_SLOP = 8
const DISMISS_DISTANCE = 72

/** Only the sheet handle owns this tracker; sheet contents keep their native scrolling. */
export function createSheetDrag() {
  let origin: SheetTouch | null = null
  let vertical = false
  let distance = 0
  let horizontalDistance = 0

  const cancel = (): SheetDragState => {
    origin = null
    vertical = false
    distance = 0
    horizontalDistance = 0
    return IDLE
  }

  const valid = (touch: SheetTouch) => Number.isFinite(touch.clientX) && Number.isFinite(touch.clientY)

  const start = (touches: SheetTouch[]): SheetDragState => {
    cancel()
    if (touches.length === 1 && valid(touches[0])) origin = { ...touches[0] }
    return IDLE
  }

  const move = (touches: SheetTouch[]): SheetDragState => {
    if (!origin) return IDLE
    if (touches.length !== 1 || touches[0].identifier !== origin.identifier || !valid(touches[0])) return cancel()
    distance = touches[0].clientY - origin.clientY
    horizontalDistance = Math.abs(touches[0].clientX - origin.clientX)
    if (!vertical) {
      if (Math.max(Math.abs(distance), horizontalDistance) < DIRECTION_SLOP) return IDLE
      if (distance <= 0 || distance <= horizontalDistance * 1.2) return cancel()
      vertical = true
    }
    return { offset: Math.min(180, Math.max(0, distance) * 0.72), dragging: true }
  }

  const end = (changedTouches: SheetTouch[], remainingTouches: SheetTouch[]): boolean => {
    if (!origin || remainingTouches.length !== 0 || changedTouches.length !== 1) {
      cancel()
      return false
    }
    move(changedTouches)
    const dismiss = Boolean(origin && vertical && distance >= DISMISS_DISTANCE && distance > horizontalDistance * 1.2)
    cancel()
    return dismiss
  }

  return { start, move, end, cancel }
}
