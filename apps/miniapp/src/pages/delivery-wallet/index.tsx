import React, { useState } from 'react'
import { Button, Text, View } from '@tarojs/components'
import { PageHeader, PageShell } from '../../components'
import Taro from '../../services/taro'
import './index.scss'

const ledger = [
  ['配送收入 · #A2044', '+1.50'],
  ['配送收入 · #A2039', '+1.50'],
  ['提现 · 平台审核转账', '-20.00'],
  ['配送收入 · #A1998', '+1.50'],
  ['配送收入 · #A1979', '+1.50']
]

export default function DeliveryWalletPage() {
  const params = Taro.getCurrentInstance().router?.params || {}
  const role = params.role === 'merchant' ? 'merchant' : 'rider'
  const [sheet, setSheet] = useState(params.action === 'recharge')

  return (
    <PageShell plain refreshTitle='钱包已刷新'>
      <PageHeader title='我的钱包' />
      <View className='wallet-hero-live'>
        <Text>钱包余额（元）</Text>
        <Text>{role === 'merchant' ? '86.50' : '128.50'}</Text>
        <Button className='secondary-btn' onClick={() => setSheet(true)}>{role === 'merchant' ? '充值' : '提现'}</Button>
      </View>
      <View className='zk-card'>
        <View className='section-head-live'>
          <Text>流水记录</Text>
          <Text>近 30 天</Text>
        </View>
        {ledger.map((item) => (
          <View key={item[0]} className='ledger-row-live'>
            <Text>{item[0]}</Text>
            <Text className={item[1].startsWith('+') ? 'ledger-row-live__in' : 'ledger-row-live__out'}>{item[1]}</Text>
          </View>
        ))}
      </View>
      {sheet && <WalletSheet role={role} onClose={() => setSheet(false)} />}
    </PageShell>
  )
}

function WalletSheet({ role, onClose }: { role: string; onClose: () => void }) {
  const isMerchant = role === 'merchant'
  return (
    <View className='sheet-live'>
      <View className='sheet-live__panel'>
        <View className='sheet-live__head'>
          <Text>{isMerchant ? '选择充值金额' : '申请提现'}</Text>
          <Text onClick={onClose}>关闭</Text>
        </View>
        {isMerchant ? (
          <View className='amount-grid-live'>
            {['¥50', '¥100', '¥200', '¥300', '¥500', '¥1000'].map((item) => (
              <Text key={item} className={item === '¥100' ? 'amount-grid-live__active' : ''}>{item}</Text>
            ))}
          </View>
        ) : (
          <View className='withdraw-card-live'>
            <Text>本次提现</Text>
            <Text>¥128.50</Text>
            <Text>提交后由平台审核处理，结果会通过消息提醒你。</Text>
          </View>
        )}
        <Button className='primary-btn sheet-live__button' onClick={onClose}>{isMerchant ? '确认支付 ¥100' : '确认提现'}</Button>
      </View>
    </View>
  )
}
