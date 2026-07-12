import React, { useCallback, useEffect, useRef, useState } from 'react'
import TaroRuntime from '@tarojs/taro'
import { Image, Input, Text, View } from '@tarojs/components'
import { isApiError } from '../../services/api'
import { getCommunityPost, getPostCommentsPage } from '../../services/b-community'
import Taro from '../../services/taro'
import type { CommunityComment, CommunityPost } from '../../types'
import './index.scss'

type RuntimeIntersectionObserver = {
  relativeTo(selector: string): RuntimeIntersectionObserver
  observe(selector: string, callback: (entry: { intersectionRatio: number }) => void): void
  disconnect(): void
}

type ObserverRuntime = {
  getCurrentInstance(): { page?: object }
  createIntersectionObserver(page: object, options: { thresholds: number[] }): RuntimeIntersectionObserver
}

const observerRuntime = TaroRuntime as unknown as ObserverRuntime
const commentTones = ['sky', 'peach', 'green', 'mint'] as const

function mergeCommentsById(current: CommunityComment[], incoming: CommunityComment[]) {
  const values = new Map(current.map((item) => [item.id, item]))
  incoming.forEach((item) => values.set(item.id, item))
  return Array.from(values.values())
}

function commentToneFor(id: string) {
  const value = Array.from(id).reduce((hash, char) => ((hash * 31) + char.charCodeAt(0)) >>> 0, 7)
  return commentTones[value % commentTones.length]
}

function showReadOnlyToast() {
  void Taro.showToast({ title: '互动功能下一阶段接入', icon: 'none', duration: 1400 })
}

