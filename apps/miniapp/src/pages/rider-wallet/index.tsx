import React, { useCallback, useEffect, useRef, useState } from 'react'
import { Input, ScrollView, Text, View } from '@tarojs/components'
import { useDidHide, useDidShow } from '@tarojs/taro'
import { CapsuleSystemRow } from '../../components/capsule-page-header'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import { AppActionSheet, AppButton, AppIconButton, AppNotice } from '../../components/ui'
import { useIdentityGuard } from '../../hooks/use-identity-guard'
import { getDeliveryRiderTodayStats, type DeliveryRiderTodayStats } from '../../services/delivery-rider'
import {
  createRiderWithdrawal, getRiderWallet, listRiderEarnings, listRiderLedger, listRiderPayoutQueue, listRiderWithdrawals,
  type RiderEarningView, type RiderLedgerView, type RiderPayoutQueueView, type RiderWalletSummary, type RiderWithdrawalView,
} from '../../services/delivery-rider-wallet'
import { createDeliveryClientRequestId } from '../../services/delivery-merchant'
import { isApiError } from '../../services/api'
import { clearPendingRiderWithdrawal, parseWithdrawalCents, readPendingRiderWithdrawal, savePendingRiderWithdrawal, type PendingRiderWithdrawal } from './withdrawal-state'
import Taro from '../../services/taro'
import { useSessionStore } from '../../stores/session'
import './index.scss'

function money(cents: number) { return `¥${(cents / 100).toFixed(2)}` }
function time(value: string) { const date = new Date(value); return Number.isNaN(date.getTime()) ? value.replace('T', ' ').slice(0, 16) : date.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }) }
function statusLabel(value: RiderEarningView['settlementStatus']) { return value === 'paid_offline' ? '后台已登记' : value === 'settled' ? '已结算' : '待结算' }
function settlementTime(value?: string) {
  if (!value) return '结算时间待同步'
  const instant = Date.parse(value)
  if (!Number.isFinite(instant)) return '结算时间待同步'
  // Beijing has a fixed UTC+08 offset. Do not use the phone's timezone or
  // require Intl timezone support in older Mini Program runtimes.
  const beijing = new Date(instant + 8 * 60 * 60 * 1000)
  const pad = (part: number) => String(part).padStart(2, '0')
  return `${beijing.getUTCMonth() + 1}月${beijing.getUTCDate()}日 ${pad(beijing.getUTCHours())}:${pad(beijing.getUTCMinutes())}（北京时间）`
}
function earningSettlementNote(item: RiderEarningView) {
  if (item.settlementStatus === 'settled') return '已计入平台余额，不代表外部账户到账'
  if (item.settlementStatus === 'paid_offline') return '后台已登记打款，以收款记录为准'
  return item.settleAt ? `预计 ${settlementTime(item.settleAt)}结算` : '预计次日北京时间 00:00 结算，具体时间待同步'
}
function queueLabel(value: RiderPayoutQueueView) { return value.status === 'queued' ? '待结算' : value.status === 'cancelled' ? '已释放占用' : '来源待核实' }
function withdrawalLabel(value: RiderWithdrawalView) { return value.status === 'processing' ? '提现中' : value.status === 'paid' ? '已登记打款' : '已退回余额' }
type RecordKind = 'earnings' | 'ledger' | 'queue' | 'withdrawals'
const initialPages = () => ({ earnings: { page: 1, total: 0 }, ledger: { page: 1, total: 0 }, queue: { page: 1, total: 0 }, withdrawals: { page: 1, total: 0 } })
function appendUnique<T>(previous: T[], next: T[], key: keyof T) { return Array.from(new Map([...previous, ...next].map(item => [item[key], item])).values()) }

