import React, { useMemo, useState } from 'react'
import { Image, Input, ScrollView, Text, Textarea, View } from '@tarojs/components'
import Taro from '../../services/taro'
import './index.scss'

type SecondhandCategory = '数码' | '书籍资料' | '生活用品' | '代步' | '其他'

const categories: SecondhandCategory[] = ['数码', '书籍资料', '生活用品', '代步', '其他']

function inputValue(event: { detail: { value?: string } }) {
  return String(event.detail.value ?? '')
}

export default function PublishSecondhandPage() {
  const [images, setImages] = useState<string[]>([])
  const [title, setTitle] = useState('')
  const [price, setPrice] = useState('')
  const [category, setCategory] = useState<SecondhandCategory | null>(null)
  const [description, setDescription] = useState('')

  const priceLabel = useMemo(() => {
    const normalized = price.trim()
    return normalized.length > 0 ? normalized : '0'
  }, [price])

  const chooseImages = async () => {
    if (images.length >= 9) {
      await Taro.showToast({ title: '最多添加 9 张图片', icon: 'none', duration: 1400 })
      return
    }

    try {
      const result = await Taro.chooseMedia({
        count: 9 - images.length,
        mediaType: ['image'],
        sourceType: ['album', 'camera'],
        sizeType: ['compressed']
      })
      setImages((current) => [
        ...current,
        ...result.tempFiles.map((file) => file.tempFilePath)
      ].slice(0, 9))
    } catch {
      // 用户取消选择时保持当前表单，不额外打扰。
    }
  }

  const removeCover = () => {
    setImages((current) => current.slice(1))
  }

  const handlePublish = () => {
    if (images.length === 0) {
      void Taro.showToast({ title: '请先添加宝贝实拍图', icon: 'none', duration: 1400 })
      return
    }
    if (!title.trim()) {
      void Taro.showToast({ title: '请填写宝贝标题', icon: 'none', duration: 1400 })
      return
    }
    if (!price.trim() || Number(price) <= 0) {
      void Taro.showToast({ title: '请填写有效价格', icon: 'none', duration: 1400 })
      return
    }
    if (!category) {
      void Taro.showToast({ title: '请选择宝贝分类', icon: 'none', duration: 1400 })
      return
    }
    void Taro.showToast({ title: '静态演示，暂未提交数据', icon: 'none', duration: 1600 })
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
                <Image className='publish-secondhand-upload__preview' src={images[0]} mode='aspectFill' />
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
            <Text>实拍图更容易卖出去</Text>
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
              onInput={(event) => setTitle(inputValue(event))}
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
                onInput={(event) => setPrice(inputValue(event).replace(/[^\d.]/g, ''))}
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
                  onClick={() => setCategory(item)}
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
              onInput={(event) => setDescription(inputValue(event))}
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
        <View className='publish-secondhand-footer__button' onClick={handlePublish}>
          <Text>发布</Text>
        </View>
      </View>
    </View>
  )
}
