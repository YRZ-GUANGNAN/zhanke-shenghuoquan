import React, { useEffect, useMemo, useState } from 'react'
import { Button, Input, ScrollView, Text, View } from '@tarojs/components'
import Taro from '../../services/taro'
import {
  centsToYuan,
  claimOrder,
  completeFulfillment,
  createWithdrawal,
  getRiderMe,
  getWallet,
  listClaimableOrders,
  listFulfillments,
  listWithdrawals,
  markPicked,
  registerRider,
  startToStore,
  startToUser,
  type FulfillmentDetail,
  type FulfillmentStatus,
  type Rider,
  type RiderOrderDetail,
  type RiderStatus,
  type Wallet,
  type Withdraw,
  type WithdrawStatus
} from '../../services/g-takeout-rider'
import './index.scss'

type ActiveTab = 'claim' | 'delivery' | 'wallet'

const riderStatusText: Record<RiderStatus, string> = {
  active: '可接单',
  resting: '休息中',
  suspended: '已暂停'
}

const fulfillmentStatusText: Record<FulfillmentStatus, string> = {
  ASSIGNED: '待到店',
  TO_STORE: '到店取餐',
  PICKED: '已取餐',
  TO_USER: '送达中',
  DELIVERED: '已送达'
}

const withdrawStatusText: Record<WithdrawStatus, string> = {
  pending: '处理中',
  paid: '已打款',
  rejected: '已拒绝'
}

const emptyRegister = { realName: '', phone: '' }

