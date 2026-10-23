import { useEffect, useRef, useState } from 'react'

export type LayerPhase = 'closed' | 'enter' | 'open' | 'exit'

/** 弹层离场保留时间；只承载呈现，不延迟业务回调。 */
export const LAYER_EXIT_MS = 240

/** 先让 enter 样式完成一帧，再进入 open；缺少 rAF 时使用短时回退。 */
export function scheduleLayerFrame(callback: () => void): () => void {
  if (typeof requestAnimationFrame === 'function' && typeof cancelAnimationFrame === 'function') {
    const frame = requestAnimationFrame(callback)
    return () => cancelAnimationFrame(frame)
  }
  const timer = setTimeout(callback, 16)
  return () => clearTimeout(timer)
}

interface LayerState {
  mounted: boolean
  phase: LayerPhase
}

const CLOSED: LayerState = { mounted: false, phase: 'closed' }

export function useLayerPresence(visible: boolean, exitMs = LAYER_EXIT_MS): LayerState {
  const [state, setState] = useState<LayerState>(() => visible ? { mounted: true, phase: 'enter' } : CLOSED)
  const mountedRef = useRef(state.mounted)
  const phaseRef = useRef(state.phase)
  const exitTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const commit = (next: LayerState) => {
    phaseRef.current = next.phase
    mountedRef.current = next.mounted
    setState(next)
  }
  useEffect(() => {
    if (visible) {
      if (mountedRef.current && phaseRef.current === 'exit') {
        // A quick reopen reverses the existing exit in place. Restarting at
        // enter would flash the panel from off-screen and lose the gesture's
        // visual continuity.
        if (exitTimerRef.current) clearTimeout(exitTimerRef.current)
        exitTimerRef.current = null
        commit({ mounted: true, phase: 'open' })
        return
      }
      mountedRef.current = true
      commit({ mounted: true, phase: 'enter' })
      // 双帧推进，避免 enter 起始样式和 open 在同帧被合并。
      let cancelSecond: (() => void) | null = null
      const cancelFirst = scheduleLayerFrame(() => {
        cancelSecond = scheduleLayerFrame(() => {
          if (mountedRef.current) commit({ mounted: true, phase: 'open' })
        })
      })
      return () => { cancelFirst(); cancelSecond?.() }
    }

    if (!mountedRef.current) {
      commit(CLOSED)
      return
    }

    commit({ mounted: true, phase: 'exit' })
    const timer = setTimeout(() => {
      exitTimerRef.current = null
      commit(CLOSED)
    }, Math.max(0, exitMs))
    exitTimerRef.current = timer
    return () => {
      clearTimeout(timer)
      if (exitTimerRef.current === timer) exitTimerRef.current = null
    }
  }, [exitMs, visible])

  useEffect(() => () => {
    if (exitTimerRef.current) clearTimeout(exitTimerRef.current)
    exitTimerRef.current = null
    mountedRef.current = false
  }, [])

  return state
}
