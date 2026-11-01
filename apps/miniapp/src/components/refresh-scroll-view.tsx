import { useEffect, useRef, useState, type ComponentProps } from 'react'
import { ScrollView, Slot, Text, View } from '@tarojs/components'
import { useDidHide, useDidShow } from '@tarojs/taro'
import { StatusGlyph } from './ui/status-glyph'
import TaroUxLoading from './ui/taro-ux-loading'
import { useBottomNavScroll } from '../utils/bottom-nav-motion'
import { taroUxBlurStyle, UX_BLUR_END, UX_BLUR_LOADING } from '../utils/taro-ux-blur'
import './refresh-scroll-view.scss'

type Phase = 'idle' | 'pulling' | 'ready' | 'refreshing' | 'success' | 'error'
type Props = Omit<ComponentProps<typeof ScrollView>, 'refresherEnabled' | 'refresherTriggered' | 'onRefresherRefresh'> & {
  enabled?: boolean
  busy?: boolean
  refreshKey: string
  onRefresh: () => Promise<boolean>
}
const labels: Record<Phase, string> = {
  idle: '下拉刷新', pulling: '下拉刷新', ready: '松开刷新',
  refreshing: '正在更新', success: '已更新', error: '更新失败，请重试'
}
/** Native scrolling owns gesture direction and resistance; this owns request feedback only. */
export default function RefreshScrollView({ enabled = true, busy = false, refreshKey, onRefresh, children, onScroll, className = '', style, ...props }: Props) {
  const navScroll = useBottomNavScroll(enabled, refreshKey)
  const [phase, setPhase] = useState<Phase>('idle')
  const [triggered, setTriggered] = useState(false)
  const [blur, setBlur] = useState(0)
  const generation = useRef(0)
  const locked = useRef(false)
  const mounted = useRef(true)
  const visible = useRef(true)
  const timer = useRef<ReturnType<typeof setTimeout>>()
  const blurTimer = useRef<ReturnType<typeof setTimeout>>()
  const current = useRef({ enabled, busy, refreshKey, onRefresh })
  current.current = { enabled, busy, refreshKey, onRefresh }
  const invalidate = () => {
    generation.current += 1
    locked.current = false
    clearTimeout(timer.current)
    clearTimeout(blurTimer.current)
  }
  const reset = () => { invalidate(); setTriggered(false); setPhase('idle'); setBlur(0) }
  const isCurrentOwner = () => mounted.current && visible.current && current.current.refreshKey === refreshKey
  useEffect(() => {
    mounted.current = true
    return () => { mounted.current = false; invalidate() }
  }, [])
  useEffect(() => {
    reset()
    return invalidate
  }, [enabled, refreshKey])
  useDidHide(() => { if (mounted.current) { visible.current = false; reset() } })
  useDidShow(() => { if (mounted.current) visible.current = true })

  const refresh = async () => {
    // Native events may arrive after the page or account that owned the pull.
    if (!isCurrentOwner() || !enabled || locked.current) return
    if (!current.current.enabled || current.current.busy) {
      setTriggered(false)
      setPhase('idle')
      setBlur(0)
      return
    }
    locked.current = true
    const version = ++generation.current
    const key = current.current.refreshKey
    setTriggered(true)
    setPhase('refreshing')
    clearTimeout(blurTimer.current)
    // Original BlurLoading startLoading uses 60, not a full-screen opaque mask.
    setBlur(UX_BLUR_LOADING)
    let ok = false
    try { ok = await current.current.onRefresh() } catch { /* The page owns its actionable error. */ }
    if (generation.current !== version || !isCurrentOwner() || !current.current.enabled || current.current.refreshKey !== key) return
    setPhase(ok ? 'success' : 'error')
    blurTimer.current = setTimeout(() => {
      if (generation.current === version && isCurrentOwner()) setBlur(0)
    }, 300)
    timer.current = setTimeout(() => {
      if (generation.current !== version) return
      locked.current = false
      setTriggered(false)
      setPhase('idle')
    }, ok ? 450 : 800)
  }
  const resting = () => {
    // A native abort is not cancellation of a request already in flight.
    if (isCurrentOwner() && !locked.current) {
      clearTimeout(blurTimer.current)
      setTriggered(false); setPhase('idle'); setBlur(0)
    }
  }
  const blurStyle = taroUxBlurStyle(blur)
  const showDots = locked.current || blur > UX_BLUR_END
  // Keep the caller's flex/height class on the root. The overlay is a sibling
  // of ScrollView, so long lists cannot move it or grow it beyond the viewport.
  return <View className={`taro-ux-refresh ${className}`.trim()} style={style}>
    <ScrollView {...props} className='taro-ux-refresh__scroll'
    refresherEnabled={enabled && (!busy || locked.current)} refresherThreshold={60}
    refresherDefaultStyle='none' refresherBackground='transparent' refresherTriggered={triggered}
    onRefresherPulling={event => {
      if (isCurrentOwner() && enabled && !locked.current && current.current.enabled && !current.current.busy) {
        const value = Number((event.detail as { dy?: number }).dy || 0)
        const dy = Number.isFinite(value) ? Math.max(0, value) : 0
        setPhase(dy >= 60 ? 'ready' : 'pulling')
        // Upstream observes pulls within START..END+20. Clamp the native dy
        // adapter to that decoration range; native refresher still triggers.
        setBlur(Math.min(dy, UX_BLUR_END + 20))
      }
    }}
    onRefresherRefresh={() => { void refresh() }}
    onRefresherRestore={resting} onRefresherAbort={resting}
    onScroll={event => {
      navScroll(Number(event.detail?.scrollTop))
      onScroll?.(event)
    }}>
    <Slot name='refresher' className={`refresh-feedback refresh-feedback--${phase}`}>
      {phase === 'success' || phase === 'error' ? <StatusGlyph small kind={phase === 'success' ? 'success' : 'offline'} /> : null}
      <Text>{labels[phase]}</Text>
    </Slot>
    <View className='taro-ux-refresh__content'>{children}</View>
    </ScrollView>
    {blurStyle ? <TaroUxLoading
      className={`taro-ux-loading--overlay${showDots ? '' : ' taro-ux-loading--hide-dots'}`}
      style={blurStyle}
    /> : null}
  </View>
}
