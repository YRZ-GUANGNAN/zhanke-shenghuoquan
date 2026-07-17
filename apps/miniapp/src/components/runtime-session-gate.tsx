import { useRef, type ReactNode } from 'react'
import { Text, View } from '@tarojs/components'
import { useSessionStore } from '../stores/session'

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
    return (
      <View className='runtime-session-state'>
        <Text className='runtime-session-state__title'>正在登录...</Text>
      </View>
    )
  }

  if (status !== 'authenticated') {
    return (
      <View className='runtime-session-state'>
        <Text className='runtime-session-state__title'>暂时无法登录</Text>
        <Text className='runtime-session-state__message'>{error || '请重新尝试微信登录'}</Text>
        <View className='runtime-session-state__retry' onClick={() => void retry()}>
          <Text>重试</Text>
        </View>
      </View>
    )
  }

  return <>{children}</>
}
