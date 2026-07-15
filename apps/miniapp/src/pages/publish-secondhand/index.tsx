import React, { useMemo, useRef, useState } from 'react'
import { Image, Input, ScrollView, Text, Textarea, View } from '@tarojs/components'
import { isApiError } from '../../services/api'
import { createProduct } from '../../services/c-trade'
import { createClientRequestId } from '../../services/client-request-id'
import { chooseAndUploadImages, waitForMediaApproval, type UploadedMedia } from '../../services/media'
import Taro from '../../services/taro'
import './index.scss'

type SecondhandCategory = '数码' | '书籍资料' | '生活用品' | '代步' | '其他'

const categories: SecondhandCategory[] = ['数码', '书籍资料', '生活用品', '代步', '其他']

function inputValue(event: { detail: { value?: string } }) {
  return String(event.detail.value ?? '')
}

export function priceTextToCents(value: string): number | null {
  const match = /^(0|[1-9]\d*)(?:\.(\d{1,2}))?$/.exec(value.trim())
  if (!match) return null
  const cents = Number(match[1]) * 100 + Number((match[2] ?? '').padEnd(2, '0'))
  return Number.isSafeInteger(cents) && cents > 0 ? cents : null
}

function publishErrorMessage(error: unknown): string {
  if (!isApiError(error)) return error instanceof Error ? error.message : '发布失败，请稍后重试'
  if (error.status === 0) return '网络连接失败，保持内容不变可安全重试'
  if (error.status === 400) return '发布信息不符合要求，请检查后重试'
  if (error.status === 401 || error.status === 403) return '登录状态已失效，请重新登录'
  if (error.status === 409) return '本次发布请求冲突，请检查内容后重新发布'
  if (error.status === 503 || error.status === 504) return '发布服务暂不可用，保持内容不变可安全重试'
  return error.message || '发布失败，请稍后重试'
}

