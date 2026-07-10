import React from 'react'
import { Text, View } from '@tarojs/components'
import Taro from '../../services/taro'
import './index.scss'

type PublishEntry = {
  key: 'post' | 'secondhand' | 'delivery'
  icon: string
  title: string
  subtitle: string
  tag: string
  tone: 'blue' | 'orange' | 'green'
  url: string
}

const entries: PublishEntry[] = [
  {
    key: 'post',
    icon: '帖',
    title: '发帖子',
    subtitle: '分享校园动态、求助、攻略和日常',
    tag: '学生',
    tone: 'blue',
    url: '/pages/publish-post/index'
  },
  {
    key: 'secondhand',
    icon: '二',
    title: '发布二手',
    subtitle: '转让闲置、书籍资料、数码和生活用品',
    tag: '二手',
    tone: 'orange',
    url: '/pages/publish-secondhand/index'
  },
  {
    key: 'delivery',
    icon: '配',
    title: '发布配送单',
    subtitle: '填写取货点和送达点，交给校园骑手配送',
    tag: '商家',
    tone: 'green',
    url: '/pages/merchant-publish/index'
  }
]

export default function PublishEntryPage() {
  const openEntry = (entry: PublishEntry) => {
    void Taro.navigateTo({ url: entry.url })
  }

  return (
    <View className='publish-entry-page'>
      <View className='publish-entry-topbar'>
        <View className='publish-entry-topbar__back' onClick={() => Taro.navigateBack()}>
          <Text>‹</Text>
        </View>
        <Text className='publish-entry-topbar__title'>发布</Text>
        <View className='publish-entry-topbar__ghost' />
      </View>

      <View className='publish-entry-hero'>
        <View className='publish-entry-hero__orb publish-entry-hero__orb--large' />
        <View className='publish-entry-hero__orb publish-entry-hero__orb--small' />
        <Text className='publish-entry-hero__eyebrow'>选择你要发布的内容</Text>
        <Text className='publish-entry-hero__title'>今天想让校园帮你做什么？</Text>
        <Text className='publish-entry-hero__desc'>帖子、二手和配送单统一从这里进入，后续页面会继续按新设计重做。</Text>
      </View>

      <View className='publish-entry-list'>
        {entries.map((entry) => (
          <View
            key={entry.key}
            className={`publish-entry-card publish-entry-card--${entry.tone}`}
            onClick={() => openEntry(entry)}
          >
            <View className='publish-entry-card__icon'>
              <Text>{entry.icon}</Text>
            </View>
            <View className='publish-entry-card__main'>
              <View className='publish-entry-card__title-row'>
                <Text className='publish-entry-card__title'>{entry.title}</Text>
                <Text className='publish-entry-card__tag'>{entry.tag}</Text>
              </View>
              <Text className='publish-entry-card__subtitle'>{entry.subtitle}</Text>
            </View>
            <Text className='publish-entry-card__arrow'>›</Text>
          </View>
        ))}
      </View>

      <View className='publish-entry-note'>
        <Text className='publish-entry-note__icon'>i</Text>
        <Text>发布后会进入对应表单；本页只负责选择发布类型，不处理接口和支付。</Text>
      </View>
    </View>
  )
}

