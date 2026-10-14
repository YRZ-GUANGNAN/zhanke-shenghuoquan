import { useEffect, useRef, useState } from 'react'
import { useDidHide, useDidShow, useResize } from '@tarojs/taro'
import Taro from '../services/taro'
import { getCapsuleLayout } from '../utils/capsule-safe'
import { createGalleryCounter, galleryViewportHeight, type GalleryViewport, type MediaSize } from '../utils/gallery-presentation'

function readViewport(): GalleryViewport {
  try {
    const window = Taro.getWindowInfo()
    return {
      width: window.windowWidth,
      height: window.windowHeight ?? 667,
      headerHeight: getCapsuleLayout().totalHeaderHeight,
      safeBottom: window.safeArea && window.screenHeight ? Math.max(0, window.screenHeight - window.safeArea.bottom) : 0,
    }
  } catch {
    return { width: 375, height: 667, headerHeight: 88, safeBottom: 0 }
  }
}

export function useGalleryPresentation(contentKey: string, count: number, media: MediaSize | undefined, mode: 'post' | 'product', hasActions: boolean) {
  const [viewport, setViewport] = useState(readViewport)
  const [counterVisible, setCounterVisible] = useState(false)
  const visible = useRef(true)
  const counter = useRef<ReturnType<typeof createGalleryCounter> | null>(null)
  if (!counter.current) counter.current = createGalleryCounter(setCounterVisible)
  const enabled = count > 1

  useResize(() => setViewport(readViewport()))
  useDidHide(() => {
    visible.current = false
    counter.current?.hide()
  })
  useDidShow(() => {
    visible.current = true
    setViewport(readViewport())
    if (enabled) counter.current?.reveal()
  })
  useEffect(() => {
    if (enabled && visible.current) counter.current?.reveal()
    else counter.current?.hide()
    return () => counter.current?.dispose()
  }, [contentKey, enabled])

  return {
    height: galleryViewportHeight(media, viewport, mode, hasActions ? mode === 'post' ? 112 : 120 : 0),
    counterVisible,
    onTouchStart: () => { if (enabled && visible.current) counter.current?.hold() },
    onTouchEnd: () => { if (enabled && visible.current) counter.current?.reveal() },
    revealCounter: () => { if (enabled && visible.current) counter.current?.reveal() },
  }
}