export default function PublishSecondhandPage() {
  const [images, setImages] = useState<UploadedMedia[]>([])
  const [title, setTitle] = useState('')
  const [price, setPrice] = useState('')
  const [category, setCategory] = useState<SecondhandCategory | null>(null)
  const [description, setDescription] = useState('')
  const [uploading, setUploading] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [uploadStatus, setUploadStatus] = useState('实拍图更容易卖出去')
  const clientRequestIdRef = useRef<string | null>(null)

  const priceLabel = useMemo(() => {
    const normalized = price.trim()
    return normalized.length > 0 ? normalized : '0'
  }, [price])

  const invalidateRequest = () => {
    clientRequestIdRef.current = null
  }

  const chooseImages = async () => {
    if (uploading || submitting) return
    if (images.length >= 9) {
      await Taro.showToast({ title: '最多添加 9 张图片', icon: 'none', duration: 1400 })
      return
    }

    setUploading(true)
    setUploadStatus('正在安全上传图片')
    try {
      const uploaded = await chooseAndUploadImages('product_image', 9 - images.length, (progress) => {
        setUploadStatus(progress.message)
      })
      invalidateRequest()
      setImages((current) => [...current, ...uploaded].slice(0, 9))
      if (uploaded.length > 0) {
        await Taro.showToast({ title: '图片已上传', icon: 'success', duration: 1400 })
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : '图片上传未完成，请重试'
      await Taro.showToast({ title: message, icon: 'none', duration: 1800 })
    } finally {
      setUploading(false)
      setUploadStatus('实拍图更容易卖出去')
    }
  }

  const removeCover = () => {
    if (uploading || submitting) return
    invalidateRequest()
    setImages((current) => current.slice(1))
  }

  const handlePublish = async () => {
    if (uploading || submitting) return
    if (images.length === 0) {
      await Taro.showToast({ title: '请先添加宝贝实拍图', icon: 'none', duration: 1400 })
      return
    }
    if (!title.trim()) {
      await Taro.showToast({ title: '请填写宝贝标题', icon: 'none', duration: 1400 })
      return
    }
    const priceCents = priceTextToCents(price)
    if (priceCents === null) {
      await Taro.showToast({ title: '价格最多保留两位小数', icon: 'none', duration: 1400 })
      return
    }
    if (!category) {
      await Taro.showToast({ title: '请选择宝贝分类', icon: 'none', duration: 1400 })
      return
    }
    if (!description.trim()) {
      await Taro.showToast({ title: '请补充宝贝描述', icon: 'none', duration: 1400 })
      return
    }

    const clientRequestId = clientRequestIdRef.current ?? createClientRequestId()
    clientRequestIdRef.current = clientRequestId
    setSubmitting(true)
    setUploadStatus('正在确认图片审核状态')
    try {
      await Promise.all(images.map((image) => waitForMediaApproval(image.assetId)))
      await createProduct({
        title: title.trim(),
        description: description.trim(),
        price: priceCents,
        category,
        imageAssetIds: images.map((image) => image.assetId),
        clientRequestId
      })
      clientRequestIdRef.current = null
      await Taro.showToast({ title: '发布成功，等待审核', icon: 'success', duration: 1600 })
      await Taro.redirectTo({ url: '/pages/student-secondhand/index' })
    } catch (error) {
      if (isApiError(error) && error.status >= 400 && error.status < 500) {
        clientRequestIdRef.current = null
      }
      await Taro.showToast({ title: publishErrorMessage(error), icon: 'none', duration: 2200 })
    } finally {
      setSubmitting(false)
      setUploadStatus('实拍图更容易卖出去')
    }
  }

  return (
    <View className='publish-secondhand-page'>
      <View className='publish-secondhand-nav'>
        <View className='publish-secondhand-nav__back' onClick={() => void Taro.navigateBack()}>
          <Text>‹</Text>
        </View>
        <Text className='publish-secondhand-nav__title'>发布二手</Text>
        <View className='publish-secondhand-nav__spacer' />
      </View>

      <ScrollView scrollY className='publish-secondhand-scroll' enhanced showScrollbar={false}>
        <View className='publish-secondhand-upload-card'>
          <View className='publish-secondhand-upload' onClick={() => void chooseImages()}>
            {images.length > 0 ? (
              <>
                <Image className='publish-secondhand-upload__preview' src={images[0].previewPath} mode='aspectFill' />
                <View className='publish-secondhand-upload__count'>
                  <Text>{images.length}/9</Text>
                </View>
              </>
            ) : (
              <>
                <View className='publish-secondhand-upload__icon'>
                  <View className='publish-secondhand-upload__icon-sun' />
                </View>
                <Text className='publish-secondhand-upload__label'>添加图片</Text>
              </>
            )}
          </View>

          <View className='publish-secondhand-upload-copy'>
            <Text>最多 9 张，第一张为封面</Text>
            <Text>{uploadStatus}</Text>
            {images.length > 0 && (
              <Text className='publish-secondhand-upload-copy__remove' onClick={removeCover}>移除封面</Text>
            )}
          </View>
        </View>

        <View className='publish-secondhand-form-card'>
          <View className='publish-secondhand-field'>
            <Text className='publish-secondhand-field__label'>标题</Text>
            <Input
              className='publish-secondhand-field__input'
              value={title}
              maxlength={40}
              placeholder='品牌型号、新旧程度、入手渠道...'
              placeholderClass='publish-secondhand-field__placeholder'
              onInput={(event) => {
                invalidateRequest()
                setTitle(inputValue(event))
              }}
            />
          </View>

          <View className='publish-secondhand-field publish-secondhand-field--price'>
            <Text className='publish-secondhand-field__label'>价格</Text>
            <View className='publish-secondhand-price'>
              <Text className='publish-secondhand-price__currency'>¥</Text>
              <Input
                className='publish-secondhand-price__input'
                type='digit'
                value={price}
                maxlength={8}
                placeholder='0'
                placeholderClass='publish-secondhand-price__placeholder'
                onInput={(event) => {
                  invalidateRequest()
                  setPrice(inputValue(event).replace(/[^\d.]/g, ''))
                }}
              />
            </View>
            <Text className='publish-secondhand-field__aside'>仅展示，线下私下交易</Text>
          </View>

          <View className='publish-secondhand-section'>
            <Text className='publish-secondhand-section__label'>分类</Text>
            <View className='publish-secondhand-categories'>
              {categories.map((item) => (
                <View
                  key={item}
                  className={`publish-secondhand-category ${category === item ? 'publish-secondhand-category--active' : ''}`}
                  onClick={() => {
                    invalidateRequest()
                    setCategory(item)
                  }}
                >
                  <Text>{item}</Text>
                </View>
              ))}
            </View>
          </View>

          <View className='publish-secondhand-description'>
            <View className='publish-secondhand-description__header'>
              <Text className='publish-secondhand-section__label'>描述</Text>
              {description.length > 0 && <Text>{description.length}/300</Text>}
            </View>
            <Textarea
              className='publish-secondhand-description__textarea'
              value={description}
              maxlength={300}
              placeholder='说说宝贝的使用情况、交易方式（如宿舍楼下自提）...'
              placeholderClass='publish-secondhand-description__placeholder'
              onInput={(event) => {
                invalidateRequest()
                setDescription(inputValue(event))
              }}
            />
          </View>
        </View>

        <View className='publish-secondhand-warning'>
          <Text className='publish-secondhand-warning__icon'>△</Text>
          <Text>平台仅展示信息，不参与交易。价格仅供参考，交易请与买家私下当面完成。</Text>
        </View>

        <View className='publish-secondhand-scroll__spacer' />
      </ScrollView>

      <View className='publish-secondhand-footer'>
        <View
          className={`publish-secondhand-footer__button ${uploading || submitting ? 'publish-secondhand-footer__button--disabled' : ''}`}
          onClick={() => void handlePublish()}
        >
          <Text>{submitting ? '发布中' : uploading ? '图片上传中' : '发布'}</Text>
        </View>
      </View>
    </View>
  )
}
