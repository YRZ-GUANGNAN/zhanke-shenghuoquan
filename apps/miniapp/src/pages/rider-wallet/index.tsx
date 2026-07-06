import React, { useState } from 'react'
import { Text, View } from '@tarojs/components'
import Taro from '../../services/taro'
import './index.scss'

type LedgerDirection = 'income' | 'withdraw'

interface LedgerItem {
  id: string
  title: string
  time: string
  amount: string
  direction: LedgerDirection
  status?: string
}

const ledgerItems: LedgerItem[] = [
  { id: 'A2044', title: '配送收入 · #A2044', time: '今天 12:52', amount: '+1.50', direction: 'income' },
  { id: 'A2039', title: '配送收入 · #A2039', time: '今天 12:31', amount: '+1.50', direction: 'income' },
  { id: 'A2031', title: '配送收入 · #A2031', time: '今天 11:47', amount: '+1.50', direction: 'income' },
  {
    id: 'W2001',
    title: '提现 · 平台微信转账',
    time: '昨天 18:20',
    amount: '-20.00',
    direction: 'withdraw',
    status: '已到账'
  },
  { id: 'A1996', title: '配送收入 · #A1996', time: '昨天 17:02', amount: '+1.50', direction: 'income' },
  { id: 'A1988', title: '配送收入 · #A1988', time: '昨天 16:15', amount: '+1.50', direction: 'income' },
  { id: 'A1979', title: '配送收入 · #A1979', time: '6月30日 19:40', amount: '+1.50', direction: 'income' },
  {
    id: 'W1999',
    title: '提现 · 平台微信转账',
    time: '6月28日 20:05',
    amount: '-35.00',
    direction: 'withdraw',
    status: '已到账'
  }
]

function showStaticToast(title: string) {
  void Taro.showToast({ title, icon: 'none', duration: 1400 })
}

export default function RiderWalletPage() {
  const [withdrawVisible, setWithdrawVisible] = useState(false)

  const closeWithdraw = () => setWithdrawVisible(false)

  const submitWithdraw = () => {
    showStaticToast('提现申请已提交（静态示意）')
    closeWithdraw()
  }

  return (
    <View className='rider-wallet-page'>
      <View className='rider-wallet-hero'>
        <View className='rider-wallet-hero__orb rider-wallet-hero__orb--one' />
        <View className='rider-wallet-hero__orb rider-wallet-hero__orb--two' />

        <View className='rider-wallet-nav'>
          <View className='rider-wallet-nav__back' onClick={() => void Taro.navigateBack()}>
            <Text>‹</Text>
          </View>
          <Text className='rider-wallet-nav__title'>我的钱包</Text>
          <View className='rider-wallet-nav__ghost' />
        </View>

        <View className='rider-wallet-balance'>
          <Text className='rider-wallet-balance__label'>钱包余额（元）</Text>
          <View className='rider-wallet-balance__amount-row'>
            <Text className='rider-wallet-balance__currency'>¥</Text>
            <Text className='rider-wallet-balance__amount'>128.50</Text>
          </View>
          <Text className='rider-wallet-balance__hint'>ⓘ 提现由平台微信转账，1-2 天内到账</Text>
          <View className='rider-wallet-balance__withdraw' onClick={() => setWithdrawVisible(true)}>
            <Text className='rider-wallet-balance__withdraw-icon'>↑</Text>
            <Text>提现</Text>
          </View>
        </View>
      </View>

      <View className='rider-wallet-ledger'>
        <View className='rider-wallet-ledger__head'>
          <Text className='rider-wallet-ledger__title'>流水记录</Text>
          <Text className='rider-wallet-ledger__range'>仅展示最近 30 天</Text>
        </View>

        <View className='rider-wallet-ledger__list'>
          {ledgerItems.map((item) => (
            <View key={item.id} className='rider-wallet-ledger-row'>
              <View
                className={`rider-wallet-ledger-row__icon rider-wallet-ledger-row__icon--${item.direction}`}
              >
                <Text>{item.direction === 'income' ? '↓' : '↑'}</Text>
              </View>
              <View className='rider-wallet-ledger-row__main'>
                <Text className='rider-wallet-ledger-row__title'>{item.title}</Text>
                <Text className='rider-wallet-ledger-row__time'>{item.time}</Text>
              </View>
              <View className='rider-wallet-ledger-row__right'>
                <Text
                  className={`rider-wallet-ledger-row__amount rider-wallet-ledger-row__amount--${item.direction}`}
                >
                  {item.amount}
                </Text>
                {item.status && <Text className='rider-wallet-ledger-row__status'>{item.status}</Text>}
              </View>
            </View>
          ))}
        </View>
        <Text className='rider-wallet-ledger__end'>— 到底了 —</Text>
      </View>

      {withdrawVisible && (
        <View className='rider-wallet-mask'>
          <View className='rider-wallet-sheet'>
            <View className='rider-wallet-sheet__top'>
              <Text className='rider-wallet-sheet__title'>申请提现</Text>
              <View className='rider-wallet-sheet__close' onClick={closeWithdraw}>
                <Text>×</Text>
              </View>
            </View>

            <View className='rider-wallet-withdraw-card'>
              <Text className='rider-wallet-withdraw-card__label'>提现金额（全部余额）</Text>
              <View className='rider-wallet-withdraw-card__amount-row'>
                <Text className='rider-wallet-withdraw-card__currency'>¥</Text>
                <Text className='rider-wallet-withdraw-card__amount'>128.50</Text>
              </View>
            </View>

            <View className='rider-wallet-sheet__tip'>
              <Text>ⓘ 提交后由平台通过微信转账打款，一般 1-2 天内到账，请留意平台消息。</Text>
            </View>

            <View className='rider-wallet-sheet__submit' onClick={submitWithdraw}>
              <Text>申请提现 ¥128.50</Text>
            </View>
          </View>
        </View>
      )}
    </View>
  )
}

