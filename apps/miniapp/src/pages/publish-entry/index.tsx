import React from 'react'
import { Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import Taro from '../../services/taro'
import './index.scss'

type PublishEntry = {
  key: 'post' | 'secondhand'
  icon: string
  title: string
  subtitle: string
  tag: string
  tone: 'blue' | 'orange'
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
  }
]

export default function PublishEntryPage() {
  const openEntry = (entry: PublishEntry) => {
    void Taro.navigateTo({ url: entry.url })
  }

  return (
    <View className='publish-entry-page'>
      <CapsulePageHeader
        title='发布'
        left={
          <View className='publish-entry-topbar__back' onClick={() => void Taro.navigateBack()}>
            <Text>‹</Text>
          </View>
        }
      />

      <View className='publish-entry-hero'>
        <View className='publish-entry-hero__orb publish-entry-hero__orb--large' />
        <View className='publish-entry-hero__orb publish-entry-hero__orb--small' />
        <Text className='publish-entry-hero__eyebrow'>选择你要发布的内容</Text>
        <Text className='publish-entry-hero__title'>今天想让校园帮你做什么？</Text>
        <Text className='publish-entry-hero__desc'>帖子与二手内容从这里进入，选择后填写对应信息。</Text>
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
        <Text>选择内容类型后进入对应发布表单。</Text>
      </View>
    </View>
  )
}

