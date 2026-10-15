import { useEffect, useMemo, useRef, type ReactNode } from 'react'
import { Swiper, SwiperItem, View } from '@tarojs/components'

const CHANNEL_DURATION_MS = 200
const RELEASE_GUARD_MS = 120

export type ChannelInteraction = {
  canActivate: () => boolean
  onTouchStart: () => void
  onTouchEnd: () => void
  onTouchCancel: () => void
  onTransition: () => void
  onAnimationFinish: () => void
}

// This only guards the click emitted after a native swipe. Direction, distance,
// velocity and edge resistance remain owned by the native Swiper.
export function createChannelInteraction(now: () => number = Date.now): ChannelInteraction {
  let touching = false
  let moved = false
  let blockedUntil = 0
  const release = () => {
    touching = false
    if (moved) blockedUntil = Math.max(blockedUntil, now() + RELEASE_GUARD_MS)
    moved = false
  }
  return {
    canActivate: () => !(touching && moved) && now() >= blockedUntil,
    onTouchStart: () => { touching = true; moved = false },
    onTouchEnd: release,
    onTouchCancel: release,
    onTransition: () => {
      if (touching) moved = true
      // Some clients omit animationfinish after cancel; this deadline also
      // guarantees that the page never remains permanently unclickable.
      blockedUntil = now() + CHANNEL_DURATION_MS + RELEASE_GUARD_MS
    },
    onAnimationFinish: () => { blockedUntil = now() + RELEASE_GUARD_MS }
  }
}

export function useChannelInteraction(): ChannelInteraction {
  return useMemo(() => createChannelInteraction(), [])
}

export type ChannelPagerProps<T extends string> = {
  keys: readonly T[]
  activeKey: T
  onChange: (key: T) => void
  renderPage: (key: T) => ReactNode
  className?: string
  interaction?: ChannelInteraction
}

export default function ChannelPager<T extends string>({
  keys,
  activeKey,
  onChange,
  renderPage,
  className = '',
  interaction
}: ChannelPagerProps<T>) {
  const ownInteraction = useChannelInteraction()
  const controls = interaction ?? ownInteraction
  const visibleKeys = useMemo(() => Array.from(new Set(keys)), [keys])
  const selectedIndex = visibleKeys.indexOf(activeKey)
  const current = Math.max(0, selectedIndex)
  const keySignature = JSON.stringify(visibleKeys)
  const keysAtRender = useRef(keySignature)
  const keysAtTouch = useRef<string | null>(keySignature)
  if (keysAtRender.current !== keySignature) {
    keysAtRender.current = keySignature
    // Ignore a late swipe event whose old index now identifies another page.
    keysAtTouch.current = null
  }

  useEffect(() => {
    if (selectedIndex < 0 && visibleKeys.length > 0) onChange(visibleKeys[0])
  }, [onChange, selectedIndex, visibleKeys])

  if (visibleKeys.length === 0) return <View className={`channel-pager channel-pager--empty ${className}`} />

  return (
    <Swiper
      className={`channel-pager ${className}`}
      current={current}
      duration={CHANNEL_DURATION_MS}
      easingFunction='easeOutCubic'
      circular={false}
      autoplay={false}
      indicatorDots={false}
      onTouchStart={() => {
        keysAtTouch.current = keySignature
        controls.onTouchStart()
      }}
      onTouchEnd={controls.onTouchEnd}
      onTouchCancel={controls.onTouchCancel}
      onTransition={(event) => {
        if (event.detail.dx !== 0 || event.detail.dy !== 0) controls.onTransition()
      }}
      onAnimationFinish={controls.onAnimationFinish}
      onChange={(event) => {
        const { current: nextIndex, source, currentItemId } = event.detail
        if (!Number.isInteger(nextIndex) || nextIndex < 0 || nextIndex >= visibleKeys.length) return
        const nextKey = visibleKeys[nextIndex]
        if (currentItemId && currentItemId !== nextKey) return
        // Programmatic change events may arrive after another tab click or a
        // visibility update. Only a native user swipe changes the active key.
        if (source !== 'touch' || keysAtTouch.current !== keySignature) return
        controls.onTransition()
        if (nextKey !== activeKey) onChange(nextKey)
      }}
    >
      {visibleKeys.map((key) => (
        <SwiperItem key={key} itemId={key} className='channel-pager__item'>
          {renderPage(key)}
        </SwiperItem>
      ))}
    </Swiper>
  )
}
