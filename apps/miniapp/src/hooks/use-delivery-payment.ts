import { useCallback, useEffect, useRef, useState } from 'react'
import { useDidHide, useDidShow } from '@tarojs/taro'
import { request } from '../services/api'
import { useSessionStore } from '../stores/session'

// A real server provider is the authority. A build flag cannot make an
// unconfigured provider payable, and an old server safely remains disabled.
export function useDeliveryPayment() {
  const userId = useSessionStore(state => state.profile.id)
  const token = useSessionStore(state => state.token)
  const sessionKey = `${userId}:${token}`
  const activeKey = useRef(sessionKey)
  activeKey.current = sessionKey
  const [result, setResult] = useState({
    key: sessionKey,
    enabled: false,
    merchantEarningsEnabled: false,
    merchantEarningsReason: 'capability_unconfirmed',
    checking: true,
    error: ''
  })
  const epoch = useRef(0)
  const busy = useRef(false)
  const visible = useRef(true)
  const refresh = useCallback(async () => {
    if (!visible.current || activeKey.current !== sessionKey || busy.current) return
    if (!userId || !token) {
      setResult({ key: sessionKey, enabled: false, merchantEarningsEnabled: false, merchantEarningsReason: 'capability_unconfirmed', checking: false, error: '' })
      return
    }
    const version = ++epoch.current
    busy.current = true
    const current = () => epoch.current === version && visible.current && activeKey.current === sessionKey
    setResult(previous => ({
      key: sessionKey,
      enabled: previous.key === sessionKey && previous.enabled,
      merchantEarningsEnabled: previous.key === sessionKey && previous.merchantEarningsEnabled,
      merchantEarningsReason: previous.key === sessionKey ? previous.merchantEarningsReason : 'capability_unconfirmed',
      checking: true,
      error: ''
    }))
    try {
      const capability = await request<{ wechatPayment: boolean; merchantEarnings?: boolean; merchantEarningsReason?: string }>('/delivery/capabilities', { url: '', timeout: 10000 })
      if (current()) setResult({
        key: sessionKey,
        enabled: capability.wechatPayment === true,
        merchantEarningsEnabled: capability.merchantEarnings === true,
        merchantEarningsReason: typeof capability.merchantEarningsReason === 'string' && capability.merchantEarningsReason
          ? capability.merchantEarningsReason : 'capability_unconfirmed',
        checking: false,
        error: ''
      })
    } catch {
      if (current()) setResult({ key: sessionKey, enabled: false, merchantEarningsEnabled: false, merchantEarningsReason: 'capability_unconfirmed', checking: false, error: '支付状态暂不可用，稍后重试' })
    } finally {
      if (epoch.current === version) busy.current = false
    }
  }, [userId, token, sessionKey])
  useEffect(() => {
    epoch.current += 1; busy.current = false
    setResult({ key: sessionKey, enabled: false, merchantEarningsEnabled: false, merchantEarningsReason: 'capability_unconfirmed', checking: true, error: '' })
    void refresh()
    return () => { epoch.current += 1; busy.current = false }
  }, [refresh])
  useDidHide(() => { visible.current = false; epoch.current += 1; busy.current = false })
  useDidShow(() => { visible.current = true; void refresh() })
  return {
    enabled: result.key === sessionKey && result.enabled,
    merchantEarningsEnabled: result.key === sessionKey && result.merchantEarningsEnabled,
    merchantEarningsReason: result.key === sessionKey ? result.merchantEarningsReason : 'capability_unconfirmed',
    checking: result.key !== sessionKey || result.checking,
    error: result.key === sessionKey ? result.error : '',
    refresh
  }
}
