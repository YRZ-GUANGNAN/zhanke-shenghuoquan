import { useCallback, useRef, useState } from 'react'
import { useDidHide, useDidShow, useResize } from '@tarojs/taro'
import Taro from '../services/taro'
import { resolveKeyboardInset } from '../utils/keyboard-inset'

type KeyboardHeightEvent = number | { height?: unknown; detail?: { height?: unknown } }

function readWindowHeight(): number {
  try {
    const value = Number(Taro.getWindowInfo().windowHeight)
    return Number.isFinite(value) && value > 0 ? value : 0
  } catch {
    return 0
  }
}

function eventHeight(event: KeyboardHeightEvent): unknown {
  if (typeof event === 'number') return event
  return event?.detail?.height ?? event?.height ?? 0
}

/**
 * 为输入页面提供唯一的键盘避让来源。
 * 页面只需要绑定 `onKeyboardHeightChange`，并把 `inset` 用于底部栏与滚动区。
 */
export function useKeyboardInset() {
  const baselineHeightRef = useRef(readWindowHeight())
  const lastReportedHeightRef = useRef(0)
  const activeRef = useRef(false)
  const [inset, setInset] = useState(0)

  const reset = useCallback(() => {
    activeRef.current = false
    lastReportedHeightRef.current = 0
    baselineHeightRef.current = readWindowHeight()
    setInset(0)
  }, [])

  const onKeyboardHeightChange = useCallback((event: KeyboardHeightEvent) => {
    const reportedHeight = eventHeight(event)
    const currentHeight = readWindowHeight()
    const normalizedReported = Number.isFinite(Number(reportedHeight)) ? Math.max(0, Number(reportedHeight)) : 0
    // blur/hide/show 已清理当前输入生命周期时，忽略基础库迟到的旧事件。
    if (!activeRef.current && normalizedReported > 0) return
    if (normalizedReported <= 0) {
      reset()
      return
    }
    // 基线由首次渲染、窗口变化、前后台恢复维护；这里不能用已缩短的
    // windowHeight 覆盖它，否则会把平台已完成的避让再次计算进去。
    activeRef.current = true
    lastReportedHeightRef.current = normalizedReported
    setInset(resolveKeyboardInset(normalizedReported, {
      baselineHeight: baselineHeightRef.current,
      windowHeight: currentHeight,
    }))
  }, [reset])

  const onFocus = useCallback(() => {
    // 在键盘事件到达前锁住当前窗口基线，避免某些基础库先触发
    // useResize（windowHeight 已缩短）再触发 onKeyboardHeightChange。
    activeRef.current = true
    if (baselineHeightRef.current <= 0) baselineHeightRef.current = readWindowHeight()
  }, [])

  useResize(() => {
    const currentHeight = readWindowHeight()
    if (!activeRef.current) {
      if (currentHeight > 0) baselineHeightRef.current = currentHeight
      return
    }
    setInset(resolveKeyboardInset(lastReportedHeightRef.current, {
      baselineHeight: baselineHeightRef.current,
      windowHeight: currentHeight,
    }))
  })
  useDidHide(reset)
  useDidShow(reset)

  return { inset, onFocus, onKeyboardHeightChange, reset }
}
