import React, { useState } from 'react'
import { Text, View } from '@tarojs/components'
import Taro from '../../services/taro'
import './index.scss'

interface BlockedUser {
  id: string
  avatar: string
  name: string
  meta: string
  tone: 'muted' | 'warm'
}

const initialBlockedUsers: BlockedUser[] = [
  {
    id: 'anonymous-spam',
    avatar: '匿',
    name: '匿名小号',
    meta: '发布骚扰私信 · 6月20日拉黑',
    tone: 'muted'
  },
  {
    id: 'ad-spam',
    avatar: '代',
    name: '代刷广告号',
    meta: '刷屏广告 · 6月15日拉黑',
    tone: 'warm'
  }
]

export default function BlacklistPage() {
  const [blockedUsers, setBlockedUsers] = useState<BlockedUser[]>(initialBlockedUsers)

  const removeBlockedUser = (id: string) => {
    setBlockedUsers((users) => users.filter((user) => user.id !== id))
    void Taro.showToast({ title: '已移出', icon: 'none', duration: 1200 })
  }

  return (
    <View className='student-blacklist-page'>
      <View className='student-blacklist-nav'>
        <View className='student-blacklist-nav__back' onClick={() => void Taro.navigateBack()}>
          <Text>‹</Text>
        </View>
        <Text className='student-blacklist-nav__title'>黑名单</Text>
      </View>

      <View className='student-blacklist-content'>
        <Text className='student-blacklist-tip'>
          被拉黑的用户无法给你发私信、评论你的内容，也看不到你的主页。
        </Text>

        <View className='student-blacklist-card'>
          {blockedUsers.length > 0 ? (
            blockedUsers.map((user) => (
              <View key={user.id} className='student-blacklist-user'>
                <View className={`student-blacklist-user__avatar student-blacklist-user__avatar--${user.tone}`}>
                  <Text>{user.avatar}</Text>
                </View>
                <View className='student-blacklist-user__body'>
                  <Text className='student-blacklist-user__name'>{user.name}</Text>
                  <Text className='student-blacklist-user__meta'>{user.meta}</Text>
                </View>
                <View className='student-blacklist-user__remove' onClick={() => removeBlockedUser(user.id)}>
                  <Text>移出</Text>
                </View>
              </View>
            ))
          ) : (
            <View className='student-blacklist-empty'>
              <Text>暂无黑名单用户</Text>
            </View>
          )}
        </View>
      </View>
    </View>
  )
}

