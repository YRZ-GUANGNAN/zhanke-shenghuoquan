import React from 'react'
import { Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import { AppIconButton } from '../../components/ui'
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
      <CapsulePageHeader title='关于' left={<AppIconButton name='back' label='返回' className='about-nav__back' onClick={() => void Taro.navigateBack()} />} />
      <View className='about-content'>
        <View className='about-brand'><View className='about-brand__mark'><Text>湛</Text></View><Text className='about-brand__name'>湛科生活圈</Text><Text className='about-brand__version'>{versionLabel()}</Text></View>
        <Text className='about-description'>为校园日常整理可查证的信息：图书馆使用、阅读资源与校区地址。每篇指南附公开来源与核验日期，可在小程序内完整阅读。当前版本提供校园指南、个人资料与私密反馈，不开放社区发帖、二手交易或新订单服务。本应用独立运营，非学校官方应用，具体安排以学校和馆方最新通知为准。</Text>
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
