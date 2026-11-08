import React, { useCallback, useEffect, useRef, useState } from 'react'
import { Input, ScrollView, Text, View } from '@tarojs/components'
import { useDidHide, useDidShow } from '@tarojs/taro'
import { CapsuleSystemRow } from '../../components/capsule-page-header'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import { AppActionSheet, AppButton, AppIconButton, AppNotice } from '../../components/ui'
import { useDeliveryPayment } from '../../hooks/use-delivery-payment'
import { useIdentityGuard } from '../../hooks/use-identity-guard'
import {
  getDeliveryWallet,
  listDeliveryWalletLedger,
  type DeliveryLedgerEntryType,
  type DeliveryWallet,
  type DeliveryWalletLedger,
} from '../../services/delivery-payment'
import Taro from '../../services/taro'
import { useSessionStore } from '../../stores/session'
import { createDeliveryClientRequestId } from '../../services/delivery-merchant'
import { clearPendingWalletRecharge, readPendingWalletRecharge, savePendingWalletRecharge, type PendingWalletRecharge } from './wallet-state'
import { runWalletRecharge } from './wallet-recharge'
import './index.scss'

const rechargeOptions = [50, 100, 200, 300, 500, 1000]

function parseYuanToCents(value: string): number | null {
  const normalized = value.trim()
  if (!/^(?:0|[1-9]\d{0,5})(?:\.\d{1,2})?$/.test(normalized)) return null
  const [yuan, fraction = ''] = normalized.split('.')
  const cents = Number(yuan) * 100 + Number(fraction.padEnd(2, '0'))
  return cents > 0 && cents <= 10_000_000 ? cents : null
}

function formatBalance(value?: number) {
  return value == null ? '--' : (value / 100).toFixed(2)
}

function formatTime(value: string) {
  return value.replace('T', ' ').slice(0, 16)
}

function entryTitle(entryType: DeliveryLedgerEntryType) {
  if (entryType === 'recharge') return '钱包充值'
  if (entryType === 'refund') return '配送费退款'
  return '支付配送费'
}

function entrySymbol(entryType: DeliveryLedgerEntryType) {
  if (entryType === 'recharge') return '↓'
  if (entryType === 'refund') return '↶'
  return '账'
}

function MerchantEarningsStatus({ checking, error, enabled, reason }: { checking?: boolean; error?: string; enabled?: boolean; reason?: string }) {
  const unavailable = reason === 'external_order_source_unavailable'
  return <View className='merchant-wallet-earnings-card'>
    <View className='merchant-wallet-earnings-card__header'>
      <Text className='merchant-wallet-earnings-card__title'>收益账户</Text>
      <Text className={`merchant-wallet-earnings-card__chip ${enabled ? 'merchant-wallet-earnings-card__chip--ready' : ''}`}>
        {checking ? '核实中' : enabled ? '已接通' : unavailable ? '待接入' : '状态待确认'}
      </Text>
    </View>
    <Text className='merchant-wallet-earnings-card__description'>
      {enabled
        ? '仅展示已由官方外部订单接口核验的真实应收，配送充值余额与商品收益始终分账。'
        : unavailable
          ? '外部订单官方接口尚未提供真实应收凭证，当前不入账、不开放收益提现；配送充值余额不会被当作商品收益。'
          : error || '收益能力尚未由服务端确认，当前不入账、不开放提现。'}
    </Text>
  </View>
}

