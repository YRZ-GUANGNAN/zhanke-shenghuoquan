import React from 'react'
import { Text, View } from '@tarojs/components'
import { ImageTile, PageHeader, PageShell } from '../../components'
import './index.scss'

const items = [
  ['帖子图片', '已通过'],
  ['配送取货照', '订单相关人可见'],
  ['配送消息图片', '订单相关人可见']
]

export default function MediaHistoryPage() {
  return (
    <PageShell plain refreshTitle='图片记录已刷新'>
      <PageHeader title='图片记录' />
      <View className='list-stack'>
        {items.map((item, index) => (
          <View key={item[0]} className='media-row-live'>
            <ImageTile tone={index} label='图' />
            <View>
              <Text>{item[0]}</Text>
              <Text>内容安全检查 · {item[1]}</Text>
            </View>
          </View>
        ))}
      </View>
    </PageShell>
  )
}
