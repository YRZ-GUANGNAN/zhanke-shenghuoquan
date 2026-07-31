import React, { useRef, useState } from 'react'
import { Text, View } from '@tarojs/components'
import { useDidShow } from '@tarojs/taro'
import { CapsuleSystemRow } from '../../components/capsule-page-header'
import IdentitySwitchModal from '../../components/IdentitySwitchModal'
import AppIcon from '../../components/app-icon'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import StudentBottomNav from '../../components/student-bottom-nav'
import { identityRoute, useIdentityGuard } from '../../hooks/use-identity-guard'
import Taro from '../../services/taro'
import { getMyCommunitySummary } from '../../services/b-community'
import { getMyProductsPage } from '../../services/c-trade'
import { useSessionStore } from '../../stores/session'
import type { IdentityAvailability, IdentityRoleKey } from '../../types'
import { getCapsuleSafeStyle } from '../../utils/capsule-safe'
import './index.scss'

type RoleKey = IdentityRoleKey
type MyContentTabKey = 'favorites' | 'posts' | 'comments' | 'items'

interface RoleOption {
  key: RoleKey
  title: string
  subtitle: string
}

interface SettingItem {
  key: string
  title: string
  value?: string
  icon: string
  arrow?: boolean
}

const roles: RoleOption[] = [
  { key: 'student', title: '学生', subtitle: '社交 · 校园服务' },
  { key: 'rider', title: '骑手', subtitle: '抢单 · 配送' },
  { key: 'merchant', title: '商家', subtitle: '发单 · 配送管理' }
]

const settings: SettingItem[] = [
  { key: 'settings', title: '设置', icon: 'settings', arrow: true },
  { key: 'privacy', title: '用户协议与隐私政策', icon: 'document', arrow: true },
	{ key: 'feedback', title: '意见反馈', icon: 'refresh', arrow: true },
  { key: 'about', title: '关于湛科生活圈', icon: 'info', arrow: true }
]

function showPlaceholder(title: string) {
  void Taro.showToast({ title, icon: 'none', duration: 1400 })
}

function handleSettingClick(item: SettingItem) {
  if (item.key === 'settings') {
    void Taro.navigateTo({ url: '/pages/settings/index' })
    return
  }

  if (item.key === 'privacy') {
    void Taro.navigateTo({ url: '/pages/legal/index' })
    return
  }

	if (item.key === 'feedback') {
		void Taro.navigateTo({ url: '/pages/feedback/index' })
		return
	}

	if (item.key === 'about') {
		void Taro.navigateTo({ url: '/pages/about/index' })
		return
	}

  showPlaceholder(item.title)
}

function unavailableMessage(value?: IdentityAvailability): string {
  if (!value) return '该身份暂不可用'
  if (value.status === 'disabled') return '配送功能当前未开放'
  if (value.status === 'pending') return '身份申请正在审核，请耐心等待'
  if (value.status === 'rejected') return '身份申请未通过，可修改资料后重新提交'
  if (value.status === 'resting') return '骑手当前为休息状态，请先恢复接单'
  if (value.status === 'suspended') return '该身份已暂停，请联系平台'
  if (value.status === 'missing') return value.identity === 'rider'
    ? '请先申请骑手身份，审核通过后才能进入'
    : '请先申请商家身份，审核通过后才能进入'
  return value.reason || '该身份暂不可用'
}

