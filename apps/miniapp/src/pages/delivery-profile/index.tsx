import React, { useEffect, useRef, useState } from 'react'
import { Image, ScrollView, Text, View } from '@tarojs/components'
import { useDidHide, useDidShow } from '@tarojs/taro'
import { CapsuleSystemRow } from '../../components/capsule-page-header'
import { AppButton, AppDialog } from '../../components/ui'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import DeliveryWorkspaceNav, { useDeliveryWorkspaceNavigation } from '../../components/delivery-workspace-nav'
import ChatAvatar from '../../components/chat-avatar'
import AppIcon from '../../components/app-icon'
import { identityRoute, useIdentityGuard } from '../../hooks/use-identity-guard'
import { getIdentityApplication } from '../../services/a-user'
import { useDeliveryPayment } from '../../hooks/use-delivery-payment'
import Taro from '../../services/taro'
import { useSessionStore } from '../../stores/session'
import type { IdentityApplicationView } from '../../types'
import './index.scss'

function DeliveryProfileContent() {
  const { enabled: deliveryPaymentEnabled } = useDeliveryPayment()
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
  const [failedCover, setFailedCover] = useState('')
  const workspace = useDeliveryWorkspaceNavigation({ role, active: 'mine' }, allowed && !confirmSwitch && !switching)
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
    if (!workspace.swipe.canActivate()) return
    void Taro.navigateTo({ url }).catch(() => Taro.showToast({ title: '页面打开失败，请重试', icon: 'none' }))
  }
  if (!allowed) return <View className='delivery-profile' />
  const roleName = role === 'merchant' ? '商家' : '骑手'
  const name = (role === 'merchant' ? application?.merchantName : application?.realName) || profile.nickname || roleName
  const hasCover = Boolean(profile.cover && failedCover !== profile.cover)
  return <View className={`delivery-profile delivery-profile--${role}`} {...workspace.swipe.handlers}>
    <ScrollView scrollY enhanced showScrollbar={false} className='delivery-profile__scroll'>
      <View className={`delivery-profile__hero${hasCover ? ' delivery-profile__hero--cover' : ''}`}>
        {hasCover ? <Image src={profile.cover || ''} mode='aspectFill' className='delivery-profile__hero-image' ariaLabel={`${roleName}个人封面`} onError={() => setFailedCover(profile.cover || '')} /> : null}
        <CapsuleSystemRow className='delivery-profile__system-row' />
        <View className={`delivery-profile__hero-title${hasCover ? ' delivery-profile__hero-title--cover' : ''}`}><Text>我的</Text></View>
      </View>
      <View className={`delivery-profile__identity${hasCover ? ' delivery-profile__identity--cover' : ''}`}>
        <ChatAvatar className='delivery-profile__avatar' src={profile.avatar} name={name} />
        <View className='delivery-profile__name'><Text className='delivery-profile__role'>{roleName}工作区</Text><Text>{name}</Text><Text className='delivery-profile__caption'>{role === 'merchant' ? '管理店铺资料与配送资金' : '管理骑手资料与配送身份'}</Text></View>
        <View className='delivery-profile__edit' ariaRole='button' ariaLabel='编辑头像、昵称与背景图' hoverClass='delivery-profile__edit--pressed' onClick={() => open('/pages/edit-profile/index')}><AppIcon name='document' /><Text>编辑资料</Text></View>
      </View>
      <View className='delivery-profile__section delivery-profile__section--funds'>
        <Text className='delivery-profile__heading'>{role === 'merchant' ? '配送资金' : '配送收入'}</Text>
        {role === 'merchant' ? <View className='delivery-profile__link' ariaRole='button' ariaLabel='查看商家钱包与账单' onClick={() => open('/pages/merchant-wallet/index')}>
          <AppIcon name='wallet' /><View className='delivery-profile__link-copy'><Text>钱包与账单</Text><Text className='delivery-profile__caption'>{deliveryPaymentEnabled ? '查看真实余额、配送支出与充值记录' : '支付功能未开通，查看资金说明'}</Text></View><AppIcon name='chevron-right' />
        </View> : <View className='delivery-profile__link' ariaRole='button' ariaLabel='查看骑手收入与结算' onClick={() => open('/pages/rider-wallet/index')}>
          <AppIcon name='wallet' /><View className='delivery-profile__link-copy'><Text>收入与结算</Text><Text className='delivery-profile__caption'>查看账面未结款、排队占用与结算记录</Text></View><AppIcon name='chevron-right' />
        </View>}
      </View>
      <View className='delivery-profile__section'>
        <View className='delivery-profile__heading-row'>
          <Text className='delivery-profile__heading'>{roleName}资料</Text>
          <View className='delivery-profile__status' ariaRole='status' ariaLabel='资料审核状态'>
            {error ? '资料待重试' : !application ? '资料读取中' : application.status === 'approved' ? '已通过' : application.status === 'pending' ? '审核中' : '未通过'}
          </View>
        </View>
        {error ? <View className='delivery-profile__retry' ariaRole='button' ariaLabel='重新读取资料' onClick={() => setReload(value => value + 1)}><Text>{error} · 点击重试</Text></View> : !application ? <Text className='delivery-profile__caption'>正在读取已提交资料…</Text> : <>
          <View className='delivery-profile__fact'><Text>联系人</Text><Text>{application.realName || '未填写'}</Text></View>
          <View className='delivery-profile__fact'><Text>联系电话</Text><Text>{application.phone || '未填写'}</Text></View>
          {role === 'merchant' ? <View className='delivery-profile__fact'><Text>取货地址</Text><Text>{application.merchantAddress || '未填写'}</Text></View> : null}
        </>}
      </View>
      <View className='delivery-profile__section delivery-profile__section--account'>
        <Text className='delivery-profile__heading'>账号与身份</Text>
        <Text className='delivery-profile__caption'>在本工作区处理{roleName === '商家' ? '商家订单和配送消息' : '接单、取送货和配送消息'}。切换身份后可返回学生端。</Text>
        <AppButton variant='secondary' onClick={() => { if (workspace.swipe.canActivate()) { setSwitchError(''); setConfirmSwitch(true) } }}>切换到学生端</AppButton>
      </View>
    </ScrollView>
    <DeliveryWorkspaceNav role={role} active='mine' gestureGuard={workspace.swipe.canActivate} />
    <AppDialog visible={confirmSwitch} title='切换到学生端' description='配送订单会保留，再次进入对应身份即可继续处理。'
      confirmText='确认切换' loading={switching} error={switchError}
      onConfirm={() => void switchToStudent()} onCancel={() => { if (!switching) setConfirmSwitch(false) }} />
  </View>
}

export default function DeliveryProfilePage() {
  return <RuntimeSessionGate preserveAuthenticatedChildrenDuringRetry><DeliveryProfileContent /></RuntimeSessionGate>
}
