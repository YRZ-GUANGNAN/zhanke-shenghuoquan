import React from 'react'
import { Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import Taro from '../../services/taro'
import './index.scss'

function versionLabel(): string {
  try {
    const value = Taro.getAccountInfoSync().miniProgram
    return value.version || (value.envVersion === 'release' ? '正式版' : value.envVersion || '体验版')
  } catch {
    return '体验版'
  }
}

export default function AboutPage() {
  return (
    <View className='about-page'>
      <CapsulePageHeader title='关于' left={<View className='about-nav__back' onClick={() => void Taro.navigateBack()}><Text>‹</Text></View>} />
      <View className='about-content'>
        <View className='about-brand'><View className='about-brand__mark'><Text>湛</Text></View><Text className='about-brand__name'>湛科生活圈</Text><Text className='about-brand__version'>{versionLabel()}</Text></View>
        <Text className='about-description'>面向湛科校园的帖子、二手交易、配送与生活服务入口。让校内信息更清楚，让每一次连接更可靠。</Text>
        <View className='about-card'>
          <View className='about-row' onClick={() => void Taro.navigateTo({ url: '/pages/legal/index?tab=agreement' })}><Text>用户协议</Text><Text>›</Text></View>
          <View className='about-row' onClick={() => void Taro.navigateTo({ url: '/pages/legal/index?tab=privacy' })}><Text>隐私政策</Text><Text>›</Text></View>
          <View className='about-row' onClick={() => void Taro.navigateTo({ url: '/pages/feedback/index' })}><Text>意见反馈</Text><Text>›</Text></View>
        </View>
        <Text className='about-footer'>© 2026 湛科生活圈</Text>
      </View>
    </View>
  )
}
