import React, { useCallback, useEffect, useRef, useState } from 'react'
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
import { useFeatureVisibility } from '../../hooks/use-feature-visibility'

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

function handleSettingClick(item: SettingItem, navigate: (url: string) => void) {
  if (item.key === 'settings') {
    navigate('/pages/settings/index')
    return
  }

  if (item.key === 'privacy') {
    navigate('/pages/legal/index')
    return
  }

	if (item.key === 'feedback') {
		navigate('/pages/feedback/index')
		return
	}

	if (item.key === 'about') {
		navigate('/pages/about/index')
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

async function getMyItemCount() {
  return (await getMyProductsPage(1, 1)).total
}

// Each domain owns its failure/loading state. An unread value is not zero.
function useProfileCount<T>(enabled: boolean, read: () => Promise<T>) {
  const [state, setState] = useState<{ data: T | null; loading: boolean; error: boolean }>({ data: null, loading: enabled, error: false })
  const versionRef = useRef(0)
  const busyRef = useRef(false)
  const enabledRef = useRef(enabled)
  enabledRef.current = enabled

  const reload = useCallback(async () => {
    if (!enabledRef.current || busyRef.current) return
    busyRef.current = true
    const version = ++versionRef.current
    setState((current) => ({ ...current, loading: true, error: false }))
    try {
      const data = await read()
      if (version === versionRef.current && enabledRef.current) setState({ data, loading: false, error: false })
    } catch {
      if (version === versionRef.current && enabledRef.current) setState((current) => ({ ...current, loading: false, error: true }))
    } finally {
      if (version === versionRef.current) busyRef.current = false
    }
  }, [read])

  useEffect(() => {
    if (enabled) void reload()
    else setState({ data: null, loading: false, error: false })
    return () => { versionRef.current += 1; busyRef.current = false }
  }, [enabled, reload])

  return { ...state, reload }
}

function countValue(value: number | null | undefined): string {
  return typeof value === 'number' && Number.isFinite(value) && value >= 0 ? String(value) : '—'
}

function ProfileContent() {
  const visibility = useFeatureVisibility()
  const [pendingRole, setPendingRole] = useState<RoleKey | null>(null)
  const [switching, setSwitching] = useState(false)
  const [switchError, setSwitchError] = useState('')
  const [failedAvatar, setFailedAvatar] = useState('')
  const [failedCover, setFailedCover] = useState('')
  const profileNavigationLockRef = useRef(false)
  const roleSwitchLockRef = useRef(false)
  const didShowOnce = useRef(false)
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
  const communityCount = useProfileCount(identityAllowed && visibility.forum, getMyCommunitySummary)
  const itemCount = useProfileCount(identityAllowed && visibility.secondhand, getMyItemCount)
  const countsLoading = visibility.forum && communityCount.loading || visibility.secondhand && itemCount.loading
  const countsError = visibility.forum && communityCount.error || visibility.secondhand && itemCount.error

  useDidShow(() => {
    profileNavigationLockRef.current = false
    void refreshIdentity().catch(() => {
      // Keep the last known summary for transient failures. A 401 is handled
      // centrally by the session store and returns the app to login state.
    })
    if (!didShowOnce.current) { didShowOnce.current = true; return }
    void communityCount.reload()
    void itemCount.reload()
  })

	const stats: Array<{ value: string; label: string; tab: MyContentTabKey }> = [
		{ value: countValue(communityCount.data?.favorites), label: '收藏', tab: 'favorites' },
		{ value: countValue(communityCount.data?.posts), label: '帖子', tab: 'posts' },
		{ value: countValue(communityCount.data?.comments), label: '评论', tab: 'comments' },
		{ value: countValue(itemCount.data), label: '宝贝', tab: 'items' }
	].filter(item => item.tab === 'items' ? visibility.secondhand : item.tab === 'favorites' ? visibility.forum && visibility.favorites : item.tab === 'comments' ? visibility.forum && visibility.comment : visibility.forum) as Array<{ value: string; label: string; tab: MyContentTabKey }>

  const navigateProfile = (url: string) => {
    if (profileNavigationLockRef.current || roleSwitchLockRef.current) return
    profileNavigationLockRef.current = true
    void Taro.navigateTo({ url }).catch(() => {
      profileNavigationLockRef.current = false
      showPlaceholder('页面打开失败，请重试')
    })
  }

  const handleRoleClick = (role: RoleOption) => {
    if (roleSwitchLockRef.current || profileNavigationLockRef.current) return
    if (role.key === activeRole) {
      showPlaceholder(`当前为${role.title}身份`)
      return
    }
    const availability = identity?.identities.find((item) => item.identity === role.key)
    if (!availability?.available) {
      if ((role.key === 'rider' || role.key === 'merchant') && availability?.status === 'missing') {
        if (!visibility.identity_application) { showPlaceholder('身份申请暂未开放'); return }
        navigateProfile(`/pages/identity-application/index?role=${role.key}`)
        return
      }
      showPlaceholder(unavailableMessage(availability))
      return
    }
    setSwitchError('')
    setPendingRole(role.key)
  }

  const openProfileEditor = () => {
    navigateProfile('/pages/edit-profile/index')
  }

  const confirmRoleSwitch = async () => {
    if (!pendingRole || roleSwitchLockRef.current) return
    roleSwitchLockRef.current = true
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
      roleSwitchLockRef.current = false
      setSwitching(false)
    }
  }

  if (!identityAllowed) return <View className='student-profile-page' />

  return (
    <View className='student-profile-page' style={getCapsuleSafeStyle()}>
      <View className='student-profile-hero'>
        <CapsuleSystemRow />
        <Text className='student-profile-hero__title'>我的</Text>
        <View className='student-profile-cover'>
          {profile.cover && profile.cover !== failedCover && <Image className='student-profile-cover__image' src={profile.cover} mode='aspectFill' ariaLabel='我的个人封面' onError={() => setFailedCover(profile.cover || '')} />}
          <View className='student-profile-cover__edit' ariaRole='button' ariaLabel={profile.cover ? '更换个人封面' : '设置个人封面'} onClick={openProfileEditor}>
            <AppIcon name='image' /><Text>{profile.cover ? '更换封面' : '设置封面'}</Text>
          </View>
        </View>
        <View className='student-profile-user' onClick={openProfileEditor} ariaRole='button' ariaLabel='编辑个人资料'>
          <View className='student-profile-user__avatar'>
            {profile.avatar && profile.avatar !== failedAvatar ? (
              <Image className='student-profile-user__avatar-image' src={profile.avatar} mode='aspectFill' onError={() => setFailedAvatar(profile.avatar)} />
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
        {stats.length > 0 && <View className='student-profile-card student-profile-content-card'>
          <View className='student-profile-card__header'>
            <View>
              <Text className='student-profile-card__title'>我的内容</Text>
              <Text className='student-profile-card__description'>{countsLoading ? '正在读取内容数量…' : countsError ? '部分数量暂未更新，已有内容仍可查看' : '收藏与发布内容集中管理'}</Text>
            </View>
            <View className='student-profile-card__link' ariaRole='button' ariaLabel='查看全部我的内容' onClick={() => navigateProfile(`/pages/my-content/index?tab=${stats[0].tab}`)}>
              <Text>查看全部</Text>
              <AppIcon name='chevron-right' className='student-profile-card__link-icon' />
            </View>
          </View>
          <View className='student-profile-stats' style={{ gridTemplateColumns: `repeat(${stats.length}, minmax(0, 1fr))` }}>
            {stats.map((item) => (
              <View
                key={item.tab}
                className='student-profile-stats__item'
                ariaRole='button'
                ariaLabel={`我的${item.label}，${item.value === '—' ? '数量暂未读取' : item.value}`}
                onClick={() => navigateProfile(`/pages/my-content/index?tab=${item.tab}`)}
              >
                <Text className='student-profile-stats__value'>{item.value}</Text>
                <Text className='student-profile-stats__label'>{item.label}</Text>
              </View>
            ))}
          </View>
          {countsError && <View className='student-profile-count-retry' ariaRole='button' ariaLabel='重新读取未更新的内容数量' onClick={() => {
            if (communityCount.error) void communityCount.reload()
            if (itemCount.error) void itemCount.reload()
          }}><AppIcon name='refresh' /><Text>重新读取未更新的数量</Text></View>}
        </View>}

        {(visibility.presentationMode === 'standard' || visibility.delivery) && <View className='student-profile-card student-profile-roles'>
          <View className='student-profile-card__header'>
            <View>
              <Text className='student-profile-card__title'>身份与工作台</Text>
              <Text className='student-profile-card__description'>切换后进入对应的校园工作台</Text>
            </View>
          </View>
          <View className='student-profile-role-grid'>
            {roles.filter(role => role.key === 'student' || visibility.delivery && visibility[role.key]).map((role) => {
              const availability = identity?.identities.find((item) => item.identity === role.key)
              const status = roleStatus(role.key, activeRole, availability)
              return (
                <View
                  key={role.key}
                  className={`student-profile-role ${activeRole === role.key ? 'student-profile-role--active' : ''}`}
                  onClick={() => handleRoleClick(role)}
                  ariaRole='button'
                  ariaLabel={`${role.title}，${status.label}`}
                >
                  <View className='student-profile-role__icon'>
                    <AppIcon name={role.key} active={activeRole === role.key} className='student-profile-role__icon-image' />
                  </View>
                  <View className='student-profile-role__copy'>
                    <Text className='student-profile-role__title'>{role.title}</Text>
                    <Text className='student-profile-role__subtitle'>{role.subtitle}</Text>
                  </View>
                  <Text className={`student-profile-role__status student-profile-role__status--${status.tone}`}>{status.label}</Text>
                  <AppIcon name='chevron-right' className='student-profile-role__arrow' />
                </View>
              )
            })}
          </View>
        </View>}

        {identity?.identities.filter(item => item.identity !== 'student' && item.available && (!visibility.delivery || !visibility[item.identity])).map(item => <View className='student-profile-card student-profile-history' key={item.identity} ariaRole='button' ariaLabel={`查看已有${item.identity === 'merchant' ? '商家' : '骑手'}订单与资金`} onClick={() => handleRoleClick(roles.find(role => role.key === item.identity)!)}><View><Text className='student-profile-card__title'>已有{item.identity === 'merchant' ? '商家' : '骑手'}订单与资金</Text><Text className='student-profile-card__description'>新业务暂停，历史订单仍可处理</Text></View><AppIcon name='chevron-right' className='student-profile-role__arrow' /></View>)}

        <View className='student-profile-card student-profile-settings'>
          <View className='student-profile-settings__header'>
            <Text className='student-profile-card__title'>账户与支持</Text>
          </View>
          {settings.map((item) => (
            <View
              key={item.key}
              className='student-profile-setting'
              onClick={() => handleSettingClick(item, navigateProfile)}
              ariaRole='button'
              ariaLabel={item.title}
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
  const accountScope = useSessionStore((state) => `${state.profile.id}:${state.token}`)
  return (
    <RuntimeSessionGate>
      <ProfileContent key={accountScope} />
    </RuntimeSessionGate>
  )
}
