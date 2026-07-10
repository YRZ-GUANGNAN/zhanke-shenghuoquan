import React, { useState } from 'react'
import { ScrollView, Text, View } from '@tarojs/components'
import Taro from '../../services/taro'
import './index.scss'

type ProductCopy = {
  title: string
  price: number
  status: '出售中' | '已售'
  hero: string
  description: string
  tags: string[]
  meta: string
  sellerId?: string
  sellerName?: string
  sellerAvatar?: string
  sellerAccount?: string
  sellerMeta?: string
}

const productCopies: Record<string, ProductCopy> = {
  bike: {
    title: '九成新自行车，毕业急出，车况好无锈',
    price: 150,
    status: '出售中',
    hero: '图 · 卖家上传（自行车实拍）',
    description: '去年 10 月买的，平时只在校内骑，链条和刹车都刚保养过。带车锁和车筐，宿舍楼下自提，可以先来看车试骑。',
    tags: ['代步工具', '可小刀'],
    meta: '昨天 20:14 发布 · 浏览 86'
  },
  'desk-lamp': {
    title: '护眼台灯，用了半年，搬宿舍不想带了',
    price: 35,
    status: '出售中',
    hero: '图 · 卖家上传（台灯实拍）',
    description: '三档色温和亮度可调，灯臂转动正常，宿舍书桌使用很合适。配原装电源，可当面验货。',
    tags: ['生活用品', '宿舍好物'],
    meta: '今天 09:40 发布 · 浏览 24'
  },
  earphones: {
    title: '半入耳蓝牙耳机，盒装齐全，功能正常',
    price: 120,
    status: '出售中',
    hero: '图 · 卖家上传（耳机实拍）',
    description: '续航和左右耳连接都正常，附充电线和包装盒。因换新耳机闲置，支持当面试听。',
    tags: ['数码', '耳机'],
    meta: '今天 08:55 发布 · 浏览 31'
  },
  guitar: {
    title: '初学者民谣吉他，送变调夹和背袋',
    price: 180,
    status: '已售',
    hero: '图 · 卖家上传（吉他实拍）',
    description: '琴颈和弦距正常，适合入门练习，随琴送背袋、拨片和变调夹。该商品已经售出。',
    tags: ['乐器', '已售'],
    meta: '6月28日发布 · 浏览 112'
  },
  fridge: {
    title: '寝室小冰箱，制冷正常，可小刀',
    price: 99,
    status: '出售中',
    hero: '图 · 卖家上传（冰箱实拍）',
    description: '宿舍用小冰箱，冷藏制冷正常，外观有轻微使用痕迹。体积不大，需要买家自提。',
    tags: ['生活用品', '自提'],
    meta: '昨天 18:20 发布 · 浏览 64'
  },
  books: {
    title: '管理学教材 + 科学计算器打包',
    price: 30,
    status: '出售中',
    hero: '图 · 卖家上传（教材实拍）',
    description: '教材有少量笔记，计算器功能正常，适合低年级同学备课和考试使用。',
    tags: ['书籍资料', '打包出'],
    meta: '6月30日发布 · 浏览 47'
  },
  keyboard: {
    title: '机械键盘 87 键茶轴，手感好',
    price: 110,
    status: '出售中',
    hero: '图 · 卖家上传（键盘实拍）',
    description: '所有按键测试正常，茶轴声音适中，带原装数据线。可以在宿舍区当面试键。',
    tags: ['数码', '机械键盘'],
    meta: '今天 10:15 发布 · 浏览 38'
  },
  'lamp-small': {
    title: '床头小夜灯，暖光可调节',
    price: 18,
    status: '出售中',
    hero: '图 · 卖家上传（小夜灯实拍）',
    description: '触控调光，暖光柔和，夜间起床或看书都方便。充电线一并附送。',
    tags: ['生活用品', '小夜灯'],
    meta: '今天 11:30 发布 · 浏览 16'
  },
  'exam-pack': {
    title: '四级真题一套+听力网课，半价出',
    price: 25,
    status: '已售',
    hero: '图 · 卖家上传（四级资料）',
    description: '真题册有少量标记，听力资料完整，适合备考复习。该商品已经售出。',
    tags: ['书籍资料', '四级'],
    meta: '6月26日发布 · 浏览 73'
  },
  'aze-calculator': {
    title: '科学计算器，考试课可用',
    price: 28,
    status: '出售中',
    hero: '图 · 卖家上传（计算器实拍）',
    description: '按键和屏幕显示都正常，统计与科学计算功能可用。附保护壳，适合考试课和日常作业使用。',
    tags: ['书籍资料', '计算器'],
    meta: '今天 10:06 发布 · 浏览 18',
    sellerId: 'current-user',
    sellerName: '阿泽',
    sellerAvatar: '泽',
    sellerAccount: 'zk_aze233',
    sellerMeta: '湛江科技学院 · 在售 2 件 · 已卖出 1 件'
  },
  'aze-books': {
    title: '大二公共课教材和复习资料',
    price: 35,
    status: '出售中',
    hero: '图 · 卖家上传（教材实拍）',
    description: '教材有少量课堂笔记，配套复习资料齐全，适合低年级同学备课。可在教学楼附近当面看书。',
    tags: ['书籍资料', '复习资料'],
    meta: '昨天 17:35 发布 · 浏览 26',
    sellerId: 'current-user',
    sellerName: '阿泽',
    sellerAvatar: '泽',
    sellerAccount: 'zk_aze233',
    sellerMeta: '湛江科技学院 · 在售 2 件 · 已卖出 1 件'
  },
  'north-lamp': {
    title: '宿舍护眼台灯，亮度三档可调',
    price: 35,
    status: '出售中',
    hero: '图 · 卖家上传（护眼台灯实拍）',
    description: '三档亮度和色温可调，灯臂转动正常，配原装电源。毕业清物，可在宿舍楼下当面验货。',
    tags: ['生活用品', '宿舍好物'],
    meta: '昨天 15:20 发布 · 浏览 33'
  }
}

