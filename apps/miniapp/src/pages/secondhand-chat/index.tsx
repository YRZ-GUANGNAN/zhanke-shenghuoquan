import React, { useState } from 'react'
import { Image, Input, ScrollView, Text, View } from '@tarojs/components'
import Taro from '../../services/taro'
import './index.scss'

function showToast(title: string) {
  void Taro.showToast({ title, icon: 'none', duration: 1400 })
}

export default function SecondhandChatPage() {
  const [draft, setDraft] = useState('')
  const [pickedImage, setPickedImage] = useState('')

  const goBack = () => {
    void Taro.navigateBack().catch(() => Taro.redirectTo({ url: '/pages/secondhand-detail/index' }))
  }

  const pickImage = async () => {
    try {
      const result = await Taro.chooseMedia({
        count: 1,
        mediaType: ['image'],
        sourceType: ['album', 'camera'],
        sizeType: ['compressed']
      })
      const imagePath = result.tempFiles[0]?.tempFilePath
      if (imagePath) {
        setPickedImage(imagePath)
        showToast('图片已加入私信')
      }
    } catch {
      showToast('已取消选择图片')
    }
  }

  const sendText = () => {
    if (!draft.trim()) {
      showToast('先输入消息')
      return
    }
    setDraft('')
    showToast('静态演示：消息已发送')
  }

  return (
    <View className='secondhand-chat-page'>
      <View className='secondhand-chat-header'>
        <View className='secondhand-chat-header__back' onClick={goBack} />
        <View className='secondhand-chat-seller'>
          <View className='secondhand-chat-seller__avatar'>
            <Text>向</Text>
          </View>
          <View className='secondhand-chat-seller__meta'>
            <Text className='secondhand-chat-seller__name'>向北</Text>
            <Text className='secondhand-chat-seller__state'>在线 · 湛江科技学院</Text>
          </View>
        </View>
        <View className='secondhand-chat-header__home' onClick={() => showToast('卖家主页下一页接入')}>
          <Text>主页</Text>
        </View>
      </View>

      <View className='secondhand-chat-product'>
        <View className='secondhand-chat-product__image'>
          <Text>图</Text>
        </View>
        <View className='secondhand-chat-product__main'>
          <Text className='secondhand-chat-product__title'>九成新自行车，毕业急出</Text>
          <Text className='secondhand-chat-product__price'>¥150</Text>
        </View>
        <View className='secondhand-chat-product__button' onClick={() => Taro.navigateTo({ url: '/pages/secondhand-detail/index' })}>
          <Text>查看</Text>
        </View>
      </View>

      <ScrollView className='secondhand-chat-scroll' scrollY enhanced showScrollbar={false}>
        <View className='secondhand-chat-safe-tip'>
          <Text>平台不担保交易，请当面验货后再付款</Text>
        </View>

        <View className='secondhand-chat-row secondhand-chat-row--seller'>
          <View className='secondhand-chat-avatar'>
            <Text>向</Text>
          </View>
          <View className='secondhand-chat-bubble secondhand-chat-bubble--seller'>
            <Text>同学你好，看到你收藏了自行车，需要的话可以约看车</Text>
          </View>
        </View>

        <View className='secondhand-chat-row secondhand-chat-row--me'>
          <View className='secondhand-chat-bubble secondhand-chat-bubble--me'>
            <Text>在的在的，车子还在吗？可以小刀不</Text>
          </View>
          <View className='secondhand-chat-avatar secondhand-chat-avatar--me'>
            <Text>泽</Text>
          </View>
        </View>

        <View className='secondhand-chat-row secondhand-chat-row--seller'>
          <View className='secondhand-chat-avatar'>
            <Text>向</Text>
          </View>
          <View className='secondhand-chat-bubble secondhand-chat-bubble--seller'>
            <Text>在的。最多让 10 块，带锁带车筐，链条刚保养过</Text>
          </View>
        </View>

        <View className='secondhand-chat-row secondhand-chat-row--me'>
          <View className='secondhand-chat-image-message'>
            {pickedImage ? <Image className='secondhand-chat-image-message__img' src={pickedImage} mode='aspectFill' /> : <Text>图片 · 车筐细节</Text>}
          </View>
          <View className='secondhand-chat-avatar secondhand-chat-avatar--me'>
            <Text>泽</Text>
          </View>
        </View>

        <View className='secondhand-chat-row secondhand-chat-row--seller'>
          <View className='secondhand-chat-avatar'>
            <Text>向</Text>
          </View>
          <View className='secondhand-chat-bubble secondhand-chat-bubble--seller'>
            <Text>今晚七点宿舍楼下可以来试骑</Text>
          </View>
        </View>
      </ScrollView>

      <View className='secondhand-chat-inputbar'>
        <View className='secondhand-chat-image-button' onClick={pickImage}>
          <View className='secondhand-chat-image-button__icon' />
        </View>
        <View className='secondhand-chat-input-wrap'>
          <Input
            className='secondhand-chat-input'
            value={draft}
            placeholder='发消息...'
            placeholderClass='secondhand-chat-input__placeholder'
            confirmType='send'
            onInput={(event) => setDraft(String(event.detail.value ?? ''))}
            onConfirm={sendText}
          />
        </View>
        <View className={`secondhand-chat-send ${draft.trim() ? 'secondhand-chat-send--active' : ''}`} onClick={sendText}>
          <Text>发送</Text>
        </View>
      </View>
    </View>
  )
}
