import React, { useCallback, useEffect, useRef, useState } from 'react'
import { ScrollView, Text, View } from '@tarojs/components'
import { useDidHide, useDidShow } from '@tarojs/taro'
import { CapsuleSystemRow } from '../../components/capsule-page-header'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import { AppIconButton, AppNotice } from '../../components/ui'
import { useIdentityGuard } from '../../hooks/use-identity-guard'
import { getDeliveryRiderTodayStats, type DeliveryRiderTodayStats } from '../../services/delivery-rider'
import {
  getRiderWallet, listRiderEarnings, listRiderLedger, listRiderPayoutQueue,
  type RiderEarningView, type RiderLedgerView, type RiderPayoutQueueView, type RiderWalletSummary,
} from '../../services/delivery-rider-wallet'
import Taro from '../../services/taro'
import { useSessionStore } from '../../stores/session'
import './index.scss'

function money(cents: number) { return `¥${(cents / 100).toFixed(2)}` }
function time(value: string) { const date = new Date(value); return Number.isNaN(date.getTime()) ? value.replace('T', ' ').slice(0, 16) : date.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }) }
function statusLabel(value: RiderEarningView['settlementStatus']) { return value === 'paid_offline' ? '已登记线下结款' : value === 'settled' ? '平台待结算' : '待核验' }
function queueLabel(value: RiderPayoutQueueView) { return value.status === 'queued' ? '等待打款通道' : value.status === 'cancelled' ? '已取消排队' : '来源待核验' }

