import React, { useEffect, useRef, useState } from 'react'
import { ScrollView, Text, View } from '@tarojs/components'
import { useDidHide, useDidShow } from '@tarojs/taro'
import CapsulePageHeader from '../../components/capsule-page-header'
import { AppButton, AppDialog } from '../../components/ui'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import DeliveryWorkspaceNav from '../../components/delivery-workspace-nav'
import AppIcon from '../../components/app-icon'
import { identityRoute, useIdentityGuard } from '../../hooks/use-identity-guard'
import { getIdentityApplication } from '../../services/a-user'
import { deliveryPaymentEnabled } from '../../feature-flags'
import Taro from '../../services/taro'
import { useSessionStore } from '../../stores/session'
import type { IdentityApplicationView } from '../../types'
import './index.scss'

function DeliveryProfileContent() {
  const role = Taro.getCurrentInstance().router?.params.role === 'merchant' ? 'merchant' : 'rider'
  const allowed = useIdentityGuard(role)
  const profile = useSessionStore(state => state.profile)
  const switchIdentity = useSessionStore(state => state.switchIdentity)
  const [application, setApplication] = useState<IdentityApplicationView | null>(null)
  const [error, setError] = useState('')
  const [reload, setReload] = useState(0)
  const [confirmSwitch, setConfirmSwitch] = useState(false)
  const [switching, setSwitching] = useState(false)
  const [switchError, setSwitchError] = useState('')
  const lock = useRef(false)
  const visible = useRef(true)
  const epoch = useRef(0)
  const token = useSessionStore(state => state.token)
  useDidHide(() => { visible.current = false; epoch.current += 1; lock.current = false })
  useDidShow(() => {
    visible.current = true
    setSwitching(false)
    setConfirmSwitch(false)
    setReload(value => value + 1)
  })
  useEffect(() => {
    let active = true
    const version = ++epoch.current
    const current = () => active && visible.current && epoch.current === version
    lock.current = false
    setSwitching(false)
    setConfirmSwitch(false)
    setSwitchError('')
    setApplication(null)
    setError('')
    if (allowed) void getIdentityApplication(role).then(value => {
      if (current()) setApplication(value)
    }).catch(reason => { if (current()) setError(reason instanceof Error ? reason.message : '资料读取失败') })
    return () => { active = false; epoch.current += 1; lock.current = false }
  }, [allowed, role, profile.id, token, reload])
  const switchToStudent = async () => {
    if (lock.current || !visible.current || !allowed) return
    const version = epoch.current
    const accountId = profile.id
    const current = () => {
      const session = useSessionStore.getState()
      return visible.current && version === epoch.current && session.status === 'authenticated'
        && session.profile.id === accountId && session.token === token
    }
    lock.current = true
    setSwitching(true)
    setSwitchError('')
    try {
      const identity = await switchIdentity('student')
      if (!current() || useSessionStore.getState().identity !== identity || identity.currentIdentity !== 'student') return
      await Taro.reLaunch({ url: identityRoute('student') })
    } catch (reason) {
      if (current()) setSwitchError(reason instanceof Error ? reason.message : '切换失败，请重试')
    } finally { if (current()) { lock.current = false; setSwitching(false) } }
  }
  const open = (url: string) => {
    void Taro.navigateTo({ url }).catch(() => Taro.showToast({ title: '页面打开失败，请重试', icon: 'none' }))
  }
  if (!allowed) return <View className='delivery-profile' />
  const roleName = role === 'merchant' ? '商家' : '骑手'
  const name = (role === 'merchant' ? application?.merchantName : application?.realName) || profile.nickname || roleName
  return <View className='delivery-profile'>
    <CapsulePageHeader title='我的' />
    <ScrollView scrollY enhanced showScrollbar={false} className='delivery-profile__scroll'>
      <View className='delivery-profile__identity'>
        <View className='delivery-profile__avatar'><Text>{name.slice(0, 1)}</Text></View>
        <View className='delivery-profile__name'><Text>{name}</Text><Text className='delivery-profile__caption'>{roleName}工作区</Text></View>
      </View>
      <View className='delivery-profile__section'>
        <Text className='delivery-profile__heading'>{roleName}资料</Text>
        {error ? <Text className='delivery-profile__retry' onClick={() => setReload(value => value + 1)}>{error} · 点击重试</Text> : !application ? <Text className='delivery-profile__caption'>正在读取已提交资料…</Text> : <>
          <View className='delivery-profile__fact'><Text>审核状态</Text><Text>{application.status === 'approved' ? '已通过' : application.status === 'pending' ? '审核中' : '未通过'}</Text></View>
          <View className='delivery-profile__fact'><Text>联系人</Text><Text>{application.realName || '未填写'}</Text></View>
          <View className='delivery-profile__fact'><Text>联系电话</Text><Text>{application.phone || '未填写'}</Text></View>
          {role === 'merchant' ? <View className='delivery-profile__fact'><Text>取货地址</Text><Text>{application.merchantAddress || '未填写'}</Text></View> : null}
        </>}
      </View>
      <View className='delivery-profile__section'>
        <Text className='delivery-profile__heading'>{role === 'merchant' ? '配送资金' : '配送收入'}</Text>
        {role === 'merchant' ? <View className='delivery-profile__link' ariaRole='button' ariaLabel='查看商家钱包与账单' onClick={() => open('/pages/merchant-wallet/index')}>
          <AppIcon name='wallet' /><View className='delivery-profile__link-copy'><Text>钱包与账单</Text><Text className='delivery-profile__caption'>{deliveryPaymentEnabled ? '查看真实余额、配送支出与充值记录' : '支付功能未开通，查看资金说明'}</Text></View><AppIcon name='chevron-right' />
        </View> : <Text className='delivery-profile__caption'>配送收入以后端结算记录为准，当前未开放骑手端余额查询与提现。</Text>}
      </View>
      <View className='delivery-profile__section'>
        <Text className='delivery-profile__heading'>账号与身份</Text>
        <Text className='delivery-profile__caption'>在本工作区处理{roleName === '商家' ? '商家订单和配送消息' : '接单、取送货和配送消息'}。切换身份后可返回学生端。</Text>
        <AppButton variant='secondary' onClick={() => { setSwitchError(''); setConfirmSwitch(true) }}>切换到学生端</AppButton>
      </View>
    </ScrollView>
    <DeliveryWorkspaceNav role={role} active='mine' />
    <AppDialog visible={confirmSwitch} title='切换到学生端' description='配送订单会保留，再次进入对应身份即可继续处理。'
      confirmText='确认切换' loading={switching} error={switchError}
      onConfirm={() => void switchToStudent()} onCancel={() => { if (!switching) setConfirmSwitch(false) }} />
  </View>
}

export default function DeliveryProfilePage() {
  return <RuntimeSessionGate preserveAuthenticatedChildrenDuringRetry><DeliveryProfileContent /></RuntimeSessionGate>
}
