import React from 'react'
import { Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import { AppIconButton } from '../../components/ui'
import { useFeatureVisibility } from '../../hooks/use-feature-visibility'
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
  const visibility = useFeatureVisibility()
  return (
    <View className='about-page'>
      <CapsulePageHeader title='关于' left={<AppIconButton name='back' label='返回' className='about-nav__back' onClick={() => void Taro.navigateBack()} />} />
      <View className='about-content'>
        <View className='about-brand'><View className='about-brand__mark'><Text>湛</Text></View><Text className='about-brand__name'>湛科生活圈</Text><Text className='about-brand__version'>{versionLabel()}</Text></View>
        <Text className='about-description'>{visibility.presentationMode === 'campus_info' ? '当前提供校园指南阅读、个人资料与私密意见反馈。指南附公开来源和核验日期，本应用独立运营，非学校官方应用，具体安排以学校和馆方最新通知为准。' : '面向湛科校园的帖子、二手交易、配送与生活服务入口。让校内信息更清楚，让每一次连接更可靠。具体可用功能以当前开放的入口为准。'}</Text>
        <View className='about-card'>
          <View className='about-row' onClick={() => void Taro.navigateTo({ url: '/pages/legal/index?tab=agreement' })}><Text>用户协议</Text><AppIconButton name='chevron-right' label='查看用户协议' size='small' /></View>
          <View className='about-row' onClick={() => void Taro.navigateTo({ url: '/pages/legal/index?tab=privacy' })}><Text>隐私政策</Text><AppIconButton name='chevron-right' label='查看隐私政策' size='small' /></View>
          <View className='about-row' onClick={() => void Taro.navigateTo({ url: '/pages/feedback/index' })}><Text>意见反馈</Text><AppIconButton name='chevron-right' label='进入意见反馈' size='small' /></View>
        </View>
        <Text className='about-footer'>© 2026 湛科生活圈</Text>
      </View>
    </View>
  )
}
