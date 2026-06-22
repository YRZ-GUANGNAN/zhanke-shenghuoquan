import React, { useEffect, useState } from 'react'
import { Button, Text, View } from '@tarojs/components'
import Taro from '../../services/taro'
import {
  chooseAndUploadImages,
  getMediaHistory,
  retryUploadCompletion,
  type MediaPurpose,
  type MediaStatus
} from '../../services/media'
import './index.scss'

const purposeLabel = { post: '帖子图片', product: '商品图片', avatar: '用户头像' } as const

export default function MediaHistoryPage() {
  const [items, setItems] = useState<MediaStatus[]>([])
  const [loading, setLoading] = useState(false)

  const load = async () => {
    setLoading(true)
    try {
      const value = await getMediaHistory(1, 50)
      setItems(value.items)
    } catch (error) {
      Taro.showToast({ title: error instanceof Error ? error.message : '加载失败', icon: 'none' })
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void load() }, [])

  const continueCompletion = async (value: MediaStatus) => {
    try {
      await retryUploadCompletion(value.ticketId)
      Taro.showToast({ title: '已继续处理', icon: 'success' })
      await load()
    } catch (error) {
      Taro.showToast({ title: error instanceof Error ? error.message : '继续处理失败', icon: 'none' })
    }
  }

  const restart = async (value: MediaStatus) => {
    const purpose: MediaPurpose = value.purpose === 'post' ? 'post_image' : value.purpose === 'product' ? 'product_image' : 'avatar'
    try {
      await chooseAndUploadImages(purpose, 1)
      await load()
    } catch (error) {
      Taro.showToast({ title: error instanceof Error ? error.message : '重新上传失败', icon: 'none' })
    }
  }

  return (
    <View className='page media-history'>
      <View className='card media-history__head'>
        <Text className='media-history__title'>图片上传与审核记录</Text>
        <Text className='media-history__message'>所有状态均由媒体服务返回；审核通过前不会接入帖子、商品或正式头像。</Text>
        <Button className='cc-button' disabled={loading} onClick={load}>{loading ? '刷新中' : '刷新状态'}</Button>
      </View>

      {!loading && items.length === 0 && <View className='card media-history__item'><Text>暂无图片上传记录</Text></View>}
      {items.map((item) => (
        <View key={item.assetId} className='card media-history__item'>
          <Text className='media-history__title'>{purposeLabel[item.purpose]}</Text>
          <Text className='media-history__state'>{item.message}</Text>
          <Text className='media-history__meta'>资产号：{item.assetId}</Text>
          <Text className='media-history__meta'>技术状态：{item.technicalStatus} · 审核状态：{item.reviewStatus}</Text>
          <Text className='media-history__meta'>更新时间：{new Date(item.updatedAt).toLocaleString()}</Text>
          <View className='media-history__actions'>
            {item.recovery === 'complete_upload' && <Button size='mini' onClick={() => continueCompletion(item)}>继续确认上传</Button>}
            {item.recovery === 'new_upload' && <Button size='mini' onClick={() => restart(item)}>重新选择图片</Button>}
          </View>
        </View>
      ))}
    </View>
  )
}
