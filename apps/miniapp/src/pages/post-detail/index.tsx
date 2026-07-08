import React, { useState } from 'react'
import { Input, Text, View } from '@tarojs/components'
import Taro from '../../services/taro'
import './index.scss'

type CommentTone = 'sky' | 'peach' | 'green' | 'mint'

type CommentItem = {
  id: string
  avatar: string
  author: string
  content: string
  time: string
  likes: number
  tone: CommentTone
}

const tags = ['#奶茶推荐', '#三食堂', '#湛科日常']

const comments: CommentItem[] = [
  {
    id: 'north',
    avatar: '向',
    author: '向北',
    content: '昨天刚喝，波波确实弹，就是下午去要排一会儿',
    time: '1 小时前',
    likes: 6,
    tone: 'sky'
  },
  {
    id: 'pomelo',
    avatar: '柚',
    author: '柚子茶',
    content: '甜度三分党+1，五分有点腻',
    time: '58 分钟前',
    likes: 3,
    tone: 'peach'
  },
  {
    id: 'morning',
    avatar: '早',
    author: '早八战士',
    content: '学生证减 5 毛是真的吗？？明天就去',
    time: '40 分钟前',
    likes: 2,
    tone: 'green'
  },
  {
    id: 'deer',
    avatar: '鹿',
    author: '小鹿乱撞',
    content: '蹲一个代喝，今天出不了门，有人帮带吗（跑腿费好说）',
    time: '12 分钟前',
    likes: 1,
    tone: 'mint'
  }
]

export default function PostDetailPage() {
  const [followed, setFollowed] = useState(false)
  const [liked, setLiked] = useState(false)
  const [favorited, setFavorited] = useState(false)
  const [draft, setDraft] = useState('')

  const likeCount = 128 + (liked ? 1 : 0)
  const favoriteCount = 23 + (favorited ? 1 : 0)

  const goBack = () => {
    void Taro.navigateBack().catch(() => {
      void Taro.redirectTo({ url: '/pages/home/index' })
    })
  }

  const showToast = (title: string) => {
    void Taro.showToast({ title, icon: 'none', duration: 1200 })
  }

  const submitComment = () => {
    if (!draft.trim()) {
      showToast('先写点什么吧')
      return
    }
    setDraft('')
    showToast('评论已记录为本地预览')
  }

  return (
    <View className='post-detail-page'>
      <View className='post-detail-nav'>
        <View className='post-detail-nav__back' onClick={goBack} />
        <View className='post-detail-author' onClick={() => showToast('用户主页下一页接入')}>
          <View className='post-detail-author__avatar'>
            <Text>奶</Text>
          </View>
          <View className='post-detail-author__main'>
            <Text className='post-detail-author__name'>奶茶脑袋</Text>
            <Text className='post-detail-author__meta'>湛江科技学院 · 大三</Text>
          </View>
        </View>
        <View
          className={`post-detail-follow ${followed ? 'post-detail-follow--active' : ''}`}
          onClick={() => setFollowed((value) => !value)}
        >
          <Text>{followed ? '已关注' : '关注'}</Text>
        </View>
      </View>

      <View className='post-detail-scroll'>
        <View className='post-detail-hero' onClick={() => showToast('图片预览下一页接入')}>
          <Text>图 · 用户上传（奶茶实拍）</Text>
        </View>

        <View className='post-detail-content'>
          <Text className='post-detail-content__title'>三食堂新出的芋泥波波真的绝，一天一杯根本停不下来</Text>
          <Text className='post-detail-content__body'>
            中杯 8 块，加一份波波才 1 块，芋泥是真材实料那种，甜度选三分刚刚好。下午四点后人少不用排队，学生证还能再减 5 毛。
          </Text>
          <View className='post-detail-tags'>
            {tags.map((tag) => (
              <View key={tag} className='post-detail-tag' onClick={() => showToast('标签搜索下一页接入')}>
                <Text>{tag}</Text>
              </View>
            ))}
          </View>
          <Text className='post-detail-content__meta'>今天 12:32 发布 · 湛江科技学院</Text>
        </View>

        <View className='post-detail-comments'>
          <Text className='post-detail-comments__title'>共 12 条评论</Text>
          {comments.map((item) => (
            <View key={item.id} className='post-detail-comment'>
              <View className={`post-detail-comment__avatar post-detail-comment__avatar--${item.tone}`}>
                <Text>{item.avatar}</Text>
              </View>
              <View className='post-detail-comment__main'>
                <Text className='post-detail-comment__author'>{item.author}</Text>
                <Text className='post-detail-comment__content'>{item.content}</Text>
                <View className='post-detail-comment__meta'>
                  <Text>{item.time}</Text>
                  <Text onClick={() => showToast('回复下一页接入')}>回复</Text>
                </View>
              </View>
              <View className='post-detail-comment__like' onClick={() => showToast('评论点赞本地预览')}>
                <Text>♡</Text>
                <Text>{item.likes}</Text>
              </View>
            </View>
          ))}
          <Text className='post-detail-comments__bottom'>— 到底了 —</Text>
        </View>
      </View>

      <View className='post-detail-actionbar'>
        <View className='post-detail-input'>
          <Input
            value={draft}
            placeholder='说点什么...'
            confirmType='send'
            onInput={(event) => setDraft(String(event.detail.value))}
            onConfirm={submitComment}
          />
        </View>
        <View className={`post-detail-action ${liked ? 'post-detail-action--active' : ''}`} onClick={() => setLiked((value) => !value)}>
          <Text className='post-detail-action__icon'>{liked ? '♥' : '♡'}</Text>
          <Text>{likeCount}</Text>
        </View>
        <View className={`post-detail-action ${favorited ? 'post-detail-action--active' : ''}`} onClick={() => setFavorited((value) => !value)}>
          <Text className='post-detail-action__icon'>{favorited ? '★' : '☆'}</Text>
          <Text>{favoriteCount}</Text>
        </View>
        <View className='post-detail-action' onClick={() => showToast('已在评论区')}>
          <Text className='post-detail-action__icon'>▱</Text>
          <Text>12</Text>
        </View>
      </View>
    </View>
  )
}
