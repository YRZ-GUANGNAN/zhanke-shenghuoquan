import { useEffect, useRef } from 'react'
import type { CommonEvent, ITouchEvent } from '@tarojs/components'
import { useDidHide, useDidShow } from '@tarojs/taro'

// Navigation surfaces only. Forms and native content pagers own their gestures.
export function createHorizontalNavigation() {
  let origin: { x: number; y: number; id: number } | null = null
  let horizontal = false
  let blockedUntil = 0
  const cancel = () => {
    if (horizontal) blockedUntil = Date.now() + 350
    origin = null
    horizontal = false
  }
  return {
    cancel,
    canActivate: () => !horizontal && Date.now() >= blockedUntil,
    start(event: ITouchEvent) {
      cancel()
      const point = event.touches?.[0]
      if (event.touches?.length === 1 && point.clientX > 24) origin = { x: point.clientX, y: point.clientY, id: point.identifier }
    },
    move(event: ITouchEvent) {
      const point = event.touches?.[0]
      if (!origin || event.touches?.length !== 1 || point.identifier !== origin.id) { cancel(); return }
      const dx = Math.abs(point.clientX - origin.x), dy = Math.abs(point.clientY - origin.y)
      if (!horizontal && dy > 12 && dy >= dx) { cancel(); return }
      if (dx > 12 && dx > dy * 1.5) horizontal = true
    },
    end(event: ITouchEvent): -1 | 0 | 1 {
      const point = event.changedTouches?.[0]
      const dx = point && origin ? point.clientX - origin.x : 0
      const dy = point && origin ? point.clientY - origin.y : 0
      const valid = origin && point && point.identifier === origin.id && !event.touches?.length && horizontal && Math.abs(dx) >= 64 && Math.abs(dx) > Math.abs(dy) * 1.5
      if (horizontal) blockedUntil = Date.now() + 350
      cancel()
      return valid ? dx < 0 ? 1 : -1 : 0
    }
  }
}

export function useHorizontalNavigation(onStep: (direction: -1 | 1) => void, enabled = true) {
  const tracker = useRef(createHorizontalNavigation())
  const active = useRef(true)
  useEffect(() => { active.current = true; return () => { active.current = false; tracker.current.cancel() } }, [])
  useEffect(() => { tracker.current.cancel() }, [enabled])
  useDidHide(() => { active.current = false; tracker.current.cancel() })
  useDidShow(() => { active.current = true; tracker.current.cancel() })
  return {
    canActivate: () => enabled && active.current && tracker.current.canActivate(),
    handlers: {
      onTouchStart: (event: CommonEvent) => { event.stopPropagation(); if (enabled && active.current) tracker.current.start(event as ITouchEvent) },
      onTouchMove: (event: CommonEvent) => { if (enabled && active.current) tracker.current.move(event as ITouchEvent) },
      onTouchEnd: (event: CommonEvent) => { event.stopPropagation(); const step = tracker.current.end(event as ITouchEvent); if (enabled && active.current && step) onStep(step) },
      onTouchCancel: () => tracker.current.cancel()
    }
  }
}
