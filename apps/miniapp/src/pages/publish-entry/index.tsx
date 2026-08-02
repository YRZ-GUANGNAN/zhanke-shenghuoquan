import React from 'react'
import { Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import AppIcon, { type AppIconName } from '../../components/app-icon'
import { AppIconButton } from '../../components/ui'
import Taro from '../../services/taro'
import './index.scss'

type PublishEntry = {
  key: 'post' | 'secondhand'
  icon: AppIconName
  title: string
  subtitle: string
  meta: string
  url: string
}

const entries: PublishEntry[] = [
  {
    key: 'post',
    icon: 'comment',
    title: '发帖子',
    subtitle: '分享动态、提问或求助',
    meta: '校园交流',
    url: '/pages/publish-post/index',
  },
  {
    key: 'secondhand',
    icon: 'market',
    title: '卖闲置',
    subtitle: '发布书籍、数码与生活用品',
    meta: '二手交易',
    url: '/pages/publish-secondhand/index',
  },
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
          <AppIconButton
            name='back'
            label='返回'
            className='publish-entry-topbar__back'
            onClick={() => void Taro.navigateBack()}
          />
        }
      />

      <View className='publish-entry-intro'>
        <Text className='publish-entry-intro__title'>想发布什么？</Text>
        <Text className='publish-entry-intro__desc'>选择一种内容类型开始发布</Text>
      </View>

      <View className='publish-entry-list'>
        {entries.map((entry) => (
          <View
            key={entry.key}
            className={`publish-entry-card publish-entry-card--${entry.key}`}
            hoverClass='publish-entry-card--pressed'
            hoverStayTime={80}
            role='button'
            aria-label={`${entry.title}：${entry.subtitle}`}
            onClick={() => openEntry(entry)}
          >
            <View className='publish-entry-card__top'>
              <View className='publish-entry-card__icon-wrap'>
                <AppIcon
                  name={entry.icon}
                  active={entry.key === 'secondhand'}
                  className='publish-entry-card__icon'
                />
              </View>
              <View className='publish-entry-card__arrow-wrap'>
                <AppIcon name='chevron-right' className='publish-entry-card__arrow' />
              </View>
            </View>
            <View className='publish-entry-card__main'>
              <Text className='publish-entry-card__title'>{entry.title}</Text>
              <Text className='publish-entry-card__subtitle'>{entry.subtitle}</Text>
            </View>
            <Text className='publish-entry-card__meta'>{entry.meta}</Text>
          </View>
        ))}
      </View>
    </View>
  )
}
