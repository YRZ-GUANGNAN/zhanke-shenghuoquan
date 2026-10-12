import { withFeaturePage } from '../../components/feature-page-gate'
import React, { useState } from 'react'
import { Button, Image, Input, Text, Textarea, View } from '@tarojs/components'
import Taro from '../../services/taro'
import { createCommunityPost } from '../../services/b-community'
import { createProduct } from '../../services/c-trade'
import { chooseAndUploadImages, getMediaStatus, type UploadedMedia } from '../../services/media'
import './index.scss'

function PublishPage() {
  const router = Taro.getCurrentInstance().router
  const isProduct = router?.params.type === 'product'
  const [title, setTitle] = useState('')
  const [content, setContent] = useState('')
  const [price, setPrice] = useState('')
  const [images, setImages] = useState<UploadedMedia[]>([])
  const [uploading, setUploading] = useState(false)
  const [progress, setProgress] = useState('')

  const chooseImages = async () => {
    const remaining = 9 - images.length
    if (remaining <= 0 || uploading) return
    setUploading(true)
    try {
      const uploaded = await chooseAndUploadImages(
        isProduct ? 'product_image' : 'post_image',
        remaining,
        (value) => setProgress(`第 ${value.index + 1}/${value.total} 张 · ${value.percent}% · ${value.message}`)
      )
      setImages((current) => [...current, ...uploaded].slice(0, 9))
      Taro.showToast({ title: '图片已上传，等待审核', icon: 'success' })
    } catch (error) {
      Taro.showToast({ title: error instanceof Error ? error.message : '图片上传失败', icon: 'none' })
    } finally {
      setUploading(false)
      setProgress('')
    }
  }

  const refreshImages = async () => {
    try {
      const refreshed = await Promise.all(images.map(async (item) => ({
        ...item,
        ...(await getMediaStatus(item.assetId))
      })))
      setImages(refreshed)
      Taro.showToast({ title: '审核状态已刷新', icon: 'success' })
    } catch (error) {
      Taro.showToast({ title: error instanceof Error ? error.message : '刷新失败', icon: 'none' })
    }
  }

  const submit = async () => {
    if (!title.trim()) {
      Taro.showToast({ title: '请先填写标题', icon: 'none' })
      return
    }
    try {
      if (uploading) throw new Error('请等待图片上传完成')
      if (images.some((item) => !item.canUse)) throw new Error('图片审核通过后才能随内容提交')
      if (images.length > 0) throw new Error('图片已通过媒体审核；业务绑定将在整体联调阶段启用')
      if (isProduct) await createProduct({ title, description: content, price: Math.round(Number(price) * 100), imageAssetIds: [] })
      else await createCommunityPost({ title, content, imageAssetIds: [] })
      Taro.showToast({ title: '已提交，等待业务审核', icon: 'success' })
      setTimeout(() => Taro.navigateBack(), 800)
    } catch (error) {
      Taro.showToast({ title: error instanceof Error ? error.message : '提交失败', icon: 'none' })
    }
  }

  return (
    <View className='page publish-page'>
      <View className='publish-head'>
        <Text className='publish-head__title'>{isProduct ? '发布二手商品' : '发布校园帖子'}</Text>
        <Text className='publish-head__desc'>图片先完成安全上传与媒体审核，审核通过后才能随内容提交。</Text>
      </View>

      <View className='publish-form card'>
        <Text className='field-label'>标题</Text>
        <Input className='cc-field' value={title} placeholder='请输入标题' onInput={(event) => setTitle(String(event.detail.value))} />
        {isProduct && (
          <>
            <Text className='field-label'>价格</Text>
            <Input className='cc-field' value={price} placeholder='请输入售价' type='digit' onInput={(event) => setPrice(String(event.detail.value))} />
          </>
        )}
        <Text className='field-label'>内容</Text>
        <Textarea className='cc-textarea' value={content} placeholder='请输入内容' maxlength={600} onInput={(event) => setContent(String(event.detail.value))} />

        <View className='image-list'>
          {images.map((item) => (
            <View key={item.assetId} className='image-preview'>
              <Image src={item.previewPath} mode='aspectFill' />
              <Text onClick={() => setImages((current) => current.filter((image) => image.assetId !== item.assetId))}>×</Text>
              <Text>{item.canUse ? '已通过' : item.message}</Text>
            </View>
          ))}
          {images.length < 9 && (
            <View className='image-uploader' onClick={chooseImages}>
              <Text>+</Text><Text>{uploading ? '处理中' : '上传图片'}</Text>
            </View>
          )}
        </View>
        {progress && <Text>{progress}</Text>}
        {images.length > 0 && <Button className='cc-button' onClick={refreshImages}>刷新图片审核状态</Button>}
      </View>

      <Button className='cc-button cc-button--primary cc-button--block submit-button' onClick={submit}>提交审核</Button>
    </View>
  )
}

export default withFeaturePage(PublishPage)
