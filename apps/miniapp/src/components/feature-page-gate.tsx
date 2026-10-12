import React from 'react'
import { Button, Text, View } from '@tarojs/components'
import { useRouter } from '@tarojs/taro'
import { useFeatureVisibility } from '../hooks/use-feature-visibility'
import { featurePageKeys } from '../services/feature-policy-model'
import { getFeatureVisibility } from '../services/feature-visibility'
import Taro from '../services/taro'
import './feature-page-gate.scss'

export function FeaturePageGate({ children }: { children: React.ReactNode }) {
  const router = useRouter()
  const visibility = useFeatureVisibility()
  const keys = featurePageKeys(router.path, router.params)
  const servicePage = router.path.includes('student-takeout')
  const contentPage = router.path.includes('my-content')
  const allowed = keys.every(key => visibility[key]) && (!servicePage || visibility.takeout || visibility.campus_tools) && (!contentPage || visibility.forum || visibility.secondhand)
  if (visibility.available && allowed) return <>{children}</>
  return <View className='feature-page-state'>
    <Text className='feature-page-state__title'>{visibility.available ? '该功能暂未开放' : '正在确认功能状态'}</Text>
    <Text className='feature-page-state__description'>{visibility.available ? '可返回首页使用其他已开放的校园服务。' : '请检查网络并重试，功能状态确认后会自动显示。'}</Text>
    <Button className='cc-button cc-button--primary' onClick={() => void getFeatureVisibility(true)}>重新读取</Button>
    <Button className='cc-button' onClick={() => void Taro.reLaunch({ url: '/pages/home/index' })}>返回首页</Button>
    <Button className='cc-button' onClick={() => void Taro.reLaunch({ url: '/pages/profile/index' })}>我的账号</Button>
  </View>
}

// A closed page never mounts its business component or starts its effects.
export function withFeaturePage<P extends object>(Page: React.ComponentType<P>) {
  return function FeatureProtectedPage(props: P) {
    return <FeaturePageGate><Page {...props} /></FeaturePageGate>
  }
}
