import React from 'react'
import { Text, View } from '@tarojs/components'
import { AvatarMark, PageShell } from '../../components'
import Taro from '../../services/taro'
import './index.scss'

export default function ProfilePage() {
  return (
    <PageShell active='profile' refreshTitle='我的页面已刷新'>
      <View className='profile-hero-live'>
        <AvatarMark label='泽' size='large' />
        <View>
          <Text>阿泽</Text>
          <Text>湛科号：zk_aze233 · 湛江科技学院</Text>
        </View>
      </View>
      <View className='profile-stats-live'>
        {[
          ['23', '收藏', 'favorites'],
          ['8', '帖子', 'posts'],
          ['41', '评论', 'comments'],
          ['2', '宝贝', 'products']
        ].map((item) => (
          <View key={item[1]} onClick={() => Taro.navigateTo({ url: `/pages/my-content/index?tab=${item[2]}` })}>
            <Text>{item[0]}</Text>
            <Text>{item[1]}</Text>
          </View>
        ))}
      </View>
      <View className='zk-card'>
        <View className='section-head-live'>
          <Text>身份切换</Text>
          <Text>切换后进入不同工作台</Text>
        </View>
        <View className='role-grid-live'>
          <RoleCard title='学生' sub='社交 · 二手 · 校园服务' active />
          <RoleCard title='骑手' sub='抢单 · 配送 · 钱包' onClick={() => Taro.navigateTo({ url: '/pages/delivery-rider/index' })} />
          <RoleCard title='商家' sub='发单 · 账单 · 消息' onClick={() => Taro.navigateTo({ url: '/pages/delivery-merchant/index' })} />
        </View>
      </View>
      <View className='settings-card-live'>
        <Text onClick={() => Taro.navigateTo({ url: '/pages/conversations/index' })}>消息中心</Text>
        <Text onClick={() => Taro.navigateTo({ url: '/pages/media-history/index' })}>图片记录</Text>
        <Text onClick={() => Taro.navigateTo({ url: '/pages/notifications/index' })}>通知</Text>
        <Text>关于湛科生活圈</Text>
      </View>
    </PageShell>
  )
}

function RoleCard({ title, sub, active, onClick }: { title: string; sub: string; active?: boolean; onClick?: () => void }) {
  return (
    <View className={active ? 'role-card-live role-card-live--active' : 'role-card-live'} onClick={onClick}>
      <Text>{title.slice(0, 1)}</Text>
      <Text>{title}</Text>
      <Text>{sub}</Text>
    </View>
  )
}
