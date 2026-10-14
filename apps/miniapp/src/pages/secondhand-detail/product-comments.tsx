import React, { useCallback, useEffect, useRef, useState } from 'react'
import { Image, Input, Text, View } from '@tarojs/components'
import { isApiError } from '../../services/api'
import { createProductComment, listProductComments, replyProductComment } from '../../services/c-trade'
import { createClientRequestId } from '../../services/client-request-id'
import Taro from '../../services/taro'
import type { Product, ProductComment } from '../../types'
import { formatPostDate } from '../../utils/post-presentation'
import './product-comments.scss'

type CommentLoadState = 'loading' | 'ready' | 'empty' | 'error'
type ReplyTarget = { commentId: string; parentCommentId: string; authorName: string } | null
interface ProductCommentsProps {
  productId: string
  productStatus: Product['status']
  targetCommentId?: string
  onTargetReady?: (elementId: string) => void
  onLoadMoreReady?: (handler: (() => void) | null) => void
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

export default function ProductComments({ productId, productStatus, targetCommentId = '', onTargetReady, onLoadMoreReady }: ProductCommentsProps) {
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
  const pendingSubmission = useRef<PendingSubmission | null>(null)
  const requestVersion = useRef(0)
  const readingRef = useRef<number | null>(null)
  const submittingRef = useRef(false)
  const locatedTarget = useRef('')

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
    const targetKey = productId + ':' + targetCommentId
    if (!targetCommentId || state !== 'ready' || locatedTarget.current === targetKey) return
    if (items.some((item) => item.id === targetCommentId || item.replies.some((reply) => reply.id === targetCommentId))) {
      locatedTarget.current = targetKey
      onTargetReady?.('product-comment-' + targetCommentId)
    }
  }, [items, onTargetReady, productId, state, targetCommentId])

  const beginReply = (comment: ProductComment) => {
    if (productStatus === 'sold' || submittingRef.current) return
    setReplyTarget({ commentId: comment.id, parentCommentId: comment.parentCommentId || comment.id, authorName: comment.author.nickname })
    pendingSubmission.current = null
    setSubmitError('')
  }

  const submit = async () => {
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
  return (
    <View className='product-comments'>
      <View className='product-comments__head'><Text className='product-comments__title'>留言</Text><Text className='product-comments__count'>{page > 0 ? total + ' 条' : ''}</Text></View>
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
              <Text className='product-comment__content'>{item.content}</Text>
              <View className='product-comment__foot'><Text className='product-comment__time'>{formatPostDate(item.createdAt)}</Text>{productStatus !== 'sold' && <Text className='product-comment__reply-action'>回复</Text>}</View>
            </View>
          </View>
          {item.replies.map((reply) => (
            <View className='product-comment product-comment--reply' id={'product-comment-' + reply.id} key={reply.id}>
              {renderAvatar(reply)}
              <View className='product-comment__main' onClick={() => beginReply(reply)}>
                <View className='product-comment__meta'>
                  <Text className='product-comment__author'>{reply.author.nickname}</Text>
                  {reply.isSeller && <Text className='product-comment__seller'>卖家</Text>}
                  {reply.replyTo && <Text className='product-comment__reply-to'>回复 {reply.replyTo.nickname}</Text>}
                  {reviewLabel(reply.reviewStatus) && <Text className={'product-comment__review product-comment__review--' + reply.reviewStatus}>{reviewLabel(reply.reviewStatus)}</Text>}
                </View>
                <Text className='product-comment__content'>{reply.content}</Text>
                <View className='product-comment__foot'><Text className='product-comment__time'>{formatPostDate(reply.createdAt)}</Text>{productStatus !== 'sold' && <Text className='product-comment__reply-action'>回复</Text>}</View>
              </View>
            </View>
          ))}
        </View>
      ))}
      {loadMoreError ? <Text className='product-comments__state product-comments__state--retry' onClick={() => void loadMore()}>{loadMoreError}</Text>
        : loadingMore ? <Text className='product-comments__state'>正在加载更多留言…</Text>
          : items.length > 0 && items.length >= total ? <Text className='product-comments__state'>已显示全部留言</Text> : null}
      <View className='product-comments__composer'>
        {replyTarget && productStatus !== 'sold' && <View className='product-comments__replying'>
          <Text className='product-comments__reply-name'>回复 {replyTarget.authorName}</Text>
          <Text className='product-comments__cancel' onClick={() => { if (submittingRef.current) return; setReplyTarget(null); pendingSubmission.current = null; setSubmitError('') }}>取消</Text>
        </View>}
        {productStatus === 'sold' ? <Text className='product-comments__closed'>商品已售出，不能继续留言</Text> : (
          <View className='product-comments__input-row'>
            <Input className='product-comments__input' value={draft} maxlength={500} adjustPosition disabled={submitting}
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
}
