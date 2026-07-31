import React, { useState } from 'react'
import { Text, View } from '@tarojs/components'
import { useDidShow } from '@tarojs/taro'
import CapsulePageHeader from '../../components/capsule-page-header'
import { AppDialog } from '../../components/ui'
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
  const [removeCandidate, setRemoveCandidate] = useState<BlockedUserView | null>(null)

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
    setRemoving(value.userId)
    try {
      await removeBlockedUser(value.userId)
      setBlockedUsers((users) => users.filter((user) => user.userId !== value.userId))
      setRemoveCandidate(null)
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
                <View className='student-blacklist-user__remove' onClick={() => setRemoveCandidate(user)}><Text>{removing === user.userId ? '处理中' : '移出'}</Text></View>
              </View>
            ))
          ) : (
            <View className='student-blacklist-empty'><Text>暂无黑名单用户</Text></View>
          )}
        </View>
      </View>
      <AppDialog
        visible={removeCandidate !== null}
        title='移出黑名单'
        description={removeCandidate ? `确认将“${removeCandidate.nickname}”移出黑名单吗？` : ''}
        confirmText='确认移出'
        tone='danger'
        loading={removing !== null}
        onCancel={() => setRemoveCandidate(null)}
        onConfirm={() => { if (removeCandidate) void remove(removeCandidate) }}
      />
    </View>
  )
}
