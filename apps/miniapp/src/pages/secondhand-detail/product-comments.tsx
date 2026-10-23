import React, { useCallback, useEffect, useRef, useState } from 'react'
import { Image, Input, ScrollView, Text, View } from '@tarojs/components'
import { nextTick } from '@tarojs/taro'
import { isApiError } from '../../services/api'
import { createProductComment, listProductComments, replyProductComment } from '../../services/c-trade'
import { createClientRequestId } from '../../services/client-request-id'
import Taro from '../../services/taro'
import type { Product, ProductComment } from '../../types'
import { ExpandableText, ReplyReveal } from '../../components/content-motion'
import AppIcon from '../../components/app-icon'
import { formatPostDate } from '../../utils/post-presentation'
import { AppIconButton } from '../../components/ui'
import { useLayerPresence } from '../../hooks/use-layer-presence'
import { useSheetDismiss } from '../../hooks/use-sheet-dismiss'
import { useKeyboardInset } from '../../hooks/use-keyboard-inset'
import './product-comments.scss'

type CommentLoadState = 'loading' | 'ready' | 'empty' | 'error'
type ReplyTarget = { commentId: string; parentCommentId: string; authorName: string } | null
interface ProductCommentsProps {
  productId: string
  productStatus: Product['status']
  targetCommentId?: string
  layoutReady?: boolean
  onTargetReady?: (elementId: string) => void
  onLoadMoreReady?: (handler: (() => void) | null) => void
  sheetVisible?: boolean
  onClose?: () => void
}
interface PendingSubmission { key: string; clientRequestId: string }

function reviewLabel(status: ProductComment['reviewStatus']): string {
  if (status === 'submitted') return '审核中'
  if (status === 'reviewing') return '等待人工审核'
  if (status === 'rejected') return '未通过'
  return ''
}
function submissionError(error: unknown): string {
  if (!isApiError(error)) return error instanceof Error ? error.message : '留言失败，请稍后重试'
  if (error.status === 0) return '网络连接失败，可点击发送重试'
  if (error.status === 401 || error.status === 403) return '登录状态已失效，请重新登录'
  if (error.status === 404) return '商品或留言不存在'
  if (error.status === 409) return error.message || '当前商品不能继续留言'
  if (error.status === 429) return '留言太频繁，请稍后再试'
  return error.message || '留言失败，请稍后重试'
}
function upsertReply(items: ProductComment[], reply: ProductComment, parentId: string): ProductComment[] {
  return items.map((item) => item.id === parentId
    ? { ...item, replies: [...item.replies.filter((value) => value.id !== reply.id), reply] }
    : item)
}
function mergeComments(current: ProductComment[], incoming: ProductComment[]): ProductComment[] {
  const byId = new Map(current.map((comment) => [comment.id, comment]))
  incoming.forEach((comment) => byId.set(comment.id, comment))
  return Array.from(byId.values())
}

