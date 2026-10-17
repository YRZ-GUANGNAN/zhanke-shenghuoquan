import { useRef, type ReactNode } from 'react'
import { useSessionStore } from '../stores/session'
import { AppState } from './ui'

interface RuntimeSessionGateProps {
  children: ReactNode
  preserveAuthenticatedChildrenDuringRetry?: boolean
}

export default function RuntimeSessionGate({
  children,
  preserveAuthenticatedChildrenDuringRetry = false
}: RuntimeSessionGateProps) {
  const status = useSessionStore((state) => state.status)
  const error = useSessionStore((state) => state.error)
  const retry = useSessionStore((state) => state.retry)
  const previousStatus = useRef(status)
  const preserveCurrentBoot = useRef(false)

  if (status === 'booting' && previousStatus.current === 'authenticated') {
    preserveCurrentBoot.current = true
  }
  if (status !== 'booting') preserveCurrentBoot.current = false
  previousStatus.current = status

  if (status === 'booting' && preserveAuthenticatedChildrenDuringRetry && preserveCurrentBoot.current) {
    return <>{children}</>
  }

  if (status === 'idle' || status === 'booting') {
    return <AppState kind='loading' title='正在登录…' description='正在安全校验当前微信会话' />
  }

  if (status !== 'authenticated') {
    return <AppState kind='error' title='微信登录暂时不可用' description={error || '请使用当前微信重新登录，账号会自动恢复'} actionText='重新登录' onAction={() => void retry()} />
  }

  return <>{children}</>
}
