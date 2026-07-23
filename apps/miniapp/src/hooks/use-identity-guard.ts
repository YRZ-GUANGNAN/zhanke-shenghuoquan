import { useEffect, useRef } from 'react'
import Taro from '../services/taro'
import { useSessionStore } from '../stores/session'
import type { IdentityRoleKey } from '../types'

const routeForIdentity: Record<IdentityRoleKey, string> = {
  student: '/pages/profile/index',
  rider: '/pages/delivery-rider/index',
  merchant: '/pages/merchant-home/index'
}

export function identityRoute(identity: IdentityRoleKey): string {
  return routeForIdentity[identity]
}

export function useIdentityGuard(expected: IdentityRoleKey): boolean {
  const status = useSessionStore((state) => state.status)
  const identity = useSessionStore((state) => state.identity)
  const redirected = useRef(false)
  const availability = identity?.identities.find((item) => item.identity === expected)
  const allowed = status === 'authenticated'
    && identity?.currentIdentity === expected
    && (expected === 'student' || availability?.available === true)

  useEffect(() => {
    if (allowed) {
      redirected.current = false
      return
    }
    if (status !== 'authenticated' || !identity || redirected.current) return
    redirected.current = true
    const redirect = async () => {
      try {
        await Taro.showToast({ title: '当前身份无权访问该页面', icon: 'none' })
      } catch {
        // The redirect remains mandatory even if the toast API is unavailable.
      }
      try {
        await Taro.reLaunch({ url: identityRoute(identity.currentIdentity) })
      } catch {
        redirected.current = false
      }
    }
    void redirect()
  }, [allowed, expected, identity, status])

  return allowed
}
