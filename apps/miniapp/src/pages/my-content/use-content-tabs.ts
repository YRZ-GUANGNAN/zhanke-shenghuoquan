import { useCallback, useEffect, useRef, useState } from 'react'
import { useDidShow } from '@tarojs/taro'
import type { CommunityPost, Product } from '../../types'
import type { MyCommunityComment } from '../../services/b-community'

export type ContentTabKey = 'favorites' | 'posts' | 'comments' | 'items'
type ContentItem = CommunityPost | Product | MyCommunityComment
type ContentPage = { items: ContentItem[]; total: number; page: number }
export type ContentReaders = Record<ContentTabKey, (page: number) => Promise<ContentPage>>

interface TabState {
  items: ContentItem[]
  page: number
  total: number
  hasMore: boolean
  phase: 'idle' | 'loading' | 'ready' | 'error'
  loadingMore: boolean
  refreshing: boolean
  error: string
  loadMoreError: string
  refreshError: string
}

const keys: ContentTabKey[] = ['favorites', 'posts', 'comments', 'items']
const emptyTab = (): TabState => ({ items: [], page: 0, total: 0, hasMore: false, phase: 'idle', loadingMore: false, refreshing: false, error: '', loadMoreError: '', refreshError: '' })
const emptyTabs = (): Record<ContentTabKey, TabState> => ({ favorites: emptyTab(), posts: emptyTab(), comments: emptyTab(), items: emptyTab() })
const emptyPositions = () => ({ favorites: 0, posts: 0, comments: 0, items: 0 })
const emptyBusy = () => ({ favorites: false, posts: false, comments: false, items: false })

// Scoped to one mounted account/page session; never persisted across accounts.
export function useContentTabs(initialTab: ContentTabKey, enabledTabs: ContentTabKey[], readers: ContentReaders) {
  const [requestedTab, setActiveTab] = useState(initialTab)
  const [states, setStates] = useState(emptyTabs)
  const statesRef = useRef(states)
  const versions = useRef(emptyPositions())
  const positions = useRef(emptyPositions())
  const busy = useRef(emptyBusy())
  const mounted = useRef(true)
  const didShowOnce = useRef(false)
  const enabledRef = useRef(enabledTabs)
  enabledRef.current = enabledTabs
  const enabledKey = enabledTabs.join('|')
  const activeTab = enabledTabs.includes(requestedTab) ? requestedTab : enabledTabs[0] ?? 'posts'
  const activeRef = useRef(activeTab)
  activeRef.current = activeTab

  const update = useCallback((tab: ContentTabKey, change: Partial<TabState>) => {
    const next = { ...statesRef.current, [tab]: { ...statesRef.current[tab], ...change } }
    statesRef.current = next
    setStates(next)
  }, [])

  const load = useCallback(async (tab: ContentTabKey, mode: 'initial' | 'more' | 'refresh' = 'initial') => {
    if (!mounted.current || !enabledRef.current.includes(tab) || busy.current[tab]) return
    const previous = statesRef.current[tab]
    if (mode === 'more' && (previous.phase !== 'ready' || !previous.hasMore)) return
    busy.current[tab] = true
    const version = ++versions.current[tab]
    const current = () => mounted.current && enabledRef.current.includes(tab) && versions.current[tab] === version
    const background = mode === 'refresh' && previous.phase === 'ready'
    update(tab, {
      phase: mode === 'more' || background ? previous.phase : 'loading',
      loadingMore: mode === 'more', refreshing: background,
      error: '', loadMoreError: '', refreshError: '',
    })
    try {
      const pages = mode === 'refresh'
        ? Array.from({ length: Math.max(1, previous.page) }, (_, index) => index + 1)
        : [mode === 'more' ? previous.page + 1 : 1]
      const results = await Promise.all(pages.map((page) => readers[tab](page)))
      if (!current()) return
      const merged = new Map((mode === 'more' ? previous.items : []).map((item) => [item.id, item]))
      results.forEach((result) => result.items.forEach((item) => merged.set(item.id, item)))
      const items = Array.from(merged.values())
      const last = results[results.length - 1]
      update(tab, {
        items, total: last.total, page: last.page || pages[pages.length - 1], phase: 'ready',
        hasMore: last.items.length > 0 && items.length < last.total,
      })
    } catch (cause) {
      if (!current()) return
      const message = cause instanceof Error ? cause.message : '内容加载失败'
      if (mode === 'more') update(tab, { loadMoreError: `${message}，点击重试` })
      else if (background) update(tab, { refreshError: '更新失败，已保留当前内容。点击重试' })
      else update(tab, { phase: 'error', error: message })
    } finally {
      if (current()) {
        busy.current[tab] = false
        update(tab, { loadingMore: false, refreshing: false })
      }
    }
  }, [readers, update])

  useEffect(() => {
    if (requestedTab !== activeTab) setActiveTab(activeTab)
    keys.forEach((tab) => {
      if (enabledRef.current.includes(tab)) return
      versions.current[tab] += 1
      busy.current[tab] = false
      positions.current[tab] = 0
      if (statesRef.current[tab].phase !== 'idle') update(tab, emptyTab())
    })
    if (enabledRef.current.includes(activeTab) && statesRef.current[activeTab].phase === 'idle') void load(activeTab)
  }, [enabledKey, activeTab, requestedTab, load, update])

  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
      keys.forEach((tab) => { versions.current[tab] += 1; busy.current[tab] = false })
    }
  }, [])

  useDidShow(() => {
    if (!didShowOnce.current) { didShowOnce.current = true; return }
    void load(activeRef.current, 'refresh')
  })

  // Native swipers keep neighbouring scroll views mounted. Their late scroll
  // events must update their own position, never the newly selected channel.
  const rememberScrollFor = (tab: ContentTabKey, scrollTop: number) => {
    if (enabledRef.current.includes(tab) && Number.isFinite(scrollTop)) positions.current[tab] = Math.max(0, scrollTop)
  }
  const getScrollTop = (tab: ContentTabKey) => positions.current[tab]
  const loadMoreFor = (tab: ContentTabKey) => {
    if (tab === activeRef.current) void load(tab, 'more')
  }
  const reloadFor = (tab: ContentTabKey) => {
    if (tab === activeRef.current) void load(tab, statesRef.current[tab].phase === 'ready' ? 'refresh' : 'initial')
  }

  return {
    activeTab, setActiveTab, states, current: states[activeTab],
    scrollTop: positions.current[activeTab],
    rememberScroll: (scrollTop: number) => rememberScrollFor(activeTab, scrollTop),
    loadMore: () => loadMoreFor(activeTab),
    reload: () => reloadFor(activeTab),
    rememberScrollFor, getScrollTop, loadMoreFor, reloadFor,
  }
}
