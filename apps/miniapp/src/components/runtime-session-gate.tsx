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
    return <AppState kind='loading' scene='login' title='正在登录…' description='正在连接你的微信账号' />
  }

  if (status !== 'authenticated') {
    const offline = /网络|超时|network|timeout/i.test(error || '')
    return <AppState kind={offline ? 'offline' : 'error'} title={offline ? '暂时无法连接' : '微信登录暂时不可用'} description={error || '请使用当前微信重新登录，账号会自动恢复'} actionText='重新登录' onAction={() => void retry()} />
  }

  return <>{children}</>
}
