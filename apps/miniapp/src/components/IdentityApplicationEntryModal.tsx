import { useCallback, useEffect, useRef, useState } from 'react'
import { ScrollView, Text, View } from '@tarojs/components'
import { useDidHide } from '@tarojs/taro'
import { getIdentityApplication } from '../services/a-user'
import { isApiError } from '../services/api'
import { useSessionStore } from '../stores/session'
import type { IdentityApplicationRole, IdentityApplicationView } from '../types'
import { AppDialog, AppNotice, AppSkeleton } from './ui'
import { ApplicationActivity, ApplicationSummary, applicationRoleCopy, applicationStatusCopy, applicationTime } from './identity-application-summary'
import './IdentityApplicationEntryModal.scss'

interface Props {
  role: IdentityApplicationRole
  onCancel: () => void
  onContinue: (role: IdentityApplicationRole) => Promise<void>
}

export default function IdentityApplicationEntryModal({ role, onCancel, onContinue }: Props) {
  const [application, setApplication] = useState<IdentityApplicationView | null>(null)
  const [loading, setLoading] = useState(true)
  const [loaded, setLoaded] = useState(false)
  const [error, setError] = useState('')
  const [opening, setOpening] = useState(false)
  const [visible, setVisible] = useState(true)
  const aliveRef = useRef(true)
  const sequenceRef = useRef(0)
  const requestLockRef = useRef(false)
  const navigationLockRef = useRef(false)
  const ownerRef = useRef(useSessionStore.getState())
  const isCurrent = useCallback((sequence: number) => {
    const session = useSessionStore.getState()
    return aliveRef.current && sequence === sequenceRef.current
      && session.token === ownerRef.current.token && session.profile.id === ownerRef.current.profile.id
      && session.status === 'authenticated'
  }, [])

  const load = useCallback(async () => {
    if (!isCurrent(sequenceRef.current) || requestLockRef.current || navigationLockRef.current) return
    requestLockRef.current = true
    const sequence = ++sequenceRef.current
    setLoading(true)
    setError('')
    try {
      const value = await getIdentityApplication(role)
      if (!isCurrent(sequence)) return
      if (value.role !== role || value.userId !== ownerRef.current.profile.id) throw new Error('申请资料与当前账号不一致，请重新进入')
      setApplication(value)
      setLoaded(true)
    } catch (cause) {
      if (!isCurrent(sequence)) return
      if (isApiError(cause) && cause.status === 404) {
        setApplication(null)
        setLoaded(true)
      } else {
        setError(isApiError(cause) && cause.status === 0 ? '网络连接失败，请检查网络后重试。' : '暂时无法读取申请状态，请重试。')
      }
    } finally {
      if (isCurrent(sequence)) { requestLockRef.current = false; setLoading(false) }
    }
  }, [isCurrent, role])

  useEffect(() => {
    aliveRef.current = true
    void load()
    return () => { aliveRef.current = false; sequenceRef.current += 1; requestLockRef.current = false }
  }, [load])

  const close = () => {
    aliveRef.current = false
    sequenceRef.current += 1
    setVisible(false)
    onCancel()
  }
  useDidHide(close)

  const proceed = async () => {
    if (!isCurrent(sequenceRef.current) || requestLockRef.current || navigationLockRef.current) return
    if (!loaded) { await load(); return }
    navigationLockRef.current = true
    const sequence = ++sequenceRef.current
    setOpening(true)
    setError('')
    try {
      await onContinue(role)
      if (isCurrent(sequence)) close()
    } catch {
      if (isCurrent(sequence)) setError('页面打开失败，申请资料仍保留，请重试。')
    } finally {
      if (isCurrent(sequence)) { navigationLockRef.current = false; setOpening(false) }
    }
  }

  return <View className='identity-application-entry' catchMove>
    <AppDialog visible={visible} title={applicationRoleCopy[role].title}
      confirmText={loaded ? applicationStatusCopy(application).action : '重新读取'} cancelText='暂不进入'
      loading={loading || opening} error={error} onCancel={close} onConfirm={() => void proceed()}>
      <ScrollView scrollY className='identity-application-entry__scroll'>
        {loading ? <View ariaLabel='正在读取申请状态'><AppSkeleton rows={2} /><View className='identity-application-entry__waiting'><ApplicationActivity /><Text>正在读取申请状态</Text></View></View> : loaded ? <>
          <ApplicationSummary role={role} application={application} compact />
          {application && <View className='identity-application-entry__time'><Text>{application.reviewedAt ? '审核时间' : '提交时间'}</Text><Text>{applicationTime(application.reviewedAt || application.submittedAt)}</Text></View>}
          {application?.status === 'rejected' && <AppNotice tone='danger' title='审核意见'>{application.reviewReason || '资料不符合审核要求，请核对后重新提交。'}</AppNotice>}
          {!application && <View className='identity-application-entry__checklist'><Text>请提前准备</Text><Text>{role === 'merchant' ? '负责人姓名、联系电话、商家名称与地址' : '真实姓名、联系电话'}</Text><Text>1–6 张清晰的证明图片</Text></View>}
        </> : <View className='identity-application-entry__unread'><Text>申请状态暂未确认</Text><Text>连接恢复后可重新读取，不会重复提交申请。</Text></View>}
      </ScrollView>
    </AppDialog>
  </View>
}
