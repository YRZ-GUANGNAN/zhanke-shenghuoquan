import React from 'react'
import { Text, View } from '@tarojs/components'
import { PageHeader, PageShell } from '../../components'
import './index.scss'

const notices = [
  ['评论提醒', '卷心菜评论了你的帖子「晚霞真的绝」', '刚刚'],
  ['配送提醒', '骑手已上传取货照片', '18:48'],
  ['展示通过', '二手宝贝已通过展示审核，正在列表中展示', '昨天']
]

export default function NotificationsPage() {
  return (
    <PageShell plain refreshTitle='通知已刷新'>
      <PageHeader title='通知' />
      <View className='list-stack'>
        {notices.map((item) => (
          <View key={item[0]} className='notice-row-live'>
            <Text>{item[0].slice(0, 1)}</Text>
            <View>
              <Text>{item[0]}</Text>
              <Text>{item[1]}</Text>
            </View>
            <Text>{item[2]}</Text>
          </View>
        ))}
      </View>
    </PageShell>
  )
}
