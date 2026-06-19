import React, { useEffect, useState } from 'react'
import { Button, Image, Text, View } from '@tarojs/components'
import { AvatarMark, BottomNav } from '../../components'
import { listCommunityPosts, listFollowingPosts } from '../../services/b-community'
import { listNavigationItems } from '../../services/d-navigation'
import Taro from '../../services/taro'
import type { CommunityPost, ToolLink } from '../../types'
import './index.scss'

export default function HomePage() {
	const [posts, setPosts] = useState<CommunityPost[]>([])
	const [tools, setTools] = useState<ToolLink[]>([])
	const [feed, setFeed] = useState<'recent'|'following'>('recent')

  useEffect(() => {
    let active = true
	Promise.all([listCommunityPosts(), listNavigationItems()]).then(([nextPosts, nextTools]) => {
      if (!active) return
      setPosts(nextPosts)
      setTools(nextTools)
	}).catch(() => undefined)
    return () => {
      active = false
    }
  }, [])

  const switchFeed = async (next:'recent'|'following') => {
    try {
      setFeed(next)
      setPosts(next==='recent'?await listCommunityPosts():await listFollowingPosts())
    } catch (error) {
      void Taro.showToast({title:error instanceof Error?error.message:'加载失败',icon:'none'})
    }
  }

  return (
    <View className='page home-page'>
      <View className='home-header'>
        <View>
          <Text className='home-header__name'>广海空间</Text>
          <Text className='home-header__school'>海大人的综合信息服务平台</Text>
        </View>
        <Button className='cc-button cc-button--primary cc-button--small' onClick={() => Taro.navigateTo({ url: '/pages/publish/index?type=post' })}>
          发布
        </Button>
      </View>

      <View className='cc-search'>搜索二手、帖子、资料、工具</View>

      <View className='tool-grid card'>
        {tools.slice(0, 8).map((tool) => (
          <View key={tool.id} className='tool-grid__item' onClick={() => Taro.navigateTo({ url: '/pages/tools/index' })}>
            <View className='tool-grid__icon' style={{ background: tool.color }}>{tool.iconText}</View>
            <Text>{tool.title}</Text>
          </View>
        ))}
      </View>

      <View className='notice card'>
        <View>
          <Text className='notice__title'>校园服务导航已上线</Text>
          <Text className='notice__desc'>聚合社区、二手交易、校内工具跳转和私聊提醒</Text>
        </View>
        <Button className='cc-button cc-button--small' onClick={() => Taro.navigateTo({ url: '/pages/tools/index' })}>查看</Button>
      </View>

      <View className='feed-tabs'>
        <Text className={feed==='recent'?'feed-tabs__active':''} onClick={()=>void switchFeed('recent')}>最新</Text>
        <Text className={feed==='following'?'feed-tabs__active':''} onClick={()=>void switchFeed('following')}>关注</Text>
        <Text>二手精选</Text>
      </View>

      <View className='post-list'>
        {posts.map((post) => (
          <View key={post.id} className='post-card card' onClick={() => Taro.navigateTo({ url: `/pages/post-detail/index?id=${post.id}` })}>
            {post.pinned && <View className='post-card__pin'>置顶</View>}
            <View className='between'>
              <View className='row post-card__author'>
                <AvatarMark size='small' label={post.author} src={post.avatar} />
                <View>
                  <Text className='post-card__name'>{post.author}</Text>
                  <Text className='post-card__meta'>{post.school} · {post.createdAt}</Text>
                </View>
              </View>
              <Text className='cc-tag'>{post.topic}</Text>
            </View>
            <Text className='post-card__title'>{post.title}</Text>
            <Text className='post-card__content'>{post.content}</Text>
            {post.images.length > 0 && (
              <View className={`post-card__images post-card__images--${Math.min(post.images.length, 3)}`}>
                {post.images.slice(0, 3).map((img) => <Image key={img} src={img} mode='aspectFill' />)}
              </View>
            )}
            <View className='post-card__actions'>
              <Text>赞 {post.likedCount}</Text>
              <Text>评论 {post.commentCount}</Text>
              <Text>分享</Text>
            </View>
          </View>
        ))}
      </View>

      <BottomNav active='home' />
    </View>
  )
}
