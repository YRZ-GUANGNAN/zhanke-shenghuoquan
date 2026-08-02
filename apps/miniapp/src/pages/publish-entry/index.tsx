import React from 'react'
import { Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import { AppIconButton } from '../../components/ui'
import Taro from '../../services/taro'
import './index.scss'

type PublishEntry = {
  key: 'post' | 'secondhand'
  order: string
  title: string
  subtitle: string
  meta: string
  url: string
}

const entries: PublishEntry[] = [
  {
    key: 'post',
    order: '01',
    title: '发布帖子',
    subtitle: '分享校园动态、求助、攻略和日常',
    meta: '交流内容',
    url: '/pages/publish-post/index',
  },
  {
    key: 'secondhand',
    order: '02',
    title: '发布二手',
    subtitle: '转让闲置、书籍资料、数码和生活用品',
    meta: '交易内容',
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
        <Text className='publish-entry-intro__eyebrow'>选择发布类型</Text>
        <Text className='publish-entry-intro__title'>把内容发到正确的频道</Text>
        <Text className='publish-entry-intro__desc'>帖子用于交流，二手用于交易。</Text>
      </View>

      <View className='publish-entry-list'>
        {entries.map((entry) => (
          <View
            key={entry.key}
            className={`publish-entry-card publish-entry-card--${entry.key}`}
            hoverClass='publish-entry-card--pressed'
            hoverStayTime={80}
            onClick={() => openEntry(entry)}
          >
            <View className='publish-entry-card__index'>
              <Text>{entry.order}</Text>
            </View>
            <View className='publish-entry-card__main'>
              <Text className='publish-entry-card__meta'>{entry.meta}</Text>
              <Text className='publish-entry-card__title'>{entry.title}</Text>
              <Text className='publish-entry-card__subtitle'>{entry.subtitle}</Text>
            </View>
            <View className='publish-entry-card__action'>
              <Text>去发布</Text>
              <AppIconButton
                name='chevron-right'
                label={`进入${entry.title}`}
                size='small'
                className='publish-entry-card__arrow'
              />
            </View>
          </View>
        ))}
      </View>
    </View>
  )
}
