import React, { useState } from 'react'
import { Text, View } from '@tarojs/components'
import Taro from '../../services/taro'
import './index.scss'

type ToggleKey = 'orderNotice' | 'socialNotice' | 'quietHours' | 'personalized'

interface ToggleRow {
  key: ToggleKey
  title: string
  subtitle: string
}

const noticeRows: ToggleRow[] = [
  { key: 'orderNotice', title: '订单通知', subtitle: '接单、取货、送达等状态提醒' },
  { key: 'socialNotice', title: '互动通知', subtitle: '点赞、评论、私聊消息提醒' },
  { key: 'quietHours', title: '夜间免打扰', subtitle: '23:00-7:00 不推送通知' }
]

const privacyRows: ToggleRow[] = [
  { key: 'personalized', title: '个性化推荐', subtitle: '根据浏览记录推荐内容' }
]

function showStaticToast(title: string) {
  void Taro.showToast({ title, icon: 'none', duration: 1400 })
}

export default function SettingsPage() {
  const [toggles, setToggles] = useState<Record<ToggleKey, boolean>>({
    orderNotice: true,
    socialNotice: true,
    quietHours: false,
    personalized: true
  })

  const toggle = (key: ToggleKey) => {
    setToggles((current) => ({
      ...current,
      [key]: !current[key]
    }))
  }

  return (
    <View className='student-settings-page'>
      <View className='student-settings-nav'>
        <View className='student-settings-nav__back' onClick={() => void Taro.navigateBack()}>
          <Text>‹</Text>
        </View>
        <Text className='student-settings-nav__title'>设置</Text>
      </View>

      <View className='student-settings-content'>
        <View className='student-settings-section'>
          <Text className='student-settings-section__label'>账号与绑定</Text>
          <View className='student-settings-card'>
            <View className='student-settings-row'>
              <Text className='student-settings-row__title'>手机号</Text>
              <Text className='student-settings-row__value'>159****8834</Text>
            </View>
            <View className='student-settings-row'>
              <Text className='student-settings-row__title'>微信</Text>
              <View className='student-settings-bind'>
                <View className='student-settings-bind__dot' />
                <Text>已绑定</Text>
              </View>
            </View>
          </View>
        </View>

        <View className='student-settings-section'>
          <Text className='student-settings-section__label'>消息通知</Text>
          <View className='student-settings-card'>
            {noticeRows.map((row) => (
              <View key={row.key} className='student-settings-row student-settings-row--tall'>
                <View className='student-settings-row__main'>
                  <Text className='student-settings-row__title'>{row.title}</Text>
                  <Text className='student-settings-row__subtitle'>{row.subtitle}</Text>
                </View>
                <View
                  className={`student-settings-switch ${toggles[row.key] ? 'student-settings-switch--on' : ''}`}
                  onClick={() => toggle(row.key)}
                >
                  <View className='student-settings-switch__knob' />
                </View>
              </View>
            ))}
          </View>
        </View>

        <View className='student-settings-section'>
          <Text className='student-settings-section__label'>隐私</Text>
          <View className='student-settings-card'>
            {privacyRows.map((row) => (
              <View key={row.key} className='student-settings-row student-settings-row--tall'>
                <View className='student-settings-row__main'>
                  <Text className='student-settings-row__title'>{row.title}</Text>
                  <Text className='student-settings-row__subtitle'>{row.subtitle}</Text>
                </View>
                <View
                  className={`student-settings-switch ${toggles[row.key] ? 'student-settings-switch--on' : ''}`}
                  onClick={() => toggle(row.key)}
                >
                  <View className='student-settings-switch__knob' />
                </View>
              </View>
            ))}
            <View className='student-settings-row' onClick={() => void Taro.navigateTo({ url: '/pages/blacklist/index' })}>
              <Text className='student-settings-row__title'>黑名单管理</Text>
              <View className='student-settings-row__right'>
                <Text className='student-settings-row__value'>2 人</Text>
                <Text className='student-settings-row__arrow'>›</Text>
              </View>
            </View>
          </View>
        </View>

        <View className='student-settings-section'>
          <Text className='student-settings-section__label'>通用</Text>
          <View className='student-settings-card'>
            <View className='student-settings-row' onClick={() => showStaticToast('缓存已清理（静态）')}>
              <Text className='student-settings-row__title'>清理缓存</Text>
              <Text className='student-settings-row__value'>12.6MB</Text>
            </View>
            <View className='student-settings-row' onClick={() => showStaticToast('当前已是最新版本')}>
              <Text className='student-settings-row__title'>检查更新</Text>
              <Text className='student-settings-row__value'>v2.4.0</Text>
            </View>
          </View>
        </View>

        <View className='student-settings-logout' onClick={() => showStaticToast('退出登录静态占位')}>
          <Text>退出登录</Text>
        </View>
      </View>
    </View>
  )
}

