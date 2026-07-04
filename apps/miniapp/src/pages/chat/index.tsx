import React, { useState } from 'react'
import { Button, Input, Text, View } from '@tarojs/components'
import { AvatarMark, PageHeader, PageShell } from '../../components'
import './index.scss'

const rows = [
  ['left', '你好，自行车还在吗？'],
  ['right', '还在，今天晚上可以北门看车。'],
  ['left', '好，我下课后联系你。']
]

export default function ChatPage() {
  const [value, setValue] = useState('')
  const [messages, setMessages] = useState(rows)

  return (
    <PageShell plain refreshTitle='聊天已刷新'>
      <PageHeader title='二手咨询' />
      <View className='chat-list-live'>
        {messages.map((row, index) => (
          <View key={index} className={`chat-msg-live chat-msg-live--${row[0]}`}>
            {row[0] === 'left' && <AvatarMark label='向' size='small' />}
            <Text>{row[1]}</Text>
          </View>
        ))}
      </View>
      <View className='chat-input-live'>
        <Input value={value} onInput={(event) => setValue(String(event.detail.value || ''))} placeholder='输入消息' />
        <Button
          onClick={() => {
            if (!value.trim()) return
            setMessages([...messages, ['right', value.trim()]])
            setValue('')
          }}
        >
          发送
        </Button>
      </View>
    </PageShell>
  )
}
