import { useCallback, useEffect, useRef, useState } from 'react'
import { Input, ScrollView, Text, Textarea, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import AppIcon from '../../components/app-icon'
import { AppButton, AppIconButton, AppSkeleton, AppState } from '../../components/ui'
import Taro from '../../services/taro'
import {
  listAuthorNotes,
  listFeatureComments,
  listFeatureSuggestions,
  setFeatureSuggestionSupport,
  submitFeatureComment,
  submitFeatureSuggestion,
  type AuthorNote,
  type FeatureComment,
  type FeatureSuggestion
} from '../../services/featureboard'
import { formatPostDate } from '../../utils/post-presentation'
import { isApiError } from '../../services/api'
import './index.scss'

type SortKey = 'supports' | 'latest'
type PageStatus = 'loading' | 'ready' | 'empty' | 'error'

const PAGE_SIZE = 10
const MAX_CONTENT = 500

const sortOptions: Array<{ key: SortKey; label: string }> = [
  { key: 'supports', label: '最受欢迎' },
  { key: 'latest', label: '最新' }
]

export default function FeatureBoardPage() {
  const [sort, setSort] = useState<SortKey>('supports')
  const [items, setItems] = useState<FeatureSuggestion[]>([])
  const [status, setStatus] = useState<PageStatus>('loading')
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [loadingMore, setLoadingMore] = useState(false)
  const [notes, setNotes] = useState<AuthorNote[] | null>(null)
  const [draft, setDraft] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [supportBusyId, setSupportBusyId] = useState('')
  const [openThreadId, setOpenThreadId] = useState('')
  const [comments, setComments] = useState<FeatureComment[]>([])
  const [commentsLoading, setCommentsLoading] = useState(false)
  const [commentsError, setCommentsError] = useState(false)
  const [commentsTotal, setCommentsTotal] = useState(0)
  const [commentDraft, setCommentDraft] = useState('')
  const [commentSending, setCommentSending] = useState(false)
  const threadSeq = useRef(0)
  const openThreadRef = useRef('')
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

  const loadNotes = useCallback(async () => {
    try {
      const values = await listAuthorNotes()
      if (mountedRef.current) setNotes(values)
    } catch {
      if (mountedRef.current) setNotes([])
    }
  }, [])

  const load = useCallback(async (nextSort: SortKey) => {
    const version = ++requestVersion.current
    setStatus('loading')
    setLoadingMore(false)
    loadMoreLock.current = false
    try {
      const result = await listFeatureSuggestions(nextSort, 1, PAGE_SIZE)
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

  useEffect(() => { void load(sort) }, [load, sort])
  useEffect(() => { void loadNotes() }, [loadNotes])

  const loadMore = async () => {
    if (loadMoreLock.current || status !== 'ready' || items.length >= total) return
    loadMoreLock.current = true
    setLoadingMore(true)
    const version = requestVersion.current
    try {
      const result = await listFeatureSuggestions(sort, page + 1, PAGE_SIZE)
      if (!mountedRef.current || requestVersion.current !== version) return
      setItems((current) => {
        const seen = new Set(current.map((item) => item.id))
        return [...current, ...result.items.filter((item) => !seen.has(item.id))]
      })
      setPage(result.page)
      setTotal(result.total)
    } catch {
      // Keep the current list; the next pull-down or re-entry retries.
    } finally {
      if (mountedRef.current && requestVersion.current === version) {
        setLoadingMore(false)
        loadMoreLock.current = false
      } else {
        loadMoreLock.current = false
      }
    }
  }

  const submit = async () => {
    const content = draft.trim()
    if (submitting) return
    if (content.length < 2) {
      void Taro.showToast({ title: '请先写下你的建议', icon: 'none' })
      return
    }
    setSubmitting(true)
    try {
      await submitFeatureSuggestion(content)
      if (!mountedRef.current) return
      setDraft('')
      void Taro.showToast({ title: '建议已提交', icon: 'success' })
      await load(sort)
    } catch (error) {
      if (mountedRef.current) {
        void Taro.showToast({ title: error instanceof Error ? error.message : '提交失败，请重试', icon: 'none' })
      }
    } finally {
      if (mountedRef.current) setSubmitting(false)
    }
  }

  const toggleSupport = async (item: FeatureSuggestion) => {
    if (supportBusyId) return
    setSupportBusyId(item.id)
    try {
      const result = await setFeatureSuggestionSupport(item.id, !item.supported)
      if (!mountedRef.current) return
      setItems((current) => current.map((value) => value.id === item.id
        ? { ...value, supported: result.supported, supportCount: result.supportCount }
        : value))
      if (sort === 'supports' && page === 1) void load(sort)
    } catch (error) {
      if (!mountedRef.current) return
      void Taro.showToast({ title: error instanceof Error ? error.message : '操作失败，请重试', icon: 'none' })
      void load(sort)
    } finally {
      if (mountedRef.current) setSupportBusyId('')
    }
  }

  const loadThread = async (suggestionId: string) => {
    const seq = ++threadSeq.current
    setComments([])
    setCommentsError(false)
    setCommentsLoading(true)
    setCommentsTotal(0)
    try {
      const page = await listFeatureComments(suggestionId)
      if (!mountedRef.current || threadSeq.current !== seq) return
      setComments(page.items)
      setCommentsTotal(page.total)
    } catch {
      if (!mountedRef.current || threadSeq.current !== seq) return
      setCommentsError(true)
    } finally {
      if (mountedRef.current && threadSeq.current === seq) setCommentsLoading(false)
    }
  }

  const toggleThread = (item: FeatureSuggestion) => {
    if (openThreadId === item.id) {
      threadSeq.current += 1
      openThreadRef.current = ''
      setOpenThreadId('')
      setComments([])
      setCommentsError(false)
      setCommentDraft('')
      return
    }
    openThreadRef.current = item.id
    setOpenThreadId(item.id)
    setCommentDraft('')
    void loadThread(item.id)
  }

  const commentErrorMessage = (error: unknown): string => {
    if (isApiError(error)) {
      if (error.status === 401) return '登录已过期，请重新进入'
      if (error.status === 403) return '账号已被禁言，暂时无法发言'
      if (error.status === 404) return '这条建议不存在或已被移除'
      if (error.status === 409) return '该建议已关闭，暂时无法发言'
    }
    return error instanceof Error ? error.message : '发送失败，请重试'
  }

  const sendComment = async () => {
    const targetId = openThreadId
    const content = commentDraft.trim()
    if (!targetId || commentSending || !content) return
    setCommentSending(true)
    try {
      const comment = await submitFeatureComment(targetId, content)
      if (!mountedRef.current) return
      setItems((current) => current.map((value) => value.id === targetId ? { ...value, commentCount: value.commentCount + 1 } : value))
      if (openThreadRef.current === targetId) {
        setComments((current) => [...current, comment])
        setCommentsTotal((current) => current + 1)
        setCommentDraft('')
      }
    } catch (error) {
      if (mountedRef.current) void Taro.showToast({ title: commentErrorMessage(error), icon: 'none' })
    } finally {
      if (mountedRef.current) setCommentSending(false)
    }
  }

  return (
    <View className='feature-board-page'>
      <CapsulePageHeader
        title='功能意见与需求'
        divider={false}
        left={<AppIconButton name='back' label='返回' className='feature-board-nav__back' onClick={() => void Taro.navigateBack()} />}
      />
      <ScrollView
        className='feature-board-content'
        scrollY
        enhanced
        showScrollbar={false}
        onScrollToLower={() => void loadMore()}
      >
        <View className='feature-board-compose'>
          <Textarea
            className='feature-board-compose__input'
            value={draft}
            maxlength={MAX_CONTENT}
            placeholder='写下你希望我们增加或改进的功能…'
            onInput={(event) => setDraft(String(event.detail.value ?? ''))}
          />
          <View className='feature-board-compose__footer'>
            <Text className='feature-board-compose__count'>{draft.length}/{MAX_CONTENT}</Text>
            <AppButton variant='primary' size='small' loading={submitting} onClick={() => void submit()}>提交建议</AppButton>
          </View>
        </View>

        <View className='feature-board-sort'>
          {sortOptions.map((option) => (
            <View
              key={option.key}
              className={`feature-board-sort__item${sort === option.key ? ' feature-board-sort__item--active' : ''}`}
              role='button'
              ariaLabel={`${option.label}${sort === option.key ? '，已选中' : ''}`}
              onClick={() => { if (option.key !== sort) setSort(option.key) }}
            >
              <Text>{option.label}</Text>
            </View>
          ))}
          <Text className='feature-board-sort__total'>{total} 条建议</Text>
        </View>

        {status === 'loading' && <AppSkeleton rows={4} />}
        {status === 'error' && (
          <AppState kind='error' title='建议墙加载失败' description='请检查网络后重试。' actionText='重新加载' onAction={() => void load(sort)} />
        )}
        {status === 'empty' && (
          <AppState kind='empty' title='还没有建议' description='成为第一个提出功能建议的人。' />
        )}
        {status === 'ready' && (
          <>
            {items.map((item) => (
              <View key={item.id} className='feature-board-card'>
                <Text className='feature-board-card__content'>{item.content}</Text>
                <View className='feature-board-card__meta'>
                  <Text className='feature-board-card__date'>{formatPostDate(item.createdAt)}</Text>
                  {item.status === 'accepted' && <Text className='feature-board-card__badge'>已采纳</Text>}
                  <View
                    className={`feature-board-card__thread-toggle${openThreadId === item.id ? ' feature-board-card__thread-toggle--open' : ''}`}
                    role='button'
                    ariaLabel={`${openThreadId === item.id ? '收起' : '展开'}对话，当前 ${item.commentCount} 条`}
                    onClick={() => void toggleThread(item)}
                  >
                    <AppIcon name='comment' className='feature-board-card__thread-icon' />
                    <Text>对话 {item.commentCount}</Text>
                  </View>
                  <View
                    className={`feature-board-card__support${item.supported ? ' feature-board-card__support--active' : ''}`}
                    role='button'
                    ariaLabel={`${item.supported ? '取消支持' : '支持'}，当前 ${item.supportCount} 人支持`}
                    onClick={() => void toggleSupport(item)}
                  >
                    <AppIcon name='heart' active={item.supported} className='feature-board-card__support-icon' />
                    <Text>{item.supportCount}</Text>
                  </View>
                </View>
                {openThreadId === item.id && (
                  <View className='feature-board-thread'>
                    {commentsLoading ? (
                      <Text className='feature-board-thread__hint'>正在加载对话…</Text>
                    ) : commentsError ? (
                      <Text className='feature-board-thread__hint' onClick={() => void loadThread(item.id)}>对话加载失败，点击重试</Text>
                    ) : comments.length === 0 ? (
                      <Text className='feature-board-thread__hint'>还没有对话，来说两句。</Text>
                    ) : comments.map((comment) => (
                      <View key={comment.id} className={`feature-board-comment${comment.isMaintainer ? ' feature-board-comment--maintainer' : ''}`}>
                        <View className='feature-board-comment__head'>
                          <Text className='feature-board-comment__author'>{comment.isMaintainer ? '作者' : '用户'}</Text>
                          <Text className='feature-board-comment__date'>{formatPostDate(comment.createdAt)}</Text>
                        </View>
                        <Text className='feature-board-comment__content'>{comment.content}</Text>
                      </View>
                    ))}
                    {comments.length > 0 && comments.length < commentsTotal && (
                      <Text className='feature-board-thread__hint'>仅显示最近 {comments.length} 条对话</Text>
                    )}
                    <View className='feature-board-thread__compose'>
                      <Input
                        className='feature-board-thread__input'
                        value={commentDraft}
                        maxlength={300}
                        placeholder='参与讨论…'
                        confirmType='send'
                        onInput={(event) => setCommentDraft(String(event.detail.value ?? ''))}
                        onConfirm={() => void sendComment()}
                      />
                      <AppButton variant='primary' size='small' loading={commentSending} onClick={() => void sendComment()}>发送</AppButton>
                    </View>
                  </View>
                )}
              </View>
            ))}
            {loadingMore && <Text className='feature-board-more'>正在加载更多…</Text>}
            {!loadingMore && items.length >= total && items.length > 0 && <Text className='feature-board-more'>已经到底了</Text>}
          </>
        )}

        {notes && notes.length > 0 && (
          <View className='feature-board-notes'>
            <Text className='feature-board-notes__title'>作者的话</Text>
            {notes.map((note) => (
              <View key={note.id} className='feature-board-note'>
                <Text className='feature-board-note__content'>{note.content}</Text>
                <Text className='feature-board-note__date'>{formatPostDate(note.createdAt)}</Text>
              </View>
            ))}
          </View>
        )}
      </ScrollView>
    </View>
  )
}
