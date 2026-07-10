import React, { useState } from 'react'
import { ScrollView, Text, View } from '@tarojs/components'
import Taro from '../../services/taro'
import './index.scss'

const tags = ['代步工具', '可小刀']

function showToast(title: string) {
  void Taro.showToast({ title, icon: 'none', duration: 1400 })
}

export default function SecondhandDetailPage() {
  const [favorited, setFavorited] = useState(false)
  const [contactVisible, setContactVisible] = useState(false)

  const goBack = () => {
    void Taro.navigateBack().catch(() => Taro.redirectTo({ url: '/pages/student-secondhand/index' }))
  }

  const openChat = () => {
    setContactVisible(false)
    void Taro.navigateTo({ url: '/pages/secondhand-chat/index' })
  }

  return (
    <View className='secondhand-detail-page'>
      <View className='secondhand-detail-header'>
        <View className='secondhand-detail-header__back' onClick={goBack} />
        <Text className='secondhand-detail-header__title'>宝贝详情</Text>
      </View>

      <ScrollView className='secondhand-detail-scroll' scrollY enhanced showScrollbar={false}>
        <View className='secondhand-detail-hero' onClick={() => showToast('图片预览下一页接入')}>
          <Text>图 · 卖家上传（自行车实拍）</Text>
        </View>

        <View className='secondhand-detail-info'>
          <View className='secondhand-detail-info__price-row'>
            <Text className='secondhand-detail-info__price'>
              <Text className='secondhand-detail-info__currency'>¥</Text>150
            </Text>
            <View className='secondhand-detail-info__status'>
              <Text>出售中</Text>
            </View>
          </View>

          <Text className='secondhand-detail-info__title'>九成新自行车，毕业急出，车况好无锈</Text>
          <Text className='secondhand-detail-info__desc'>
            去年 10 月买的，平时只在校内骑，链条和刹车都刚保养过。带车锁和车筐，宿舍楼下自提，可以先来看车试骑。
          </Text>

          <View className='secondhand-detail-tags'>
            {tags.map((tag) => (
              <View className='secondhand-detail-tag' key={tag}>
                <Text>{tag}</Text>
              </View>
            ))}
          </View>

          <Text className='secondhand-detail-info__meta'>昨天 20:14 发布 · 浏览 86</Text>
        </View>

        <View className='secondhand-detail-section'>
          <View className='secondhand-detail-seller'>
            <View className='secondhand-detail-seller__avatar'>
              <Text>向</Text>
            </View>
            <View className='secondhand-detail-seller__main'>
              <Text className='secondhand-detail-seller__name'>向北</Text>
              <Text className='secondhand-detail-seller__meta'>湛江科技学院 · 在售 2 件 · 已卖出 5 件</Text>
            </View>
            <View className='secondhand-detail-seller__button' onClick={() => void Taro.navigateTo({ url: '/pages/user-profile/index?userId=north&source=secondhand&tab=secondhand' })}>
              <Text>主页</Text>
            </View>
          </View>

          <View className='secondhand-detail-warning'>
            <Text className='secondhand-detail-warning__icon'>△</Text>
            <Text>平台仅展示信息、不参与交易。请与卖家私下当面交易，验货后再付款，谨防转账诈骗。</Text>
          </View>
        </View>
      </ScrollView>

      <View className='secondhand-detail-actionbar'>
        <View
          className={`secondhand-detail-favorite ${favorited ? 'secondhand-detail-favorite--active' : ''}`}
          onClick={() => setFavorited((value) => !value)}
        >
          <View className='secondhand-detail-favorite__star' />
          <Text>收藏</Text>
        </View>
        <View className='secondhand-detail-contact' onClick={() => setContactVisible(true)}>
          <View className='secondhand-detail-contact__icon' />
          <Text>联系卖家</Text>
        </View>
      </View>

      {contactVisible && (
        <View className='secondhand-contact-modal'>
          <View className='secondhand-contact-modal__mask' onClick={() => setContactVisible(false)} />
          <View className='secondhand-contact-sheet'>
            <View className='secondhand-contact-sheet__head'>
              <Text className='secondhand-contact-sheet__title'>联系卖家</Text>
              <View className='secondhand-contact-sheet__close' onClick={() => setContactVisible(false)}>
                <Text>×</Text>
              </View>
            </View>

            <View className='secondhand-contact-option' onClick={openChat}>
              <View className='secondhand-contact-option__icon secondhand-contact-option__icon--chat' />
              <View className='secondhand-contact-option__main'>
                <Text className='secondhand-contact-option__title'>发起私聊</Text>
                <Text className='secondhand-contact-option__desc'>在湛科生活圈内给卖家发消息，可发送图片</Text>
              </View>
              <Text className='secondhand-contact-option__arrow'>›</Text>
            </View>

            <View className='secondhand-contact-option' onClick={() => showToast('微信号 zk_xiangbei 已复制')}>
              <View className='secondhand-contact-option__icon secondhand-contact-option__icon--wechat' />
              <View className='secondhand-contact-option__main'>
                <Text className='secondhand-contact-option__title'>加微信联系</Text>
                <Text className='secondhand-contact-option__desc'>微信号 zk_xiangbei · 点击复制</Text>
              </View>
              <Text className='secondhand-contact-option__arrow'>›</Text>
            </View>

            <Text className='secondhand-contact-sheet__safe'>交易由买卖双方私下完成，平台不担保、不代收款</Text>
          </View>
        </View>
      )}
    </View>
  )
}
