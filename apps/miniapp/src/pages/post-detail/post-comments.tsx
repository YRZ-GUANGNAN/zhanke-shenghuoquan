import { useEffect, useMemo, useSyncExternalStore } from 'react'
import { Image, Text, View } from '@tarojs/components'
import AppIcon from '../../components/app-icon'
import { useSessionStore } from '../../stores/session'
import { CommentSession, type CommentPageState, type CommentSessionState } from '../../services/comment-session'
import { createCommunityClientRequestId, getCommentReplies, getCommentThreads, setCommentLiked, submitThreadComment } from '../../services/b-community'
import { formatPostDate } from '../../utils/post-presentation'
import type { CommunityThreadComment } from '../../types'
import './post-comments.scss'

export function usePostComments(postId: string, enabled: boolean) {
  const profile = useSessionStore(value => value.profile)
  const token = useSessionStore(value => value.token)
  const session = useMemo(() => new CommentSession(postId, profile, {
    roots: getCommentThreads, replies: getCommentReplies, submit: submitThreadComment,
    like: setCommentLiked, key: createCommunityClientRequestId,
  }), [postId, token, profile.id, enabled])
  const state = useSyncExternalStore(session.subscribe, session.snapshot)
  useEffect(() => {
    if (enabled) void session.load()
    return session.dispose
  }, [session, enabled])
  return { session, state }
}

const statusLabel = { submitted: '待审核 · 仅你可见', reviewing: '审核中 · 仅你可见', rejected: '未通过审核 · 仅你可见', approved: '', taken_down: '已下架' }
const countLabel = (count: number) => count > 9999 ? `${Math.floor(count / 1000) / 10}万` : String(count)

export interface CommentListProps {
  state: CommentSessionState
  publicCount: number
  canReply: boolean
  canLike: boolean
  onReply: (comment: CommunityThreadComment) => void
  onLike: (id: string) => void
  onLoad: (rootId?: string) => void
  onExpand: (rootId: string) => void
  onCollapse: (rootId: string) => void
}

export function PostCommentList(props: CommentListProps) {
  const { state, canReply, canLike } = props
  const row = (comment: CommunityThreadComment, reply = false) => (
    <View className={`post-detail-comment${reply ? ' post-detail-comment--reply' : ''}`} key={comment.id}>
      <View className='post-detail-comment__avatar'>
        {comment.avatar ? <Image src={comment.avatar} mode='aspectFill' /> : (comment.author || '同').slice(0, 1)}
      </View>
      <View className='post-detail-comment__main'>
        <Text className='post-detail-comment__author'>{comment.author || '同学'}</Text>
        {reply && <Text className='post-detail-comment__recipient'>回复 {comment.replyToAuthor || '同学'}</Text>}
        <Text className='post-detail-comment__content' selectable>{comment.content}</Text>
        <View className='post-detail-comment__footer'>
          <Text className='post-detail-comment__meta'>{formatPostDate(comment.createdAt)}</Text>
          {canReply && comment.reviewStatus === 'approved' && <View className='post-detail-comment__reply-button' role='button' aria-label={`回复${comment.author}`} onClick={() => props.onReply(comment)}><Text>回复</Text></View>}
        </View>
        {statusLabel[comment.reviewStatus] && <Text className='post-detail-comment__status'>{statusLabel[comment.reviewStatus]}</Text>}
      </View>
      {canLike && comment.reviewStatus === 'approved' && <View
        className={`post-detail-comment__like${comment.liked ? ' post-detail-comment__like--active' : ''}${state.likeBusy[comment.id] ? ' post-detail-comment__like--pending' : ''}`}
        role='button' aria-disabled={Boolean(state.likeBusy[comment.id]) || comment.liked === undefined}
        aria-label={`${comment.liked ? '取消点赞' : '点赞评论'}，${comment.likedCount}赞`}
        onClick={() => props.onLike(comment.id)}
      >
        <AppIcon name='heart' active={Boolean(comment.liked)} className='post-detail-comment__like-icon' />
        <Text>{comment.likedCount > 0 ? countLabel(comment.likedCount) : '赞'}</Text>
      </View>}
    </View>
  )
  const pageControl = (page: CommentPageState, rootId?: string) => page.loading ? (
    <View className='post-detail-comments__loading' role='status'><Text>{page.page ? '正在加载更多…' : '正在读取…'}</Text></View>
  ) : page.error ? (
    <View className='post-detail-comments__more' role='button' onClick={() => props.onLoad(rootId)}><Text>{page.error}</Text></View>
  ) : page.hasMore ? (
    <View className='post-detail-comments__more' role='button' onClick={() => props.onLoad(rootId)}><Text>{rootId ? '查看更多回复' : '查看更多评论'}</Text><AppIcon name='chevron-right' /></View>
  ) : null

  return <View className='post-detail-comments' id='post-comments'>
    <Text className='post-detail-comments__title'>共 {props.publicCount} 条评论</Text>
    {state.roots.items.map(comment => {
      const replies = state.replies[comment.id]
      return <View className='post-detail-comment-thread' key={comment.id}>
        {row(comment)}
        {(comment.replyCount > 0 || replies?.items.length) ? <View className='post-detail-comment-thread__replies'>
          {replies?.expanded ? <>
            {replies.items.map(value => row(value, true))}
            <View className='post-detail-comments__reply-controls'>
              {pageControl(replies, comment.id)}
              <View className='post-detail-comments__collapse' role='button' aria-label='收起楼内回复' onClick={() => props.onCollapse(comment.id)}><Text>收起</Text></View>
            </View>
          </> : <View className='post-detail-comments__more' role='button' aria-label={`查看${comment.replyCount}条回复`} onClick={() => props.onExpand(comment.id)}>
            <View className='post-detail-comments__connector' /><Text>查看 {comment.replyCount} 条回复</Text><AppIcon name='chevron-right' />
          </View>}
        </View> : null}
      </View>
    })}
    {pageControl(state.roots)}
    {!state.roots.loading && !state.roots.error && state.roots.items.length === 0 && <View className='post-detail-comments__empty'>
      <Text className='post-detail-comments__empty-title'>还没有评论</Text>
      <Text className='post-detail-comments__empty-desc'>{canReply ? '说说你的看法吧' : '评论通过审核后会显示在这里'}</Text>
    </View>}
    {state.roots.items.length > 0 && !state.roots.hasMore && !state.roots.loading && !state.roots.error && <Text className='post-detail-comments__bottom'>已显示全部评论</Text>}
  </View>
}
