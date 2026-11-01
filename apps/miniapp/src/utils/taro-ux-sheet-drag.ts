// Ported from nanxiaobei/taro-ux-kits/draggable-modal/Modal.jsx
// at f85a5d714639006a9c3f12c67838087a30255c0b. Keep its angle, distance
// and quick-swipe thresholds; touch identity/direction guards are app adapters.
export interface TaroUxSheetTouch {
  identifier: number
  pageX?: number
  pageY?: number
  clientX: number
  clientY: number
}

export interface TaroUxSheetDragState { offset: number; dragging: boolean }
const IDLE: TaroUxSheetDragState = { offset: 0, dragging: false }
const getAngle = (x: number, y: number) => Math.atan2(y, x) * (180 / Math.PI)

export function createTaroUxSheetDrag(now = Date.now) {
  let origin: { identifier: number; pageX: number; pageY: number } | null = null
  let startTime = 0
  let vertical = false
  let offset = 0
  const point = (touch: TaroUxSheetTouch) => ({
    // Native Touch properties can be prototype getters: do not spread Touch.
    identifier: touch.identifier,
    pageX: touch.pageX ?? touch.clientX,
    pageY: touch.pageY ?? touch.clientY,
  })
  const cancel = (): TaroUxSheetDragState => { origin = null; vertical = false; offset = 0; return IDLE }
  const start = (touches: TaroUxSheetTouch[]) => {
    cancel()
    if (touches.length !== 1) return IDLE
    const touch = point(touches[0])
    if (Number.isFinite(touch.pageX) && Number.isFinite(touch.pageY)) {
      origin = touch
      startTime = now()
    }
    return IDLE
  }
  const getDiffY = (touch: { pageX: number; pageY: number }) => {
    if (!origin) return 0
    const diffY = touch.pageY - origin.pageY
    if (diffY <= 0) return 0
    const angle = getAngle(touch.pageX - origin.pageX, diffY)
    if (angle < 45 || angle > 135) return 0
    return diffY
  }
  const move = (touches: TaroUxSheetTouch[]): TaroUxSheetDragState => {
    if (!origin) return IDLE
    if (touches.length !== 1) return cancel()
    const touch = point(touches[0])
    if (touch.identifier !== origin.identifier || !Number.isFinite(touch.pageX) || !Number.isFinite(touch.pageY)) return cancel()
    const diffY = getDiffY(touch)
    // Preserve the app's horizontal-gesture isolation, absent in the demo.
    if (!vertical && Math.max(Math.abs(touch.pageX - origin.pageX), Math.abs(touch.pageY - origin.pageY)) >= 8) {
      if (!diffY) return cancel()
      vertical = true
    }
    // Upstream keeps the last positive offset during an out-of-angle move;
    // release, not that intermediate event, decides whether to reset/close.
    if (diffY) offset = diffY
    return { offset, dragging: offset > 0 }
  }
  const end = (changedTouches: TaroUxSheetTouch[], remainingTouches: TaroUxSheetTouch[]) => {
    if (!origin || remainingTouches.length !== 0 || changedTouches.length !== 1) { cancel(); return false }
    move(changedTouches)
    const diffY = getDiffY(point(changedTouches[0]))
    const dismiss = Boolean(origin && (diffY > 200 || (diffY > 10 && now() - startTime < 200)))
    cancel()
    return dismiss
  }
  return { start, move, end, cancel }
}
