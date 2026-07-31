import { useEffect, useMemo, useState } from 'react'
import { Image, Input, Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import AppIcon from '../../components/app-icon'
import { AppIconButton, AppState } from '../../components/ui'
import Taro from '../../services/taro'
import type { CommunityComment, CommunityPost } from '../../types'
import { isApiError } from '../../services/api'
import {
  addPostComment,
  getCommunityPost,
  getCommunityViewerState,
  getMyCommunityPost,
  listPostComments,
  setPostFavorited,
  setPostLiked,
  setUserFollowing,
} from '../../services/b-community'
import { getCapsuleSafeStyle } from '../../utils/capsule-safe'
import './index.scss'

const commentTone = ['sky', 'peach', 'green', 'mint'] as const

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

export default function PostDetailPage() {
  const params = Taro.getCurrentInstance().router?.params ?? {}
  const postId = params.id ? decodeURIComponent(params.id) : ''
  const [post, setPost] = useState<CommunityPost | null>(null)
  const [comments, setComments] = useState<CommunityComment[]>([])
  const [liked, setLiked] = useState(false)
  const [favorited, setFavorited] = useState(false)
  const [following, setFollowing] = useState(false)
  const [commentText, setCommentText] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [commentError, setCommentError] = useState('')
  const [actioning, setActioning] = useState(false)
  const [submittingComment, setSubmittingComment] = useState(false)
  const [reloadKey, setReloadKey] = useState(0)
  const [heroAvailable, setHeroAvailable] = useState(false)

  const heroImage = post?.images[0] ?? ''
  const showHero = Boolean(heroImage && heroAvailable)
  const tags = useMemo(() => {
    if (!post) return []
    const values = [post.topic, ...(post.tags ?? [])]
      .map((value) => value.replace(/^#+/, '').trim())
      .filter(Boolean)
    return Array.from(new Set(values))
  }, [post])
  const isApproved = post?.reviewStatus === 'approved'
  const reviewState = post?.reviewStatus ? reviewCopy[post.reviewStatus] : null

  useEffect(() => {
    let alive = true
    setHeroAvailable(false)

    if (heroImage) {
      void Taro.getImageInfo({ src: heroImage })
        .then(() => {
          if (alive) setHeroAvailable(true)
        })
        .catch(() => {
          if (alive) setHeroAvailable(false)
        })
    }

    return () => {
      alive = false
    }
  }, [postId, heroImage])

  useEffect(() => {
    let alive = true
    const load = async () => {
      setLoading(true)
      setError('')
      setCommentError('')
      setPost(null)
      setComments([])
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

        if (nextPost.reviewStatus === 'approved') {
          const [nextComments, viewerState] = await Promise.all([
            listPostComments(postId).catch((commentsError) => {
              console.error('load post comments failed', commentsError)
              if (alive) setCommentError('评论暂时无法读取，点击重试')
              return [] as CommunityComment[]
            }),
            getCommunityViewerState(postId).catch(() => ({
              liked: false,
              favorited: false,
              followingAuthor: false,
            })),
          ])
          if (!alive) return
          setComments(nextComments)
          setLiked(viewerState.liked)
          setFavorited(viewerState.favorited)
          setFollowing(viewerState.followingAuthor)
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
    }
  }, [postId, reloadKey])

  const withActionGuard = async (run: () => Promise<void>) => {
    if (actioning || !post || !isApproved) return
    setActioning(true)
    try {
      await run()
    } catch (actionError) {
      console.error('post action failed', actionError)
      showToast('操作失败，请稍后再试', 'error')
    } finally {
      setActioning(false)
    }
  }

  const handleFollow = () =>
    withActionGuard(async () => {
      if (!post) return
      const next = !following
      await setUserFollowing(post.authorId, next)
      setFollowing(next)
      showToast(next ? '已关注' : '已取消关注', 'success')
    })

  const handleLike = () =>
    withActionGuard(async () => {
      if (!post) return
      const next = !liked
      await setPostLiked(post.id, next)
      setLiked(next)
      setPost((current) => current ? { ...current, likedCount: Math.max(0, current.likedCount + (next ? 1 : -1)) } : current)
    })

  const handleFavorite = () =>
    withActionGuard(async () => {
      if (!post) return
      const next = !favorited
      await setPostFavorited(post.id, next)
      setFavorited(next)
      setPost((current) => current ? { ...current, favoriteCount: Math.max(0, current.favoriteCount + (next ? 1 : -1)) } : current)
    })

  const handleSubmitComment = async () => {
    if (!post || !isApproved) return
    const content = commentText.trim()
    if (!content) {
      showToast('先写一句评论吧')
      return
    }
    if (submittingComment) return
    setSubmittingComment(true)
    try {
      const comment = await addPostComment(post.id, content)
      setComments((current) => [comment, ...current])
      setPost((current) => current ? { ...current, commentCount: current.commentCount + 1 } : current)
      setCommentText('')
      showToast('评论已提交审核', 'success')
    } catch (commentSubmitError) {
      console.error('submit comment failed', commentSubmitError)
      showToast('评论失败，请稍后再试', 'error')
    } finally {
      setSubmittingComment(false)
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
        left={<AppIconButton name='back' label='返回' className='post-detail-nav__back' onClick={() => void Taro.navigateBack()} />}
        center={headerCenter}
        right={post && isApproved ? (
          <View className='post-detail-follow-hitbox' onClick={handleFollow}>
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
          <View className="post-detail-scroll">
            {reviewState?.title ? (
              <View className={`post-detail-review post-detail-review--${post.reviewStatus}`}>
                <Text className="post-detail-review__title">{reviewState.title}</Text>
                <Text className="post-detail-review__desc">{reviewState.description}</Text>
              </View>
            ) : null}

            {showHero ? (
              <View className='post-detail-hero'>
                <Image
                  className='post-detail-hero__image'
                  src={heroImage}
                  mode='aspectFill'
                  onError={() => setHeroAvailable(false)}
                />
              </View>
            ) : null}

            <View className={`post-detail-content${showHero ? '' : ' post-detail-content--without-media'}`}>
              <Text className="post-detail-content__title">{post.title || '校园动态'}</Text>
              <Text className="post-detail-content__body">{post.content}</Text>
              {tags.length > 0 ? (
                <View className="post-detail-tags">
                  {tags.map((tag) => <Text className="post-detail-tag" key={tag}>#{tag}</Text>)}
                </View>
              ) : null}
              <Text className="post-detail-content__meta">
                {[post.createdAt ? `${post.createdAt} 发布` : '', post.school].filter(Boolean).join(' · ')}
              </Text>
            </View>

            {isApproved ? (
              <View className="post-detail-comments">
                <Text className="post-detail-comments__title">共 {post.commentCount || comments.length} 条评论</Text>
                {comments.map((comment, index) => (
                  <View className="post-detail-comment" key={comment.id}>
                    <View className={`post-detail-comment__avatar post-detail-comment__avatar--${commentTone[index % commentTone.length]}`}>
                      {comment.avatar ? <Image src={comment.avatar} mode='aspectFill' /> : (comment.author || '同').slice(0, 1)}
                    </View>
                    <View className="post-detail-comment__main">
                      <Text className="post-detail-comment__author">{comment.author || '同学'}</Text>
                      <Text className="post-detail-comment__content">{comment.content}</Text>
                      <Text className="post-detail-comment__meta">{comment.createdAt}　回复</Text>
                    </View>
                  </View>
                ))}
                {commentError ? (
                  <Text className="post-detail-comments__bottom post-detail-comments__bottom--retry" onClick={() => setReloadKey((value) => value + 1)}>
                    {commentError}
                  </Text>
                ) : comments.length === 0 ? (
                  <View className="post-detail-comments__empty">
                    <Text className="post-detail-comments__empty-title">还没有评论</Text>
                    <Text className="post-detail-comments__empty-desc">说说你的看法，成为第一个评论的人</Text>
                  </View>
                ) : (
                  <Text className="post-detail-comments__bottom">— 到底了 —</Text>
                )}
              </View>
            ) : null}
          </View>

          {isApproved ? (
            <View className="post-detail-actionbar">
              <Input
                className="post-detail-input"
                value={commentText}
                placeholder={submittingComment ? '发送中…' : '说点什么…'}
                onInput={(event) => setCommentText(event.detail.value)}
                confirmType="send"
                onConfirm={handleSubmitComment}
              />
              <View className={`post-detail-action${liked ? ' post-detail-action--active' : ''}`} onClick={handleLike}>
                <AppIcon name='heart' active={liked} className='post-detail-action__icon' />
                <Text>{post.likedCount}</Text>
              </View>
              <View className={`post-detail-action${favorited ? ' post-detail-action--active post-detail-action--favorite' : ''}`} onClick={handleFavorite}>
                <AppIcon name='star' active={favorited} className='post-detail-action__icon' />
                <Text>{post.favoriteCount}</Text>
              </View>
              <View className="post-detail-action" onClick={handleSubmitComment}>
                <AppIcon name='comment' className='post-detail-action__icon' />
                <Text>{post.commentCount || comments.length}</Text>
              </View>
            </View>
          ) : null}
        </>
      )}
    </View>
  )
}
