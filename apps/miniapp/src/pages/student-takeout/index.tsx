import React from 'react'
import { Text, View } from '@tarojs/components'
import StudentBottomNav from '../../components/student-bottom-nav'
import Taro from '../../services/taro'
import './index.scss'

type MerchantStatus = '营业中' | '休息中'

interface MerchantCard {
  id: string
  avatar: string
  name: string
  status: MerchantStatus
  description: string
  tone: 'tea' | 'wallace' | 'zhengxin' | 'yidiandian' | 'tastien' | 'mixue'
}

const merchants: MerchantCard[] = [
  {
    id: 'tea',
    avatar: '茶',
    name: '茶知味·大学城店',
    status: '营业中',
    description: '奶茶果茶 · 人气：芋泥波波',
    tone: 'tea'
  },
  {
    id: 'wallace',
    avatar: '华',
    name: '华莱士·西门店',
    status: '营业中',
    description: '炸鸡汉堡 · 人气：全鸡桶',
    tone: 'wallace'
  },
  {
    id: 'zhengxin',
    avatar: '正',
    name: '正新鸡排·二食堂',
    status: '营业中',
    description: '小吃炸串 · 人气：香辣鸡排',
    tone: 'zhengxin'
  },
  {
    id: 'yidiandian',
    avatar: '一',
    name: '一点点·北门店',
    status: '营业中',
    description: '奶茶 · 人气：波霸奶茶三分甜',
    tone: 'yidiandian'
  },
  {
    id: 'tastien',
    avatar: '塔',
    name: '塔斯汀·大学城店',
    status: '营业中',
    description: '中式汉堡 · 人气：香辣鸡腿堡',
    tone: 'tastien'
  },
  {
    id: 'mixue',
    avatar: '蜜',
    name: '蜜雪冰城·大学城店',
    status: '休息中',
    description: '冰淇淋与茶 · 人气：柠檬水',
    tone: 'mixue'
  }
]

function showMiniProgramToast() {
  void Taro.showToast({ title: '即将跳转商家小程序', icon: 'none', duration: 1400 })
}

export default function StudentTakeoutPage() {
  return (
    <View className='student-takeout-page'>
      <View className='student-takeout-header'>
        <Text className='student-takeout-header__title'>外卖</Text>
        <Text className='student-takeout-header__subtitle'>校内合作商家 · 点击跳转对应小程序下单</Text>
      </View>

      <View className='student-takeout-scroll'>
        <View className='student-takeout-tip'>
          <Text className='student-takeout-tip__icon'>i</Text>
          <Text className='student-takeout-tip__text'>
            在商家小程序下单后，商家会通过湛科生活圈发配送单，由校园骑手送到宿舍楼下。
          </Text>
        </View>

        <View className='student-takeout-list'>
          {merchants.map((merchant) => (
            <View
              key={merchant.id}
              className='student-takeout-merchant'
              onClick={showMiniProgramToast}
            >
              <View className={`student-takeout-merchant__avatar student-takeout-merchant__avatar--${merchant.tone}`}>
                <Text>{merchant.avatar}</Text>
              </View>
              <View className='student-takeout-merchant__main'>
                <View className='student-takeout-merchant__title-row'>
                  <Text className='student-takeout-merchant__name'>{merchant.name}</Text>
                  <View className={`student-takeout-status ${merchant.status === '营业中' ? 'student-takeout-status--open' : 'student-takeout-status--closed'}`}>
                    <View className='student-takeout-status__dot' />
                    <Text>{merchant.status}</Text>
                  </View>
                </View>
                <Text className='student-takeout-merchant__desc'>{merchant.description}</Text>
              </View>
              <View className='student-takeout-merchant__action'>
                <Text>小程序</Text>
              </View>
              <Text className='student-takeout-merchant__arrow'>›</Text>
            </View>
          ))}
        </View>

        <Text className='student-takeout-footer'>合作商家陆续入驻中</Text>
      </View>

      <StudentBottomNav active='takeout' />
    </View>
  )
}
