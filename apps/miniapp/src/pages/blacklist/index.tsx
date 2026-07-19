import React, { useState } from 'react'
import { Text, View } from '@tarojs/components'
import { useDidShow } from '@tarojs/taro'
import CapsulePageHeader from '../../components/capsule-page-header'
import { listBlockedUsers, removeBlockedUser, type BlockedUserView } from '../../services/a-user'
import Taro from '../../services/taro'
import './index.scss'

function blockedAt(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '已拉黑'
  return `${date.getMonth() + 1}月${date.getDate()}日拉黑`
}

export default function BlacklistPage() {
  const [blockedUsers, setBlockedUsers] = useState<BlockedUserView[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [removing, setRemoving] = useState<number | null>(null)

  const load = async () => {
    setLoading(true)
    setError('')
    try {
      setBlockedUsers(await listBlockedUsers())
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '黑名单加载失败')
    } finally {
      setLoading(false)
    }
  }

  useDidShow(() => { void load() })

  const remove = async (value: BlockedUserView) => {
    if (removing !== null) return
    const result = await Taro.showModal({ title: '移出黑名单', content: `确认将“${value.nickname}”移出黑名单吗？`, confirmText: '移出' })
    if (!result.confirm) return
    setRemoving(value.userId)
    try {
      await removeBlockedUser(value.userId)
      setBlockedUsers((users) => users.filter((user) => user.userId !== value.userId))
      void Taro.showToast({ title: '已移出黑名单', icon: 'success' })
    } catch (cause) {
      void Taro.showToast({ title: cause instanceof Error ? cause.message : '移出失败', icon: 'none' })
    } finally {
      setRemoving(null)
    }
  }

  return (
    <View className='student-blacklist-page'>
      <CapsulePageHeader title='黑名单' left={<View className='student-blacklist-nav__back' onClick={() => void Taro.navigateBack()}><Text>‹</Text></View>} />

      <View className='student-blacklist-content'>
        <Text className='student-blacklist-tip'>被拉黑的用户无法继续与你互动。移出后将恢复正常的站内互动权限。</Text>
        <View className='student-blacklist-card'>
          {loading ? (
            <View className='student-blacklist-empty'><Text>正在加载…</Text></View>
          ) : error ? (
            <View className='student-blacklist-empty student-blacklist-empty--retry' onClick={() => void load()}><Text>{error}，点击重试</Text></View>
          ) : blockedUsers.length > 0 ? (
            blockedUsers.map((user) => (
              <View key={user.userId} className='student-blacklist-user'>
                <View className='student-blacklist-user__avatar student-blacklist-user__avatar--muted'><Text>{user.nickname.trim().slice(0, 1) || '匿'}</Text></View>
                <View className='student-blacklist-user__body'>
                  <Text className='student-blacklist-user__name'>{user.nickname || '匿名用户'}</Text>
                  <Text className='student-blacklist-user__meta'>{blockedAt(user.createdAt)}</Text>
                </View>
                <View className='student-blacklist-user__remove' onClick={() => void remove(user)}><Text>{removing === user.userId ? '处理中' : '移出'}</Text></View>
              </View>
            ))
          ) : (
            <View className='student-blacklist-empty'><Text>暂无黑名单用户</Text></View>
          )}
        </View>
      </View>
    </View>
  )
}