function RiderWalletContent() {
  const allowed = useIdentityGuard('rider')
  const [wallet, setWallet] = useState<RiderWalletSummary | null>(null)
  const [today, setToday] = useState<DeliveryRiderTodayStats | null>(null)
  const [earnings, setEarnings] = useState<RiderEarningView[]>([])
  const [ledger, setLedger] = useState<RiderLedgerView[]>([])
  const [queue, setQueue] = useState<RiderPayoutQueueView[]>([])
  const [withdrawals, setWithdrawals] = useState<RiderWithdrawalView[]>([])
  const [withdrawVisible, setWithdrawVisible] = useState(false)
  const [withdrawAmount, setWithdrawAmount] = useState('')
  const [withdrawSubmitting, setWithdrawSubmitting] = useState(false)
  const [pendingWithdrawal, setPendingWithdrawal] = useState<PendingRiderWithdrawal | null>(null)
  const [withdrawError, setWithdrawError] = useState('')
  const withdrawLock = useRef(false)
  const withdrawEpoch = useRef(0)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(false)
  const [pages, setPages] = useState(initialPages)
  const [moreLoading, setMoreLoading] = useState<Partial<Record<RecordKind, boolean>>>({})
  const moreBusy = useRef<Partial<Record<RecordKind, boolean>>>({})
  const visible = useRef(true)
  const mounted = useRef(true)
  const epoch = useRef(0)
  const busy = useRef(false)
  const accountId = useSessionStore(state => state.profile.id)
  const token = useSessionStore(state => state.token)
  const sessionKey = `${accountId}:${token}`
  const activeSessionKey = useRef(sessionKey)
  activeSessionKey.current = sessionKey
  const activeAllowed = useRef(allowed)
  activeAllowed.current = allowed
  const withdrawalReady = wallet?.withdrawalEnabled === true && !loading && !errors.wallet
  const activeWithdrawalReady = useRef(false)
  activeWithdrawalReady.current = Boolean(withdrawalReady)
  const withdrawalAvailability = loading ? '正在核实提现状态'
    : errors.wallet ? '提现状态暂时无法确认，请重试'
    : withdrawalReady ? '提现由后台审核处理' : '提现暂未开放，收入和记录仍可查看'

  const load = useCallback(async () => {
    if (!allowed || !mounted.current || !visible.current || busy.current || activeSessionKey.current !== sessionKey
      || useSessionStore.getState().profile.id !== accountId || useSessionStore.getState().token !== token) return
    busy.current = true
    activeWithdrawalReady.current = false
    const version = ++epoch.current
    const current = () => visible.current && epoch.current === version && activeSessionKey.current === sessionKey
      && useSessionStore.getState().profile.id === accountId && useSessionStore.getState().token === token
    setLoading(true)
    moreBusy.current = {}; setMoreLoading({})
    const firstPage = (kind: RecordKind, total: number) => setPages(previous => ({ ...previous, [kind]: { page: 1, total } }))
    const updateError = (key: string, value: unknown) => { if (current()) setErrors(previous => ({ ...previous, [key]: value instanceof Error ? value.message : '暂时无法加载' })) }
    await Promise.all([
      getRiderWallet().then(value => { if (current()) { setWallet(value); setErrors(previous => ({ ...previous, wallet: '' })) } }).catch(error => updateError('wallet', error)),
      getDeliveryRiderTodayStats().then(value => { if (current()) { setToday(value); setErrors(previous => ({ ...previous, today: '' })) } }).catch(error => updateError('today', error)),
      listRiderEarnings().then(value => { if (current()) { setEarnings(value.items); firstPage('earnings', value.total); setErrors(previous => ({ ...previous, earnings: '' })) } }).catch(error => updateError('earnings', error)),
      listRiderLedger().then(value => { if (current()) { setLedger(value.items); firstPage('ledger', value.total); setErrors(previous => ({ ...previous, ledger: '' })) } }).catch(error => updateError('ledger', error)),
      listRiderPayoutQueue().then(value => { if (current()) { setQueue(value.items); firstPage('queue', value.total); setErrors(previous => ({ ...previous, queue: '' })) } }).catch(error => updateError('queue', error)),
      listRiderWithdrawals().then(value => { if (current()) { setWithdrawals(value.items); firstPage('withdrawals', value.total); setErrors(previous => ({ ...previous, withdrawals: '' })) } }).catch(error => updateError('withdrawals', error)),
    ])
    if (current()) setLoading(false)
    if (epoch.current === version) busy.current = false
  }, [allowed, sessionKey])

  const submitWithdrawal = async () => {
    const current = () => mounted.current && visible.current && activeAllowed.current && activeSessionKey.current === sessionKey
      && useSessionStore.getState().profile.id === accountId && useSessionStore.getState().token === token
    if (!current() || withdrawLock.current || busy.current) return
    // Check the current capability as well as disabled buttons. A stale handler
    // or a restored pending request must not bypass a failed capability refresh.
    if (!activeWithdrawalReady.current) {
      setWithdrawError('提现暂未开放或状态尚未确认，请先核对提现记录。')
      return
    }
    const amountCents = pendingWithdrawal?.amountCents ?? parseWithdrawalCents(withdrawAmount)
    if (amountCents === null) {
      await Taro.showToast({ title: '请输入有效提现金额', icon: 'none' }); return
    }
    if (!pendingWithdrawal && (!wallet || errors.wallet || amountCents > Math.max(0, wallet.availableBalanceCents - wallet.reservedCents))) {
      await Taro.showToast({ title: '金额超过未占用余额，请刷新核实', icon: 'none' }); return
    }
    withdrawLock.current = true
    const version = ++withdrawEpoch.current
    setWithdrawSubmitting(true)
    setWithdrawError('')
    try {
      const pending = pendingWithdrawal ?? { accountId, amountCents, idempotencyKey: createDeliveryClientRequestId() }
      // Persist before sending. A timeout or remount must retry the same request.
      savePendingRiderWithdrawal(pending)
      setPendingWithdrawal(pending)
      const value = await createRiderWithdrawal(pending.amountCents, pending.idempotencyKey)
      if (!current() || version !== withdrawEpoch.current) return
      clearPendingRiderWithdrawal(accountId); setPendingWithdrawal(null)
      setWithdrawals(previous => [value, ...previous.filter(item => item.withdrawPublicId !== value.withdrawPublicId)])
      setWithdrawVisible(false); setWithdrawAmount('')
      epoch.current += 1; busy.current = false
      void Taro.showToast({ title: withdrawalLabel(value), icon: 'none' })
      await load()
    } catch (error) {
      if (!current() || version !== withdrawEpoch.current) return
      // Explicit validation/permission failure is known not to have committed.
      if (isApiError(error) && [400, 401, 403, 422].includes(error.status)) {
        clearPendingRiderWithdrawal(accountId); setPendingWithdrawal(null)
        setWithdrawError(error.message)
      } else {
        setWithdrawError('暂未确认结果，请重试原申请；不会重复扣款。')
      }
    } finally {
      if (version === withdrawEpoch.current) { withdrawLock.current = false; if (mounted.current) setWithdrawSubmitting(false) }
    }
  }

  const loadMore = async (kind: RecordKind) => {
    if (!allowed || !visible.current || busy.current || moreBusy.current[kind]) return
    const version = epoch.current
    const current = () => visible.current && epoch.current === version && activeSessionKey.current === sessionKey
      && useSessionStore.getState().profile.id === accountId && useSessionStore.getState().token === token
    if (!current()) return
    moreBusy.current[kind] = true
    setMoreLoading(previous => ({ ...previous, [kind]: true }))
    const nextPage = pages[kind].page + 1
    try {
      let total: number
      if (kind === 'earnings') {
        const result = await listRiderEarnings(nextPage)
        if (!current()) return
        setEarnings(previous => appendUnique(previous, result.items, 'earningPublicId')); total = result.total
      } else if (kind === 'ledger') {
        const result = await listRiderLedger(nextPage)
        if (!current()) return
        setLedger(previous => appendUnique(previous, result.items, 'ledgerPublicId')); total = result.total
      } else if (kind === 'queue') {
        const result = await listRiderPayoutQueue(nextPage)
        if (!current()) return
        setQueue(previous => appendUnique(previous, result.items, 'payoutQueuePublicId')); total = result.total
      } else {
        const result = await listRiderWithdrawals(nextPage)
        if (!current()) return
        setWithdrawals(previous => appendUnique(previous, result.items, 'withdrawPublicId')); total = result.total
      }
      setPages(previous => ({ ...previous, [kind]: { page: nextPage, total } }))
      setErrors(previous => ({ ...previous, [kind]: '' }))
    } catch (error) {
      if (current()) setErrors(previous => ({ ...previous, [kind]: error instanceof Error ? error.message : '更多记录暂时无法加载' }))
    } finally {
      if (current()) { moreBusy.current[kind] = false; setMoreLoading(previous => ({ ...previous, [kind]: false })) }
    }
  }
  const moreButton = (kind: RecordKind, count: number) => count < pages[kind].total
    ? <AppButton block variant='text' loading={Boolean(moreLoading[kind])} disabled={loading} onClick={() => void loadMore(kind)}>加载更多</AppButton> : null

  useDidHide(() => { visible.current = false; activeWithdrawalReady.current = false; epoch.current += 1; busy.current = false; moreBusy.current = {} })
  useDidShow(() => { visible.current = true; void load() })
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; epoch.current += 1 } }, [])

  useEffect(() => {
    withdrawEpoch.current += 1; withdrawLock.current = false
    setWithdrawSubmitting(false); setWithdrawVisible(false); setWithdrawError(''); setWithdrawAmount('')
    setPendingWithdrawal(readPendingRiderWithdrawal(accountId))
    epoch.current += 1
    busy.current = false
    moreBusy.current = {}; setMoreLoading({}); setPages(initialPages())
    setWallet(null); setToday(null); setEarnings([]); setLedger([]); setQueue([]); setWithdrawals([]); setErrors({})
    if (allowed) void load()
    return () => { epoch.current += 1; busy.current = false }
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
          <Text className='rider-wallet-eyebrow'>已结算余额（元）</Text>
          <Text className='rider-wallet-balance'>{wallet ? money(wallet.availableBalanceCents) : '—'}</Text>
          <Text className='rider-wallet-muted'>次日北京时间 00:00 结算后计入余额 · {withdrawalAvailability}</Text>
          <AppButton className='rider-wallet-withdraw' size='small' variant='secondary' disabled={!withdrawalReady || withdrawSubmitting || (!pendingWithdrawal && (!wallet || wallet.availableBalanceCents <= wallet.reservedCents))} onClick={() => { if (activeWithdrawalReady.current) setWithdrawVisible(true) }}>{pendingWithdrawal ? '继续原提现申请' : '提现'}</AppButton>
          {errors.wallet ? <Text className='rider-wallet-error' onClick={() => void load()}>{errors.wallet} · 点击重试</Text> : null}
        </View>
        <View className='rider-wallet-stats'>
          <View><Text className='rider-wallet-stat-label'>今日完成单量</Text><Text className='rider-wallet-stat-value'>{today ? today.todayOrders : '—'}<Text className='rider-wallet-stat-unit'>单</Text></Text></View>
          <View><Text className='rider-wallet-stat-label'>今日预计收入</Text><Text className='rider-wallet-stat-value'>{today ? money(today.todayIncomeCents) : '—'}</Text></View>
        </View>
        {errors.today ? <Text className='rider-wallet-error' onClick={() => void load()}>{errors.today} · 点击重试今日统计</Text> : null}
        <View className='rider-wallet-summary-row'>
          <View><Text>预计待结算收入</Text><Text className='rider-wallet-summary-value'>{wallet?.pendingSettlementCents !== undefined ? money(wallet.pendingSettlementCents) : '—'}</Text></View>
          <View><Text>最近结算时间</Text><Text className='rider-wallet-item-time'>{wallet?.nextSettlementAt ? settlementTime(wallet.nextSettlementAt) : wallet?.pendingSettlementCents === 0 ? '暂无待结算收入' : '结算时间待同步'}</Text></View>
        </View>
        <View className='rider-wallet-summary-row'>
          <View><Text>结算占用</Text><Text className='rider-wallet-summary-value'>{wallet ? money(wallet.reservedCents) : '—'}</Text></View>
          <View><Text>待核实收入</Text><Text className='rider-wallet-summary-value'>{wallet ? money(wallet.unverifiedCents) : '—'}</Text></View>
        </View>
        <View className='rider-wallet-explanation'><AppNotice title='收入与到账分开查看'>今日预计收入统计今天已完成并产生收入的配送单，预计次日北京时间 00:00 统一结算。已结算仅表示计入平台余额，不代表外部账户到账；待核实收入不计入可提现余额。{withdrawalReady ? '提现提交后显示“提现中”，由后台审核后更新状态。' : '提现暂未开放或状态尚未确认，历史提现记录仍可查看。'}</AppNotice></View>
        <Section title='收入记录' error={errors.earnings} retry={load} empty={earnings.length === 0} loading={loading && earnings.length === 0}>
          {earnings.map(item => <View key={item.earningPublicId} className='rider-wallet-list-item'><View><Text className='rider-wallet-item-title'>{item.deliveryOrderPublicId.slice(-8)}</Text><Text className='rider-wallet-item-time'>{time(item.createdAt)}</Text><Text className='rider-wallet-item-time'>{earningSettlementNote(item)}</Text></View><View className='rider-wallet-item-end'><Text className='rider-wallet-item-amount'>{money(item.amountCents)}</Text><Text className='rider-wallet-chip'>{statusLabel(item.settlementStatus)}</Text></View></View>)}
          {moreButton('earnings', earnings.length)}
        </Section>
        <Section title='账本流水' error={errors.ledger} retry={load} empty={ledger.length === 0} loading={loading && ledger.length === 0}>
          {ledger.map(item => <View key={item.ledgerPublicId} className='rider-wallet-list-item'><View><Text className='rider-wallet-item-title'>{item.entryType === 'income' ? '配送收入' : item.entryType === 'payout' ? '结款 / 提现扣减' : '账本调整 / 退回'}</Text><Text className='rider-wallet-item-time'>{time(item.createdAt)}</Text></View><Text className={`rider-wallet-item-amount ${item.direction === 'credit' ? 'rider-wallet-item-amount--positive' : ''}`}>{item.direction === 'credit' ? '+' : '-'}{money(item.amountCents)}</Text></View>)}
          {moreButton('ledger', ledger.length)}
        </Section>
        <Section title='结算进度' error={errors.queue} retry={load} empty={queue.length === 0} loading={loading && queue.length === 0}>
          {queue.map(item => <View key={item.payoutQueuePublicId} className='rider-wallet-list-item'><View><Text className='rider-wallet-item-title'>{item.deliveryOrderPublicId.slice(-8)}</Text><Text className='rider-wallet-item-time'>{time(item.createdAt)}</Text></View><View className='rider-wallet-item-end'><Text className='rider-wallet-item-amount'>{money(item.amountCents)}</Text><Text className='rider-wallet-chip'>{queueLabel(item)}</Text></View></View>)}
          {moreButton('queue', queue.length)}
        </Section>
        <Section title='提现记录' error={errors.withdrawals} retry={load} empty={withdrawals.length === 0} loading={loading && withdrawals.length === 0}>
          {withdrawals.map(item => <View key={item.withdrawPublicId} className='rider-wallet-list-item'><View><Text className='rider-wallet-item-title'>提现申请</Text><Text className='rider-wallet-item-time'>{time(item.createdAt)}{item.reason ? ` · ${item.reason}` : ''}</Text></View><View className='rider-wallet-item-end'><Text className='rider-wallet-item-amount'>-{money(item.amountCents)}</Text><Text className={`rider-wallet-chip rider-wallet-chip--${item.status}`}>{withdrawalLabel(item)}</Text></View></View>)}
          {moreButton('withdrawals', withdrawals.length)}
        </Section>
      </View>
    </ScrollView>
    <AppActionSheet visible={withdrawVisible} title='申请提现' description={withdrawalReady ? '提现申请提交后由后台处理，余额会先行扣减' : withdrawalAvailability} loading={withdrawSubmitting} onClose={() => setWithdrawVisible(false)}>
      <View className='rider-wallet-withdraw-sheet'>
        <Text className='rider-wallet-stat-label'>未占用余额 {wallet ? money(Math.max(0, wallet.availableBalanceCents - wallet.reservedCents)) : '—'}，以后端核验为准</Text>
        {pendingWithdrawal ? <Text>原申请金额 {money(pendingWithdrawal.amountCents)}</Text> : <Input className='rider-wallet-withdraw-input' type='digit' maxlength={9} disabled={withdrawSubmitting || !withdrawalReady} value={withdrawAmount} placeholder='请输入提现金额（元）' onInput={event => { if (!withdrawLock.current) setWithdrawAmount(String(event.detail.value ?? '')) }} />}
        {withdrawError ? <Text className='rider-wallet-error'>{withdrawError}</Text> : null}
        <AppButton block loading={withdrawSubmitting} disabled={!withdrawalReady} onClick={() => void submitWithdrawal()}>{pendingWithdrawal ? '重试原提现申请' : '提交提现申请'}</AppButton>
      </View>
    </AppActionSheet>
  </View>
}

function Section({ title, error, retry, empty, loading, children }: { title: string; error?: string; retry: () => Promise<void>; empty: boolean; loading: boolean; children: React.ReactNode }) {
  return <View className='rider-wallet-section'><View className='rider-wallet-section-head'><Text className='rider-wallet-section-title'>{title}</Text></View>
    {error ? <Text className='rider-wallet-error' onClick={() => void retry()}>{error}{!empty ? ' · 保留上次记录' : ''} · 点击重试</Text> : null}
    {!empty ? children : loading ? <Text className='rider-wallet-empty'>正在加载…</Text> : !error ? <Text className='rider-wallet-empty'>暂无记录，资金状态以后端为准</Text> : null}
  </View>
}

export default function RiderWalletPage() { return <RuntimeSessionGate preserveAuthenticatedChildrenDuringRetry><RiderWalletContent /></RuntimeSessionGate> }
