import React, { useEffect, useState } from 'react'
import { Button, Image, Text, Textarea, View } from '@tarojs/components'
import { AvatarMark } from '../../components'
import { addPostComment, getCommunityPost, listPostComments, togglePostFavorite, togglePostLike, toggleUserFollow } from '../../services/b-community'
import Taro from '../../services/taro'
import type { CommunityComment, CommunityPost } from '../../types'
import './index.scss'

export default function PostDetailPage() {
  const params = Taro.getCurrentInstance().router?.params || {}
  const postId = params.id || ''
  const [post, setPost] = useState<CommunityPost | null>(null)
  const [comments, setComments] = useState<CommunityComment[]>([])
  const [comment, setComment] = useState('')
  const [liked, setLiked] = useState(false)
  const [favorited, setFavorited] = useState(false)
  const [followed, setFollowed] = useState(false)

  const load = async () => {
    if (!postId) return
    const [nextPost, nextComments] = await Promise.all([getCommunityPost(postId), listPostComments(postId)])
    setPost(nextPost)
    setComments(nextComments)
  }

  useEffect(() => { void load().catch(showError) }, [postId])

  const act = async (run:()=>Promise<void>) => { try { await run() } catch (error) { showError(error) } }
  const submitComment = () => act(async()=>{
    const value=comment.trim()
    if(!value){await Taro.showToast({title:'请输入评论',icon:'none'});return}
    await addPostComment(postId,value)
    setComment('')
    await Taro.showToast({title:'评论已提交审核',icon:'none'})
  })

  if (!post) return <View className='page post-detail-page'><View className='card'>内容加载中</View></View>

  return <View className='page post-detail-page'>
    <View className='post-detail card'>
      <View className='between'>
        <View className='row post-detail__author'>
          <AvatarMark label={post.author} src={post.avatar}/>
          <View><Text className='post-detail__name'>{post.author}</Text><Text className='post-detail__meta'>{post.school} · {post.createdAt}</Text></View>
        </View>
        <Button className='cc-button cc-button--small' onClick={()=>act(async()=>{const value=await toggleUserFollow(post.authorId);setFollowed(value)})}>{followed?'已关注':'关注'}</Button>
      </View>
      <Text className='cc-tag'>{post.topic}</Text>
      <Text className='post-detail__title'>{post.title}</Text>
      <Text className='post-detail__content'>{post.content}</Text>
      {post.images.map(img=><Image key={img} src={img} mode='widthFix' className='post-detail__image'/>) }
    </View>

    <View className='comment-panel card'>
      <Text className='comment-panel__title'>互动</Text>
      <View className='comment-panel__stats'>
        <Text onClick={()=>act(async()=>{const value=await togglePostLike(post.id);setLiked(value);setPost({...post,likedCount:Math.max(0,post.likedCount+(value?1:-1))})})}>{liked?'已赞':'赞'} {post.likedCount}</Text>
        <Text>评论 {post.commentCount}</Text>
        <Text onClick={()=>act(async()=>{const value=await togglePostFavorite(post.id);setFavorited(value);setPost({...post,favoriteCount:Math.max(0,post.favoriteCount+(value?1:-1))})})}>{favorited?'已收藏':'收藏'} {post.favoriteCount}</Text>
      </View>
      <Textarea className='comment-panel__input' value={comment} maxlength={500} placeholder='写下你的评论' onInput={event=>setComment(String(event.detail.value))}/>
      <Button className='cc-button cc-button--primary cc-button--block' onClick={submitComment}>提交评论</Button>
      <View className='comment-list'>{comments.map(item=><View key={item.id} className='comment-item'><AvatarMark size='small' label={item.author} src={item.avatar}/><View><Text className='comment-item__author'>{item.author} · {item.createdAt}</Text><Text className='comment-item__content'>{item.content}</Text></View></View>)}</View>
    </View>
  </View>
}

function showError(error:unknown){void Taro.showToast({title:error instanceof Error?error.message:'操作失败',icon:'none'})}
