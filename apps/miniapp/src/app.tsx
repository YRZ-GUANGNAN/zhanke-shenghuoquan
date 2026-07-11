import React, { useEffect, useRef } from 'react'
import { identityRoute } from './hooks/use-identity-guard'
import Taro from './services/taro'
import { useSessionStore } from './stores/session'
import './app.scss'

export default function App(props: { children: React.ReactNode }) {
  const status = useSessionStore((state) => state.status)
  const identity = useSessionStore((state) => state.identity)
  const bootstrap = useSessionStore((state) => state.bootstrap)
  const restoredIdentityRoute = useRef(false)

  useEffect(() => {
    void bootstrap()
  }, [bootstrap])

  useEffect(() => {
    if (status !== 'authenticated') {
      restoredIdentityRoute.current = false
      return
    }
    if (!identity || restoredIdentityRoute.current) return

    let cancelled = false
    let retryTimer: ReturnType<typeof setTimeout> | undefined

    const restoreIdentityRoute = (attempt = 0) => {
      if (cancelled || restoredIdentityRoute.current) return

      const path = Taro.getCurrentInstance().router?.path?.replace(/^\//, '')
      if (!path) {
        if (attempt < 20) {
          retryTimer = setTimeout(() => restoreIdentityRoute(attempt + 1), 50)
        }
        return
      }

      restoredIdentityRoute.current = true
      if (identity.currentIdentity === 'student') return

      if (path === 'pages/home/index' || path === 'pages/profile/index') {
        void Taro.reLaunch({ url: identityRoute(identity.currentIdentity) }).catch(() => {
          restoredIdentityRoute.current = false
        })
      }
    }

    restoreIdentityRoute()
    return () => {
      cancelled = true
      if (retryTimer) clearTimeout(retryTimer)
    }
  }, [identity, status])

  return props.children
}
