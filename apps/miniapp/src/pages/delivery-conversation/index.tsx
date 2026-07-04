import React, { useState } from 'react'
import { Button, Input, Text, View } from '@tarojs/components'
import { AvatarMark, ImageTile, PageHeader, PageShell } from '../../components'
import './index.scss'

const initialRows = [
  ['left', '你好，我已经接单，十分钟内到店取货。'],
  ['right', '好的，饮品已经做好，麻烦轻放。'],
  ['left', '我到店了，取货照片已上传。']
]

export default function DeliveryConversationPage() {
  const [value, setValue] = useState('')
  const [rows, setRows] = useState(initialRows)

  return (
    <PageShell plain refreshTitle='配送会话已刷新'>
      <PageHeader title='配送详情' />
      <View className='zk-card delivery-summary-live'>
        <Text>配送中 · 配送费 ¥1.50</Text>
        <Text>#A2048 · 茶知味大学城店 → 3栋 4楼</Text>
      </View>
      <View className='zk-card route-timeline-live'>
        <Text>取货点　茶知味·大学城店</Text>
        <Text>送达点　3栋 4楼 412室</Text>
        <Text>备注　冰淇淋打包好别洒了，放门口就行</Text>
      </View>
      <View className='proof-grid-live'>
        <ImageTile tone={0} label='取货照' />
        <ImageTile tone={2} label='送达照' />
      </View>
      <View className='delivery-chat-list-live'>
        {rows.map((row, index) => (
          <View key={index} className={`chat-msg-live chat-msg-live--${row[0]}`}>
            {row[0] === 'left' && <AvatarMark label='李' size='small' />}
            <Text>{row[1]}</Text>
          </View>
        ))}
      </View>
      <View className='chat-input-live'>
        <Input value={value} onInput={(event) => setValue(String(event.detail.value || ''))} placeholder='输入消息' />
        <Button
          onClick={() => {
            if (!value.trim()) return
            setRows([...rows, ['right', value.trim()]])
            setValue('')
          }}
        >
          发送
        </Button>
      </View>
    </PageShell>
  )
}
