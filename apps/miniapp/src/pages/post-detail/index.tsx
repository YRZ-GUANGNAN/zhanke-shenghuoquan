import { withFeaturePage } from '../../components/feature-page-gate'
import { useFeatureVisibility } from '../../hooks/use-feature-visibility'
import { usePageShare } from '../../hooks/use-page-share'
import { publicShareId } from '../../services/share-model'
import { useEffect, useMemo, useRef, useState } from 'react'
import { nextTick } from '@tarojs/taro'
import { Image, Input, ScrollView, Swiper, SwiperItem, Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import AppIcon from '../../components/app-icon'
import { AppIconButton, AppState } from '../../components/ui'
import Taro from '../../services/taro'
import type { CommunityComment, CommunityPost } from '../../types'
import { isApiError } from '../../services/api'
import {
  addPostComment,
  createCommunityClientRequestId,
  getCommunityPost,
  getCommunityViewerState,
  getMyCommunityPost,
  getPostCommentsPage,
  setPostFavorited,
  setPostLiked,
  setUserFollowing,
} from '../../services/b-community'
import { getCapsuleSafeStyle } from '../../utils/capsule-safe'
import { navigateBackOrHome } from '../../utils/navigation'
import { formatPostDate, postBodyWithoutTitle, preparePostMedia, removeFailedPostMedia, type PostGalleryState, type PostMedia } from '../../utils/post-presentation'
import { useGalleryPresentation } from '../../hooks/use-gallery-presentation'
import './index.scss'

const COMMENT_PAGE_SIZE = 20

interface PendingCommentSubmission {
  key: string
  clientRequestId: string
}

function mergeComments(current: CommunityComment[], incoming: CommunityComment[]): CommunityComment[] {
  const byId = new Map(current.map((comment) => [comment.id, comment]))
  incoming.forEach((comment) => byId.set(comment.id, comment))
  return Array.from(byId.values())
}

const showToast = (title: string, icon: 'success' | 'error' | 'none' = 'none') => {
  void Taro.showToast({ title, icon, duration: 1600 })
}

const reviewCopy: Record<NonNullable<CommunityPost['reviewStatus']>, { title: string; description: string }> = {
  submitted: {
    title: '帖子已提交审核',
    description: '这是你刚发布的真实内容，审核通过后会出现在首页。',
  },
  reviewing: {
    title: '帖子正在审核',
    description: '这是你发布的真实内容，平台正在处理，请稍候。',
  },
  approved: {
    title: '',
    description: '',
  },
  rejected: {
    title: '帖子未通过审核',
    description: '这条内容仅你自己可见，可返回“我的帖子”查看状态。',
  },
  taken_down: {
    title: '帖子已下架',
    description: '这条内容已停止公开展示。',
  },
}

const readableError = (error: unknown): string => {
  if (isApiError(error)) {
    if (error.status === 404) return '帖子不存在、尚未完成提交，或你没有查看权限'
    if (error.status === 401) return '登录状态已失效，请返回后重新进入'
    if (error.status === 0) return '网络连接失败，请检查网络后重试'
    return error.message || '帖子读取失败，请稍后重试'
  }
  return error instanceof Error ? error.message : '帖子读取失败，请稍后重试'
}

function PostDetailPage() {
  const visibility = useFeatureVisibility()
  const params = Taro.getCurrentInstance().router?.params ?? {}
  const postId = publicShareId(params.id)
  const [post, setPost] = useState<CommunityPost | null>(null)
  const [comments, setComments] = useState<CommunityComment[]>([])
  const [commentPage, setCommentPage] = useState(0)
  const [commentTotal, setCommentTotal] = useState(0)
  const [commentsLoading, setCommentsLoading] = useState(false)
  const [commentsLoadingMore, setCommentsLoadingMore] = useState(false)
  const [liked, setLiked] = useState(false)
  const [favorited, setFavorited] = useState(false)
  const [following, setFollowing] = useState(false)
  const [commentText, setCommentText] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [commentError, setCommentError] = useState('')
  const [actioning, setActioning] = useState(false)
  const [viewerLoading, setViewerLoading] = useState(true)
  const [submittingComment, setSubmittingComment] = useState(false)
  const [reloadKey, setReloadKey] = useState(0)
  const [gallery, setGallery] = useState<PostGalleryState>({ items: [], index: 0 })
  const [mediaPending, setMediaPending] = useState(false)
  const [preparedMediaKey, setPreparedMediaKey] = useState('')
  const [mediaSize, setMediaSize] = useState<PostMedia | undefined>()
  const [commentsAnchor, setCommentsAnchor] = useState('')
  const pendingComment = useRef<PendingCommentSubmission | null>(null)
  const actionLock = useRef(false)
  const commentLock = useRef(false)
  const commentsLoadLock = useRef(false)
  const detailVersion = useRef(0)

  const { items: media, index: mediaIndex } = gallery
  const mediaKey = JSON.stringify([post?.id, post?.images])
  const showHero = media.length > 0
  const body = post ? postBodyWithoutTitle(post.title, post.content) : ''
  const tags = useMemo(() => {
    if (!post) return []
    const values = [post.topic, ...(post.tags ?? [])]
      .map((value) => value.replace(/^#+/, '').trim())
      .filter(Boolean)
    return Array.from(new Set(values))
  }, [post])
  const isApproved = post?.reviewStatus === 'approved'
  const hasActions = isApproved && (visibility.canComment || visibility.comment || visibility.likes || visibility.favorites)
  const galleryView = useGalleryPresentation(mediaKey, media.length, mediaSize, 'post', hasActions)
  const interactionPending = actioning || viewerLoading
  const reviewState = post?.reviewStatus ? reviewCopy[post.reviewStatus] : null
  usePageShare({ kind: 'post', enabled: visibility.forum && !loading && !error, id: postId, content: post })

  useEffect(() => {
    let alive = true
    const images = post?.images ?? []
    setGallery({ items: [], index: 0 })
    setMediaSize(undefined)
    setMediaPending(images.length > 0)
    void preparePostMedia(images, (src) => Taro.getImageInfo({ src })).then((items) => {
      if (!alive) return
      setGallery({ items, index: 0 })
      setMediaSize(items[0])
      setPreparedMediaKey(mediaKey)
      setMediaPending(false)
    })

    return () => {
      alive = false
    }
  }, [mediaKey])

  useEffect(() => {
    if (!loading && !commentsLoading && preparedMediaKey === mediaKey && isApproved && visibility.comment && params.anchor === 'comments') {
      nextTick(() => setCommentsAnchor('post-comments'))
    }
  }, [loading, commentsLoading, preparedMediaKey, mediaKey, isApproved, visibility.comment, params.anchor])

  const showComments = () => {
    setCommentsAnchor('')
    nextTick(() => setCommentsAnchor('post-comments'))
  }

  const loadCommentsPage = async (id: string, nextPage: number, version: number) => {
    if (commentsLoadLock.current || commentLock.current) return
    commentsLoadLock.current = true
    setCommentsLoading(nextPage === 1)
    setCommentsLoadingMore(nextPage > 1)
    setCommentError('')
    try {
      const result = await getPostCommentsPage(id, { page: nextPage, pageSize: COMMENT_PAGE_SIZE })
      if (detailVersion.current !== version) return
      setComments((current) => nextPage === 1 ? result.items : mergeComments(current, result.items))
      setCommentPage(result.page || nextPage)
      setCommentTotal(result.total)
    } catch (loadError) {
      if (detailVersion.current !== version) return
      console.error('load post comments failed', loadError)
      setCommentError(nextPage === 1 ? '评论暂时无法读取，点击重试' : '更多评论加载失败，点击重试')
    } finally {
      if (detailVersion.current === version) {
        commentsLoadLock.current = false
        setCommentsLoading(false)
        setCommentsLoadingMore(false)
      }
    }
  }

  useEffect(() => {
    let alive = true
    const version = ++detailVersion.current
    commentsLoadLock.current = false
    const load = async () => {
      setLoading(true)
      setError('')
      setCommentError('')
      setPost(null)
      setComments([])
      setCommentPage(0)
      setCommentTotal(0)
      setCommentsLoading(false)
      setCommentsLoadingMore(false)
      setViewerLoading(true)
      setCommentsAnchor('')
      if (!postId) {
        setError('帖子链接缺少编号，无法读取')
        setLoading(false)
        return
      }

      try {
        let nextPost: CommunityPost
        try {
          nextPost = await getCommunityPost(postId)
        } catch (publicError) {
          try {
            nextPost = await getMyCommunityPost(postId)
          } catch {
            throw publicError
          }
        }
        if (!alive) return
        setPost(nextPost)
        setLoading(false)

        if (nextPost.reviewStatus === 'approved') {
          await Promise.all([
            visibility.comment ? loadCommentsPage(postId, 1, version) : Promise.resolve(),
            getCommunityViewerState(postId).catch(() => ({
              liked: false,
              favorited: false,
              followingAuthor: false,
            })).then((viewerState) => {
              if (!alive) return
              setLiked(viewerState.liked)
              setFavorited(viewerState.favorited)
              setFollowing(viewerState.followingAuthor)
              setViewerLoading(false)
            }),
          ])
        }
      } catch (loadError) {
        console.error('load post detail failed', loadError)
        if (alive) setError(readableError(loadError))
      } finally {
        if (alive) setLoading(false)
      }
    }
    void load()
    return () => {
      alive = false
      detailVersion.current += 1
    }
  }, [postId, reloadKey, visibility.comment])

  const loadMoreComments = async () => {
    if (!visibility.comment || !post || !isApproved || commentPage === 0 || comments.length >= commentTotal) return
    await loadCommentsPage(post.id, commentPage + 1, detailVersion.current)
  }

  const retryComments = () => {
    if (!visibility.comment || !post || !isApproved) return
    void loadCommentsPage(post.id, commentPage === 0 ? 1 : commentPage + 1, detailVersion.current)
  }

  const withActionGuard = async (run: () => Promise<void>) => {
    if (actionLock.current || viewerLoading || !post || !isApproved) return
    actionLock.current = true
    setActioning(true)
    try {
      await run()
    } catch (actionError) {
      console.error('post action failed', actionError)
      showToast('操作失败，请稍后再试', 'error')
    } finally {
      actionLock.current = false
      setActioning(false)
    }
  }

  const handleFollow = () =>
    withActionGuard(async () => {
      if (!post || !visibility.following) return
      const next = !following
      await setUserFollowing(post.authorId, next)
      setFollowing(next)
      showToast(next ? '已关注' : '已取消关注', 'success')
    })

  const handleLike = () =>
    withActionGuard(async () => {
      if (!post || !visibility.likes) return
      const next = !liked
      await setPostLiked(post.id, next)
      setLiked(next)
      setPost((current) => current ? { ...current, likedCount: Math.max(0, current.likedCount + (next ? 1 : -1)) } : current)
    })

  const handleFavorite = () =>
    withActionGuard(async () => {
      if (!post || !visibility.favorites) return
      const next = !favorited
      await setPostFavorited(post.id, next)
      setFavorited(next)
      setPost((current) => current ? { ...current, favoriteCount: Math.max(0, current.favoriteCount + (next ? 1 : -1)) } : current)
    })

  const handleSubmitComment = async () => {
    if (!visibility.canComment || !post || !isApproved) return
    if (commentsLoadLock.current) {
      showToast('评论正在读取，请稍候')
      return
    }
    const content = commentText.trim()
    if (!content) {
      showToast('先写一句评论吧')
      return
    }
    if (commentLock.current) return
    commentLock.current = true
    const clientRequestId = pendingComment.current?.key === content
      ? pendingComment.current.clientRequestId
      : createCommunityClientRequestId()
    pendingComment.current = { key: content, clientRequestId }
    const version = detailVersion.current
    setSubmittingComment(true)
    try {
      const comment = await addPostComment(post.id, content, clientRequestId)
      if (detailVersion.current !== version) return
      setComments((current) => mergeComments([comment], current))
      setCommentTotal((current) => current + 1)
      setPost((current) => current ? { ...current, commentCount: current.commentCount + 1 } : current)
      setCommentText('')
      pendingComment.current = null
      showToast('评论已提交审核', 'success')
    } catch (commentSubmitError) {
      if (detailVersion.current !== version) return
      console.error('submit comment failed', commentSubmitError)
      if (isApiError(commentSubmitError) && commentSubmitError.status >= 400 && commentSubmitError.status < 500) {
        pendingComment.current = null
      }
      showToast('评论失败，请稍后再试', 'error')
    } finally {
      commentLock.current = false
      if (detailVersion.current === version) setSubmittingComment(false)
    }
  }

  const headerCenter = post ? (
    <View className='post-detail-author'>
      <View className='post-detail-author__avatar'>
        {post.avatar ? <Image src={post.avatar} mode='aspectFill' /> : (post.author || '校').slice(0, 1)}
      </View>
      <View className='post-detail-author__main'>
        <Text className='post-detail-author__name'>{post.author || '校园用户'}</Text>
        <Text className='post-detail-author__meta'>{post.school || '校园用户'}</Text>
      </View>
    </View>
  ) : <Text className='post-detail-nav__title'>帖子详情</Text>

  return (
    <View className="post-detail-page" style={getCapsuleSafeStyle()}>
      <CapsulePageHeader
        className='post-detail-nav'
        variant='standard'
        left={<AppIconButton name='back' label='返回' className='post-detail-nav__back' onClick={() => void navigateBackOrHome()} />}
        center={headerCenter}
        right={post && isApproved && visibility.following ? (
          <View className={`post-detail-follow-hitbox${interactionPending ? ' post-detail-follow-hitbox--pending' : ''}`} role='button' aria-disabled={interactionPending} aria-label={following ? '取消关注' : '关注作者'} onClick={handleFollow}>
            <Text className={`post-detail-follow${following ? ' post-detail-follow--active' : ''}`}>
              {following ? '已关注' : '关注'}
            </Text>
          </View>
        ) : <View className='post-detail-nav__spacer' />}
      />

      {loading ? (
        <AppState kind='loading' title='正在读取帖子' description='请稍候…' />
      ) : error || !post ? (
        <AppState
          kind='error'
          title='帖子读取失败'
          description={error || '没有读取到帖子内容'}
          actionText='重新读取'
          onAction={() => setReloadKey((value) => value + 1)}
        />
      ) : (
        <>
          <ScrollView
            className={`post-detail-scroll${hasActions ? ' post-detail-scroll--with-actions' : ''}`}
            scrollY
            scrollIntoView={commentsAnchor}
            scrollWithAnimation
            enhanced
            showScrollbar={false}
            lowerThreshold={120}
            onScrollToLower={() => void loadMoreComments()}
          >
            {reviewState?.title ? (
              <View className={`post-detail-review post-detail-review--${post.reviewStatus}`}>
                <Text className="post-detail-review__title">{reviewState.title}</Text>
                <Text className="post-detail-review__desc">{reviewState.description}</Text>
              </View>
            ) : null}

            {mediaPending ? (
              <View className='post-detail-hero post-detail-hero--loading' style={{ height: `${galleryView.height}px` }}><Text>正在读取图片…</Text></View>
            ) : showHero ? (
              <View className='post-detail-hero' style={{ height: `${galleryView.height}px` }} onTouchStart={galleryView.onTouchStart} onTouchEnd={galleryView.onTouchEnd} onTouchCancel={galleryView.onTouchEnd}>
                <Swiper
                  className='post-detail-gallery'
                  current={Math.min(mediaIndex, media.length - 1)}
                  circular={false}
                  duration={200}
                  onChange={(event) => {
                    if (event.detail.source !== 'touch') return
                    setGallery((current) => ({ ...current, index: Math.max(0, Math.min(event.detail.current, current.items.length - 1)) }))
                    galleryView.revealCounter()
                  }}
                >
                  {media.map((image) => (
                    <SwiperItem key={image.src}>
                      <Image
                        className='post-detail-hero__image'
                        src={image.src}
                        mode='aspectFit'
                        onError={() => setGallery((current) => removeFailedPostMedia(current, image.src))}
                      />
                    </SwiperItem>
                  ))}
                </Swiper>
                {media.length > 1 && <Text className={`post-detail-gallery__count${galleryView.counterVisible ? ' post-detail-gallery__count--visible' : ''}`}>{Math.min(mediaIndex + 1, media.length)} / {media.length}</Text>}
              </View>
            ) : post.images.length > 0 ? (
              <View className='post-detail-hero post-detail-media-error' style={{ height: `${galleryView.height}px` }}>
                <Text>图片暂时无法显示，正文仍可阅读</Text>
              </View>
            ) : null}

            <View className={`post-detail-content${post.images.length > 0 ? '' : ' post-detail-content--without-media'}`}>
              <Text className="post-detail-content__title">{post.title || '校园动态'}</Text>
              {body ? <Text className="post-detail-content__body" selectable>{body}</Text> : null}
              {tags.length > 0 ? (
                <View className="post-detail-tags">
                  {tags.map((tag) => <Text className="post-detail-tag" key={tag}>#{tag}</Text>)}
                </View>
              ) : null}
              <Text className="post-detail-content__meta">
                {[post.createdAt ? `${formatPostDate(post.createdAt)} 发布` : '', post.school].filter(Boolean).join(' · ')}
              </Text>
            </View>

            {isApproved && visibility.comment ? (
              <View className="post-detail-comments" id='post-comments'>
                <Text className="post-detail-comments__title">共 {commentPage > 0 ? commentTotal : post.commentCount} 条评论</Text>
                {comments.map((comment) => (
                  <View className="post-detail-comment" key={comment.id}>
                    <View className='post-detail-comment__avatar'>
                      {comment.avatar ? <Image src={comment.avatar} mode='aspectFill' /> : (comment.author || '同').slice(0, 1)}
                    </View>
                    <View className="post-detail-comment__main">
                      <Text className="post-detail-comment__author">{comment.author || '同学'}</Text>
                      <Text className="post-detail-comment__content">{comment.content}</Text>
                      <Text className="post-detail-comment__meta">{formatPostDate(comment.createdAt)}</Text>
                    </View>
                  </View>
                ))}
                {commentsLoading ? <Text className='post-detail-comments__bottom'>正在读取评论…</Text> : commentError ? (
                  <Text
                    className="post-detail-comments__bottom post-detail-comments__bottom--retry"
                    onClick={retryComments}
                  >
                    {commentError}
                  </Text>
                ) : comments.length === 0 ? (
                  <View className="post-detail-comments__empty">
                    <Text className="post-detail-comments__empty-title">还没有评论</Text>
                    <Text className="post-detail-comments__empty-desc">说说你的看法，成为第一个评论的人</Text>
                  </View>
                ) : commentsLoadingMore ? (
                  <Text className="post-detail-comments__bottom">正在加载更多评论…</Text>
                ) : comments.length >= commentTotal ? (
                  <Text className="post-detail-comments__bottom">— 到底了 —</Text>
                ) : null}
              </View>
            ) : null}
          </ScrollView>

          {hasActions ? (
            <View className="post-detail-actionbar">
              {visibility.canComment && <Input
                className="post-detail-input"
                value={commentText}
                disabled={submittingComment}
                placeholder={submittingComment ? '发送中…' : '说点什么…'}
                onInput={(event) => {
                  setCommentText(event.detail.value)
                  if (pendingComment.current?.key !== event.detail.value.trim()) pendingComment.current = null
                }}
                confirmType="send"
                onConfirm={handleSubmitComment}
              />}
              {visibility.canComment && commentText.trim() ? (
                <View className={`post-detail-send${submittingComment ? ' post-detail-send--pending' : ''}`} role='button' aria-label='发送评论' onClick={handleSubmitComment}>
                  <Text>{submittingComment ? '发送中' : '发送'}</Text>
                </View>
              ) : <>
              {visibility.likes && <View className={`post-detail-action${liked ? ' post-detail-action--active' : ''}${interactionPending ? ' post-detail-action--pending' : ''}`} role='button' aria-disabled={interactionPending} aria-label={`${liked ? '取消点赞' : '点赞'}，${post.likedCount}`} onClick={handleLike}>
                <AppIcon name='heart' active={liked} className='post-detail-action__icon' />
                <Text>{post.likedCount}</Text>
              </View>}
              {visibility.favorites && <View className={`post-detail-action${favorited ? ' post-detail-action--active post-detail-action--favorite' : ''}${interactionPending ? ' post-detail-action--pending' : ''}`} role='button' aria-disabled={interactionPending} aria-label={`${favorited ? '取消收藏' : '收藏'}，${post.favoriteCount}`} onClick={handleFavorite}>
                <AppIcon name='star' active={favorited} className='post-detail-action__icon' />
                <Text>{post.favoriteCount}</Text>
              </View>}
              {visibility.comment && <View className="post-detail-action" role='button' aria-label='查看评论' onClick={showComments}>
                <AppIcon name='comment' className='post-detail-action__icon' />
                <Text>{post.commentCount || comments.length}</Text>
              </View>}
              </>}
            </View>
          ) : null}
        </>
      )}
    </View>
  )
}

export default withFeaturePage(PostDetailPage, RuntimeSessionGate)