function MerchantWalletContent() {
  const {
    enabled: deliveryPaymentEnabled,
    merchantEarningsEnabled,
    merchantEarningsReason,
    checking: paymentChecking,
    error: paymentError,
    refresh: refreshPayment
  } = useDeliveryPayment()
  const identityAllowed = useIdentityGuard('merchant')
  const [wallet, setWallet] = useState<DeliveryWallet | null>(null)
  const [ledger, setLedger] = useState<DeliveryWalletLedger[]>([])
  const [selectedAmount, setSelectedAmount] = useState(100)
  const [customAmount, setCustomAmount] = useState('')
  const rechargeAmountCents = customAmount !== '' ? parseYuanToCents(customAmount) : selectedAmount * 100
  const [sheetVisible, setSheetVisible] = useState(false)
  const sheetOpen = useRef(false)
  const showRechargeSheet = (open: boolean) => { sheetOpen.current = open; setSheetVisible(open) }
  const [loading, setLoading] = useState(false)
  const [paying, setPaying] = useState(false)
  const [loadError, setLoadError] = useState('')
  const [rechargeNotice, setRechargeNotice] = useState('')
  const [pendingRecharge, setPendingRecharge] = useState<PendingWalletRecharge | null>(null)
  const [ledgerPage, setLedgerPage] = useState(1)
  const [ledgerTotal, setLedgerTotal] = useState(0)
  const [moreLoading, setMoreLoading] = useState(false)
  const accountId = useSessionStore(state => state.profile.id)
  const sessionToken = useSessionStore(state => state.token)
  const sessionKey = `${accountId}:${sessionToken}`
  const rechargeLock = useRef(false)
  const cancellationLock = useRef(false)
  const cancellationRequested = useRef(false)
  const nativePaymentOpen = useRef(false)
  const rechargeEpoch = useRef(0)
  const mounted = useRef(true)
  const activeSessionKey = useRef(sessionKey)
  const visible = useRef(true)
  const loadEpoch = useRef(0)
  const loadingRef = useRef(false)
  const paymentContext = `${sessionKey}:${identityAllowed}:${deliveryPaymentEnabled}`
  const activePaymentContext = useRef(paymentContext)
  activePaymentContext.current = paymentContext
  activeSessionKey.current = sessionKey

  const loadWallet = useCallback(async (more = false) => {
    if (!identityAllowed || !deliveryPaymentEnabled || !mounted.current || !visible.current || loadingRef.current
      || activeSessionKey.current !== sessionKey || useSessionStore.getState().profile.id !== accountId || useSessionStore.getState().token !== sessionToken) return
    const version = ++loadEpoch.current
    const current = () => visible.current && loadEpoch.current === version && activeSessionKey.current === sessionKey
    loadingRef.current = true
    if (more) setMoreLoading(true)
    else setLoading(true)
    setLoadError('')
    try {
      if (!more) {
        const walletValue = await getDeliveryWallet()
        if (!current()) return
        setWallet(walletValue)
      }
      const nextPage = more ? ledgerPage + 1 : 1
      const ledgerValue = await listDeliveryWalletLedger(nextPage, 30)
      if (current()) {
        setLedger(previous => more ? Array.from(new Map([...previous, ...ledgerValue.items].map(item => [item.ledgerPublicId, item])).values()) : ledgerValue.items)
        setLedgerPage(nextPage); setLedgerTotal(ledgerValue.total)
      }
    } catch (error) {
      if (current()) setLoadError(error instanceof Error ? error.message : '钱包加载失败')
    } finally {
      if (current()) { setLoading(false); setMoreLoading(false) }
      if (loadEpoch.current === version) loadingRef.current = false
    }
  }, [identityAllowed, deliveryPaymentEnabled, sessionKey, ledgerPage])

  useDidHide(() => {
    visible.current = false; loadEpoch.current += 1; loadingRef.current = false
    if (!nativePaymentOpen.current) { rechargeEpoch.current += 1; rechargeLock.current = false; cancellationLock.current = false; setPaying(false); showRechargeSheet(false) }
  })
  useDidShow(() => {
    visible.current = true
    void loadWallet()
    if (readPendingWalletRecharge(accountId)?.cancelRequested) void confirmRecharge(true, true)
  })

  useEffect(() => {
    loadEpoch.current += 1
    loadingRef.current = false
    rechargeEpoch.current += 1; rechargeLock.current = false; cancellationLock.current = false; cancellationRequested.current = false
    setWallet(null); setLedger([]); setLoadError(''); setLedgerPage(1); setLedgerTotal(0)
    setPaying(false); showRechargeSheet(false); setRechargeNotice('')
    setPendingRecharge(readPendingWalletRecharge(accountId))
    if (identityAllowed && deliveryPaymentEnabled) {
      void loadWallet()
      if (readPendingWalletRecharge(accountId)?.cancelRequested) void confirmRecharge(true, true)
    }
  }, [identityAllowed, deliveryPaymentEnabled, sessionKey])
  useEffect(() => {
    mounted.current = true
    return () => { mounted.current = false; loadEpoch.current += 1; rechargeEpoch.current += 1 }
  }, [])

  const confirmRecharge = async (verifyOnly = false, cancelOnly = false) => {
    if (!identityAllowed || !deliveryPaymentEnabled || !visible.current || !accountId) return
    if (cancelOnly ? cancellationLock.current : paying || rechargeLock.current) return
    const version = ++rechargeEpoch.current
    const current = () => mounted.current && rechargeEpoch.current === version && activeSessionKey.current === sessionKey
      && activePaymentContext.current === paymentContext
      && useSessionStore.getState().profile.id === accountId && useSessionStore.getState().token === sessionToken
    if (!current()) return
    rechargeLock.current = true
    cancellationLock.current = cancelOnly
    cancellationRequested.current = cancelOnly
    setPaying(true)
    setRechargeNotice('')
    try {
      const existing = readPendingWalletRecharge(accountId)
      if (verifyOnly && !existing) return
      if (!existing && !sheetOpen.current) return
      const amountCents = existing?.amountCents ?? rechargeAmountCents
      if (amountCents === null) {
        await Taro.showToast({ title: '请输入 0.01 至 100000 元的有效金额', icon: 'none' })
        return
      }
      const pending = existing || { accountId, amountCents, clientRequestId: createDeliveryClientRequestId(), createdAt: Date.now() }
      savePendingWalletRecharge(pending)
      setPendingRecharge(pending)
      const latest = await runWalletRecharge({ pending, verifyOnly, cancelOnly, isCurrent: current,
        shouldCancel: () => cancellationRequested.current,
        onCancelRequested: cancelledPending => {
          if (!current()) return
          cancellationRequested.current = true
          cancellationLock.current = true
          showRechargeSheet(false)
          setPendingRecharge(cancelledPending)
          setRechargeNotice('已退出付款，正在确认充值订单的取消结果。')
        }, requestPayment: async parameters => {
        nativePaymentOpen.current = true
        try { return await Taro.requestPayment(parameters) } finally { nativePaymentOpen.current = false }
      } })
      if (!current()) return
      if (['paid', 'failed', 'closed'].includes(latest.status)) {
        clearPendingWalletRecharge(accountId); setPendingRecharge(null); showRechargeSheet(false)
        setRechargeNotice(latest.status === 'paid' ? '充值已确认到账，余额与流水正在更新。' : latest.status === 'closed' ? '原充值订单已关闭，可以重新选择金额充值。' : '原充值订单支付失败，可以重新选择金额充值。')
        if (latest.status === 'paid') await loadWallet()
      } else {
        setPendingRecharge(readPendingWalletRecharge(accountId))
        showRechargeSheet(false)
        setRechargeNotice(cancellationRequested.current || pending.cancelRequested || latest.cancelRequestedAt
          ? '已退出付款，取消结果待核实。可查询状态，余额以后端确认为准。'
          : '充值结果待核实。请查询状态，或取消这笔充值。')
      }
    } catch (error) {
      if (!current()) return
      setPendingRecharge(readPendingWalletRecharge(accountId))
      const message = error instanceof Error ? error.message : String((error as { errMsg?: string })?.errMsg || '充值结果暂未确认')
      showRechargeSheet(false)
      setRechargeNotice(cancellationRequested.current || readPendingWalletRecharge(accountId)?.cancelRequested
        ? '已退出付款，取消结果待核实。网络恢复后可查询状态。'
        : `${message}。充值结果待核实，可查询状态或取消充值。`)
    } finally {
      if (current()) { setPaying(false); rechargeLock.current = false; cancellationLock.current = false }
    }
  }

  const closeRechargeSheet = () => {
    if (!mounted.current || activeSessionKey.current !== sessionKey
      || useSessionStore.getState().profile.id !== accountId || useSessionStore.getState().token !== sessionToken) return
    showRechargeSheet(false)
    const pending = readPendingWalletRecharge(accountId)
    if (!pending) return
    cancellationRequested.current = true
    const cancelledPending = { ...pending, cancelRequested: true }
    savePendingWalletRecharge(cancelledPending)
    setPendingRecharge(cancelledPending)
    setRechargeNotice('已退出付款，正在确认充值订单的取消结果。')
    // Supersede any in-flight creation/query immediately. The backend serializes
    // this cancellation with that attempt using the same client request ID.
    void confirmRecharge(true, true)
  }

  if (!identityAllowed) return <View className='merchant-wallet-page' />
  if (paymentChecking || paymentError || !deliveryPaymentEnabled) {
    return (
      <View className='merchant-wallet-page'>
        <View className='merchant-wallet-hero merchant-wallet-hero--disabled'>
          <CapsuleSystemRow />
          <View className='merchant-wallet-nav'>
            <AppIconButton name='back' label='返回' className='merchant-wallet-nav__back' onClick={() => void Taro.navigateBack()} />
            <Text className='merchant-wallet-nav__title'>商家钱包</Text>
            <View className='merchant-wallet-nav__spacer' />
          </View>
          <View className='merchant-wallet-balance'>
            <Text className='merchant-wallet-balance__label'>钱包余额（元）</Text>
            <Text className='merchant-wallet-balance__amount merchant-wallet-balance__amount--disabled'>{paymentChecking ? '检查中' : paymentError ? '暂不可用' : '未开通'}</Text>
            <Text className='merchant-wallet-balance__hint'>{paymentChecking ? '正在检查支付能力…' : paymentError ? paymentError : '平台尚未配置微信商户支付'}</Text>
          </View>
        </View>
        <MerchantEarningsStatus checking={paymentChecking} error={paymentError} enabled={merchantEarningsEnabled} reason={merchantEarningsReason} />
        <View className='merchant-wallet-unavailable'>
          <Text className='merchant-wallet-unavailable__title'>{paymentChecking ? '正在检查商家钱包' : paymentError ? '商家钱包暂时不可用' : '商家钱包暂未开通'}</Text>
          <Text className='merchant-wallet-unavailable__description'>
            {paymentChecking ? '正在确认支付能力，请稍后重试。' : paymentError ? '支付状态暂时无法确认，网络恢复后可重试。' : '当前不会发起充值、支付或资金账本请求。完成微信支付商户配置后，本页面会自动启用真实钱包功能。'}
          </Text>
          <View className='merchant-wallet-unavailable__back' onClick={() => paymentError ? void refreshPayment() : void Taro.navigateBack()}>
            <Text>{paymentError ? '重试检查' : '返回商家工作台'}</Text>
          </View>
        </View>
      </View>
    )
  }

  return (
    <View className='merchant-wallet-page'>
      <View className='merchant-wallet-hero'>
        <View className='merchant-wallet-hero__orb merchant-wallet-hero__orb--left' />
        <View className='merchant-wallet-hero__orb merchant-wallet-hero__orb--right' />
        <CapsuleSystemRow />
        <View className='merchant-wallet-nav'>
          <AppIconButton name='back' label='返回' className='merchant-wallet-nav__back' onClick={() => void Taro.navigateBack()} />
          <Text className='merchant-wallet-nav__title'>商家钱包</Text>
          <View className='merchant-wallet-nav__spacer' />
        </View>
        <View className='merchant-wallet-balance'>
          <Text className='merchant-wallet-balance__label'>钱包余额（元）</Text>
          <View className='merchant-wallet-balance__amount-row'>
            {wallet ? <Text className='merchant-wallet-balance__currency'>¥</Text> : null}
            <Text className='merchant-wallet-balance__amount'>{formatBalance(wallet?.availableBalanceCents)}</Text>
          </View>
          <Text className='merchant-wallet-balance__hint'>用于手动发单的配送费支付</Text>
          <AppButton className='merchant-wallet-balance__button' disabled={paying} onClick={() => pendingRecharge ? void confirmRecharge(true) : showRechargeSheet(true)}>{pendingRecharge ? '查询充值状态' : '充值'}</AppButton>
        </View>
      </View>

      <ScrollView scrollY className='merchant-wallet-scroll' enhanced showScrollbar={false}>
        <MerchantEarningsStatus checking={paymentChecking} error={paymentError} enabled={merchantEarningsEnabled} reason={merchantEarningsReason} />
        {pendingRecharge || rechargeNotice ? <View className='merchant-wallet-recovery'>
          <Text className='merchant-wallet-recovery__title'>{pendingRecharge ? `充值待核实 ¥${formatBalance(pendingRecharge.amountCents)}` : '充值状态'}</Text>
          <Text className='merchant-wallet-recovery__description'>{rechargeNotice || (pendingRecharge?.cancelRequested ? '已退出付款，取消结果待核实，可查询状态。' : '上次充值结果待核实，可查询状态或取消充值。')}</Text>
          {pendingRecharge ? <View className='merchant-wallet-recovery__actions'>
            <AppButton size='small' variant='secondary' loading={paying} onClick={() => void confirmRecharge(true)}>查询状态</AppButton>
            {!pendingRecharge.cancelRequested ? <AppButton size='small' variant='text' onClick={closeRechargeSheet}>取消充值</AppButton> : null}
          </View> : null}
        </View> : null}
        <View className='merchant-wallet-ledger-card'>
          <View className='merchant-wallet-ledger-card__header'>
            <Text className='merchant-wallet-ledger-card__title'>账单流水</Text>
            <AppButton className='merchant-wallet-ledger-card__range' size='small' variant='text' disabled={loading || moreLoading} onClick={() => void loadWallet()}>刷新</AppButton>
          </View>
          {loading ? <Text className='merchant-wallet-ledger-card__end'>正在加载真实账本…</Text> : null}
          {!loading && loadError ? (
            <Text className='merchant-wallet-ledger-card__end' onClick={() => void loadWallet()}>
              {loadError}，点击重试
            </Text>
          ) : null}
          {!loading && !loadError && ledger.length === 0 ? (
            <Text className='merchant-wallet-ledger-card__end'>暂无账单流水</Text>
          ) : null}
          {ledger.map((item) => {
            const positive = item.direction === 'credit'
            return (
              <View key={item.ledgerPublicId} className='merchant-wallet-ledger-item'>
                <View className={`merchant-wallet-ledger-item__icon merchant-wallet-ledger-item__icon--${item.entryType}`}>
                  <Text>{entrySymbol(item.entryType)}</Text>
                </View>
                <View className='merchant-wallet-ledger-item__main'>
                  <Text className='merchant-wallet-ledger-item__title'>{entryTitle(item.entryType)}</Text>
                  <Text className='merchant-wallet-ledger-item__time'>{formatTime(item.createdAt)}</Text>
                </View>
                <Text className={`merchant-wallet-ledger-item__amount ${positive ? 'merchant-wallet-ledger-item__amount--positive' : ''}`}>
                  {positive ? '+' : '-'}{(item.amountCents / 100).toFixed(2)}
                </Text>
              </View>
            )
          })}
          {ledger.length < ledgerTotal ? <AppButton block variant='text' loading={moreLoading} disabled={loading} onClick={() => void loadWallet(true)}>加载更多账单</AppButton> : null}
        </View>
      </ScrollView>

      <AppActionSheet visible={sheetVisible} title={pendingRecharge ? '确认充值结果' : '选择充值金额'} onClose={closeRechargeSheet}>
        <View className='merchant-wallet-sheet__content'>
            {pendingRecharge ? <Text className='merchant-wallet-recovery__title'>原充值金额 ¥{formatBalance(pendingRecharge.amountCents)}</Text> : <>
              <View className='merchant-wallet-amount-grid'>
                {rechargeOptions.map((amount) => (
                  <View
                    key={amount}
                    className={`merchant-wallet-amount ${!customAmount && selectedAmount === amount ? 'merchant-wallet-amount--active' : ''}`}
                    onClick={() => {
                      if (paying) return
                      setSelectedAmount(amount)
                      setCustomAmount('')
                    }}
                  >
                    <Text>¥{amount}</Text>
                  </View>
                ))}
              </View>
              <View className='merchant-wallet-custom-row'>
                <Text className='merchant-wallet-custom-row__label'>其他金额</Text>
                <View className='merchant-wallet-custom-field'>
                  <Text className='merchant-wallet-custom-field__prefix'>¥</Text>
                  <Input
                    className='merchant-wallet-custom-field__input'
                    type='digit'
                    disabled={paying}
                    value={customAmount}
                    placeholder='输入金额'
                    maxlength={9}
                    onInput={(event) => {
                      if (paying) return
                      setCustomAmount(String(event.detail.value ?? ''))
                    }}
                  />
                </View>
              </View>
              <Text className='merchant-wallet-custom-hint'>支持 0.01 至 100000 元</Text>
            </>}
          <AppNotice title='支付说明'>充值会创建真实微信支付订单，支付结果和余额以后端为准。</AppNotice>
          {rechargeNotice ? <Text className='merchant-wallet-recovery__description'>{rechargeNotice}</Text> : null}
          <AppButton block loading={paying} onClick={() => void confirmRecharge(Boolean(pendingRecharge))}>{pendingRecharge ? '查询状态' : rechargeAmountCents === null ? '微信支付' : `微信支付 ¥${customAmount !== '' ? formatBalance(rechargeAmountCents) : selectedAmount}`}</AppButton>
        </View>
      </AppActionSheet>
    </View>
  )
}

export default function MerchantWalletPage() {
  return (
    <RuntimeSessionGate preserveAuthenticatedChildrenDuringRetry>
      <MerchantWalletContent />
    </RuntimeSessionGate>
  )
}
