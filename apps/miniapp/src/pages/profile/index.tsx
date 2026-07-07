import React, { useState } from 'react'
import { Text, View } from '@tarojs/components'
import Taro from '../../services/taro'
import './index.scss'

type RoleKey = 'student' | 'rider' | 'merchant'

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

const stats = [
  { value: '23', label: '收藏' },
  { value: '8', label: '帖子' },
  { value: '41', label: '评论' },
  { value: '2', label: '宝贝' }
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
  { key: 'delivery', label: '外卖', icon: 'takeaway', url: '/pages/delivery-merchant/index' },
  { key: 'market', label: '二手', icon: 'market', url: '/pages/market/index' },
  { key: 'profile', label: '我的', icon: 'profile', url: '/pages/profile/index' }
]

function showPlaceholder(title: string) {
  void Taro.showToast({ title, icon: 'none', duration: 1400 })
}

export default function ProfilePage() {
  const [activeRole, setActiveRole] = useState<RoleKey>('student')

  const handleRoleClick = (role: RoleOption) => {
    if (role.key === 'rider') {
      void Taro.navigateTo({ url: '/pages/delivery-rider/index' })
      return
    }
    if (role.key === 'merchant') {
      void Taro.navigateTo({ url: '/pages/merchant-home/index' })
      return
    }
    setActiveRole(role.key)
    showPlaceholder(`${role.title}身份静态占位`)
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
              <Text className='student-profile-user__badge'>学生</Text>
            </View>
            <Text className='student-profile-user__meta'>湛科号：zk_aze233 · 湛江科技学院</Text>
          </View>
          <View className='student-profile-user__edit' onClick={() => showPlaceholder('编辑资料静态占位')}>
            <Text>编辑资料</Text>
          </View>
        </View>
      </View>

      <View className='student-profile-content'>
        <View className='student-profile-stats'>
          {stats.map((item) => (
            <View key={item.label} className='student-profile-stats__item'>
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
              onClick={() => showPlaceholder(`${item.title}后续页面`)}
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
            onClick={() => item.key === 'home' ? void Taro.redirectTo({ url: item.url }) : showPlaceholder(`${item.label}静态占位`)}
          >
            <View className={`student-bottom-nav__icon student-bottom-nav__icon--${item.icon}`} />
            <Text>{item.label}</Text>
          </View>
        ))}
        <View className='student-bottom-nav__fab' onClick={() => showPlaceholder('发布静态占位')}>
          <Text>＋</Text>
        </View>
        {navItems.slice(2).map((item) => (
          <View
            key={item.key}
            className={`student-bottom-nav__item ${item.key === 'profile' ? 'student-bottom-nav__item--active' : ''}`}
            onClick={() => item.key === 'profile' ? undefined : showPlaceholder(`${item.label}静态占位`)}
          >
            <View className={`student-bottom-nav__icon student-bottom-nav__icon--${item.icon}`} />
            <Text>{item.label}</Text>
          </View>
        ))}
      </View>
    </View>
  )
}
