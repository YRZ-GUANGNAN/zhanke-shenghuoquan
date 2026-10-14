import { useEffect, useRef, useState } from 'react'

export type LayerPhase = 'closed' | 'enter' | 'open' | 'exit'

/** 弹层离场保留时间；只承载呈现，不延迟业务回调。 */
export const LAYER_EXIT_MS = 180

interface LayerState {
  mounted: boolean
  phase: LayerPhase
}

const CLOSED: LayerState = { mounted: false, phase: 'closed' }

export function useLayerPresence(visible: boolean, exitMs = LAYER_EXIT_MS): LayerState {
  const [state, setState] = useState<LayerState>(() => visible ? { mounted: true, phase: 'enter' } : CLOSED)
  const mountedRef = useRef(state.mounted)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    if (timerRef.current !== null) {
      clearTimeout(timerRef.current)
      timerRef.current = null
    }

    if (visible) {
      mountedRef.current = true
      setState({ mounted: true, phase: 'enter' })
      // 使用零延迟将 enter 推进为 open，确保 CSS 有机会建立起始样式。
      timerRef.current = setTimeout(() => {
        timerRef.current = null
        if (mountedRef.current) setState({ mounted: true, phase: 'open' })
      }, 0)
      return
    }

    if (!mountedRef.current) {
      setState(CLOSED)
      return
    }

    setState({ mounted: true, phase: 'exit' })
    timerRef.current = setTimeout(() => {
      timerRef.current = null
      mountedRef.current = false
      setState(CLOSED)
    }, Math.max(0, exitMs))
  }, [exitMs, visible])

  useEffect(() => () => {
    if (timerRef.current !== null) clearTimeout(timerRef.current)
    timerRef.current = null
    mountedRef.current = false
  }, [])

  return state
}
