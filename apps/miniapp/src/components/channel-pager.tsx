import { useEffect, useMemo, useRef, type ReactNode } from 'react'
import { Swiper, SwiperItem, View } from '@tarojs/components'
import { useDidHide, useDidShow } from '@tarojs/taro'

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

type ChannelController = ChannelInteraction & {
  isActive: () => boolean
  setActive: (active: boolean) => void
}

// This only guards the click emitted after a native swipe. Direction, distance,
// velocity and edge resistance remain owned by the native Swiper.
export function createChannelInteraction(now: () => number = Date.now): ChannelController {
  let active = true
  let touching = false
  let moved = false
  let blockedUntil = 0
  const release = () => {
    if (!active) return
    touching = false
    if (moved) blockedUntil = Math.max(blockedUntil, now() + RELEASE_GUARD_MS)
    moved = false
  }
  return {
    isActive: () => active,
    setActive: (value) => { active = value; touching = false; moved = false; blockedUntil = 0 },
    canActivate: () => active && !(touching && moved) && now() >= blockedUntil,
    onTouchStart: () => { if (active) { touching = true; moved = false } },
    onTouchEnd: release,
    onTouchCancel: release,
    onTransition: () => {
      if (!active) return
      if (touching) moved = true
      // Some clients omit animationfinish after cancel; this deadline also
      // guarantees that the page never remains permanently unclickable.
      blockedUntil = now() + CHANNEL_DURATION_MS + RELEASE_GUARD_MS
    },
    onAnimationFinish: () => { if (active && blockedUntil > 0) blockedUntil = now() + RELEASE_GUARD_MS }
  }
}

export function useChannelInteraction(): ChannelController {
  const interaction = useMemo(() => createChannelInteraction(), [])
  useEffect(() => {
    interaction.setActive(true)
    return () => interaction.setActive(false)
  }, [interaction])
  useDidHide(() => interaction.setActive(false))
  useDidShow(() => interaction.setActive(true))
  return interaction
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
  useDidHide(() => { keysAtTouch.current = null })
  useDidShow(() => { keysAtTouch.current = null })

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
        if (!ownInteraction.isActive()) return
        keysAtTouch.current = keySignature
        controls.onTouchStart()
      }}
      onTouchEnd={controls.onTouchEnd}
      onTouchCancel={controls.onTouchCancel}
      onTransition={(event) => {
        if (ownInteraction.isActive() && keysAtTouch.current === keySignature && (event.detail.dx !== 0 || event.detail.dy !== 0)) controls.onTransition()
      }}
      onAnimationFinish={controls.onAnimationFinish}
      onChange={(event) => {
        if (!ownInteraction.isActive()) return
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
