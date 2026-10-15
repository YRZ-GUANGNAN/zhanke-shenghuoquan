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

const purposeLabel: Record<MediaStatus['purpose'], string> = { post: '帖子图片', product: '商品图片', avatar: '用户头像', profile_cover: '个人封面', identity_application: '身份申请材料', dish: '菜品图片', merchant_logo: '商家标志', delivery_proof: '配送凭证', delivery_im: '配送沟通图片', feedback: '反馈截图' }

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
    const purpose: MediaPurpose = value.purpose === 'post' ? 'post_image' : value.purpose === 'product' ? 'product_image' : value.purpose
    try {
      if (value.purpose === 'profile_cover' || value.purpose === 'avatar') {
        await Taro.navigateTo({ url: '/pages/edit-profile/index' })
        return
      }
      if (value.purpose === 'identity_application') {
        await Taro.navigateTo({ url: '/pages/profile/index' })
        return
      }
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
        <Text className='media-history__message'>帖子和商品图片自动审核；头像、个人封面上传完成后可保存。身份申请材料仅供管理员人工核验。</Text>
        <Button className='cc-button' disabled={loading} onClick={load}>{loading ? '刷新中' : '刷新状态'}</Button>
      </View>

      {!loading && items.length === 0 && <View className='card media-history__item'><Text>暂无图片上传记录</Text></View>}
      {items.map((item) => (
        <View key={item.assetId} className='card media-history__item'>
          <Text className='media-history__title'>{purposeLabel[item.purpose] || '图片'}</Text>
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