export default function ProductComments({ productId, productStatus, targetCommentId = '', layoutReady = true, onTargetReady, onLoadMoreReady, sheetVisible, onClose }: ProductCommentsProps) {
  const isSheet = sheetVisible !== undefined
  const presence = useLayerPresence(Boolean(sheetVisible))
  const keyboard = useKeyboardInset()
  const [sheetTarget, setSheetTarget] = useState('')
  const [items, setItems] = useState<ProductComment[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(0)
  const [loadingMore, setLoadingMore] = useState(false)
  const [loadMoreError, setLoadMoreError] = useState('')
  const [state, setState] = useState<CommentLoadState>('loading')
  const [draft, setDraft] = useState('')
  const [replyTarget, setReplyTarget] = useState<ReplyTarget>(null)
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState('')
  const [failedAvatars, setFailedAvatars] = useState<Set<string>>(() => new Set())
  const [expandedThreads, setExpandedThreads] = useState<Set<string>>(() => new Set())
  const [focusedCommentId, setFocusedCommentId] = useState(targetCommentId)
  const [expandedBodies, setExpandedBodies] = useState<Record<string, boolean>>({})
  const [bodyLayouts, setBodyLayouts] = useState<Record<string, boolean>>({})
  const pendingSubmission = useRef<PendingSubmission | null>(null)
  const requestVersion = useRef(0)
  const readingRef = useRef<number | null>(null)
  const submittingRef = useRef(false)
  const locatedTarget = useRef('')
  const closeSheet = () => {
    if (!sheetVisible) return
    keyboard.reset()
    void Taro.hideKeyboard().catch(() => {})
    onClose?.()
  }
  const drag = useSheetDismiss(Boolean(sheetVisible) && presence.phase === 'open', closeSheet)
  useEffect(() => { if (isSheet && !sheetVisible) keyboard.reset() }, [isSheet, sheetVisible, keyboard.reset])

  const load = useCallback(async () => {
    if (readingRef.current !== null || submittingRef.current) return
    const version = ++requestVersion.current
    readingRef.current = version
    setState('loading')
    setLoadMoreError('')
    try {
      const pageSize = 20
      const firstPage = await listProductComments(productId, 1, pageSize)
      if (requestVersion.current !== version) return
      let loadedItems = firstPage.items
      let loadedPage = firstPage.page
      let loadedTotal = firstPage.total
      const containsTarget = (values: ProductComment[]) => values.some((item) => (
        item.id === targetCommentId || item.replies.some((reply) => reply.id === targetCommentId)
      ))
      if (targetCommentId && !containsTarget(loadedItems)) {
        const pageCount = Math.ceil(firstPage.total / pageSize)
        for (let nextPage = 2; nextPage <= pageCount && !containsTarget(loadedItems); nextPage += 1) {
          try {
            const next = await listProductComments(productId, nextPage, pageSize)
            if (requestVersion.current !== version) return
            loadedItems = mergeComments(loadedItems, next.items)
            loadedPage = next.page
            loadedTotal = next.total
            if (next.items.length === 0) break
          } catch {
            if (requestVersion.current !== version) return
            setLoadMoreError('更多留言加载失败，点击重试')
            break
          }
        }
      }
      setItems(loadedItems)
      setTotal(loadedTotal)
      setPage(loadedPage)
      setState(loadedItems.length > 0 ? 'ready' : 'empty')
    } catch {
      if (requestVersion.current === version) setState('error')
    } finally {
      if (readingRef.current === version) readingRef.current = null
    }
  }, [productId, targetCommentId])

  useEffect(() => {
    readingRef.current = null
    submittingRef.current = false
    pendingSubmission.current = null
    locatedTarget.current = ''
    setItems([])
    setTotal(0)
    setPage(0)
    setDraft('')
    setReplyTarget(null)
    setSubmitting(false)
    setSubmitError('')
    setLoadingMore(false)
    setExpandedThreads(new Set())
    setFocusedCommentId(targetCommentId)
    setExpandedBodies(targetCommentId ? { [targetCommentId]: true } : {})
    setBodyLayouts({})
    void load()
    return () => { requestVersion.current += 1 }
  }, [load])

  const loadMore = useCallback(async () => {
    if (state !== 'ready' || readingRef.current !== null || submittingRef.current || items.length >= total) return
    const version = requestVersion.current
    const nextPage = page + 1
    readingRef.current = version
    setLoadingMore(true)
    setLoadMoreError('')
    try {
      const next = await listProductComments(productId, nextPage, 20)
      if (requestVersion.current !== version) return
      setItems((current) => mergeComments(current, next.items))
      setPage(next.page || nextPage)
      setTotal(next.total)
    } catch {
      if (requestVersion.current === version) setLoadMoreError('更多留言加载失败，点击重试')
    } finally {
      if (readingRef.current === version) readingRef.current = null
      if (requestVersion.current === version) setLoadingMore(false)
    }
  }, [items.length, page, productId, state, total])

  useEffect(() => {
    onLoadMoreReady?.(() => { void loadMore() })
    return () => onLoadMoreReady?.(null)
  }, [loadMore, onLoadMoreReady])

  useEffect(() => {
    const targetKey = productId + ':' + focusedCommentId
    if (!focusedCommentId || state !== 'ready' || locatedTarget.current === targetKey) return
    const root = items.find(item => item.id === focusedCommentId || item.replies.some(reply => reply.id === focusedCommentId))
    if (!root) return
    if (root.id !== focusedCommentId && !expandedThreads.has(root.id)) {
      setExpandedThreads(current => new Set(current).add(root.id))
      return
    }
    // Every visible body before the target can change its vertical position.
    const precedingBodies: string[] = []
    for (const item of items) {
      precedingBodies.push(item.id)
      if (item.id === focusedCommentId) break
      if (expandedThreads.has(item.id)) {
        for (const reply of item.replies) {
          precedingBodies.push(reply.id)
          if (reply.id === focusedCommentId) break
        }
      }
      if (item.id === root.id) break
    }
    if (!layoutReady || (isSheet && (!sheetVisible || presence.phase !== 'open')) || precedingBodies.some(id => !bodyLayouts[id])) return
    let alive = true
    nextTick(() => {
      if (!alive) return
      locatedTarget.current = targetKey
      if (isSheet) setSheetTarget('product-comment-' + focusedCommentId)
      else onTargetReady?.('product-comment-' + focusedCommentId)
    })
    return () => { alive = false }
  }, [bodyLayouts, expandedThreads, focusedCommentId, items, layoutReady, onTargetReady, productId, state, isSheet, sheetVisible, presence.phase])

  const bodyLayoutChanged = useCallback((id: string, ready: boolean) => {
    setBodyLayouts(current => current[id] === ready ? current : { ...current, [id]: ready })
  }, [])

  const beginReply = (comment: ProductComment) => {
    if (productStatus === 'sold' || submittingRef.current || comment.reviewStatus !== 'approved') return
    setReplyTarget({ commentId: comment.id, parentCommentId: comment.parentCommentId || comment.id, authorName: comment.author.nickname })
    if (isSheet) setSheetTarget('')
    pendingSubmission.current = null
    setSubmitError('')
  }

  const submit = async () => {
    if (isSheet && !sheetVisible) return
    const content = draft.trim()
    if (!content || submittingRef.current || readingRef.current !== null || (state !== 'ready' && state !== 'empty') || productStatus === 'sold') return
    submittingRef.current = true
    const version = requestVersion.current
    const submissionKey = productId + ':' + (replyTarget?.commentId ?? 'root') + '\n' + content
    const clientRequestId = pendingSubmission.current?.key === submissionKey ? pendingSubmission.current.clientRequestId : createClientRequestId()
    pendingSubmission.current = { key: submissionKey, clientRequestId }
    setSubmitting(true)
    setSubmitError('')
    try {
      const created = replyTarget
        ? await replyProductComment(productId, replyTarget.commentId, content, clientRequestId)
        : await createProductComment(productId, content, clientRequestId)
      if (requestVersion.current !== version) return
      setItems((current) => replyTarget
        ? upsertReply(current, created, created.parentCommentId || replyTarget.parentCommentId)
        : [...current.filter((value) => value.id !== created.id), created])
      if (!replyTarget && !items.some((item) => item.id === created.id)) setTotal((current) => current + 1)
      setState('ready')
      if (replyTarget) setExpandedThreads(current => new Set(current).add(created.parentCommentId || replyTarget.parentCommentId))
      setFocusedCommentId(created.id)
      setExpandedBodies(current => ({ ...current, [created.id]: true }))
      setDraft('')
      setReplyTarget(null)
      pendingSubmission.current = null
      void Taro.showToast({ title: created.reviewStatus === 'approved' ? '留言成功' : '留言已提交审核', icon: 'success' })
    } catch (error) {
      if (requestVersion.current !== version) return
      if (isApiError(error) && error.status >= 400 && error.status < 500) pendingSubmission.current = null
      const message = submissionError(error)
      setSubmitError(message)
      void Taro.showToast({ title: message, icon: 'none', duration: 1900 })
    } finally {
      if (requestVersion.current === version) {
        submittingRef.current = false
        setSubmitting(false)
      }
    }
  }

  const renderAvatar = (comment: ProductComment) => {
    const key = comment.author.id + ':' + comment.author.avatar
    return <View className='product-comment__avatar'>
      {comment.author.avatar && !failedAvatars.has(key)
        ? <Image src={comment.author.avatar} mode='aspectFill' onError={() => setFailedAvatars((current) => new Set(current).add(key))} />
        : <Text>{comment.author.nickname.charAt(0) || '校'}</Text>}
    </View>
  }
  const sendDisabled = submitting || loadingMore || state === 'loading' || state === 'error' || !draft.trim()
  const content = (
    <View className={'product-comments' + (isSheet ? ' product-comments--sheet' : '')}>
      <View className='product-comments__head'>
        <Text className='product-comments__title'>留言</Text><Text className='product-comments__count'>{page > 0 ? total + ' 条' : ''}</Text>
        {isSheet && <AppIconButton name='close' label='关闭留言' className='product-comments__close' onClick={closeSheet} />}
      </View>
      <ScrollView className='product-comments__list' scrollY={isSheet} enhanced showScrollbar={false} scrollIntoView={isSheet ? sheetTarget : undefined}
        lowerThreshold={120} onScrollToLower={isSheet ? () => void loadMore() : undefined}>
      {state === 'loading' && <Text className='product-comments__state'>正在加载留言…</Text>}
      {state === 'error' && <Text className='product-comments__state product-comments__state--retry' onClick={() => void load()}>留言加载失败，点击重试</Text>}
      {state === 'empty' && <Text className='product-comments__state'>{productStatus === 'sold' ? '暂无留言' : '还没有留言，来问问卖家吧'}</Text>}
      {items.map((item) => (
        <View className='product-comment-thread' id={`product-comment-${item.id}`} key={item.id}>
          <View className='product-comment'>
            {renderAvatar(item)}
            <View className='product-comment__main' onClick={() => beginReply(item)}>
              <View className='product-comment__meta'>
                <Text className='product-comment__author'>{item.author.nickname}</Text>
                {item.isSeller && <Text className='product-comment__seller'>卖家</Text>}
                {reviewLabel(item.reviewStatus) && <Text className={'product-comment__review product-comment__review--' + item.reviewStatus}>{reviewLabel(item.reviewStatus)}</Text>}
              </View>
              <ExpandableText text={item.content} className='product-comment__content' layoutKey={isSheet ? String(sheetVisible) + presence.phase : ''}
                expanded={expandedBodies[item.id] ?? false}
                onLayoutReady={ready => bodyLayoutChanged(item.id, ready)}
                onExpandedChange={value => setExpandedBodies(current => ({ ...current, [item.id]: value }))} />
              <View className='product-comment__foot'><Text className='product-comment__time'>{formatPostDate(item.createdAt)}</Text>{productStatus !== 'sold' && item.reviewStatus === 'approved' && <Text className='product-comment__reply-action'>回复</Text>}</View>
            </View>
          </View>
          {item.replies.length > 0 && <View className='product-comment-thread__replies'>
          <ReplyReveal expanded={expandedThreads.has(item.id)}
            immediate={item.replies.some(reply => reply.id === focusedCommentId) && locatedTarget.current !== productId + ':' + focusedCommentId}>{interactive => <>
          {item.replies.map((reply) => (
            <View className='product-comment product-comment--reply' id={'product-comment-' + reply.id} key={reply.id}>
              {renderAvatar(reply)}
              <View className='product-comment__main' onClick={() => { if (interactive) beginReply(reply) }}>
                <View className='product-comment__meta'>
                  <Text className='product-comment__author'>{reply.author.nickname}</Text>
                  {reply.isSeller && <Text className='product-comment__seller'>卖家</Text>}
                  {reply.replyTo && <Text className='product-comment__reply-to'>回复 {reply.replyTo.nickname}</Text>}
                  {reviewLabel(reply.reviewStatus) && <Text className={'product-comment__review product-comment__review--' + reply.reviewStatus}>{reviewLabel(reply.reviewStatus)}</Text>}
                </View>
                <ExpandableText text={reply.content} className='product-comment__content' disabled={!interactive} layoutKey={isSheet ? String(sheetVisible) + presence.phase : ''}
                  expanded={expandedBodies[reply.id] ?? false}
                  onLayoutReady={ready => bodyLayoutChanged(reply.id, ready)}
                  onExpandedChange={value => setExpandedBodies(current => ({ ...current, [reply.id]: value }))} />
                <View className='product-comment__foot'><Text className='product-comment__time'>{formatPostDate(reply.createdAt)}</Text>{productStatus !== 'sold' && reply.reviewStatus === 'approved' && <Text className='product-comment__reply-action'>回复</Text>}</View>
              </View>
            </View>
          ))}
          </>}</ReplyReveal>
          <View className='product-comments__disclosure' role='button' aria-expanded={expandedThreads.has(item.id)}
            aria-label={expandedThreads.has(item.id) ? '收起商品留言回复' : `查看 ${item.replies.length} 条回复`}
            onClick={() => {
              // A manual disclosure change takes precedence over a pending notification jump.
              if (item.id === focusedCommentId || item.replies.some(reply => reply.id === focusedCommentId)) setFocusedCommentId('')
              setExpandedThreads(current => {
                const next = new Set(current)
                next.has(item.id) ? next.delete(item.id) : next.add(item.id)
                return next
              })
            }}>
            <View className='product-comments__connector' />
            <Text>{expandedThreads.has(item.id) ? '收起回复' : `查看 ${item.replies.length} 条回复`}</Text>
            <AppIcon name='chevron-right' className={expandedThreads.has(item.id) ? 'product-comments__disclosure-icon product-comments__disclosure-icon--open' : 'product-comments__disclosure-icon'} />
          </View>
          </View>}
        </View>
      ))}
      {loadMoreError ? <Text className='product-comments__state product-comments__state--retry' onClick={() => void loadMore()}>{loadMoreError}</Text>
        : loadingMore ? <Text className='product-comments__state'>正在加载更多留言…</Text>
          : items.length > 0 && items.length >= total ? <Text className='product-comments__state'>已显示全部留言</Text> : null}
      </ScrollView>
      <View className='product-comments__composer'>
        {replyTarget && productStatus !== 'sold' && <View className='product-comments__replying'>
          <Text className='product-comments__reply-name'>回复 {replyTarget.authorName}</Text>
          <Text className='product-comments__cancel' onClick={() => { if (submittingRef.current) return; setReplyTarget(null); pendingSubmission.current = null; setSubmitError('') }}>取消</Text>
        </View>}
        {productStatus === 'sold' ? <Text className='product-comments__closed'>商品已售出，不能继续留言</Text> : (
          <View className='product-comments__input-row'>
            <Input className='product-comments__input' value={draft} maxlength={500} adjustPosition={!isSheet} disabled={submitting || (isSheet && !sheetVisible)}
              onFocus={keyboard.onFocus} onBlur={keyboard.reset} onKeyboardHeightChange={keyboard.onKeyboardHeightChange}
              placeholder={replyTarget ? '回复 ' + replyTarget.authorName : '问问卖家，或留下你的看法'} confirmType='send'
              onInput={(event) => { if (submittingRef.current) return; setDraft(String(event.detail.value ?? '')); pendingSubmission.current = null; setSubmitError('') }}
              onConfirm={() => void submit()} />
            <View className={'product-comments__send' + (sendDisabled ? ' product-comments__send--disabled' : '')} onClick={() => void submit()} ariaRole='button' ariaLabel='发送留言'>
              <Text>{submitting ? '发送中' : '发送'}</Text>
            </View>
          </View>
        )}
        {submitError && <Text className='product-comments__error'>{submitError}</Text>}
      </View>
    </View>
  )
  if (!isSheet) return content
  // Retain one comments instance and its draft/page/expanded replies on close.
  // Only the overlay is hidden after its exit animation; the product never remounts.
  return <View className={`product-comments-layer product-comments-layer--${presence.phase}`} style={!presence.mounted ? { display: 'none' } : undefined}>
    <View className='product-comments-layer__mask' onClick={closeSheet} catchMove />
    <View className={'product-comments-sheet' + (drag.dragging ? ' product-comments-sheet--dragging' : '')}
      role='dialog' aria-modal aria-label='商品留言'
      style={{ bottom: `${keyboard.inset}px`, maxHeight: `calc(100% - ${keyboard.inset}px - 24px)`, ...(drag.dragging ? { transform: `translateY(${drag.offset}px)` } : {}) }}>
      <View className='product-comments-sheet__handle' {...drag.handlers} catchMove ariaLabel='下滑关闭留言'><View /></View>
      {content}
    </View>
  </View>
}
