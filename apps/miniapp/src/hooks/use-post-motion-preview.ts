import { useEffect, useMemo, useRef, useState } from 'react'
import { useDidHide, useDidShow } from '@tarojs/taro'
import Taro from '../services/taro'
import { createMotionPreviewCoordinator } from '../utils/motion-preview'

export function usePostMotionPreview(active: boolean, sessionKey: string) {
  const [activeId, setActiveId] = useState('')
  const [shown, setShown] = useState(true)
  const [networkOK, setNetworkOK] = useState(false)
  const [reduced, setReduced] = useState(false)
  const coordinator = useMemo(() => createMotionPreviewCoordinator(setActiveId), [sessionKey])
  const mounted = useRef(true)
  useDidHide(() => { coordinator.setEnabled(false); setShown(false) })
  useDidShow(() => {
    setShown(true)
      void Taro.getNetworkType().then(value => { if (mounted.current) setNetworkOK(value.networkType !== 'none') }).catch(() => setNetworkOK(false))
  })
  useEffect(() => {
    mounted.current = true
    const update = (value: { networkType: string }) => { if (mounted.current) setNetworkOK(value.networkType !== 'none') }
    void Taro.getNetworkType().then(update).catch(() => setNetworkOK(false))
    try { Taro.onNetworkStatusChange(update) } catch { /* Poster fallback. */ }
    // Browser accessibility preference, when exposed by the runtime. Native
    // clients without that API retain the bounded, silent preview.
    const media = typeof globalThis.matchMedia === 'function' ? globalThis.matchMedia('(prefers-reduced-motion: reduce)') : null
    const motionChange = () => setReduced(Boolean(media?.matches))
    motionChange(); media?.addEventListener?.('change', motionChange)
    return () => { mounted.current = false; try { Taro.offNetworkStatusChange(update) } catch {} media?.removeEventListener?.('change', motionChange) }
  }, [])
  const enabled = active && shown && networkOK && !reduced
  useEffect(() => { coordinator.setEnabled(enabled); return () => coordinator.setEnabled(false) }, [coordinator, enabled])
  useEffect(() => () => coordinator.dispose(), [coordinator])
  return { activeId, enabled, coordinator }
}