const missingProductCopy: ProductCopy = {
  title: '未找到对应的静态宝贝',
  price: 0,
  status: '已售',
  hero: '图 · 未找到对应宝贝',
  description: '该链接没有匹配到已登记的静态商品，请返回二手集市重新选择。',
  tags: [],
  meta: '静态内容待接入',
  sellerId: 'current-user',
  sellerName: '卖家待接入',
  sellerAvatar: '?',
  sellerAccount: '待接入',
  sellerMeta: '未找到对应卖家信息'
}

function showToast(title: string) {
  void Taro.showToast({ title, icon: 'none', duration: 1400 })
}

export default function SecondhandDetailPage() {
  const requestedId = Taro.getCurrentInstance().router?.params?.id || 'bike'
  const productFound = Boolean(productCopies[requestedId])
  const product = productCopies[requestedId] || missingProductCopy
  const seller = {
    id: product.sellerId || 'north',
    name: product.sellerName || '向北',
    avatar: product.sellerAvatar || '向',
    account: product.sellerAccount || 'zk_xiangbei',
    meta: product.sellerMeta || '湛江科技学院 · 在售 2 件 · 已卖出 5 件'
  }
  const [favorited, setFavorited] = useState(false)
  const [contactVisible, setContactVisible] = useState(false)

  const goBack = () => {
    void Taro.navigateBack().catch(() => Taro.redirectTo({ url: '/pages/student-secondhand/index' }))
  }

  const openChat = () => {
    setContactVisible(false)
    const query = [
      `scene=${requestedId === 'bike' ? 'bike' : 'product'}`,
      `productId=${encodeURIComponent(requestedId)}`,
      `productTitle=${encodeURIComponent(product.title)}`,
      `productPrice=${encodeURIComponent(`¥${product.price}`)}`,
      `productImageLabel=${encodeURIComponent(product.hero.replace('卖家上传', '商品'))}`,
      `sellerId=${encodeURIComponent(seller.id)}`,
      `peerName=${encodeURIComponent(seller.name)}`,
      `peerAvatar=${encodeURIComponent(seller.avatar)}`
    ].join('&')
    void Taro.navigateTo({ url: `/pages/secondhand-chat/index?${query}` })
  }

  return (
    <View className='secondhand-detail-page'>
      <View className='secondhand-detail-header'>
        <View className='secondhand-detail-header__back' onClick={goBack} />
        <Text className='secondhand-detail-header__title'>宝贝详情</Text>
      </View>

      <ScrollView className='secondhand-detail-scroll' scrollY enhanced showScrollbar={false}>
        <View className='secondhand-detail-hero' onClick={() => showToast('图片预览下一页接入')}>
          <Text>{product.hero}</Text>
        </View>

        <View className='secondhand-detail-info'>
          <View className='secondhand-detail-info__price-row'>
            <Text className='secondhand-detail-info__price'>
              <Text className='secondhand-detail-info__currency'>¥</Text>{product.price}
            </Text>
            <View className='secondhand-detail-info__status'>
              <Text>{product.status}</Text>
            </View>
          </View>

          <Text className='secondhand-detail-info__title'>{product.title}</Text>
          <Text className='secondhand-detail-info__desc'>{product.description}</Text>

          <View className='secondhand-detail-tags'>
            {product.tags.map((tag) => (
              <View className='secondhand-detail-tag' key={tag}>
                <Text>{tag}</Text>
              </View>
            ))}
          </View>

          <Text className='secondhand-detail-info__meta'>{product.meta}</Text>
        </View>

        <View className='secondhand-detail-section'>
          <View className='secondhand-detail-seller'>
            <View className='secondhand-detail-seller__avatar'>
              <Text>{seller.avatar}</Text>
            </View>
            <View className='secondhand-detail-seller__main'>
              <Text className='secondhand-detail-seller__name'>{seller.name}</Text>
              <Text className='secondhand-detail-seller__meta'>{seller.meta}</Text>
            </View>
            <View className='secondhand-detail-seller__button' onClick={() => void Taro.navigateTo({ url: `/pages/user-profile/index?userId=${encodeURIComponent(seller.id)}&source=secondhand&tab=secondhand` })}>
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
        <View className='secondhand-detail-contact' onClick={() => productFound ? setContactVisible(true) : showToast('该宝贝静态内容未登记')}>
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

            <View className='secondhand-contact-option' onClick={() => showToast(`微信号 ${seller.account} 已复制`)}>
              <View className='secondhand-contact-option__icon secondhand-contact-option__icon--wechat' />
              <View className='secondhand-contact-option__main'>
                <Text className='secondhand-contact-option__title'>加微信联系</Text>
                <Text className='secondhand-contact-option__desc'>微信号 {seller.account} · 点击复制</Text>
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
