import React, { useState } from 'react'
import { Text, View } from '@tarojs/components'
import { ImageTile, PageHeader, PageShell, SegmentedTabs } from '../../components'
import Taro from '../../services/taro'
import './index.scss'

type Tab = 'favorites' | 'posts' | 'comments' | 'products'

const tabs = [
  { key: 'favorites' as const, label: '收藏' },
  { key: 'posts' as const, label: '帖子' },
  { key: 'comments' as const, label: '评论' },
  { key: 'products' as const, label: '宝贝' }
]

export default function MyContentPage() {
  const params = Taro.getCurrentInstance().router?.params || {}
  const [active, setActive] = useState<Tab>(normalize(params.tab))

  return (
    <PageShell plain refreshTitle='我的内容已刷新'>
      <PageHeader title='我的内容' />
      <SegmentedTabs items={tabs} active={active} onChange={setActive} />
      <View className='list-stack'>
        {[0, 1, 2].map((index) => (
          <View
            key={index}
            className='content-row-live'
            onClick={() => Taro.navigateTo({ url: active === 'products' ? '/pages/product-detail/index' : '/pages/post-detail/index' })}
          >
            <ImageTile tone={index} label={active === 'products' ? '宝贝' : '帖'} />
            <View>
              <Text>{active === 'products' ? '九成新自行车，毕业急出' : '三食堂新出的芋泥波波真的绝'}</Text>
              <Text>{active === 'comments' ? '评论于 40 分钟前' : '今天 · 湛科生活圈'}</Text>
            </View>
            <Text>›</Text>
          </View>
        ))}
      </View>
    </PageShell>
  )
}

function normalize(value?: string): Tab {
  if (value === 'posts' || value === 'comments' || value === 'products') return value
  return 'favorites'
}