export default function PostDetailPage() {
  const requestedId = Taro.getCurrentInstance().router?.params?.id ?? ''
  const validId = /^[1-9]\d*$/.test(requestedId)
  const [post, setPost] = useState<CommunityPost | null>(null)
  const [comments, setComments] = useState<CommunityComment[]>([])
  const [status, setStatus] = useState<'loading' | 'ready' | 'empty' | 'error'>(
    validId ? 'loading' : 'empty'
  )
  const [commentsPage, setCommentsPage] = useState(0)
  const [commentsTotal, setCommentsTotal] = useState(0)
  const [commentsError, setCommentsError] = useState(false)
  const [failedImageKeys, setFailedImageKeys] = useState<Set<string>>(() => new Set())
  const [reloadKey, setReloadKey] = useState(0)
  const [draft, setDraft] = useState('')
  const requestVersion = useRef(0)
  const commentLoadingRef = useRef(false)
  const retryPost = useCallback(() => setReloadKey((value) => value + 1), [])

  useEffect(() => {
    if (!validId) {
      setPost(null)
      setComments([])
      setCommentsPage(0)
      setCommentsTotal(0)
      setStatus('empty')
      return undefined
    }
    const version = requestVersion.current + 1
    requestVersion.current = version
    let active = true
    commentLoadingRef.current = false
    setPost(null)
    setComments([])
    setCommentsPage(0)
    setCommentsTotal(0)
    setStatus('loading')
    setCommentsError(false)

    void Promise.all([
      getCommunityPost(requestedId),
      getPostCommentsPage(requestedId, 1)
    ]).then(([nextPost, nextComments]) => {
      if (!active || requestVersion.current !== version) return
      setPost(nextPost)
      setComments(nextComments.items)
      setCommentsPage(nextComments.page)
      setCommentsTotal(nextComments.total)
      setStatus('ready')
    }).catch((error: unknown) => {
      if (!active || requestVersion.current !== version) return
      setPost(null)
      setComments([])
      setCommentsPage(0)
      setCommentsTotal(0)
      setStatus(isApiError(error) && error.status === 404 ? 'empty' : 'error')
    })

    return () => {
      active = false
      requestVersion.current += 1
    }
  }, [reloadKey, requestedId, validId])

  const commentPagesExhausted = commentsPage > 0 && commentsPage * 20 >= commentsTotal

  const loadMoreComments = useCallback(async () => {
    if (!post || commentLoadingRef.current || commentsTotal === 0 || commentPagesExhausted) return
    const version = requestVersion.current
    commentLoadingRef.current = true
    setCommentsError(false)
    try {
      const result = await getPostCommentsPage(post.id, commentsPage + 1)
      if (requestVersion.current !== version) return
      setComments((current) => mergeCommentsById(current, result.items))
      setCommentsPage(result.page)
      setCommentsTotal(result.total)
    } catch {
      if (requestVersion.current === version) setCommentsError(true)
    } finally {
      if (requestVersion.current === version) commentLoadingRef.current = false
    }
  }, [commentPagesExhausted, commentsPage, commentsTotal, post])

  useEffect(() => {
    if (status !== 'ready' || commentsError || commentsTotal === 0 || commentPagesExhausted) return undefined
    const page = observerRuntime.getCurrentInstance().page
    if (!page) return undefined
    const observer = observerRuntime.createIntersectionObserver(page, { thresholds: [0] })
    observer.relativeTo('.post-detail-scroll').observe('#post-comments-sentinel', (entry) => {
      if (entry.intersectionRatio > 0) void loadMoreComments()
    })
    return () => observer.disconnect()
  }, [commentPagesExhausted, commentsError, commentsTotal, loadMoreComments, status])

  const goBack = () => {
    void Taro.navigateBack().catch(() => {
      void Taro.redirectTo({ url: '/pages/home/index' })
    })
  }

  const openAuthorProfile = () => {
    if (!post?.authorId) return
    void Taro.navigateTo({
      url: `/pages/user-profile/index?userId=${encodeURIComponent(post.authorId)}&source=post`
    })
  }

  const openTopicSearch = () => {
    if (!post?.topic) return
    void Taro.navigateTo({
      url: `/pages/search/index?keyword=${encodeURIComponent(post.topic.replace(/^#/, ''))}`
    })
  }

  const renderState = () => {
    const copy = status === 'loading'
      ? { title: '正在加载帖子', description: '请稍候...' }
      : status === 'empty'
        ? { title: '帖子不存在或已下架', description: '请返回首页重新选择。' }
        : { title: '加载失败，点击重试', description: '网络开小差了，请稍后再试。' }
    return (
      <View
        className={`post-detail-state ${status === 'error' ? 'post-detail-state--retry' : ''}`}
        onClick={status === 'error' ? retryPost : undefined}
      >
        <Text className='post-detail-state__title'>{copy.title}</Text>
        <Text className='post-detail-state__desc'>{copy.description}</Text>
      </View>
    )
  }

  const readyPost = status === 'ready' ? post : null
  const authorAvatar = readyPost?.author.trim().slice(0, 1) || '校'
  const authorMeta = readyPost?.school || '校园社区'
  const firstImage = readyPost?.images[0]
  const topicLabel = readyPost?.topic
    ? `#${readyPost.topic.replace(/^#/, '')}`
    : ''
  const contentMeta = readyPost
    ? [readyPost.createdAt ? `${readyPost.createdAt} 发布` : '', readyPost.school].filter(Boolean).join(' · ')
    : ''

  return (
    <View className='post-detail-page'>
      <View className='post-detail-nav'>
        <View className='post-detail-nav__back' onClick={goBack} />
        <View className='post-detail-author' onClick={readyPost?.authorId ? openAuthorProfile : undefined}>
          <View className='post-detail-author__avatar'>
            <Text>{authorAvatar}</Text>
          </View>
          <View className='post-detail-author__main'>
            <Text className='post-detail-author__name'>{readyPost?.author || '帖子详情'}</Text>
            <Text className='post-detail-author__meta'>{authorMeta}</Text>
          </View>
        </View>
        {readyPost && (
          <View className='post-detail-follow' onClick={showReadOnlyToast}>
            <Text>关注</Text>
          </View>
        )}
      </View>

      <View className='post-detail-scroll'>
        {!readyPost ? renderState() : (
          <>
            <View className='post-detail-hero'>
              {firstImage && !failedImageKeys.has(readyPost.id) ? (
                <Image
                  className='post-detail-hero__image'
                  src={firstImage}
                  mode='aspectFill'
                  onError={() => setFailedImageKeys((current) => {
                    const next = new Set(current)
                    next.add(readyPost.id)
                    return next
                  })}
                />
              ) : (
                <Text>{firstImage ? '图 · 图片加载失败' : '图 · 作者未上传图片'}</Text>
              )}
            </View>

            <View className='post-detail-content'>
              <Text className='post-detail-content__title'>{readyPost.title}</Text>
              <Text className='post-detail-content__body'>{readyPost.content}</Text>
              {topicLabel && (
                <View className='post-detail-tags'>
                  <View className='post-detail-tag' onClick={openTopicSearch}>
                    <Text>{topicLabel}</Text>
                  </View>
                </View>
              )}
              <Text className='post-detail-content__meta'>{contentMeta || '发布时间未知'}</Text>
            </View>

            <View className='post-detail-comments'>
              <Text className='post-detail-comments__title'>共 {commentsTotal} 条评论</Text>
              {comments.map((item) => (
                <View key={item.id} className='post-detail-comment'>
                  <View className={`post-detail-comment__avatar post-detail-comment__avatar--${commentToneFor(item.id)}`}>
                    <Text>{item.author.trim().slice(0, 1) || '校'}</Text>
                  </View>
                  <View className='post-detail-comment__main'>
                    <Text className='post-detail-comment__author'>{item.author}</Text>
                    <Text className='post-detail-comment__content'>{item.content}</Text>
                    <View className='post-detail-comment__meta'>
                      <Text>{item.createdAt}</Text>
                      <Text onClick={showReadOnlyToast}>回复</Text>
                    </View>
                  </View>
                  <View className='post-detail-comment__like' onClick={showReadOnlyToast}>
                    <Text>♡</Text>
                  </View>
                </View>
              ))}
              {commentsTotal === 0 ? (
                <Text className='post-detail-comments__bottom'>暂时没有评论</Text>
              ) : commentsError ? (
                <View className='post-detail-comments__bottom post-detail-comments__bottom--retry' onClick={() => void loadMoreComments()}>
                  <Text>评论加载失败，点击重试</Text>
                </View>
              ) : commentPagesExhausted ? (
                <Text className='post-detail-comments__bottom'>— 到底了 —</Text>
              ) : (
                <View id='post-comments-sentinel' className='post-detail-comments__sentinel'>
                  <Text>继续上滑加载评论</Text>
                </View>
              )}
            </View>
          </>
        )}
      </View>

      {readyPost && (
        <View className='post-detail-actionbar'>
          <View className='post-detail-input'>
            <Input
              value={draft}
              placeholder='说点什么...'
              confirmType='send'
              onInput={(event) => setDraft(String(event.detail.value))}
              onConfirm={showReadOnlyToast}
            />
          </View>
          <View className='post-detail-action' onClick={showReadOnlyToast}>
            <Text className='post-detail-action__icon'>♡</Text>
            <Text>{readyPost.likedCount}</Text>
          </View>
          <View className='post-detail-action' onClick={showReadOnlyToast}>
            <Text className='post-detail-action__icon'>☆</Text>
            <Text>{readyPost.favoriteCount}</Text>
          </View>
          <View className='post-detail-action' onClick={showReadOnlyToast}>
            <Text className='post-detail-action__icon'>▱</Text>
            <Text>{readyPost.commentCount}</Text>
          </View>
        </View>
      )}
    </View>
  )
}
