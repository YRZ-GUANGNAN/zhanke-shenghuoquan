import { useState } from 'react'
import { Image, Text, View } from '@tarojs/components'
import { API_BASE } from '../services/api'
import { resolveApiResourceUrl } from '../services/resource-url'
import './chat-avatar.scss'

export default function ChatAvatar({ src = '', name, className = '' }: { src?: string; name: string; className?: string }) {
  const imageUrl = resolveApiResourceUrl(src, API_BASE)
  const [failedSrc, setFailedSrc] = useState('')
  return (
    <View className={`chat-avatar ${className}`}>
      {imageUrl && failedSrc !== imageUrl ? (
        <Image className='chat-avatar__image' src={imageUrl} mode='aspectFill' aria-label={`${name}的头像`} onError={() => setFailedSrc(imageUrl)} />
      ) : (
        <Text className='chat-avatar__initial'>{Array.from(name.trim())[0] || '聊'}</Text>
      )}
    </View>
  )
}
