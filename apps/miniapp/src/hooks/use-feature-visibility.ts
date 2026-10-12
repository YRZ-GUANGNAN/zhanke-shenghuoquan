import { useEffect, useMemo, useState } from 'react'
import { useDidHide, useDidShow } from '@tarojs/taro'
import { featureVisibilitySnapshot, getFeatureVisibility, subscribeFeatureVisibility } from '../services/feature-visibility'
import { useSessionStore } from '../stores/session'

export function useFeatureVisibility() {
  const [value, setValue] = useState(featureVisibilitySnapshot)
  const role = useSessionStore(state => state.profile.role)
  useEffect(() => {
    const unsubscribe = subscribeFeatureVisibility(setValue)
    void getFeatureVisibility().then(setValue)
    return unsubscribe
  }, [])
  useDidShow(() => { void getFeatureVisibility(true) })
  const mayWrite = value.postingMode === 'all_users' || role === 'admin' || role === 'superadmin'
  return useMemo(() => ({ ...value, post: value.post && mayWrite, canComment: value.comment && mayWrite }), [value, mayWrite])
}

export function useFeaturePolicyRefresh() {
  const [foreground, setForeground] = useState(true)
  useDidShow(() => { setForeground(true); void getFeatureVisibility(true) })
  useDidHide(() => setForeground(false))
  useEffect(() => {
    if (!foreground) return
    void getFeatureVisibility(true)
    const timer = setInterval(() => { void getFeatureVisibility(true) }, 15000)
    return () => clearInterval(timer)
  }, [foreground])
}
