import { useEffect, useMemo, useState } from 'react'
import { Image, Input, Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import Taro from '../../services/taro'
import type { CommunityComment, CommunityPost } from '../../types'
import {
  addPostComment,
  getCommunityPost,
  getCommunityViewerState,
  listPostComments,
  setPostFavorited,
  setPostLiked,
  setUserFollowing,
} from '../../services/b-community'
import { getCapsuleSafeStyle } from '../../utils/capsule-safe'
import './index.scss'

const DEMO_POST: CommunityPost = {
  id: 'demo-post',
  authorId: 'demo-author',
  author: '奶茶脑袋',
  avatar: '',
  school: '湛江科技学院 · 大三',
  topic: '#奶茶推荐',
  title: '三食堂新出的芋泥波波真的绝，一天一杯根本停不下来',
  content: '中杯 8 块，加一份波波才 1 块，芋泥是真的夹料那种，甜度选三分刚刚好。下午四点后人少不用排队，学生证还能再减 5 毛。',
  images: [],
  likedCount: 128,
  commentCount: 12,
  favoriteCount: 23,
  createdAt: '今天 12:32',
}

const DEMO_COMMENTS: CommunityComment[] = [
  { id: 'c1', author: '向北', avatar: '', content: '昨天刚喝，波波确实弹，就是下午去要排一会儿', createdAt: '1 小时前' },
  { id: 'c2', author: '柚子茶', avatar: '', content: '甜度三分党 +1，五分有点腻', createdAt: '58 分钟前' },
  { id: 'c3', author: '小鹿乱撞', avatar: '', content: '哪个窗口呀，今天出不了门，有人带带吗', createdAt: '12 分钟前' },
]

const commentTone = ['sky', 'peach', 'green', 'mint'] as const

const showToast = (title: string, icon: 'success' | 'error' | 'none' = 'none') => {
  void Taro.showToast({ title, icon, duration: 1600 })
}

export default function PostDetailPage() {
  const params = Taro.getCurrentInstance().router?.params ?? {}
  const postId = params.id ? decodeURIComponent(params.id) : DEMO_POST.id
  const [post, setPost] = useState<CommunityPost>(DEMO_POST)
  const [comments, setComments] = useState<CommunityComment[]>(DEMO_COMMENTS)
  const [liked, setLiked] = useState(false)
  const [favorited, setFavorited] = useState(false)
  const [following, setFollowing] = useState(false)
  const [commentText, setCommentText] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [actioning, setActioning] = useState(false)
  const [submittingComment, setSubmittingComment] = useState(false)

  const heroImage = useMemo(() => post.images[0], [post.images])

  useEffect(() => {
    let alive = true
    const load = async () => {
      setLoading(true)
      setError('')
      try {
        const [nextPost, nextComments, viewerState] = await Promise.all([
          postId === DEMO_POST.id ? Promise.resolve(DEMO_POST) : getCommunityPost(postId),
          postId === DEMO_POST.id ? Promise.resolve(DEMO_COMMENTS) : listPostComments(postId),
          postId === DEMO_POST.id
            ? Promise.resolve({ liked: false, favorited: false, followingAuthor: false })
            : getCommunityViewerState(postId).catch(() => ({ liked: false, favorited: false, followingAuthor: false })),
        ])
        if (!alive) return
        setPost(nextPost)
        setComments(nextComments)
        setLiked(viewerState.liked)
        setFavorited(viewerState.favorited)
        setFollowing(viewerState.followingAuthor)
      } catch (loadError) {
        console.error('load post detail failed', loadError)
        if (!alive) return
        setPost(DEMO_POST)
        setComments(DEMO_COMMENTS)
        setError('当前展示本地预览内容，联网后会读取真实帖子')
      } finally {
        if (alive) setLoading(false)
      }
    }
    void load()
    return () => {
      alive = false
    }
  }, [postId])

  const withActionGuard = async (run: () => Promise<void>) => {
    if (actioning) return
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
      const next = !following
      await setUserFollowing(post.authorId, next)
      setFollowing(next)
      showToast(next ? '已关注' : '已取消关注', 'success')
    })

  const handleLike = () =>
    withActionGuard(async () => {
      const next = !liked
      await setPostLiked(post.id, next)
      setLiked(next)
      setPost((current) => ({ ...current, likedCount: Math.max(0, current.likedCount + (next ? 1 : -1)) }))
    })

  const handleFavorite = () =>
    withActionGuard(async () => {
      const next = !favorited
      await setPostFavorited(post.id, next)
      setFavorited(next)
      setPost((current) => ({ ...current, favoriteCount: Math.max(0, current.favoriteCount + (next ? 1 : -1)) }))
    })

  const handleSubmitComment = async () => {
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
      setPost((current) => ({ ...current, commentCount: current.commentCount + 1 }))
      setCommentText('')
      showToast('评论已发布', 'success')
    } catch (commentError) {
      console.error('submit comment failed', commentError)
      showToast('评论失败，请稍后再试', 'error')
    } finally {
      setSubmittingComment(false)
    }
  }

  return (
    <View className="post-detail-page" style={getCapsuleSafeStyle()}>
      <CapsulePageHeader
        className='post-detail-nav'
        variant='standard'
        left={<Text className='post-detail-nav__back' onClick={() => void Taro.navigateBack()}>‹</Text>}
        center={
          <View className='post-detail-author'>
            <View className='post-detail-author__avatar'>
              {post.avatar ? <Image src={post.avatar} mode='aspectFill' /> : post.author.slice(0, 1)}
            </View>
            <View className='post-detail-author__main'>
              <Text className='post-detail-author__name'>{post.author}</Text>
              <Text className='post-detail-author__meta'>{post.school}</Text>
            </View>
          </View>
        }
        right={
          <Text className={`post-detail-follow${following ? ' post-detail-follow--active' : ''}`} onClick={handleFollow}>
            {following ? '已关注' : '关注'}
          </Text>
        }
      />

      {error ? (
        <View className="post-detail-state post-detail-state--retry">
          <Text className="post-detail-state__title">预览模式</Text>
          <Text className="post-detail-state__desc">{error}</Text>
        </View>
      ) : null}

      <View className="post-detail-scroll">
        <View className="post-detail-hero">
          {heroImage ? <Image className="post-detail-hero__image" src={heroImage} mode="aspectFill" /> : <Text>图 · 用户上传（奶茶实拍）</Text>}
        </View>

        <View className="post-detail-content">
          <Text className="post-detail-content__title">{post.title}</Text>
          <Text className="post-detail-content__body">{post.content}</Text>
          <View className="post-detail-tags">
            <Text className="post-detail-tag">{post.topic}</Text>
            <Text className="post-detail-tag">#三食堂</Text>
            <Text className="post-detail-tag">#湛科日常</Text>
          </View>
          <Text className="post-detail-content__meta">{post.createdAt} 发布 · 湛江科技学院</Text>
        </View>

        <View className="post-detail-comments">
          <Text className="post-detail-comments__title">共 {post.commentCount || comments.length} 条评论</Text>
          {loading ? (
            <View className="post-detail-state">
              <Text className="post-detail-state__desc">评论加载中...</Text>
            </View>
          ) : (
            comments.map((comment, index) => (
              <View className="post-detail-comment" key={comment.id}>
                <View className={`post-detail-comment__avatar post-detail-comment__avatar--${commentTone[index % commentTone.length]}`}>
                  {comment.avatar ? <Image src={comment.avatar} mode="aspectFill" /> : comment.author.slice(0, 1)}
                </View>
                <View className="post-detail-comment__main">
                  <Text className="post-detail-comment__author">{comment.author}</Text>
                  <Text className="post-detail-comment__content">{comment.content}</Text>
                  <Text className="post-detail-comment__meta">{comment.createdAt}　回复</Text>
                </View>
                <Text className="post-detail-comment__like">♡</Text>
              </View>
            ))
          )}
          <Text className="post-detail-comments__bottom">— 到底了 —</Text>
          <View className="post-detail-comments__sentinel" />
        </View>
      </View>

      <View className="post-detail-actionbar">
        <Input
          className="post-detail-input"
          value={commentText}
          placeholder={submittingComment ? '发送中...' : '说点什么...'}
          onInput={(event) => setCommentText(event.detail.value)}
          confirmType="send"
          onConfirm={handleSubmitComment}
        />
        <Text className={`post-detail-action${liked ? ' post-detail-action--active' : ''}`} onClick={handleLike}>
          <Text className="post-detail-action__icon">♡</Text>
          {post.likedCount}
        </Text>
        <Text className={`post-detail-action${favorited ? ' post-detail-action--active' : ''}`} onClick={handleFavorite}>
          <Text className="post-detail-action__icon">☆</Text>
          {post.favoriteCount}
        </Text>
        <Text className="post-detail-action" onClick={handleSubmitComment}>
          <Text className="post-detail-action__icon">▱</Text>
          {post.commentCount || comments.length}
        </Text>
      </View>
    </View>
  )
}
