import { useCallback, useEffect, useRef, useState } from 'react'
import { ScrollView, Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import { AppIconButton, AppSkeleton, AppState } from '../../components/ui'
import Taro from '../../services/taro'
import { listAuthorNotes, type AuthorNote } from '../../services/featureboard'
import { navigateBackOrHome } from '../../utils/navigation'
import './index.scss'

type PageStatus = 'loading' | 'ready' | 'empty' | 'error'

const PAGE_SIZE = 20

export default function UpdateLogPage() {
  const [items, setItems] = useState<AuthorNote[]>([])
  const [status, setStatus] = useState<PageStatus>('loading')
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [loadingMore, setLoadingMore] = useState(false)
  const mountedRef = useRef(true)
  const requestVersion = useRef(0)
  const loadMoreLock = useRef(false)

  useEffect(() => {
    mountedRef.current = true
    return () => {
      mountedRef.current = false
      requestVersion.current += 1
    }
  }, [])

  const load = useCallback(async () => {
    const version = ++requestVersion.current
    setStatus('loading')
    setLoadingMore(false)
    loadMoreLock.current = false
    try {
      const result = await listAuthorNotes(1, PAGE_SIZE)
      if (!mountedRef.current || requestVersion.current !== version) return
      setItems(result.items)
      setPage(result.page)
      setTotal(result.total)
      setStatus(result.items.length > 0 ? 'ready' : 'empty')
    } catch {
      if (!mountedRef.current || requestVersion.current !== version) return
      setStatus('error')
    }
  }, [])

  useEffect(() => { void load() }, [load])

  const loadMore = async () => {
    if (loadMoreLock.current || status !== 'ready' || items.length >= total) return
    loadMoreLock.current = true
    setLoadingMore(true)
    const version = requestVersion.current
    try {
      const result = await listAuthorNotes(page + 1, PAGE_SIZE)
      if (!mountedRef.current || requestVersion.current !== version) return
      setItems((current) => {
        const seen = new Set(current.map((item) => item.id))
        return [...current, ...result.items.filter((item) => !seen.has(item.id))]
      })
      setPage(result.page)
      setTotal(result.total)
    } catch {
      // 保留现有列表，下次触底或重进重试。
    } finally {
      if (mountedRef.current && requestVersion.current === version) {
        setLoadingMore(false)
        loadMoreLock.current = false
      } else {
        loadMoreLock.current = false
      }
    }
  }

  return (
    <View className='update-log-page'>
      <CapsulePageHeader
        title='更新日记'
        divider={false}
        left={<AppIconButton name='back' label='返回' className='update-log-nav__back' onClick={() => void navigateBackOrHome()} />}
      />
      <ScrollView
        className='update-log-content'
        scrollY
        enhanced
        showScrollbar={false}
        onScrollToLower={() => void loadMore()}
      >
        {status === 'loading' && <AppSkeleton rows={4} />}
        {status === 'error' && (
          <AppState kind='error' title='更新日记加载失败' description='请检查网络后重试。' actionText='重新加载' onAction={() => void load() } />
        )}
        {status === 'empty' && (
          <AppState kind='empty' title='还没有更新记录' description='作者发布更新后会显示在这里。' />
        )}
        {status === 'ready' && (
          <>
            {items.map((note) => (
              <View key={note.id} className='update-log-entry'>
                <View className='update-log-entry__timeline'>
                  <View className='update-log-entry__dot' />
                  <View className='update-log-entry__line' />
                </View>
                <View className='update-log-entry__body'>
                  <Text className='update-log-entry__date'>{formatUpdateDate(note.createdAt)}</Text>
                  <View className='update-log-entry__card'>
                    <Text className='update-log-entry__content'>{note.content}</Text>
                  </View>
                </View>
              </View>
            ))}
            {loadingMore && <Text className='update-log-more'>正在加载更多…</Text>}
            {!loadingMore && items.length >= total && items.length > 0 && <Text className='update-log-more'>已经到底了</Text>}
          </>
        )}
      </ScrollView>
    </View>
  )
}

function formatUpdateDate(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const pad = (input: number) => String(input).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}
