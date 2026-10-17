import React, { useEffect, useRef, useState } from 'react'
import { View } from '@tarojs/components'
import { AppButton, AppState } from './ui'
import { getCurrentInstance, hideShareMenu, useRouter, useDidShow } from '@tarojs/taro'
import { useSessionStore } from '../stores/session'
import { useFeatureVisibility } from '../hooks/use-feature-visibility'
import { featurePageKeys } from '../services/feature-policy-model'
import { getFeatureVisibility } from '../services/feature-visibility'
import Taro from '../services/taro'
import './feature-page-gate.scss'

export function FeaturePageGate({ children }: { children: React.ReactNode }) {
  const visibility = useFeatureVisibility()
  const [checking, setChecking] = useState(!visibility.available)
  const checkLock = useRef(false)
  const alive = useRef(true)
  useEffect(() => { alive.current = true; return () => { alive.current = false } }, [])
  const router = useRouter()
  const keys = featurePageKeys(router.path, router.params)
  const servicePage = router.path.includes('student-takeout')
  const contentPage = router.path.includes('my-content')
  const allowed = keys.every(key => visibility[key]) && (!servicePage || visibility.takeout || visibility.campus_tools) && (!contentPage || visibility.forum || visibility.secondhand)
  const hideBlockedShare = () => {
    if ((!visibility.available || !allowed) && getCurrentInstance().router?.$taroPath === router.$taroPath) {
      void hideShareMenu({ menus: ['shareAppMessage', 'shareTimeline'] }).catch(() => undefined)
    }
  }
  useEffect(hideBlockedShare, [visibility.available, allowed, router.path])
  useDidShow(hideBlockedShare)
  const retry = async () => {
    if (checkLock.current) return
    checkLock.current = true
    setChecking(true)
    try { await getFeatureVisibility(true) }
    finally { checkLock.current = false; if (alive.current) setChecking(false) }
  }
  useEffect(() => {
    if (!visibility.available) void retry()
  }, [visibility.available])
  if (visibility.available && allowed) return <>{children}</>
  return <View className='feature-page-state'>
    <AppState kind={checking ? 'loading' : visibility.available ? 'forbidden' : 'offline'}
      title={checking ? '正在确认功能状态' : visibility.available ? '该功能暂未开放' : '暂时无法读取功能状态'}
      description={visibility.available ? '可返回首页使用其他已开放的校园服务。' : '请检查网络后重试。'}
      actionText={checking ? undefined : '重新读取'} onAction={() => void retry()} />
    <View className='feature-page-state__actions'>
      <AppButton variant='ghost' onClick={() => void Taro.reLaunch({ url: '/pages/home/index' })}>返回首页</AppButton>
      <AppButton variant='text' onClick={() => void Taro.reLaunch({ url: '/pages/profile/index' })}>我的账号</AppButton>
    </View>
  </View>
}

// A closed page never mounts its business component or starts its effects.
export function withFeaturePage<P extends object>(Page: React.ComponentType<P>, SessionGate?: React.ComponentType<{ children: React.ReactNode }>) {
  return function FeatureProtectedPage(props: P) {
    const status = useSessionStore(state => state.status)
    const router = useRouter()
    const hideWhileLogin = () => {
      if (SessionGate && status !== 'authenticated' && getCurrentInstance().router?.$taroPath === router.$taroPath) {
        void hideShareMenu({ menus: ['shareAppMessage', 'shareTimeline'] }).catch(() => undefined)
      }
    }
    useEffect(hideWhileLogin, [status, router.path])
    useDidShow(hideWhileLogin)
    return <FeaturePageGate>{SessionGate ? <SessionGate><Page {...props} /></SessionGate> : <Page {...props} />}</FeaturePageGate>
  }
}
