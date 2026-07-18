import React, { useState } from 'react'
import { Image, Text, Textarea, View } from '@tarojs/components'
import { submitFeedback, type FeedbackCategory } from '../../services/a-user'
import { chooseAndUploadImages, waitForMediaApproval, type UploadedMedia } from '../../services/media'
import Taro from '../../services/taro'
import './index.scss'

const categories: Array<{ key: FeedbackCategory; label: string }> = [
  { key: 'bug', label: '功能异常' },
  { key: 'suggestion', label: '产品建议' },
  { key: 'report', label: '举报问题' },
  { key: 'other', label: '其他' }
]

export default function FeedbackPage() {
  const params = Taro.getCurrentInstance().router?.params || {}
  const initialCategory: FeedbackCategory = params.category === 'report' ? 'report' : 'bug'
  const targetUserId = String(params.targetUserId || '').trim()
  const [category, setCategory] = useState<FeedbackCategory>(initialCategory)
  const [content, setContent] = useState(targetUserId ? `举报用户 ${targetUserId}：` : '')
  const [images, setImages] = useState<UploadedMedia[]>([])
  const [progress, setProgress] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const chooseImages = async () => {
    if (images.length >= 3) return
    try {
      const uploaded = await chooseAndUploadImages('feedback', 3 - images.length, (value) => {
        setProgress(`第 ${value.index + 1}/${value.total} 张 · ${value.message}`)
      })
      const approved: UploadedMedia[] = []
      for (const image of uploaded) {
        await waitForMediaApproval(image.assetId, (status) => setProgress(status.message))
        approved.push(image)
      }
      setImages((current) => [...current, ...approved].slice(0, 3))
      setProgress('')
    } catch (error) {
      setProgress('')
      void Taro.showToast({ title: error instanceof Error ? error.message : '图片上传失败', icon: 'none' })
    }
  }

  const submit = async () => {
    const normalized = content.trim()
    if (normalized.length < 5) {
      void Taro.showToast({ title: '请至少填写 5 个字', icon: 'none' })
      return
    }
    if (submitting) return
    setSubmitting(true)
    try {
      const result = await submitFeedback(category, normalized, images.map((item) => item.assetId))
      await Taro.showModal({ title: '反馈已提交', content: `反馈编号：${result.id}\n我们会尽快处理。`, showCancel: false, confirmText: '完成' })
      await Taro.navigateBack()
    } catch (error) {
      void Taro.showToast({ title: error instanceof Error ? error.message : '提交失败，内容已为你保留', icon: 'none' })
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <View className='feedback-page'>
      <View className='feedback-nav'>
        <View className='feedback-nav__back' onClick={() => void Taro.navigateBack()}><Text>‹</Text></View>
        <Text className='feedback-nav__title'>意见反馈</Text>
      </View>
      <View className='feedback-content'>
        <Text className='feedback-label'>问题类型</Text>
        <View className='feedback-categories'>
          {categories.map((item) => (
            <View key={item.key} className={`feedback-chip ${category === item.key ? 'feedback-chip--active' : ''}`} onClick={() => setCategory(item.key)}><Text>{item.label}</Text></View>
          ))}
        </View>

        <Text className='feedback-label'>问题描述</Text>
        <View className='feedback-editor'>
          <Textarea
            className='feedback-textarea'
            maxlength={1000}
            value={content}
            placeholder='请描述发生了什么、如何复现，以及你希望的结果…'
            onInput={(event) => setContent(event.detail.value)}
          />
          <Text className='feedback-count'>{content.length}/1000</Text>
        </View>

        <Text className='feedback-label'>截图（选填，最多 3 张）</Text>
        <View className='feedback-images'>
          {images.map((item, index) => (
            <View className='feedback-image' key={item.assetId}>
              <Image src={item.previewPath} mode='aspectFill' onClick={() => void Taro.previewImage({ urls: images.map((value) => value.previewPath), current: item.previewPath })} />
              <View className='feedback-image__remove' onClick={() => setImages((current) => current.filter((_, currentIndex) => currentIndex !== index))}><Text>×</Text></View>
            </View>
          ))}
          {images.length < 3 && <View className='feedback-add' onClick={() => void chooseImages()}><Text className='feedback-add__plus'>＋</Text><Text>添加截图</Text></View>}
        </View>
        {progress && <Text className='feedback-progress'>{progress}</Text>}

        <View className={`feedback-submit ${submitting ? 'feedback-submit--disabled' : ''}`} onClick={() => void submit()}><Text>{submitting ? '正在提交…' : '提交反馈'}</Text></View>
      </View>
    </View>
  )
}
