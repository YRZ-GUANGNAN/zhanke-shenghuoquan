import React, { useState } from 'react'
import { Button, Image, Input, Text, Textarea, View } from '@tarojs/components'
import Taro from '../../services/taro'
import { createCommunityPost } from '../../services/b-community'
import { createProduct } from '../../services/c-trade'
import { cancelMedia, chooseAndUploadImages, type UploadedMedia } from '../../services/media'
import './index.scss'

export default function PublishPage() {
  const router = Taro.getCurrentInstance().router
  const isProduct = router?.params.type === 'product'
  const [title, setTitle] = useState('')
  const [content, setContent] = useState('')
  const [price, setPrice] = useState('')
	const [images, setImages] = useState<UploadedMedia[]>([])
	const [uploading, setUploading] = useState(false)

	const chooseImages = async () => {
		const remaining = 9 - images.length
		if (remaining <= 0) return
		setUploading(true)
		Taro.showToast({ title: '压缩、上传并审核中', icon: 'loading', duration: 2000 })
		try {
			const uploaded = await chooseAndUploadImages(isProduct ? 'product_image' : 'post_image', remaining)
			setImages((current) => [...current, ...uploaded].slice(0, 9))
			Taro.showToast({ title: '图片审核通过', icon: 'success' })
		} catch (error) {
			Taro.showToast({ title: error instanceof Error ? error.message : '图片上传失败', icon: 'none' })
		} finally {
			setUploading(false)
		}
	}

	const removeImage = async (value: UploadedMedia) => {
		try { await cancelMedia(value.assetId) } catch { /* the garbage collector remains the fallback */ }
		setImages((current) => current.filter((item) => item.assetId !== value.assetId))
	}

	const submit = async () => {
    if (!title.trim()) {
      Taro.showToast({ title: '请先填写标题', icon: 'none' })
      return
    }
	try {
	  if (uploading) throw new Error('请等待图片上传完成')
	  const imageAssetIds = images.map((item) => item.assetId)
	  if (isProduct) await createProduct({ title, description: content, price: Math.round(Number(price) * 100), imageAssetIds })
	  else await createCommunityPost({ title, content, imageAssetIds })
	  Taro.showToast({ title: '已提交，等待审核', icon: 'success' })
	  setTimeout(() => Taro.navigateBack(), 800)
	} catch (error) {
	  Taro.showToast({ title: error instanceof Error ? error.message : '提交失败', icon: 'none' })
	}
  }

  return (
    <View className='page publish-page'>
      <View className='publish-head'>
        <Text className='publish-head__title'>{isProduct ? '发布二手商品' : '发布校园帖子'}</Text>
        <Text className='publish-head__desc'>提交后会先经过微信内容安全与后台审核。</Text>
      </View>

      <View className='publish-form card'>
        <Text className='field-label'>标题</Text>
        <Input
          className='cc-field'
          value={title}
          placeholder={isProduct ? '例如：考研英语真题全套' : '例如：新生必看的宿舍入住经验'}
          onInput={(event) => setTitle(String(event.detail.value))}
        />

        {isProduct && (
          <>
            <Text className='field-label'>价格</Text>
            <Input
              className='cc-field'
              value={price}
              placeholder='请输入售价'
              type='digit'
              onInput={(event) => setPrice(String(event.detail.value))}
            />
          </>
        )}

        <Text className='field-label'>内容</Text>
        <Textarea
          className='cc-textarea'
          value={content}
          placeholder='描述清楚版本、新旧程度、交易地点或帖子正文'
          maxlength={600}
          onInput={(event) => setContent(String(event.detail.value))}
        />

		<View className='image-list'>
		  {images.map((item) => <View key={item.assetId} className='image-preview'>
			<Image src={item.previewPath} mode='aspectFill'/>
			<Text onClick={() => removeImage(item)}>×</Text>
		  </View>)}
		  {images.length < 9 && <View className='image-uploader' onClick={chooseImages}>
			<Text>+</Text>
			<Text>{uploading ? '处理中' : '上传图片'}</Text>
		  </View>}
		</View>
      </View>

      <Button className='cc-button cc-button--primary cc-button--block submit-button' onClick={submit}>提交审核</Button>
    </View>
  )
}
