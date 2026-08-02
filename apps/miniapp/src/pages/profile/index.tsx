import React, { useRef, useState } from 'react'
import { Image, Text, View } from '@tarojs/components'
import { useDidShow } from '@tarojs/taro'
import { CapsuleSystemRow } from '../../components/capsule-page-header'
import IdentitySwitchModal from '../../components/IdentitySwitchModal'
import AppIcon, { type AppIconName } from '../../components/app-icon'
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
  description: string
  icon: AppIconName
  arrow?: boolean
}

type RoleStatusTone = 'current' | 'available' | 'pending' | 'action' | 'unavailable'

const roles: RoleOption[] = [
  { key: 'student', title: '学生', subtitle: '社交 · 校园服务' },
  { key: 'rider', title: '骑手', subtitle: '抢单 · 配送' },
  { key: 'merchant', title: '商家', subtitle: '发单 · 配送管理' }
]

const settings: SettingItem[] = [
  { key: 'settings', title: '设置', description: '隐私、通知与账号管理', icon: 'settings', arrow: true },
  { key: 'privacy', title: '用户协议与隐私政策', description: '了解平台规则与数据使用方式', icon: 'document', arrow: true },
  { key: 'feedback', title: '意见反馈', description: '提交问题与改进建议', icon: 'feedback', arrow: true },
  { key: 'about', title: '关于湛科生活圈', description: '产品信息与服务说明', icon: 'info', arrow: true }
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

function roleStatus(
  role: RoleKey,
  activeRole: RoleKey,
  availability?: IdentityAvailability,
): { label: string; tone: RoleStatusTone } {
  if (role === activeRole) return { label: '当前使用', tone: 'current' }
  if (availability?.available) return { label: '已通过', tone: 'available' }
  if (availability?.status === 'pending') return { label: '审核中', tone: 'pending' }
  if (availability?.status === 'missing') return { label: '去申请', tone: 'action' }
  if (availability?.status === 'rejected') return { label: '需完善', tone: 'action' }
  return { label: '暂不可用', tone: 'unavailable' }
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
  const schoolName = profile.school && profile.school !== '未设置' ? profile.school : ''
  const campusName = profile.campus && profile.campus !== '未设置' ? profile.campus : ''
  const campusLabel = Array.from(new Set([schoolName, campusName].filter(Boolean))).join(' · ') || '学校未设置'
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
        <CapsuleSystemRow />
        <View className='student-profile-user'>
          <View className='student-profile-user__avatar'>
            {profile.avatar ? (
              <Image className='student-profile-user__avatar-image' src={profile.avatar} mode='aspectFill' />
            ) : (
              <Text>{profile.nickname.trim().slice(0, 1) || '同'}</Text>
            )}
          </View>
          <View className='student-profile-user__main'>
            <View className='student-profile-user__title-row'>
              <Text className='student-profile-user__name'>{profile.nickname || '校园用户'}</Text>
              <View className='student-profile-user__identity'>
                <Text className='student-profile-user__badge'>{roleName}</Text>
              </View>
            </View>
            <View className='student-profile-user__details'>
              <Text className='student-profile-user__meta'>用户号 {profile.id || '--'}</Text>
              <View className='student-profile-user__campus-row'>
                <View className='student-profile-user__campus-dot' />
                <Text className='student-profile-user__campus'>{campusLabel}</Text>
              </View>
            </View>
          </View>
          <View className='student-profile-user__edit'>
            <Text>编辑资料</Text>
            <AppIcon name='chevron-right' className='student-profile-user__edit-icon' />
          </View>
        </View>
      </View>

      <View className='student-profile-content'>
        <View className='student-profile-card student-profile-content-card'>
          <View className='student-profile-card__header'>
            <View>
              <Text className='student-profile-card__title'>我的内容</Text>
              <Text className='student-profile-card__description'>收藏与发布内容集中管理</Text>
            </View>
            <View className='student-profile-card__link' onClick={() => void Taro.navigateTo({ url: '/pages/my-content/index?tab=posts' })}>
              <Text>查看全部</Text>
              <AppIcon name='chevron-right' className='student-profile-card__link-icon' />
            </View>
          </View>
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
        </View>

        <View className='student-profile-card student-profile-roles'>
          <View className='student-profile-card__header'>
            <View>
              <Text className='student-profile-card__title'>身份与工作台</Text>
              <Text className='student-profile-card__description'>切换后进入对应的校园工作台</Text>
            </View>
          </View>
          <View className='student-profile-role-grid'>
            {roles.map((role) => {
              const availability = identity?.identities.find((item) => item.identity === role.key)
              const status = roleStatus(role.key, activeRole, availability)
              return (
                <View
                  key={role.key}
                  className={`student-profile-role ${activeRole === role.key ? 'student-profile-role--active' : ''}`}
                  onClick={() => handleRoleClick(role)}
                >
                  <View className='student-profile-role__top'>
                    <View className='student-profile-role__icon'>
                      <AppIcon name={role.key} active={activeRole === role.key} className='student-profile-role__icon-image' />
                    </View>
                    <Text className={`student-profile-role__status student-profile-role__status--${status.tone}`}>{status.label}</Text>
                  </View>
                  <Text className='student-profile-role__title'>{role.title}</Text>
                  <Text className='student-profile-role__subtitle'>{role.subtitle}</Text>
                </View>
              )
            })}
          </View>
        </View>

        <View className='student-profile-card student-profile-settings'>
          <View className='student-profile-settings__header'>
            <Text className='student-profile-card__title'>账户与支持</Text>
          </View>
          {settings.map((item) => (
            <View
              key={item.key}
              className='student-profile-setting'
              onClick={() => handleSettingClick(item)}
            >
              <View className='student-profile-setting__icon'>
                <AppIcon name={item.icon} className='student-profile-setting__icon-image' />
              </View>
              <View className='student-profile-setting__copy'>
                <Text className='student-profile-setting__title'>{item.title}</Text>
                <Text className='student-profile-setting__description'>{item.description}</Text>
              </View>
              {item.arrow && <AppIcon name='chevron-right' className='student-profile-setting__arrow app-icon--chevron' />}
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
