import React, { useCallback, useEffect, useRef, useState } from 'react'
import { ScrollView, Text, View } from '@tarojs/components'
import { useDidHide, useDidShow } from '@tarojs/taro'
import { CapsuleSystemRow } from '../../components/capsule-page-header'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import { AppActionSheet, AppButton, AppIconButton, AppNotice } from '../../components/ui'
import { useDeliveryPayment } from '../../hooks/use-delivery-payment'
import { useIdentityGuard } from '../../hooks/use-identity-guard'
import {
  getDeliveryWallet,
  listDeliveryWalletLedger,
  pollDeliveryPayment,
  startDeliveryWalletRecharge,
  type DeliveryLedgerEntryType,
  type DeliveryPayment,
  type DeliveryWallet,
  type DeliveryWalletLedger,
} from '../../services/delivery-payment'
import Taro from '../../services/taro'
import { useSessionStore } from '../../stores/session'
import { clearPendingWalletRecharge, readPendingWalletRecharge, savePendingWalletRecharge } from './wallet-state'
import './index.scss'

const rechargeOptions = [50, 100, 200, 300, 500, 1000]

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

function MerchantWalletContent() {
  const { enabled: deliveryPaymentEnabled, checking: paymentChecking, error: paymentError, refresh: refreshPayment } = useDeliveryPayment()
  const identityAllowed = useIdentityGuard('merchant')
  const [wallet, setWallet] = useState<DeliveryWallet | null>(null)
  const [ledger, setLedger] = useState<DeliveryWalletLedger[]>([])
  const [selectedAmount, setSelectedAmount] = useState(100)
  const [sheetVisible, setSheetVisible] = useState(false)
  const [loading, setLoading] = useState(false)
  const [paying, setPaying] = useState(false)
  const [loadError, setLoadError] = useState('')
  const accountId = useSessionStore(state => state.profile.id)
  const sessionToken = useSessionStore(state => state.token)
  const sessionKey = `${accountId}:${sessionToken}`
  const rechargeLock = useRef(false)
  const activeSessionKey = useRef(sessionKey)
  const visible = useRef(true)
  const loadEpoch = useRef(0)
  const loadingRef = useRef(false)
  activeSessionKey.current = sessionKey

  const loadWallet = useCallback(async () => {
    if (!identityAllowed || !deliveryPaymentEnabled || loadingRef.current) return
    const version = ++loadEpoch.current
    const current = () => visible.current && loadEpoch.current === version && activeSessionKey.current === sessionKey
    loadingRef.current = true
    setLoading(true)
    setLoadError('')
    try {
      const walletValue = await getDeliveryWallet()
      if (!current()) return
      setWallet(walletValue)
      try { const ledgerValue = await listDeliveryWalletLedger(1, 30); if (current()) setLedger(ledgerValue.items) } catch (error) { if (current()) setLoadError(error instanceof Error ? error.message : '账单加载失败') }
    } catch (error) {
      if (current()) setLoadError(error instanceof Error ? error.message : '钱包加载失败')
    } finally {
      if (current()) setLoading(false)
      if (loadEpoch.current === version) loadingRef.current = false
    }
  }, [identityAllowed, deliveryPaymentEnabled, sessionKey])

  useDidHide(() => { visible.current = false; loadEpoch.current += 1; loadingRef.current = false })
  useDidShow(() => { visible.current = true; void loadWallet() })

  useEffect(() => {
    loadEpoch.current += 1
    loadingRef.current = false
    setWallet(null); setLedger([]); setLoadError('')
    if (identityAllowed && deliveryPaymentEnabled) void loadWallet()
  }, [identityAllowed, deliveryPaymentEnabled, sessionKey, loadWallet])

  const confirmRecharge = async () => {
    if (!deliveryPaymentEnabled || paying || rechargeLock.current || !accountId) return
    rechargeLock.current = true
    setPaying(true)
    try {
      const existing = readPendingWalletRecharge(accountId)
      if (existing && existing.amountCents !== selectedAmount * 100) throw new Error('已有充值订单待核实，请先恢复原订单')
      const pending = existing || { accountId, amountCents: selectedAmount * 100, clientRequestId: (await import('../../services/delivery-merchant')).createDeliveryClientRequestId(), createdAt: Date.now() }
      savePendingWalletRecharge(pending)
      let payment: DeliveryPayment | null = null
      let paymentPublicId = pending.paymentPublicId
      if (!pending.paymentPublicId) {
        const result = await startDeliveryWalletRecharge(pending.amountCents, pending.clientRequestId)
        if (!result.payment) throw new Error('后端未返回充值支付记录')
        payment = result.payment
        paymentPublicId = result.payment.paymentPublicId
        savePendingWalletRecharge({ ...pending, paymentPublicId: result.payment.paymentPublicId })
        if (result.payment.provider === 'wechat' && result.payment.status !== 'paid') {
          if (!result.payment.wechat) throw new Error('后端未返回微信支付参数')
          await Taro.requestPayment(result.payment.wechat)
        }
      }
      const latest = await pollDeliveryPayment(paymentPublicId!)
      if (latest.status === 'failed' || latest.status === 'closed') { clearPendingWalletRecharge(accountId); throw new Error(latest.status === 'failed' ? '充值支付失败' : '充值订单已关闭，请重新发起') }
      if (latest.status !== 'paid') throw new Error('充值支付尚未完成，可稍后继续核实原单')
      clearPendingWalletRecharge(accountId)
      setSheetVisible(false)
      await loadWallet()
      await Taro.showToast({ title: '充值已到账', icon: 'success' })
    } catch (error) {
      void Taro.showToast({
        title: error instanceof Error ? error.message : '充值失败',
        icon: 'none',
        duration: 2200,
      })
    } finally {
      setPaying(false); rechargeLock.current = false
    }
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
          <Text className='merchant-wallet-balance__hint'>余额与流水均来自服务端资金账本</Text>
          <View className='merchant-wallet-balance__button' onClick={() => setSheetVisible(true)}>
            <Text>＋ 充值</Text>
          </View>
        </View>
      </View>

      <ScrollView scrollY className='merchant-wallet-scroll' enhanced showScrollbar={false}>
        <View className='merchant-wallet-ledger-card'>
          <View className='merchant-wallet-ledger-card__header'>
            <Text className='merchant-wallet-ledger-card__title'>账单流水</Text>
            <Text className='merchant-wallet-ledger-card__range' onClick={() => void loadWallet()}>刷新</Text>
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
        </View>
      </ScrollView>

      <AppActionSheet visible={sheetVisible} title='选择充值金额' loading={paying} onClose={() => setSheetVisible(false)}>
        <View className='merchant-wallet-sheet__content'>
            <View className='merchant-wallet-amount-grid'>
              {rechargeOptions.map((amount) => (
                <View
                  key={amount}
                  className={`merchant-wallet-amount ${selectedAmount === amount ? 'merchant-wallet-amount--active' : ''}`}
                  onClick={() => !paying && setSelectedAmount(amount)}
                >
                  <Text>¥{amount}</Text>
                </View>
              ))}
            </View>
          <AppNotice title='支付说明'>充值会创建真实微信支付订单，支付结果和余额以后端为准。</AppNotice>
          <AppButton block loading={paying} onClick={() => void confirmRecharge()}>微信支付 ¥{selectedAmount}</AppButton>
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
