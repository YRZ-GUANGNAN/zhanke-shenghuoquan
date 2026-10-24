import React, { useEffect, useRef } from 'react'
import { View, type ITouchEvent } from '@tarojs/components'
import { useDidHide, useDidShow } from '@tarojs/taro'
import { createHorizontalNavigation } from '../hooks/use-horizontal-navigation'
import ChannelPager, { type ChannelPagerProps } from './channel-pager'

type BoundaryContext = { keys: readonly string[]; activeKey: string; scopeKey: string }
const signature = ({ keys, activeKey, scopeKey }: BoundaryContext) => JSON.stringify([scopeKey, keys, activeKey])

/** 原生阶段 Swiper 负责内部切换；只有未改变阶段的首尾外滑才交给工作区导航。 */
export function createDeliveryStageBoundary() {
  const tracker = createHorizontalNavigation()
  let started: { signature: string; index: number } | null = null
  const cancel = () => { started = null; tracker.cancel() }
  return {
    cancel,
    start(event: ITouchEvent, context: BoundaryContext) {
      cancel()
      const index = context.keys.indexOf(context.activeKey)
      if (index < 0) return
      started = { signature: signature(context), index }
      tracker.start(event)
    },
    move(event: ITouchEvent) { tracker.move(event) },
    end(event: ITouchEvent, context: BoundaryContext): -1 | 0 | 1 {
      const initial = started
      const direction = tracker.end(event)
      started = null
      if (!initial || !direction || initial.signature !== signature(context)) return 0
      const next = initial.index + direction
      return next < 0 || next >= context.keys.length ? direction : 0
    },
  }
}

type Props<T extends string> = ChannelPagerProps<T> & {
  scopeKey: string
  enabled?: boolean
  onBoundarySwipe: (direction: -1 | 1) => void
}

export default function DeliveryStagePager<T extends string>(props: Props<T>) {
  const { enabled = true, disabled = false, scopeKey, onBoundarySwipe, ...pagerProps } = props
  const navigationEnabled = enabled && !disabled
  const boundary = useRef(createDeliveryStageBoundary())
  const visible = useRef(true)
  const current = useRef({ ...props, enabled: navigationEnabled })
  current.current = { ...props, enabled: navigationEnabled }
  const contextKey = signature(props)
  useEffect(() => { boundary.current.cancel() }, [contextKey, navigationEnabled])
  useEffect(() => {
    visible.current = true
    return () => { visible.current = false; boundary.current.cancel() }
  }, [])
  useDidHide(() => { visible.current = false; boundary.current.cancel() })
  useDidShow(() => { visible.current = true; boundary.current.cancel() })
  const canNavigate = () => visible.current && current.current.enabled

  return <View className='delivery-stage-pager'
    onTouchStart={event => {
      event.stopPropagation()
      if (canNavigate()) boundary.current.start(event as ITouchEvent, current.current)
      else boundary.current.cancel()
    }}
    onTouchMove={event => {
      event.stopPropagation()
      if (canNavigate()) boundary.current.move(event as ITouchEvent)
      else boundary.current.cancel()
    }}
    onTouchEnd={event => {
      event.stopPropagation()
      if (!canNavigate()) { boundary.current.cancel(); return }
      const direction = boundary.current.end(event as ITouchEvent, current.current)
      if (!direction) return
      current.current.interaction?.onTransition()
      current.current.interaction?.onTouchEnd()
      current.current.onBoundarySwipe(direction)
    }}
    onTouchCancel={event => { event.stopPropagation(); boundary.current.cancel() }}
  >
    <ChannelPager key={scopeKey} {...pagerProps} disabled={!navigationEnabled}
      onChange={key => { if (canNavigate()) current.current.onChange(key) }} />
  </View>
}
