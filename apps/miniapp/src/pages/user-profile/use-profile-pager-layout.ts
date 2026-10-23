import { useEffect, useRef, useState } from 'react'
import { createSelectorQuery, nextTick, useResize } from '@tarojs/taro'

type Tab = 'posts' | 'secondhand'
type Layout = { heights: Record<Tab, number>; collapseOffset: number; minimumBody: number }

// One vertical scroll surface owns the profile. Only the lists belong to Swiper.
export function useProfilePagerLayout(ready: boolean, revision: string) {
  const [resize, setResize] = useState(0)
  const [layout, setLayout] = useState<Layout>({ heights: { posts: 0, secondhand: 0 }, collapseOffset: 0, minimumBody: 0 })
  const [scrollTarget, setScrollTarget] = useState(0)
  const positions = useRef<Record<Tab, number>>({ posts: 0, secondhand: 0 })
  const liveTop = useRef(0)
  const switchVersion = useRef(0)
  useResize(() => setResize(value => value + 1))

  useEffect(() => {
    if (!ready) return
    let alive = true
    nextTick(() => {
      if (!alive) return
      const query = createSelectorQuery()
      query.select('.public-profile-hero').boundingClientRect()
      query.select('.public-profile-tabs').boundingClientRect()
      query.select('.public-profile-navigation').boundingClientRect()
      query.select('.public-profile-scroll').boundingClientRect()
      query.select('#public-profile-panel-posts').boundingClientRect()
      query.select('#public-profile-panel-secondhand').boundingClientRect()
      query.exec(result => {
        if (!alive) return
        const [hero, tabs, nav, viewport, postPanel, productPanel] = result || []
        if (!hero?.height || !viewport?.height) return
        const heights = { posts: 0, secondhand: 0 }
        if (typeof postPanel?.height === 'number' && Number.isFinite(postPanel.height)) heights.posts = Math.ceil(postPanel.height)
        if (typeof productPanel?.height === 'number' && Number.isFinite(productPanel.height)) heights.secondhand = Math.ceil(productPanel.height)
        setLayout({ heights, collapseOffset: Math.max(0, hero.height - (nav?.height || 0)),
          minimumBody: Math.max(0, viewport.height - (nav?.height || 0) - (tabs?.height || 0)) })
      })
    })
    return () => { alive = false }
  }, [ready, revision, resize])

  useEffect(() => () => { switchVersion.current += 1 }, [])

  const rememberScroll = (tab: Tab, top: number) => {
    if (!Number.isFinite(top)) return
    liveTop.current = Math.max(0, top)
    positions.current[tab] = liveTop.current
  }
  const switchScroll = (next: Tab) => {
    const version = ++switchVersion.current
    const current = liveTop.current
    // While the cover is visible, keep it at exactly the same vertical offset.
    // Once collapsed, restore each list without bringing the profile back down.
    const maximum = layout.collapseOffset + Math.max(0, layout.heights[next] - layout.minimumBody)
    const target = layout.minimumBody <= 0 || current < layout.collapseOffset
      ? current : Math.min(maximum, Math.max(layout.collapseOffset, positions.current[next]))
    positions.current[next] = target
    liveTop.current = target
    if (target !== current) {
      // Commit the next panel height before restoring its reading position.
      // Otherwise a short outgoing panel clamps a longer list's saved offset.
      // Acknowledge the live offset so an unchanged target still updates natively.
      setScrollTarget(current)
      nextTick(() => { if (version === switchVersion.current) setScrollTarget(target) })
    } else setScrollTarget(target)
    return target
  }
  return { layout, scrollTarget, rememberScroll, switchScroll }
}