export default function TakeoutRiderPage() {
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [registered, setRegistered] = useState(false)
  const [rider, setRider] = useState<Rider | null>(null)
  const [registerForm, setRegisterForm] = useState(emptyRegister)
  const [tab, setTab] = useState<ActiveTab>('claim')
  const [claimable, setClaimable] = useState<RiderOrderDetail[]>([])
  const [fulfillmentStatus, setFulfillmentStatus] = useState<FulfillmentStatus | ''>('')
  const [fulfillments, setFulfillments] = useState<FulfillmentDetail[]>([])
  const [wallet, setWallet] = useState<Wallet | null>(null)
  const [withdrawals, setWithdrawals] = useState<Withdraw[]>([])
  const [withdrawYuan, setWithdrawYuan] = useState('')

  const riderReady = registered && !!rider
  const canClaim = rider?.status === 'active'

  useEffect(() => {
    void loadMe()
  }, [])

  useEffect(() => {
    if (!riderReady) return
    if (tab === 'claim') void loadClaimable()
    if (tab === 'delivery') void loadFulfillments()
    if (tab === 'wallet') void loadWallet()
  }, [riderReady, tab])

  const activeFulfillmentCount = useMemo(
    () => fulfillments.filter((item) => item.fulfillment.status !== 'DELIVERED').length,
    [fulfillments]
  )

  const loadMe = async () => {
    setLoading(true)
    try {
      const value = await getRiderMe()
      setRegistered(value.registered)
      setRider(value.rider ?? null)
    } catch (error) {
      showError(error, '骑手身份加载失败')
    } finally {
      setLoading(false)
    }
  }

  const submitRegister = async () => {
    const realName = registerForm.realName.trim()
    const phone = registerForm.phone.trim()
    if (!realName) return void Taro.showToast({ title: '请填写真实姓名', icon: 'none' })
    if (!phone) return void Taro.showToast({ title: '请填写手机号', icon: 'none' })
    setSaving(true)
    try {
      const value = await registerRider({ realName, phone })
      setRegistered(true)
      setRider(value)
      setRegisterForm(emptyRegister)
      setTab('claim')
      await Taro.showToast({ title: '骑手注册成功', icon: 'success' })
      await loadClaimable()
    } catch (error) {
      showError(error, '骑手注册失败')
    } finally {
      setSaving(false)
    }
  }

  const loadClaimable = async () => {
    try {
      const page = await listClaimableOrders()
      setClaimable(page.items)
    } catch (error) {
      showError(error, '抢单池加载失败')
    }
  }

  const loadFulfillments = async (nextStatus = fulfillmentStatus) => {
    try {
      const page = await listFulfillments(nextStatus || undefined)
      setFulfillments(page.items)
    } catch (error) {
      showError(error, '配送列表加载失败')
    }
  }

  const loadWallet = async () => {
    try {
      const [walletValue, withdrawalPage] = await Promise.all([getWallet(), listWithdrawals()])
      setWallet(walletValue)
      setWithdrawals(withdrawalPage.items)
    } catch (error) {
      showError(error, '钱包加载失败')
    }
  }

  const doClaim = async (detail: RiderOrderDetail) => {
    if (!canClaim) return void Taro.showToast({ title: '当前骑手状态不可接单', icon: 'none' })
    setSaving(true)
    try {
      await claimOrder(detail.order)
      await Taro.showToast({ title: '抢单成功', icon: 'success' })
      await Promise.all([loadClaimable(), loadFulfillments()])
    } catch (error) {
      showError(error, '抢单失败')
      await loadClaimable()
    } finally {
      setSaving(false)
    }
  }

  const advanceFulfillment = async (detail: FulfillmentDetail) => {
    setSaving(true)
    try {
      if (detail.fulfillment.status === 'ASSIGNED') await startToStore(detail)
      if (detail.fulfillment.status === 'TO_STORE') await markPicked(detail)
      if (detail.fulfillment.status === 'PICKED') await startToUser(detail)
      if (detail.fulfillment.status === 'TO_USER') await completeFulfillment(detail)
      await Taro.showToast({ title: detail.fulfillment.status === 'TO_USER' ? '配送已完成' : '配送状态已更新', icon: 'success' })
      await Promise.all([loadFulfillments(), tab === 'wallet' ? loadWallet() : Promise.resolve()])
    } catch (error) {
      showError(error, '配送状态更新失败')
      await loadFulfillments()
    } finally {
      setSaving(false)
    }
  }

  const submitWithdrawal = async () => {
    const amountCents = yuanToCents(withdrawYuan)
    if (amountCents <= 0) return void Taro.showToast({ title: '请输入有效提现金额', icon: 'none' })
    setSaving(true)
    try {
      await createWithdrawal(amountCents)
      setWithdrawYuan('')
      await Taro.showToast({ title: '提现申请已提交', icon: 'success' })
      await loadWallet()
    } catch (error) {
      showError(error, '提现申请失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <View className='page takeout-rider-page'>
      <View className='rider-hero'>
        <Text className='rider-hero__title'>骑手中心</Text>
        <Text className='rider-hero__desc'>抢单、履约和钱包都在这里；抢单冲突由后端 CAS 保证，页面会友好提示并刷新。</Text>
      </View>

      {loading && <View className='card rider-card'>正在加载骑手身份...</View>}
      {!loading && !registered && renderRegister()}
      {!loading && riderReady && renderRiderHome()}
    </View>
  )

  function renderRegister() {
    return (
      <View className='card rider-card'>
        <Text className='rider-section-title'>注册成为骑手</Text>
        <Text className='rider-tip'>注册后可查看抢单池、推进配送状态，并查看自己的钱包与提现记录。</Text>
        <Input
          className='cc-field'
          value={registerForm.realName}
          placeholder='真实姓名'
          onInput={(event) => setRegisterForm((current) => ({ ...current, realName: String(event.detail.value) }))}
        />
        <Input
          className='cc-field'
          value={registerForm.phone}
          placeholder='手机号'
          type='number'
          onInput={(event) => setRegisterForm((current) => ({ ...current, phone: String(event.detail.value) }))}
        />
        <Button className='cc-button cc-button--primary cc-button--block' disabled={saving} onClick={() => void submitRegister()}>
          {saving ? '注册中...' : '注册骑手'}
        </Button>
      </View>
    )
  }

  function renderRiderHome() {
    return (
      <>
        <View className='card rider-card'>
          <View className='between'>
            <View>
              <Text className='rider-section-title'>{rider?.realName}</Text>
              <Text className='rider-tip'>{rider?.phone} · ID {rider?.riderUserId}</Text>
            </View>
            {rider && <Text className={`rider-status rider-status--${rider.status}`}>{riderStatusText[rider.status]}</Text>}
          </View>
          {!canClaim && <Text className='rider-warning'>当前状态不可抢新单，但仍可查看已有配送和钱包。</Text>}
        </View>

        <View className='rider-tabs'>
          <Text className={tab === 'claim' ? 'rider-tabs__active' : ''} onClick={() => setTab('claim')}>抢单池</Text>
          <Text className={tab === 'delivery' ? 'rider-tabs__active' : ''} onClick={() => setTab('delivery')}>我的配送{activeFulfillmentCount ? ` ${activeFulfillmentCount}` : ''}</Text>
          <Text className={tab === 'wallet' ? 'rider-tabs__active' : ''} onClick={() => setTab('wallet')}>钱包</Text>
        </View>

        {tab === 'claim' && renderClaimPool()}
        {tab === 'delivery' && renderDelivery()}
        {tab === 'wallet' && renderWallet()}
      </>
    )
  }

  function renderClaimPool() {
    return (
      <>
        <View className='rider-toolbar'>
          <Text className='rider-tip'>仅展示已出餐、可被骑手抢的订单。</Text>
          <Button className='cc-button cc-button--small' onClick={() => void loadClaimable()}>刷新</Button>
        </View>
        {claimable.length === 0 && <View className='card rider-card'>暂无可抢订单</View>}
        {claimable.map((detail) => (
          <View key={detail.order.orderPublicId} className='card rider-card'>
            <View className='between'>
              <View>
                <Text className='rider-item-title'>{detail.order.merchantName || '外卖商家'}</Text>
                <Text className='rider-tip'>订单 {shortId(detail.order.orderPublicId)} · ¥{centsToYuan(detail.order.totalAmountCents)}</Text>
              </View>
              <Text className='rider-status rider-status--ready'>可抢</Text>
            </View>
            <Text className='rider-desc'>{detail.order.addressBuilding} {detail.order.addressRoom}</Text>
            <Text className='rider-desc'>联系人：{detail.order.contactName || '-'} · {detail.order.contactPhone || '-'}</Text>
            {renderItems(detail.items)}
            <Button className='cc-button cc-button--primary cc-button--block' disabled={saving || !canClaim} onClick={() => void doClaim(detail)}>
              {saving ? '处理中...' : '抢单'}
            </Button>
          </View>
        ))}
      </>
    )
  }

  function renderDelivery() {
    return (
      <>
        <ScrollView scrollX className='rider-filter' enhanced showScrollbar={false}>
          <View className='rider-filter__inner'>
            {(['', 'ASSIGNED', 'TO_STORE', 'PICKED', 'TO_USER', 'DELIVERED'] as Array<FulfillmentStatus | ''>).map((status) => (
              <Text
                key={status || 'all'}
                className={fulfillmentStatus === status ? 'rider-filter__active' : ''}
                onClick={() => { setFulfillmentStatus(status); void loadFulfillments(status) }}
              >
                {status ? fulfillmentStatusText[status] : '全部'}
              </Text>
            ))}
          </View>
        </ScrollView>
        {fulfillments.length === 0 && <View className='card rider-card'>暂无配送任务</View>}
        {fulfillments.map((detail) => (
          <View key={detail.fulfillment.fulfillmentPublicId} className='card rider-card'>
            <View className='between'>
              <View>
                <Text className='rider-item-title'>{detail.order.merchantName || '外卖商家'}</Text>
                <Text className='rider-tip'>配送 {shortId(detail.fulfillment.fulfillmentPublicId)} · ¥{centsToYuan(detail.order.totalAmountCents)}</Text>
              </View>
              <Text className={`rider-status rider-status--${detail.fulfillment.status}`}>{fulfillmentStatusText[detail.fulfillment.status]}</Text>
            </View>
            <Text className='rider-desc'>订单：{shortId(detail.order.orderPublicId)}</Text>
            <Text className='rider-desc'>地址：{detail.order.addressBuilding} {detail.order.addressRoom}</Text>
            <Text className='rider-desc'>联系人：{detail.order.contactName || '-'} · {detail.order.contactPhone || '-'}</Text>
            {detail.order.remark && <Text className='rider-desc'>备注：{detail.order.remark}</Text>}
            {renderItems(detail.items)}
            {nextFulfillmentLabel(detail.fulfillment.status) && (
              <Button className='cc-button cc-button--primary cc-button--block' disabled={saving} onClick={() => void advanceFulfillment(detail)}>
                {saving ? '处理中...' : nextFulfillmentLabel(detail.fulfillment.status)}
              </Button>
            )}
          </View>
        ))}
      </>
    )
  }

  function renderWallet() {
    return (
      <>
        <View className='card rider-card wallet-card'>
          <Text className='rider-section-title'>可提现 ¥{centsToYuan(wallet?.availableWithdrawCents ?? 0)}</Text>
          <View className='wallet-grid'>
            <View>
              <Text className='wallet-label'>待结算</Text>
              <Text className='wallet-value'>¥{centsToYuan(wallet?.pendingSettlementCents ?? 0)}</Text>
            </View>
            <View>
              <Text className='wallet-label'>提现中</Text>
              <Text className='wallet-value'>¥{centsToYuan(wallet?.pendingWithdrawCents ?? 0)}</Text>
            </View>
            <View>
              <Text className='wallet-label'>已结算</Text>
              <Text className='wallet-value'>¥{centsToYuan(wallet?.settledCents ?? 0)}</Text>
            </View>
          </View>
          <Input className='cc-field' value={withdrawYuan} placeholder='提现金额（元）' type='digit' onInput={(event) => setWithdrawYuan(String(event.detail.value))} />
          <Button className='cc-button cc-button--primary cc-button--block' disabled={saving} onClick={() => void submitWithdrawal()}>
            {saving ? '提交中...' : '申请提现'}
          </Button>
          <Text className='rider-tip'>超过可提现余额会由后端拒绝并提示，不会误判成功。</Text>
        </View>

        <View className='card rider-card'>
          <Text className='rider-section-title'>最近收入</Text>
          {(wallet?.recentEarnings ?? []).length === 0 && <Text className='rider-tip'>暂无收入记录</Text>}
          {(wallet?.recentEarnings ?? []).map((item) => (
            <View key={item.earningPublicId} className='compact-row'>
              <Text>{shortId(item.orderPublicId)} · {item.settlementStatus === 'settled' ? '已结算' : '待结算'}</Text>
              <Text>¥{centsToYuan(item.riderFeeCents)}</Text>
            </View>
          ))}
        </View>

        <View className='card rider-card'>
          <Text className='rider-section-title'>提现记录</Text>
          {withdrawals.length === 0 && <Text className='rider-tip'>暂无提现记录</Text>}
          {withdrawals.map((item) => (
            <View key={item.withdrawPublicId} className='compact-row'>
              <Text>{withdrawStatusText[item.status]} · {formatTime(item.createdAt)}</Text>
              <Text>¥{centsToYuan(item.amountCents)}</Text>
            </View>
          ))}
        </View>
      </>
    )
  }
}

function renderItems(items: Array<{ dishPublicId: string; dishNameSnapshot: string; quantity: number; lineAmountCents: number }>) {
  return (
    <View className='rider-items'>
      {items.map((item) => (
        <View key={`${item.dishPublicId}-${item.quantity}`} className='compact-row'>
          <Text>{item.dishNameSnapshot} × {item.quantity}</Text>
          <Text>¥{centsToYuan(item.lineAmountCents)}</Text>
        </View>
      ))}
    </View>
  )
}

function nextFulfillmentLabel(status: FulfillmentStatus): string {
  if (status === 'ASSIGNED') return '到店取餐'
  if (status === 'TO_STORE') return '已取餐'
  if (status === 'PICKED') return '开始送达'
  if (status === 'TO_USER') return '送达完成'
  return ''
}

function yuanToCents(value: string): number {
  const number = Number(value)
  if (!Number.isFinite(number) || number <= 0) return 0
  return Math.round(number * 100)
}

function shortId(value: string): string {
  return value.length > 10 ? `${value.slice(0, 6)}...${value.slice(-4)}` : value
}

function formatTime(value: string): string {
  return value ? value.replace('T', ' ').slice(0, 16) : '-'
}

function friendlyMessage(error: unknown, fallback: string): string {
  const message = error instanceof Error ? error.message : fallback
  if (/already claimed|concurrent|row_version|invalid transition/i.test(message)) return '该订单已被抢或状态已变化，请刷新后重试'
  if (/in.?flight|limit/i.test(message)) return '在途订单已达上限，请先完成当前配送'
  if (/unavailable|suspended|not active/i.test(message)) return '当前骑手状态不可接单'
  if (/insufficient|balance/i.test(message)) return '超过可提现余额'
  if (/invalid/i.test(message)) return '请求参数不正确'
  return message || fallback
}

function showError(error: unknown, fallback: string) {
  void Taro.showToast({ title: friendlyMessage(error, fallback), icon: 'none' })
}
