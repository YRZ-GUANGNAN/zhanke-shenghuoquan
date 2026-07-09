import React, { useEffect, useState } from 'react'
import { Text, View } from '@tarojs/components'
import IdentitySwitchModal, { type IdentityRoleKey } from '../../components/IdentitySwitchModal'
import { currentIdentity, switchIdentity } from '../../services/a-user'
import Taro from '../../services/taro'
import type { IdentitySummary } from '../../types'
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

const stats: Array<{ value: string; label: string; tab: MyContentTabKey }> = [
  { value: '23', label: '收藏', tab: 'favorites' },
  { value: '8', label: '帖子', tab: 'posts' },
  { value: '41', label: '评论', tab: 'comments' },
  { value: '2', label: '宝贝', tab: 'items' }
]

const roles: RoleOption[] = [
  { key: 'student', title: '学生', subtitle: '社交 · 校园服务' },
  { key: 'rider', title: '骑手', subtitle: '抢单 · 配送' },
  { key: 'merchant', title: '商家', subtitle: '发单 · 配送管理' }
]

const settings: SettingItem[] = [
  { key: 'settings', title: '设置', icon: 'settings', arrow: true },
  { key: 'privacy', title: '用户协议与隐私政策', icon: 'document', arrow: true },
  { key: 'cache', title: '清理缓存', icon: 'trash', value: '12.6MB' },
  { key: 'update', title: '检查更新', icon: 'refresh', value: 'v2.4.0' },
  { key: 'about', title: '关于湛科生活圈', icon: 'info', arrow: true }
]

const navItems = [
  { key: 'home', label: '首页', icon: 'home', url: '/pages/home/index' },
  { key: 'delivery', label: '外卖', icon: 'takeaway', url: '/pages/student-takeout/index' },
  { key: 'market', label: '二手', icon: 'market', url: '/pages/student-secondhand/index' },
  { key: 'profile', label: '我的', icon: 'profile', url: '/pages/profile/index' }
]

function showPlaceholder(title: string) {
  void Taro.showToast({ title, icon: 'none', duration: 1400 })
}

function showError(error: unknown, fallback: string) {
  const title = error instanceof Error && error.message ? error.message : fallback
  void Taro.showToast({ title, icon: 'none', duration: 1800 })
}

function roleTitle(role: RoleKey) {
  return roles.find((item) => item.key === role)?.title ?? '学生'
}

function roleHome(role: RoleKey) {
  if (role === 'rider') return '/pages/delivery-rider/index'
  if (role === 'merchant') return '/pages/merchant-home/index'
  return '/pages/profile/index'
}

function handleSettingClick(item: SettingItem) {
  if (item.key === 'settings') {
    void Taro.navigateTo({ url: '/pages/settings/index' })
    return
  }

  showPlaceholder(`${item.title}后续页面`)
}

export default function ProfilePage() {
  const [activeRole, setActiveRole] = useState<RoleKey>('student')
  const [pendingRole, setPendingRole] = useState<RoleKey | null>(null)
  const [identitySummary, setIdentitySummary] = useState<IdentitySummary | null>(null)
  const [switchingRole, setSwitchingRole] = useState(false)

  useEffect(() => {
    let alive = true
    void currentIdentity()
      .then((summary) => {
        if (!alive) return
        setIdentitySummary(summary)
        setActiveRole(summary.currentIdentity)
      })
      .catch((error) => {
        if (!alive) return
        showError(error, '身份状态加载失败')
      })
    return () => {
      alive = false
    }
  }, [])

  const handleRoleClick = (role: RoleOption) => {
    if (role.key === activeRole) {
      showPlaceholder(`当前为${role.title}身份`)
      return
    }
    setPendingRole(role.key)
  }

  const confirmRoleSwitch = async () => {
    if (!pendingRole || switchingRole) return

    const target = pendingRole
    setSwitchingRole(true)
    try {
      const summary = await switchIdentity(target)
      setIdentitySummary(summary)
      setActiveRole(summary.currentIdentity)
      setPendingRole(null)
      void Taro.redirectTo({ url: roleHome(summary.currentIdentity) })
    } catch (error) {
      showError(error, '身份切换失败')
      setPendingRole(null)
    } finally {
      setSwitchingRole(false)
    }
  }

  return (
    <View className='student-profile-page'>
      <View className='student-profile-hero'>
        <View className='student-profile-hero__orb student-profile-hero__orb--small' />
        <View className='student-profile-hero__orb student-profile-hero__orb--large' />
        <View className='student-profile-user'>
          <View className='student-profile-user__avatar'>
            <Text>泽</Text>
          </View>
          <View className='student-profile-user__main'>
            <View className='student-profile-user__title-row'>
              <Text className='student-profile-user__name'>阿泽</Text>
              <Text className='student-profile-user__badge'>{roleTitle(activeRole)}</Text>
            </View>
            <Text className='student-profile-user__meta'>湛科号：zk_aze233 · 湛江科技学院</Text>
          </View>
          <View className='student-profile-user__edit' onClick={() => void Taro.navigateTo({ url: '/pages/edit-profile/index' })}>
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
                <View className={`student-profile-role__icon student-profile-role__icon--${role.key}`} />
                <Text className='student-profile-role__title'>{role.title}</Text>
                <Text className='student-profile-role__subtitle'>
                  {identitySummary?.identities.find((item) => item.identity === role.key)?.available === false
                    ? '未开通 · 服务端校验'
                    : role.subtitle}
                </Text>
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

      <View className='student-bottom-nav'>
        {navItems.slice(0, 2).map((item) => (
          <View
            key={item.key}
            className='student-bottom-nav__item'
            onClick={() => void Taro.redirectTo({ url: item.url })}
          >
            <View className={`student-bottom-nav__icon student-bottom-nav__icon--${item.icon}`} />
            <Text>{item.label}</Text>
          </View>
        ))}
        <View className='student-bottom-nav__fab' onClick={() => void Taro.navigateTo({ url: '/pages/publish-post/index' })}>
          <Text>+</Text>
        </View>
        {navItems.slice(2).map((item) => (
          <View
            key={item.key}
            className={`student-bottom-nav__item ${item.key === 'profile' ? 'student-bottom-nav__item--active' : ''}`}
            onClick={() => item.key === 'profile' ? undefined : void Taro.redirectTo({ url: item.url })}
          >
            <View className={`student-bottom-nav__icon student-bottom-nav__icon--${item.icon}`} />
            <Text>{item.label}</Text>
          </View>
        ))}
      </View>

      <IdentitySwitchModal
        visible={Boolean(pendingRole)}
        currentRole={activeRole}
        targetRole={pendingRole ?? activeRole}
        onCancel={() => setPendingRole(null)}
        onConfirm={() => void confirmRoleSwitch()}
      />
    </View>
  )
}
