import React from 'react'
import { Text, View } from '@tarojs/components'
import { AvatarMark, PageHeader, PageShell } from '../../components'
import Taro from '../../services/taro'
import './index.scss'

const rows = [
  ['配', '配送会话 #D2407018', '骑手：我到店了，取货照片已上传。', '/pages/delivery-conversation/index'],
  ['二', '二手宝贝咨询', '自行车今天还在吗？', '/pages/chat/index'],
  ['通', '系统通知', '你的二手宝贝已通过展示审核。', '/pages/notifications/index']
]

export default function ConversationsPage() {
  return (
    <PageShell plain refreshTitle='消息已刷新'>
      <PageHeader title='消息' />
      <View className='list-stack'>
        {rows.map((row) => (
          <View key={row[1]} className='message-row-live' onClick={() => Taro.navigateTo({ url: row[3] })}>
            <AvatarMark label={row[0]} />
            <View>
              <Text>{row[1]}</Text>
              <Text>{row[2]}</Text>
            </View>
            <Text>›</Text>
          </View>
        ))}
      </View>
    </PageShell>
  )
}
