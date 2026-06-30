import React, { useEffect, useState } from 'react'
import { Button, Input, Text, View } from '@tarojs/components'
import { deliveryPaymentEnabled, deliveryPaymentMockEnabled } from '../../feature-flags'
import {
  getDeliveryWallet,
  listDeliveryWalletLedger,
  pollDeliveryPayment,
  startDeliveryWalletRecharge,
  type DeliveryPayment,
  type DeliveryWallet,
  type DeliveryWalletLedger
} from '../../services/delivery-payment'
import { centsToYuan } from '../../services/delivery-merchant'
import Taro from '../../services/taro'
import './index.scss'

const rechargePresets = [1000, 2000, 5000]
const ledgerPageSize = 30

export default function DeliveryWalletPage() {
  const [wallet, setWallet] = useState<DeliveryWallet | null>(null)
  const [ledger, setLedger] = useState<DeliveryWalletLedger[]>([])
  const [ledgerPage, setLedgerPage] = useState(1)
  const [ledgerTotal, setLedgerTotal] = useState(0)
  const [amountYuan, setAmountYuan] = useState('20')
  const [loading, setLoading] = useState(true)
  const [recharging, setRecharging] = useState(false)

  useEffect(() => {
    if (deliveryPaymentEnabled) void refreshWallet()
  }, [])

  const refreshWallet = async () => {
    setLoading(true)
    try {
      const [walletValue, ledgerPage] = await Promise.all([
        getDeliveryWallet(),
        listDeliveryWalletLedger(1, ledgerPageSize)
      ])
      setWallet(walletValue)
      setLedger(ledgerPage.items)
      setLedgerPage(ledgerPage.page)
      setLedgerTotal(ledgerPage.total)
    } catch (error) {
      showWalletError(error, '钱包加载失败')
    } finally {
      setLoading(false)
    }
  }

  const loadMoreLedger = async () => {
    if (loading || ledger.length >= ledgerTotal) return
    setLoading(true)
    try {
      const nextPage = ledgerPage + 1
      const page = await listDeliveryWalletLedger(nextPage, ledgerPageSize)
      setLedger((current) => [
        ...current,
        ...page.items.filter(
          (candidate) => !current.some((item) => item.ledgerPublicId === candidate.ledgerPublicId)
        )
      ])
      setLedgerPage(page.page)
      setLedgerTotal(page.total)
    } catch (error) {
      showWalletError(error, '流水加载失败')
    } finally {
      setLoading(false)
    }
  }

  const recharge = async () => {
    const amountCents = parseYuanToCents(amountYuan)
    if (amountCents === null) {
      await Taro.showToast({ title: '请输入 0.01 至 100000 元的有效金额', icon: 'none' })
      return
    }
    setRecharging(true)
    try {
      const result = await startDeliveryWalletRecharge(amountCents)
      const payment = requirePayment(result.payment)
      const latest = await finishPayment(payment)
      await refreshWallet()
      await Taro.showToast({
        title: latest.status === 'paid' ? '充值已到账' : '充值处理中，请稍后刷新',
        icon: latest.status === 'paid' ? 'success' : 'none'
      })
    } catch (error) {
      showWalletError(error, '充值失败')
      await refreshWallet()
    } finally {
      setRecharging(false)
    }
  }

  const finishPayment = async (payment: DeliveryPayment): Promise<DeliveryPayment> => {
    if (payment.provider === 'mock') {
      return payment
    }
    if (!payment.wechat) {
      throw new Error('后端未返回微信支付参数')
    }
    await Taro.requestPayment(payment.wechat)
    return pollDeliveryPayment(payment.paymentPublicId)
  }

  if (!deliveryPaymentEnabled) {
    return (
      <View className='page delivery-wallet-page'>
        <View className='card delivery-wallet-card'>
          <Text className='delivery-section-title'>配送钱包暂未开放</Text>
        </View>
      </View>
    )
  }

  return (
    <View className='page delivery-wallet-page'>
      <View className='delivery-hero'>
        <Text className='delivery-hero__title'>配送钱包</Text>
        <Text className='delivery-hero__desc'>余额和流水均以后端资金事实为准，页面不会自行修改到账状态。</Text>
        {deliveryPaymentMockEnabled && <Text className='delivery-wallet-mock'>本地 mock 支付提示已开启</Text>}
      </View>

      <View className='card delivery-wallet-card'>
        <Text className='delivery-hint'>可用余额</Text>
        <Text className='delivery-wallet-balance'>¥{centsToYuan(wallet?.availableBalanceCents ?? 0)}</Text>
        <Text className='delivery-hint'>{loading ? '正在读取后端余额…' : `更新于 ${formatTime(wallet?.updatedAt)}`}</Text>
        <View className='delivery-wallet-presets'>
          {rechargePresets.map((amount) => (
            <Button
              key={amount}
              className='cc-button cc-button--small'
              disabled={recharging}
              onClick={() => setAmountYuan(centsToYuan(amount))}
            >
              ¥{centsToYuan(amount)}
            </Button>
          ))}
        </View>
        <Input
          className='cc-field delivery-wallet-field'
          type='digit'
          value={amountYuan}
          placeholder='充值金额（元）'
          onInput={(event) => setAmountYuan(String(event.detail.value))}
        />
        <View className='delivery-wallet-actions'>
          <Button className='cc-button cc-button--primary' disabled={recharging} onClick={() => void recharge()}>
            {recharging ? '处理中…' : '微信充值'}
          </Button>
          <Button className='cc-button' disabled={loading || recharging} onClick={() => void refreshWallet()}>刷新</Button>
        </View>
      </View>

      <View className='card delivery-wallet-card'>
        <Text className='delivery-section-title'>资金流水</Text>
        {!loading && ledger.length === 0 && <Text className='delivery-hint'>暂无流水</Text>}
        {ledger.map((item) => (
          <View key={item.ledgerPublicId} className='delivery-wallet-entry'>
            <Text className={`delivery-wallet-entry__amount ${item.direction === 'credit' ? 'delivery-wallet-entry__amount--credit' : ''}`}>
              {item.direction === 'credit' ? '+' : '-'}¥{centsToYuan(item.amountCents)}
              {' · '}{ledgerTypeText[item.entryType]}
            </Text>
            <Text className='delivery-wallet-entry__meta'>
              余额 ¥{centsToYuan(item.balanceAfterCents)} · {formatTime(item.createdAt)}
            </Text>
          </View>
        ))}
        {ledger.length < ledgerTotal && (
          <Button className='cc-button cc-button--block' disabled={loading} onClick={() => void loadMoreLedger()}>
            {loading ? '加载中...' : '加载更多'}
          </Button>
        )}
      </View>
    </View>
  )
}

const ledgerTypeText: Record<DeliveryWalletLedger['entryType'], string> = {
  recharge: '充值',
  order_fee: '配送费',
  refund: '退款'
}

function requirePayment(value: DeliveryPayment | undefined): DeliveryPayment {
  if (!value) throw new Error('后端未返回支付记录')
  return value
}

function parseYuanToCents(value: string): number | null {
  const normalized = value.trim()
  if (!/^(?:0|[1-9]\d{0,5})(?:\.\d{1,2})?$/.test(normalized)) return null
  const [yuan, fraction = ''] = normalized.split('.')
  const cents = Number(yuan) * 100 + Number(fraction.padEnd(2, '0'))
  return cents > 0 && cents <= 10_000_000 ? cents : null
}

function formatTime(value?: string): string {
  return value ? value.replace('T', ' ').slice(0, 19) : '-'
}

function showWalletError(error: unknown, fallback: string) {
  const message = error instanceof Error ? error.message : fallback
  void Taro.showToast({ title: message || fallback, icon: 'none' })
}
