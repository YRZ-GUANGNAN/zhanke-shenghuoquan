import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { createSelectorQuery, nextTick, useResize } from '@tarojs/taro'
import { Text, View } from '@tarojs/components'
import AppIcon, { type AppIconName } from './app-icon'
import { useLayerPresence } from '../hooks/use-layer-presence'
import { useConfirmedFeedback } from '../hooks/use-confirmed-feedback'
import './content-motion.scss'

let sequence = 0

/** Body text and reply disclosure deliberately own separate expansion states. */
export function ExpandableText({ text, className = '', lines = 6, reveal = false, disabled = false, layoutKey = '',
  expanded: controlled, onExpandedChange, onLayoutReady }: {
  text: string; className?: string; lines?: number; reveal?: boolean; disabled?: boolean; layoutKey?: string
  expanded?: boolean; onExpandedChange?: (value: boolean) => void
  onLayoutReady?: (ready: boolean) => void
}) {
  const [id] = useState(() => `content-fold-${++sequence}`)
  const [localExpanded, setExpanded] = useState(reveal)
  const expanded = controlled ?? localExpanded
  const [resize, setResize] = useState(0)
  const measurementKey = useMemo(() => ({}), [layoutKey, lines, resize, text])
  const [measured, setMeasured] = useState<{ key: object; full: number; closed: number } | null>(null)
  const [settledKey, setSettledKey] = useState<object | null>(null)
  const readyCallback = useRef(onLayoutReady)
  readyCallback.current = onLayoutReady
  const measure = measured?.key === measurementKey ? measured : null
  const ready = settledKey === measurementKey
  useResize(() => setResize(value => value + 1))
  useEffect(() => { setExpanded(reveal) }, [text, reveal])
  useEffect(() => {
    let alive = true
    readyCallback.current?.(false)
    nextTick(() => {
      if (!alive) return
      const query = createSelectorQuery()
      query.select(`#${id}-body`).boundingClientRect()
      query.select(`#${id}-line`).boundingClientRect()
      query.exec((result) => {
        if (!alive) return
        const full = Number(result?.[0]?.height)
        const line = Number(result?.[1]?.height)
        // Failed measurement stays naturally expanded, never a stale fixed height.
        setMeasured({ key: measurementKey, full: full > 0 && line > 0 ? full : 0, closed: line > 0 ? line * lines : 0 })
      })
    })
    return () => { alive = false }
  }, [id, measurementKey, lines])
  useEffect(() => {
    if (!measure) return
    let alive = true
    // The measured height must reach the native view before a parent can scroll.
    nextTick(() => {
      if (!alive) return
      setSettledKey(measurementKey)
      readyCallback.current?.(true)
    })
    return () => { alive = false }
  }, [measure, measurementKey])
  const foldable = Boolean(measure && measure.full > measure.closed + 1)
  return <View className={`content-fold${ready ? ' content-fold--ready' : ''} ${className}`}>
    <View className='content-fold__viewport' style={measure && measure.full > 0 ? { height: `${expanded || !foldable ? measure.full : measure.closed}px` } : undefined}>
      <Text id={`${id}-body`} className='content-fold__text' selectable>{text}</Text>
    </View>
    <Text id={`${id}-line`} className='content-fold__probe' aria-hidden>行</Text>
    {foldable && <View className='content-fold__toggle' role='button' aria-expanded={expanded} aria-disabled={disabled}
      aria-label={expanded ? '收起正文' : '展开完整正文'} onClick={(event) => {
        event.stopPropagation()
        if (disabled) return
        setExpanded(!expanded)
        onExpandedChange?.(!expanded)
      }}>
      <Text>{expanded ? '收起' : '展开全文'}</Text>
      <AppIcon name='chevron-right' className={`content-fold__arrow${expanded ? ' content-fold__arrow--open' : ''}`} />
    </View>}
  </View>
}

/** Exit content stays painted briefly but becomes noninteractive immediately. */
export function ReplyReveal({ expanded, immediate = false, children }: { expanded: boolean; immediate?: boolean; children: (interactive: boolean) => ReactNode }) {
  const presence = useLayerPresence(expanded, 200)
  const immediateSession = useRef(false)
  if (!expanded) immediateSession.current = false
  else if (immediate) immediateSession.current = true
  const instant = expanded && immediateSession.current
  if (!presence.mounted && !instant) return null
  const open = expanded && (instant || presence.phase === 'open')
  return <View className={`reply-reveal${open ? ' reply-reveal--open' : ''}${instant ? ' reply-reveal--immediate' : ''}${!expanded ? ' reply-reveal--closing' : ''}`} aria-hidden={!expanded}>
    <View className='reply-reveal__clip'><View className='reply-reveal__content'>{children(expanded)}</View></View>
  </View>
}

export function ConfirmedIcon({ name, active, event, owner, className = '' }: {
  name: AppIconName; active: boolean; event: number; owner: string; className?: string
}) {
  const pulse = useConfirmedFeedback(event, owner)
  return <View key={pulse} className={`confirmed-icon ${className}${pulse ? ' confirmed-icon--pulse' : ''}`}>
    <AppIcon name={name} active={active} />
  </View>
}
