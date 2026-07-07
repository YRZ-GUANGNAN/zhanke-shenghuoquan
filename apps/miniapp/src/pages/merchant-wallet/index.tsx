import React, { useMemo, useState } from 'react'
import { ScrollView, Text, View } from '@tarojs/components'
import Taro from '../../services/taro'
import './index.scss'

type LedgerKind = 'fee' | 'refund' | 'recharge'

interface LedgerItem {
  id: string
  kind: LedgerKind
  title: string
  time: string
  amount: number
}

const rechargeOptions = [50, 100, 200, 300, 500, 1000]

const initialLedger: LedgerItem[] = [
  { id: 'fee-B1085', kind: 'fee', title: '支付配送费 · #B1085', time: '今天 12:47', amount: -1.5 },
  { id: 'fee-B1083', kind: 'fee', title: '支付配送费 · #B1083', time: '今天 12:44', amount: -1.5 },
  { id: 'fee-B1079', kind: 'fee', title: '支付配送费 · #B1079', time: '今天 12:31', amount: -1.5 },
  { id: 'refund-B1074', kind: 'refund', title: '取消退款 · #B1074', time: '今天 12:10', amount: 1.5 },
  { id: 'recharge-today', kind: 'recharge', title: '充值 · 微信支付', time: '今天 09:20', amount: 100 },
  { id: 'fee-B1071', kind: 'fee', title: '支付配送费 · #B1071', time: '昨天 11:52', amount: -1.5 },
  { id: 'fee-B1066', kind: 'fee', title: '支付配送费 · #B1066', time: '昨天 11:20', amount: -1.5 },
  { id: 'recharge-0628', kind: 'recharge', title: '充值 · 微信支付', time: '6月28日 15:44', amount: 50 }
]

function formatBalance(value: number) {
  return value.toFixed(2)
}

function formatSignedAmount(value: number) {
  return `${value > 0 ? '+' : '-'}${Math.abs(value).toFixed(2)}`
}

function kindSymbol(kind: LedgerKind) {
  if (kind === 'recharge') return '↓'
  if (kind === 'refund') return '↶'
  return '▱'
}

export default function MerchantWalletPage() {
  const [balance, setBalance] = useState(86.5)
  const [selectedAmount, setSelectedAmount] = useState(100)
  const [sheetVisible, setSheetVisible] = useState(false)
  const [localLedger, setLocalLedger] = useState<LedgerItem[]>([])
  const [toastText, setToastText] = useState('')

  const ledger = useMemo(() => [...localLedger, ...initialLedger], [localLedger])

  const closeToastLater = () => {
    setTimeout(() => setToastText(''), 1800)
  }

  const confirmRecharge = () => {
    setBalance((current) => current + selectedAmount)
    setLocalLedger((current) => [
      {
        id: `local-recharge-${Date.now()}`,
        kind: 'recharge',
        title: '充值 · 微信支付',
        time: '刚刚',
        amount: selectedAmount
      },
      ...current
    ])
    setSheetVisible(false)
    setToastText(`充值成功，已到账 ¥${selectedAmount.toFixed(2)}`)
    closeToastLater()
  }

  return (
    <View className='merchant-wallet-page'>
      <View className='merchant-wallet-hero'>
        <View className='merchant-wallet-hero__orb merchant-wallet-hero__orb--left' />
        <View className='merchant-wallet-hero__orb merchant-wallet-hero__orb--right' />
        <View className='merchant-wallet-nav'>
          <View className='merchant-wallet-nav__back' onClick={() => Taro.navigateBack()}>
            <Text>‹</Text>
          </View>
          <Text className='merchant-wallet-nav__title'>商家钱包</Text>
          <View className='merchant-wallet-nav__spacer' />
        </View>
        <View className='merchant-wallet-balance'>
          <Text className='merchant-wallet-balance__label'>钱包余额（元）</Text>
          <View className='merchant-wallet-balance__amount-row'>
            <Text className='merchant-wallet-balance__currency'>¥</Text>
            <Text className='merchant-wallet-balance__amount'>{formatBalance(balance)}</Text>
          </View>
          <Text className='merchant-wallet-balance__hint'>ⓘ 余额用于支付配送费，充值走微信支付</Text>
          <View className='merchant-wallet-balance__button' onClick={() => setSheetVisible(true)}>
            <Text>＋ 充值</Text>
          </View>
        </View>
      </View>

      <ScrollView scrollY className='merchant-wallet-scroll' enhanced showScrollbar={false}>
        <View className='merchant-wallet-ledger-card'>
          <View className='merchant-wallet-ledger-card__header'>
            <Text className='merchant-wallet-ledger-card__title'>账单流水</Text>
            <Text className='merchant-wallet-ledger-card__range'>仅展示最近 30 天</Text>
          </View>

          {ledger.map((item) => (
            <View key={item.id} className='merchant-wallet-ledger-item'>
              <View className={`merchant-wallet-ledger-item__icon merchant-wallet-ledger-item__icon--${item.kind}`}>
                <Text>{kindSymbol(item.kind)}</Text>
              </View>
              <View className='merchant-wallet-ledger-item__main'>
                <Text className='merchant-wallet-ledger-item__title'>{item.title}</Text>
                <Text className='merchant-wallet-ledger-item__time'>{item.time}</Text>
              </View>
              <Text
                className={`merchant-wallet-ledger-item__amount ${
                  item.amount > 0 ? 'merchant-wallet-ledger-item__amount--positive' : ''
                }`}
              >
                {formatSignedAmount(item.amount)}
              </Text>
            </View>
          ))}

          <Text className='merchant-wallet-ledger-card__end'>— 到底了 —</Text>
        </View>
      </ScrollView>

      {sheetVisible && (
        <View className='merchant-wallet-modal'>
          <View className='merchant-wallet-modal__mask' onClick={() => setSheetVisible(false)} />
          <View className='merchant-wallet-sheet'>
            <View className='merchant-wallet-sheet__header'>
              <Text className='merchant-wallet-sheet__title'>选择充值金额</Text>
              <View className='merchant-wallet-sheet__close' onClick={() => setSheetVisible(false)}>
                <Text>×</Text>
              </View>
            </View>
            <View className='merchant-wallet-amount-grid'>
              {rechargeOptions.map((amount) => (
                <View
                  key={amount}
                  className={`merchant-wallet-amount ${
                    selectedAmount === amount ? 'merchant-wallet-amount--active' : ''
                  }`}
                  onClick={() => setSelectedAmount(amount)}
                >
                  <Text>¥{amount}</Text>
                </View>
              ))}
            </View>
            <View className='merchant-wallet-info'>
              <Text>ⓘ 充值金额通过微信支付进入商家钱包，用于支付配送费，可开票请联系平台运营。</Text>
            </View>
            <View className='merchant-wallet-pay-button' onClick={confirmRecharge}>
              <Text>微信支付 ¥{selectedAmount}</Text>
            </View>
          </View>
        </View>
      )}

      {toastText && (
        <View className='merchant-wallet-toast'>
          <View className='merchant-wallet-toast__check'>
            <Text>✓</Text>
          </View>
          <Text className='merchant-wallet-toast__text'>{toastText}</Text>
        </View>
      )}
    </View>
  )
}
