import React, { useState } from 'react'
import { Image, Input, ScrollView, Text, View } from '@tarojs/components'
import Taro from '../../services/taro'
import './index.scss'

function showToast(title: string) {
  void Taro.showToast({ title, icon: 'none', duration: 1400 })
}

type ChatCopy = {
  peerName: string
  peerAvatar: string
  peerState: string
  productTitle: string
  productPrice: string
  productImageLabel: string
  safeTip: string
  peerMessage1: string
  myMessage: string
  peerMessage2: string
  imageLabel: string
  peerMessage3: string
}

const chatCopies: Record<string, ChatCopy> = {
  bike: {
    peerName: '向北',
    peerAvatar: '向',
    peerState: '在线 · 湛江科技学院',
    productTitle: '九成新自行车，毕业急出',
    productPrice: '¥150',
    productImageLabel: '图 · 自行车',
    safeTip: '平台不担保交易，请当面验货后再付款',
    peerMessage1: '同学你好，看到你收藏了自行车，需要的话可以约看车',
    myMessage: '在的在的，车子还在吗？可以小刀不',
    peerMessage2: '在的。最多让 10 块，带锁带车筐，链条刚保养过',
    imageLabel: '图片 · 车筐细节',
    peerMessage3: '今晚七点宿舍楼下可以来试骑'
  },
  sea: {
    peerName: '小鹿乱撞',
    peerAvatar: '鹿',
    peerState: '在线 · 湛江科技学院',
    productTitle: '有没有周末去金沙湾的搭子？',
    productPrice: '搭子',
    productImageLabel: '图 · 海边',
    safeTip: '仅本次沟通，请勿透露与约见无关的隐私',
    peerMessage1: '明天去海边的话我可以带相机～',
    myMessage: '可以呀，大概几点出发？我想拍日落',
    peerMessage2: '下午四点后人少，不用排队，拍完还能去吃饭',
    imageLabel: '图片 · 海边机位',
    peerMessage3: '我把集合点发你，记得带充电宝'
  },
  library: {
    peerName: '卷心菜',
    peerAvatar: '卷',
    peerState: '在线 · 湛江科技学院',
    productTitle: '期末周图书馆占座攻略',
    productPrice: '资料',
    productImageLabel: '图 · 图书馆',
    safeTip: '仅本次沟通，请勿透露与学习无关的隐私',
    peerMessage1: '图书馆三楼靠窗还有位置',
    myMessage: '收到！靠近哪边呀，我现在过去',
    peerMessage2: '从东门上来右转，第三排比较安静',
    imageLabel: '图片 · 座位位置',
    peerMessage3: '我先帮你占半小时，到了说一声'
  }
}

function resolveChatCopy(): ChatCopy {
  const params = Taro.getCurrentInstance().router?.params || {}
  const scene = params.scene || 'bike'
  const base = chatCopies[scene] || chatCopies.bike
  return {
    ...base,
    peerName: params.peerName || base.peerName,
    peerAvatar: params.peerAvatar || base.peerAvatar
  }
}

export default function SecondhandChatPage() {
  const [draft, setDraft] = useState('')
  const [pickedImage, setPickedImage] = useState('')
  const copy = resolveChatCopy()

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
            <Text>{copy.peerAvatar}</Text>
          </View>
          <View className='secondhand-chat-seller__meta'>
            <Text className='secondhand-chat-seller__name'>{copy.peerName}</Text>
            <Text className='secondhand-chat-seller__state'>{copy.peerState}</Text>
          </View>
        </View>
        <View className='secondhand-chat-header__home' onClick={() => showToast('卖家主页下一页接入')}>
          <Text>主页</Text>
        </View>
      </View>

      <View className='secondhand-chat-product'>
        <View className='secondhand-chat-product__image'>
          <Text>{copy.productImageLabel}</Text>
        </View>
        <View className='secondhand-chat-product__main'>
          <Text className='secondhand-chat-product__title'>{copy.productTitle}</Text>
          <Text className='secondhand-chat-product__price'>{copy.productPrice}</Text>
        </View>
        <View className='secondhand-chat-product__button' onClick={() => Taro.navigateTo({ url: '/pages/secondhand-detail/index' })}>
          <Text>查看</Text>
        </View>
      </View>

      <ScrollView className='secondhand-chat-scroll' scrollY enhanced showScrollbar={false}>
        <View className='secondhand-chat-safe-tip'>
          <Text>{copy.safeTip}</Text>
        </View>

        <View className='secondhand-chat-row secondhand-chat-row--seller'>
          <View className='secondhand-chat-avatar'>
            <Text>{copy.peerAvatar}</Text>
          </View>
          <View className='secondhand-chat-bubble secondhand-chat-bubble--seller'>
            <Text>{copy.peerMessage1}</Text>
          </View>
        </View>

        <View className='secondhand-chat-row secondhand-chat-row--me'>
          <View className='secondhand-chat-bubble secondhand-chat-bubble--me'>
            <Text>{copy.myMessage}</Text>
          </View>
          <View className='secondhand-chat-avatar secondhand-chat-avatar--me'>
            <Text>泽</Text>
          </View>
        </View>

        <View className='secondhand-chat-row secondhand-chat-row--seller'>
          <View className='secondhand-chat-avatar'>
            <Text>{copy.peerAvatar}</Text>
          </View>
          <View className='secondhand-chat-bubble secondhand-chat-bubble--seller'>
            <Text>{copy.peerMessage2}</Text>
          </View>
        </View>

        <View className='secondhand-chat-row secondhand-chat-row--me'>
          <View className='secondhand-chat-image-message'>
            {pickedImage ? <Image className='secondhand-chat-image-message__img' src={pickedImage} mode='aspectFill' /> : <Text>{copy.imageLabel}</Text>}
          </View>
          <View className='secondhand-chat-avatar secondhand-chat-avatar--me'>
            <Text>泽</Text>
          </View>
        </View>

        <View className='secondhand-chat-row secondhand-chat-row--seller'>
          <View className='secondhand-chat-avatar'>
            <Text>{copy.peerAvatar}</Text>
          </View>
          <View className='secondhand-chat-bubble secondhand-chat-bubble--seller'>
            <Text>{copy.peerMessage3}</Text>
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
