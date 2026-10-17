import { useEffect, useRef, useState, type ComponentProps } from 'react'
import { ScrollView, Slot, Text } from '@tarojs/components'
import { useDidHide, useDidShow } from '@tarojs/taro'
import { StatusGlyph } from './ui/status-glyph'
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
export default function RefreshScrollView({ enabled = true, busy = false, refreshKey, onRefresh, children, ...props }: Props) {
  const [phase, setPhase] = useState<Phase>('idle')
  const [triggered, setTriggered] = useState(false)
  const generation = useRef(0)
  const locked = useRef(false)
  const visible = useRef(true)
  const timer = useRef<ReturnType<typeof setTimeout>>()
  const current = useRef({ enabled, busy, refreshKey, onRefresh })
  current.current = { enabled, busy, refreshKey, onRefresh }
  const invalidate = () => {
    generation.current += 1
    locked.current = false
    clearTimeout(timer.current)
  }
  const reset = () => { invalidate(); setTriggered(false); setPhase('idle') }
  useEffect(() => {
    reset()
    return invalidate
  }, [enabled, refreshKey])
  useDidHide(() => { visible.current = false; reset() })
  useDidShow(() => { visible.current = true })

  const refresh = async () => {
    if (locked.current) return
    if (!current.current.enabled || current.current.busy || !visible.current) {
      setTriggered(false)
      setPhase('idle')
      return
    }
    locked.current = true
    const version = ++generation.current
    const key = current.current.refreshKey
    setTriggered(true)
    setPhase('refreshing')
    let ok = false
    try { ok = await current.current.onRefresh() } catch { /* The page owns its actionable error. */ }
    if (generation.current !== version || !visible.current || !current.current.enabled || current.current.refreshKey !== key) return
    setPhase(ok ? 'success' : 'error')
    timer.current = setTimeout(() => {
      if (generation.current !== version) return
      locked.current = false
      setTriggered(false)
      setPhase('idle')
    }, ok ? 450 : 800)
  }
  const resting = () => {
    // A native abort is not cancellation of a request already in flight.
    if (!locked.current) { setTriggered(false); setPhase('idle') }
  }
  return <ScrollView {...props} refresherEnabled={enabled && (!busy || locked.current)} refresherThreshold={60}
    refresherDefaultStyle='none' refresherBackground='#FFFFFF' refresherTriggered={triggered}
    onRefresherPulling={event => {
      if (!locked.current && current.current.enabled && !current.current.busy) {
        const dy = Number((event.detail as { dy?: number }).dy || 0)
        setPhase(dy >= 60 ? 'ready' : 'pulling')
      }
    }}
    onRefresherRefresh={() => { void refresh() }}
    onRefresherRestore={resting} onRefresherAbort={resting}>
    <Slot name='refresher' className={`refresh-feedback refresh-feedback--${phase}`}>
      <StatusGlyph small kind={phase === 'refreshing' ? 'loading' : phase === 'success' ? 'success' : phase === 'error' ? 'offline' : 'pull'} />
      <Text>{labels[phase]}</Text>
    </Slot>
    {children}
  </ScrollView>
}
