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
  useEffect(() => {
    if (visible) {
      mountedRef.current = true
      setState({ mounted: true, phase: 'enter' })
      // 双帧推进，避免 enter 起始样式和 open 在同帧被合并。
      let cancelSecond: (() => void) | null = null
      const cancelFirst = scheduleLayerFrame(() => {
        cancelSecond = scheduleLayerFrame(() => {
          if (mountedRef.current) setState({ mounted: true, phase: 'open' })
        })
      })
      return () => { cancelFirst(); cancelSecond?.() }
    }

    if (!mountedRef.current) {
      setState(CLOSED)
      return
    }

    setState({ mounted: true, phase: 'exit' })
    const timer = setTimeout(() => {
      mountedRef.current = false
      setState(CLOSED)
    }, Math.max(0, exitMs))
    return () => clearTimeout(timer)
  }, [exitMs, visible])

  useEffect(() => () => {
    mountedRef.current = false
  }, [])

  return state
}