function ProfileContent() {
  const [pendingRole, setPendingRole] = useState<RoleKey | null>(null)
  const [switching, setSwitching] = useState(false)
  const [switchError, setSwitchError] = useState('')
	const [contentCounts, setContentCounts] = useState({ favorites: 0, posts: 0, comments: 0, items: 0 })
  const profileNavigationLockRef = useRef(false)
  const profile = useSessionStore((state) => state.profile)
  const identity = useSessionStore((state) => state.identity)
  const refreshIdentity = useSessionStore((state) => state.refreshIdentity)
  const switchIdentity = useSessionStore((state) => state.switchIdentity)
  const activeRole = identity?.currentIdentity ?? 'student'
  const roleName = roles.find((role) => role.key === activeRole)?.title ?? '学生'
  const identityAllowed = useIdentityGuard('student')

  useDidShow(() => {
    profileNavigationLockRef.current = false
    void refreshIdentity().catch(() => {
      // Keep the last known summary for transient failures. A 401 is handled
      // centrally by the session store and returns the app to login state.
    })
		void Promise.all([getMyCommunitySummary(), getMyProductsPage(1, 1)]).then(([community, products]) => {
			setContentCounts({ ...community, items: products.total })
		}).catch(() => {
			// Counts are secondary; the content pages still expose their own retry states.
		})
  })

	const stats: Array<{ value: string; label: string; tab: MyContentTabKey }> = [
		{ value: String(contentCounts.favorites), label: '收藏', tab: 'favorites' },
		{ value: String(contentCounts.posts), label: '帖子', tab: 'posts' },
		{ value: String(contentCounts.comments), label: '评论', tab: 'comments' },
		{ value: String(contentCounts.items), label: '宝贝', tab: 'items' }
	]

  const handleRoleClick = (role: RoleOption) => {
    if (role.key === activeRole) {
      showPlaceholder(`当前为${role.title}身份`)
      return
    }
    const availability = identity?.identities.find((item) => item.identity === role.key)
    if (!availability?.available) {
      if ((role.key === 'rider' || role.key === 'merchant') && availability?.status === 'missing') {
        void Taro.navigateTo({ url: `/pages/identity-application/index?role=${role.key}` })
        return
      }
      showPlaceholder(unavailableMessage(availability))
      return
    }
    setSwitchError('')
    setPendingRole(role.key)
  }

  const openProfileEditor = () => {
    if (profileNavigationLockRef.current) return
    profileNavigationLockRef.current = true
    void Taro.navigateTo({ url: '/pages/edit-profile/index' }).catch(() => {
      profileNavigationLockRef.current = false
    })
  }

  const confirmRoleSwitch = async () => {
    if (!pendingRole || switching) return
    const target = pendingRole
    setSwitching(true)
    setSwitchError('')
    try {
      await switchIdentity(target)
      await Taro.reLaunch({ url: identityRoute(target) })
      setPendingRole(null)
    } catch (error) {
      setSwitchError(error instanceof Error ? error.message : '身份切换失败，请重试')
    } finally {
      setSwitching(false)
    }
  }

  if (!identityAllowed) return <View className='student-profile-page' />

  return (
    <View className='student-profile-page' style={getCapsuleSafeStyle()}>
      <View className='student-profile-hero' onClick={openProfileEditor}>
        <View className='student-profile-hero__orb student-profile-hero__orb--small' />
        <View className='student-profile-hero__orb student-profile-hero__orb--large' />
        <CapsuleSystemRow />
        <View className='student-profile-user'>
          <View className='student-profile-user__avatar'>
            <Text>{profile.nickname.trim().slice(0, 1) || '同'}</Text>
          </View>
          <View className='student-profile-user__main'>
            <View className='student-profile-user__title-row'>
              <Text className='student-profile-user__name'>{profile.nickname || '校园用户'}</Text>
              <Text className='student-profile-user__badge'>{roleName}</Text>
            </View>
            <Text className='student-profile-user__meta'>
              用户号：{profile.id || '--'} · {profile.campus || profile.school || '校园用户'}
            </Text>
          </View>
          <View className='student-profile-user__edit'>
            <Text>编辑资料</Text>
          </View>
        </View>
      </View>

      <View className='student-profile-content'>
        <View className='student-profile-stats'>
          {stats.map((item) => (
            <View
              key={item.label}
              className='student-profile-stats__item'
              onClick={() => void Taro.navigateTo({ url: `/pages/my-content/index?tab=${item.tab}` })}
            >
              <Text className='student-profile-stats__value'>{item.value}</Text>
              <Text className='student-profile-stats__label'>{item.label}</Text>
            </View>
          ))}
        </View>

        <View className='student-profile-card student-profile-roles'>
          <View className='student-profile-card__header'>
            <Text className='student-profile-card__title'>身份切换</Text>
            <Text className='student-profile-card__hint'>切换后主界面完全不同</Text>
          </View>
          <View className='student-profile-role-grid'>
            {roles.map((role) => (
              <View
                key={role.key}
                className={`student-profile-role ${activeRole === role.key ? 'student-profile-role--active' : ''}`}
                onClick={() => handleRoleClick(role)}
              >
                {activeRole === role.key && <Text className='student-profile-role__corner'>当前</Text>}
                <View className='student-profile-role__icon'>
                  <AppIcon name={role.key} active={activeRole === role.key} className='student-profile-role__icon-image' />
                </View>
                <Text className='student-profile-role__title'>{role.title}</Text>
                <Text className='student-profile-role__subtitle'>{role.subtitle}</Text>
              </View>
            ))}
          </View>
        </View>

        <View className='student-profile-card student-profile-settings'>
          {settings.map((item) => (
            <View
              key={item.key}
              className='student-profile-setting'
              onClick={() => handleSettingClick(item)}
            >
              <View className={`student-profile-setting__icon student-profile-setting__icon--${item.icon}`} />
              <Text className='student-profile-setting__title'>{item.title}</Text>
              {item.value && <Text className='student-profile-setting__value'>{item.value}</Text>}
              {item.arrow && <Text className='student-profile-setting__arrow'>›</Text>}
            </View>
          ))}
        </View>
      </View>

      <StudentBottomNav active='profile' />

      <IdentitySwitchModal
        visible={Boolean(pendingRole)}
        currentRole={activeRole}
        targetRole={pendingRole ?? activeRole}
        onCancel={() => setPendingRole(null)}
        onConfirm={() => void confirmRoleSwitch()}
        confirming={switching}
        error={switchError}
      />
    </View>
  )
}

export default function ProfilePage() {
  return (
    <RuntimeSessionGate>
      <ProfileContent />
    </RuntimeSessionGate>
  )
}
