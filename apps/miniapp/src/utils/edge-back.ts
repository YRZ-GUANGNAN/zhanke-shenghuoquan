export interface EdgeBackTouch {
  identifier: number
  clientX: number
  clientY: number
}

export interface EdgeBackState {
  offset: number
  tracking: boolean
}

const IDLE: EdgeBackState = { offset: 0, tracking: false }
const EDGE_WIDTH = 24
const DIRECTION_SLOP = 8
const HORIZONTAL_RATIO = 1.5
const MIN_DISTANCE = 72
const MAX_DISTANCE = 120

function validTouch(touch: EdgeBackTouch | undefined): touch is EdgeBackTouch {
  if (!touch) return false
  return Number.isFinite(touch.identifier)
    && Number.isFinite(touch.clientX)
    && Number.isFinite(touch.clientY)
}

function threshold(viewportWidth: number) {
  const width = Number.isFinite(viewportWidth) && viewportWidth > 0 ? viewportWidth : 390
  return Math.min(MAX_DISTANCE, Math.max(MIN_DISTANCE, width * 0.24))
}

/**
 * Tracks only a single-finger gesture that starts at the left edge.
 * The content area remains owned by ScrollView/Swiper.
 */
export function createEdgeBack() {
  let origin: EdgeBackTouch | null = null
  let horizontal = false
  let distance = 0
  let verticalDistance = 0

  const cancel = (): EdgeBackState => {
    origin = null
    horizontal = false
    distance = 0
    verticalDistance = 0
    return IDLE
  }

  const start = (touches: EdgeBackTouch[]): EdgeBackState => {
    cancel()
    const touch = touches.length === 1 ? touches[0] : undefined
    // Browser Touch fields live on its prototype; spreading it loses coordinates.
    if (validTouch(touch) && touch.clientX >= 0 && touch.clientX <= EDGE_WIDTH) {
      origin = { identifier: touch.identifier, clientX: touch.clientX, clientY: touch.clientY }
    }
    return IDLE
  }

  const move = (touches: EdgeBackTouch[]): EdgeBackState => {
    if (!origin || touches.length !== 1 || !validTouch(touches[0]) || touches[0].identifier !== origin.identifier) return cancel()
    distance = touches[0].clientX - origin.clientX
    verticalDistance = Math.abs(touches[0].clientY - origin.clientY)
    if (!horizontal) {
      if (Math.max(Math.abs(distance), verticalDistance) < DIRECTION_SLOP) return IDLE
      if (distance <= 0 || distance <= verticalDistance * HORIZONTAL_RATIO) return cancel()
      horizontal = true
    }
    if (distance <= 0 || distance <= verticalDistance * HORIZONTAL_RATIO) return cancel()
    return { offset: Math.min(12, distance * 0.12), tracking: true }
  }

  const end = (changedTouches: EdgeBackTouch[], remainingTouches: EdgeBackTouch[], viewportWidth: number): boolean => {
    if (!origin || remainingTouches.length !== 0 || changedTouches.length !== 1 || !validTouch(changedTouches[0]) || changedTouches[0].identifier !== origin.identifier) {
      cancel()
      return false
    }
    distance = changedTouches[0].clientX - origin.clientX
    verticalDistance = Math.abs(changedTouches[0].clientY - origin.clientY)
    const shouldReturn = horizontal
      && distance >= threshold(viewportWidth)
      && distance > verticalDistance * HORIZONTAL_RATIO
    cancel()
    return shouldReturn
  }

  return { start, move, end, cancel }
}
