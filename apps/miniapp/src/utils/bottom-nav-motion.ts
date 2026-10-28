import { useEffect, useRef, useState } from 'react'
import TaroRuntime, { useDidHide, useDidShow } from '@tarojs/taro'

type TaroHooks = typeof TaroRuntime & {
  usePageScroll(callback: (event: { scrollTop: number }) => void): void
}
const TaroHooksRuntime = TaroRuntime as TaroHooks

type ScrollEvent = { page: string; source: object; scrollTop: number }
type ScrollListener = (event: ScrollEvent) => void

const listeners = new Set<ScrollListener>()
const RESTORE_DELAY = 600
const HIDE_AFTER = 72

// $taroPath distinguishes two instances of the same route in the page stack.
// Without an owner, retain a visible navigation bar instead of broadcasting.
const currentPage = () => TaroRuntime?.getCurrentInstance?.().router?.$taroPath

/** Bind each ScrollView to its page, active channel and current lifetime. */
export function useBottomNavScroll(enabled = true, ownerKey = '') {
  const page = useRef(currentPage())
  const mounted = useRef(true)
  const visible = useRef(true)
  const [, rerender] = useState(0)
  const owner = useRef({ enabled, ownerKey, source: {} })
  if (owner.current.enabled !== enabled || owner.current.ownerKey !== ownerKey) {
    owner.current = { enabled, ownerKey, source: {} }
  }
  const source = owner.current.source
  useEffect(() => {
    mounted.current = true
    return () => { mounted.current = false; owner.current.source = {} }
  }, [])
  useDidHide(() => {
    visible.current = false
    owner.current.source = {}
  })
  useDidShow(() => {
    if (!mounted.current) return
    visible.current = true
    owner.current.source = {}
    rerender(value => value + 1)
  })
  return (scrollTop: number) => {
    if (!mounted.current || !visible.current || !enabled || !owner.current.enabled
      || owner.current.source !== source || !page.current || page.current !== currentPage()
      || !Number.isFinite(scrollTop)) return
    const event = { page: page.current, source, scrollTop: Math.max(0, scrollTop) }
    listeners.forEach(listener => listener(event))
  }
}

export function useBottomNavVisibility(enabled = true) {
  const [hidden, setHidden] = useState(false)
  const lastTop = useRef(0)
  const page = useRef(currentPage())
  const pageSource = useRef({})
  const lastSource = useRef<object | null>(null)
  const mounted = useRef(true)
  const visible = useRef(true)
  const timer = useRef<ReturnType<typeof setTimeout>>()
  const enabledRef = useRef(enabled)
  enabledRef.current = enabled

  const reset = () => {
    clearTimeout(timer.current)
    lastSource.current = null
    lastTop.current = 0
    if (mounted.current) setHidden(false)
  }
  const process = (event: ScrollEvent) => {
    if (!enabledRef.current || !mounted.current || !visible.current || !page.current
      || event.page !== page.current || page.current !== currentPage() || !Number.isFinite(event.scrollTop)) return
    const top = Math.max(0, event.scrollTop)
    if (lastSource.current !== event.source) {
      reset()
      lastSource.current = event.source
      lastTop.current = top
      return
    }
    const delta = top - lastTop.current
    clearTimeout(timer.current)
    if (top <= 8) {
      setHidden(false)
    } else if (delta > 0 && top >= HIDE_AFTER) {
      setHidden(true)
      timer.current = setTimeout(() => setHidden(false), RESTORE_DELAY)
    } else if (delta < 0) {
      setHidden(false)
    } else if (top >= HIDE_AFTER) {
      timer.current = setTimeout(() => setHidden(false), RESTORE_DELAY)
    }
    lastTop.current = top
  }

  TaroHooksRuntime.usePageScroll(event => {
    if (page.current) process({ page: page.current, source: pageSource.current, scrollTop: event.scrollTop })
  })
  useDidHide(() => { visible.current = false; reset() })
  useDidShow(() => {
    if (!mounted.current) return
    visible.current = true
    reset()
  })

  useEffect(() => {
    mounted.current = true
    return () => { mounted.current = false; clearTimeout(timer.current) }
  }, [])

  useEffect(() => {
    if (!enabled) {
      reset()
      return undefined
    }
    listeners.add(process)
    return () => {
      listeners.delete(process)
      clearTimeout(timer.current)
    }
  }, [enabled])

  return hidden
}
