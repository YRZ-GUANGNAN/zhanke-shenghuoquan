import React, { useState } from 'react'
import { Button, Input, Text, View } from '@tarojs/components'
import { AvatarMark, ImageTile, PageHeader, PageShell } from '../../components'
import Taro from '../../services/taro'
import './index.scss'

const comments = [
  ['阿泽', '内容很实用，这周末我也想去看看。'],
  ['南宁', '学生证优惠还能用吗？有人一起拼吗？'],
  ['小满', '图里这个位置好像在二食堂门口。']
]

export default function PostDetailPage() {
  const [liked, setLiked] = useState(false)

  return (
    <PageShell plain refreshTitle='帖子详情已刷新'>
      <PageHeader title='帖子详情' right={<Text onClick={() => Taro.showToast({ title: '已收藏', icon: 'success' })}>收藏</Text>} />
      <View className='detail-hero-image detail-hero-image--post'>
        <ImageTile tone={1} label='校园' />
      </View>
      <View className='zk-card'>
        <View className='author-row-live'>
          <AvatarMark label='奶' />
          <View>
            <Text>奶茶脑袋</Text>
            <Text>湛江科技学院 · 今天 12:30</Text>
          </View>
          <Text className='zk-chip'>关注</Text>
        </View>
        <Text className='detail-title-live'>三食堂新出的芋泥波波真的绝，一天一杯根本停不下</Text>
        <Text className='detail-copy-live'>中杯 8 块，加一份波波 1 块。芋泥很绵密，不会太甜，下课后去人少一点，排队也快。</Text>
        <View className='tag-row-live'>
          <Text>校园美食</Text>
          <Text>三食堂</Text>
          <Text>奶茶推荐</Text>
        </View>
      </View>
      <View className='zk-card'>
        <Text className='comment-title-live'>共 12 条评论</Text>
        {comments.map((comment) => (
          <View key={comment[0]} className='comment-row-live'>
            <AvatarMark label={comment[0]} size='small' />
            <View>
              <Text>{comment[0]}</Text>
              <Text>{comment[1]}</Text>
            </View>
          </View>
        ))}
      </View>
      <View className='comment-bar-live'>
        <Input placeholder='友善评论一下' />
        <Button onClick={() => setLiked(!liked)}>{liked ? '已赞' : '♡ 赞'}</Button>
        <Button onClick={() => Taro.showToast({ title: '评论已发送', icon: 'success' })}>发送</Button>
      </View>
    </PageShell>
  )
}
