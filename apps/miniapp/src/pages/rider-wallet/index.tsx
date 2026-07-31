import React from 'react'
import { Text, View } from '@tarojs/components'
import { CapsuleSystemRow } from '../../components/capsule-page-header'
import { AppIconButton, AppNotice } from '../../components/ui'
import Taro from '../../services/taro'
import './index.scss'

export default function RiderWalletPage() {
  return (
    <View className='rider-wallet-page'>
      <View className='rider-wallet-header'>
        <CapsuleSystemRow />
        <View className='rider-wallet-nav'>
          <AppIconButton
            name='back'
            label='返回'
            className='rider-wallet-nav__back'
            onClick={() => void Taro.navigateBack()}
          />
          <Text className='rider-wallet-nav__title'>骑手结算</Text>
          <View className='rider-wallet-nav__ghost' />
        </View>
      </View>

      <View className='rider-wallet-content'>
        <View className='rider-wallet-status-card'>
          <View className='rider-wallet-status-card__icon'>¥</View>
          <Text className='rider-wallet-status-card__title'>骑手结算功能尚未开放</Text>
          <Text className='rider-wallet-status-card__description'>
            当前不会展示余额、生成提现申请或模拟账单。平台接入真实骑手资金接口后，这里将自动显示可用余额与结算记录。
          </Text>
          <AppNotice tone='warning' title='资金说明'>配送收入仍以后端订单结算记录为准，本页面不生成任何临时金额。</AppNotice>
          <View className='rider-wallet-status-card__back' onClick={() => void Taro.navigateBack()}>
            <Text>返回骑手工作台</Text>
          </View>
        </View>
      </View>
    </View>
  )
}
