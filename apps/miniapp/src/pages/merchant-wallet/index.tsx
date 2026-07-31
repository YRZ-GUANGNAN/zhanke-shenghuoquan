import React, { useCallback, useEffect, useState } from 'react'
import { ScrollView, Text, View } from '@tarojs/components'
import { CapsuleSystemRow } from '../../components/capsule-page-header'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import { AppActionSheet, AppButton, AppNotice } from '../../components/ui'
import { deliveryPaymentEnabled } from '../../feature-flags'
import { useIdentityGuard } from '../../hooks/use-identity-guard'
import {
  getDeliveryWallet,
  listDeliveryWalletLedger,
  pollDeliveryPayment,
  startDeliveryWalletRecharge,
  type DeliveryLedgerEntryType,
  type DeliveryWallet,
  type DeliveryWalletLedger,
} from '../../services/delivery-payment'
import Taro from '../../services/taro'
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
  const identityAllowed = useIdentityGuard('merchant')
  const [wallet, setWallet] = useState<DeliveryWallet | null>(null)
  const [ledger, setLedger] = useState<DeliveryWalletLedger[]>([])
  const [selectedAmount, setSelectedAmount] = useState(100)
  const [sheetVisible, setSheetVisible] = useState(false)
  const [loading, setLoading] = useState(false)
  const [paying, setPaying] = useState(false)
  const [loadError, setLoadError] = useState('')

  const loadWallet = useCallback(async () => {
    if (!identityAllowed || !deliveryPaymentEnabled || loading) return
    setLoading(true)
    setLoadError('')
    try {
      const [walletValue, ledgerValue] = await Promise.all([
        getDeliveryWallet(),
        listDeliveryWalletLedger(1, 30),
      ])
      setWallet(walletValue)
      setLedger(ledgerValue.items)
    } catch (error) {
      setWallet(null)
      setLedger([])
      setLoadError(error instanceof Error ? error.message : '钱包加载失败')
    } finally {
      setLoading(false)
    }
  }, [identityAllowed, loading])

  useEffect(() => {
    if (identityAllowed && deliveryPaymentEnabled) void loadWallet()
  }, [identityAllowed]) // eslint-disable-line react-hooks/exhaustive-deps

  const confirmRecharge = async () => {
    if (!deliveryPaymentEnabled || paying) return
    setPaying(true)
    try {
      const result = await startDeliveryWalletRecharge(selectedAmount * 100)
      if (!result.payment) throw new Error('后端未返回充值支付记录')
      if (result.payment.provider === 'wechat') {
        if (!result.payment.wechat) throw new Error('后端未返回微信支付参数')
        await Taro.requestPayment(result.payment.wechat)
      }
      const latest = await pollDeliveryPayment(result.payment.paymentPublicId)
      if (latest.status !== 'paid') throw new Error('充值支付尚未完成')
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
      setPaying(false)
    }
  }

  if (!identityAllowed) return <View className='merchant-wallet-page' />
  if (!deliveryPaymentEnabled) {
    return (
      <View className='merchant-wallet-page'>
        <View className='merchant-wallet-hero merchant-wallet-hero--disabled'>
          <CapsuleSystemRow />
          <View className='merchant-wallet-nav'>
            <View className='merchant-wallet-nav__back' onClick={() => void Taro.navigateBack()}><Text>‹</Text></View>
            <Text className='merchant-wallet-nav__title'>商家钱包</Text>
            <View className='merchant-wallet-nav__spacer' />
          </View>
          <View className='merchant-wallet-balance'>
            <Text className='merchant-wallet-balance__label'>钱包余额（元）</Text>
            <Text className='merchant-wallet-balance__amount merchant-wallet-balance__amount--disabled'>未开通</Text>
            <Text className='merchant-wallet-balance__hint'>平台尚未配置微信商户支付</Text>
          </View>
        </View>
        <View className='merchant-wallet-unavailable'>
          <Text className='merchant-wallet-unavailable__title'>商家钱包暂未开通</Text>
          <Text className='merchant-wallet-unavailable__description'>
            当前不会发起充值、支付或资金账本请求。完成微信支付商户配置后，本页面会自动启用真实钱包功能。
          </Text>
          <View className='merchant-wallet-unavailable__back' onClick={() => void Taro.navigateBack()}>
            <Text>返回商家工作台</Text>
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
          <View className='merchant-wallet-nav__back' onClick={() => void Taro.navigateBack()}><Text>‹</Text></View>
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