function RiderWalletContent() {
  const allowed = useIdentityGuard('rider')
  const [wallet, setWallet] = useState<RiderWalletSummary | null>(null)
  const [today, setToday] = useState<DeliveryRiderTodayStats | null>(null)
  const [earnings, setEarnings] = useState<RiderEarningView[]>([])
  const [ledger, setLedger] = useState<RiderLedgerView[]>([])
  const [queue, setQueue] = useState<RiderPayoutQueueView[]>([])
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(false)
  const visible = useRef(true)
  const epoch = useRef(0)
  const busy = useRef(false)
  const accountId = useSessionStore(state => state.profile.id)
  const token = useSessionStore(state => state.token)
  const sessionKey = `${accountId}:${token}`
  const activeSessionKey = useRef(sessionKey)
  activeSessionKey.current = sessionKey

  const load = useCallback(async () => {
    if (!allowed || busy.current) return
    busy.current = true
    const version = ++epoch.current
    const current = () => visible.current && epoch.current === version && activeSessionKey.current === sessionKey
    setLoading(true)
    const updateError = (key: string, value: unknown) => { if (current()) setErrors(previous => ({ ...previous, [key]: value instanceof Error ? value.message : '暂时无法加载' })) }
    await Promise.all([
      getRiderWallet().then(value => { if (current()) { setWallet(value); setErrors(previous => ({ ...previous, wallet: '' })) } }).catch(error => updateError('wallet', error)),
      getDeliveryRiderTodayStats().then(value => { if (current()) { setToday(value); setErrors(previous => ({ ...previous, today: '' })) } }).catch(error => updateError('today', error)),
      listRiderEarnings().then(value => { if (current()) { setEarnings(value.items); setErrors(previous => ({ ...previous, earnings: '' })) } }).catch(error => updateError('earnings', error)),
      listRiderLedger().then(value => { if (current()) { setLedger(value.items); setErrors(previous => ({ ...previous, ledger: '' })) } }).catch(error => updateError('ledger', error)),
      listRiderPayoutQueue().then(value => { if (current()) { setQueue(value.items); setErrors(previous => ({ ...previous, queue: '' })) } }).catch(error => updateError('queue', error)),
    ])
    if (current()) setLoading(false)
    busy.current = false
  }, [allowed, sessionKey])

  useDidHide(() => { visible.current = false; epoch.current += 1; busy.current = false })
  useDidShow(() => { visible.current = true; void load() })

  useEffect(() => {
    epoch.current += 1
    busy.current = false
    setWallet(null); setToday(null); setEarnings([]); setLedger([]); setQueue([]); setErrors({})
    if (allowed) void load()
  }, [allowed, sessionKey, load])
  if (!allowed) return <View className='rider-wallet-page' />

  return <View className='rider-wallet-page'>
    <View className='rider-wallet-header'><CapsuleSystemRow /><View className='rider-wallet-nav'>
      <AppIconButton name='back' label='返回' className='rider-wallet-nav__back' onClick={() => void Taro.navigateBack()} />
      <Text className='rider-wallet-nav__title'>收入与结算</Text><View className='rider-wallet-nav__ghost' />
    </View></View>
    <ScrollView scrollY enhanced showScrollbar={false} className='rider-wallet-scroll'>
      <View className='rider-wallet-content'>
        <View className='rider-wallet-balance-card'>
          <Text className='rider-wallet-eyebrow'>账面未结款余额</Text>
          <Text className='rider-wallet-balance'>{wallet ? money(wallet.availableBalanceCents) : '—'}</Text>
          <Text className='rider-wallet-muted'>此余额来自配送账本，不代表微信已到账</Text>
          {errors.wallet ? <Text className='rider-wallet-error' onClick={() => void load()}>{errors.wallet} · 点击重试</Text> : null}
        </View>
        <View className='rider-wallet-stats'>
          <View><Text className='rider-wallet-stat-label'>今日完成单量</Text><Text className='rider-wallet-stat-value'>{today ? today.todayOrders : '—'}<Text className='rider-wallet-stat-unit'>单</Text></Text></View>
          <View><Text className='rider-wallet-stat-label'>今日收入</Text><Text className='rider-wallet-stat-value'>{today ? money(today.todayIncomeCents) : '—'}</Text></View>
        </View>
        <View className='rider-wallet-summary-row'>
          <View><Text>排队占用</Text><Text className='rider-wallet-summary-value'>{wallet ? money(wallet.reservedCents) : '—'}</Text></View>
          <View><Text>来源待核验</Text><Text className='rider-wallet-summary-value'>{wallet ? money(wallet.unverifiedCents) : '—'}</Text></View>
        </View>
        <AppNotice title='结算说明'>平台内部队列只占用已核验收益，当前未调用微信转账；如需线下结款，请由后台先取消排队。</AppNotice>
        <Section title='收入记录' error={errors.earnings} retry={load} empty={earnings.length === 0} loading={loading && earnings.length === 0}>
          {earnings.map(item => <View key={item.earningPublicId} className='rider-wallet-list-item'><View><Text className='rider-wallet-item-title'>{item.deliveryOrderPublicId.slice(-8)}</Text><Text className='rider-wallet-item-time'>{time(item.createdAt)}</Text></View><View className='rider-wallet-item-end'><Text className='rider-wallet-item-amount'>{money(item.amountCents)}</Text><Text className='rider-wallet-chip'>{statusLabel(item.settlementStatus)}</Text></View></View>)}
        </Section>
        <Section title='账本流水' error={errors.ledger} retry={load} empty={ledger.length === 0} loading={loading && ledger.length === 0}>
          {ledger.map(item => <View key={item.ledgerPublicId} className='rider-wallet-list-item'><View><Text className='rider-wallet-item-title'>{item.entryType === 'income' ? '配送收入' : item.entryType === 'payout' ? '线下结款' : '账本调整'}</Text><Text className='rider-wallet-item-time'>{time(item.createdAt)}</Text></View><Text className={`rider-wallet-item-amount ${item.direction === 'credit' ? 'rider-wallet-item-amount--positive' : ''}`}>{item.direction === 'credit' ? '+' : '-'}{money(item.amountCents)}</Text></View>)}
        </Section>
        <Section title='待打款队列' error={errors.queue} retry={load} empty={queue.length === 0} loading={loading && queue.length === 0}>
          {queue.map(item => <View key={item.payoutQueuePublicId} className='rider-wallet-list-item'><View><Text className='rider-wallet-item-title'>{item.deliveryOrderPublicId.slice(-8)}</Text><Text className='rider-wallet-item-time'>{time(item.createdAt)}</Text></View><View className='rider-wallet-item-end'><Text className='rider-wallet-item-amount'>{money(item.amountCents)}</Text><Text className='rider-wallet-chip'>{queueLabel(item)}</Text></View></View>)}
        </Section>
        <Text className='rider-wallet-refresh' onClick={() => void load()}>{loading ? '正在刷新…' : '刷新资金记录'}</Text>
      </View>
    </ScrollView>
  </View>
}

function Section({ title, error, retry, empty, loading, children }: { title: string; error?: string; retry: () => Promise<void>; empty: boolean; loading: boolean; children: React.ReactNode }) {
  return <View className='rider-wallet-section'><View className='rider-wallet-section-head'><Text className='rider-wallet-section-title'>{title}</Text></View>{error ? <Text className='rider-wallet-error' onClick={() => void retry()}>{error} · 点击重试</Text> : loading ? <Text className='rider-wallet-empty'>正在加载…</Text> : empty ? <Text className='rider-wallet-empty'>暂无记录，资金状态以后端为准</Text> : children}</View>
}

export default function RiderWalletPage() { return <RuntimeSessionGate preserveAuthenticatedChildrenDuringRetry><RiderWalletContent /></RuntimeSessionGate> }
