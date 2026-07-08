import React, { useMemo, useState } from 'react'
import { ScrollView, Text, View } from '@tarojs/components'
import Taro from '../../services/taro'
import './index.scss'

type CategoryKey = '全部' | '数码' | '书籍资料' | '生活用品' | '代步'
type ProductTone = 'lemon' | 'lilac' | 'sky' | 'peach' | 'mint'
type ProductStatus = 'selling' | 'sold'

interface ProductCard {
  id: string
  title: string
  price: number
  category: CategoryKey
  imageLabel: string
  status: ProductStatus
  tone: ProductTone
  height: 'short' | 'medium' | 'tall'
}

const categories: CategoryKey[] = ['全部', '数码', '书籍资料', '生活用品', '代步']

const products: ProductCard[] = [
  {
    id: 'desk-lamp',
    title: '护眼台灯，用了半年，搬宿舍不想带了',
    price: 35,
    category: '生活用品',
    imageLabel: '图 · 台灯',
    status: 'selling',
    tone: 'lemon',
    height: 'short'
  },
  {
    id: 'earphones',
    title: '半入耳蓝牙耳机，盒装齐全，功能正常',
    price: 120,
    category: '数码',
    imageLabel: '图 · 耳机',
    status: 'selling',
    tone: 'lilac',
    height: 'medium'
  },
  {
    id: 'guitar',
    title: '初学者民谣吉他，送变调夹和背袋',
    price: 180,
    category: '生活用品',
    imageLabel: '图 · 吉他',
    status: 'sold',
    tone: 'peach',
    height: 'tall'
  },
  {
    id: 'fridge',
    title: '寝室小冰箱，制冷正常，可小刀',
    price: 99,
    category: '生活用品',
    imageLabel: '图 · 冰箱',
    status: 'selling',
    tone: 'sky',
    height: 'tall'
  },
  {
    id: 'books',
    title: '管理学教材 + 科学计算器打包',
    price: 30,
    category: '书籍资料',
    imageLabel: '图 · 教材',
    status: 'selling',
    tone: 'mint',
    height: 'medium'
  },
  {
    id: 'keyboard',
    title: '机械键盘 87 键茶轴，手感好',
    price: 110,
    category: '数码',
    imageLabel: '图 · 键盘',
    status: 'selling',
    tone: 'peach',
    height: 'medium'
  },
  {
    id: 'bike',
    title: '九成新自行车，毕业急出，车况好无锈',
    price: 150,
    category: '代步',
    imageLabel: '图 · 自行车',
    status: 'selling',
    tone: 'sky',
    height: 'tall'
  },
  {
    id: 'lamp-small',
    title: '床头小夜灯，暖光可调节',
    price: 18,
    category: '生活用品',
    imageLabel: '图 · 小灯',
    status: 'selling',
    tone: 'lemon',
    height: 'short'
  }
]

const navItems = [
  { key: 'home', label: '首页', icon: 'home', url: '/pages/home/index' },
  { key: 'takeout', label: '外卖', icon: 'takeaway', url: '/pages/student-takeout/index' },
  { key: 'market', label: '二手', icon: 'market', url: '/pages/student-secondhand/index' },
  { key: 'profile', label: '我的', icon: 'profile', url: '/pages/profile/index' }
]

function showToast(title: string) {
  void Taro.showToast({ title, icon: 'none', duration: 1400 })
}

function splitProducts(cards: ProductCard[]) {
  return {
    left: cards.filter((_, index) => index % 2 === 0),
    right: cards.filter((_, index) => index % 2 === 1)
  }
}

export default function StudentSecondhandPage() {
  const [activeCategory, setActiveCategory] = useState<CategoryKey>('全部')

  const visibleProducts = useMemo(
    () => activeCategory === '全部'
      ? products
      : products.filter((item) => item.category === activeCategory),
    [activeCategory]
  )

  const columns = useMemo(() => splitProducts(visibleProducts), [visibleProducts])

  const openDetail = () => {
    void Taro.navigateTo({ url: '/pages/secondhand-detail/index' })
  }

  const renderCard = (item: ProductCard) => (
    <View
      key={item.id}
      className={`secondhand-card ${item.status === 'sold' ? 'secondhand-card--sold' : ''}`}
      onClick={openDetail}
    >
      <View className={`secondhand-card__media secondhand-card__media--${item.tone} secondhand-card__media--${item.height}`}>
        <Text>{item.imageLabel}</Text>
        {item.status === 'sold' && (
          <View className='secondhand-card__sold-badge'>
            <Text>已售出</Text>
          </View>
        )}
      </View>
      <View className='secondhand-card__body'>
        <Text className='secondhand-card__title'>{item.title}</Text>
        <View className='secondhand-card__bottom'>
          <Text className='secondhand-card__price'>¥{item.price}</Text>
          <View className={`secondhand-card__status secondhand-card__status--${item.status}`}>
            <Text>{item.status === 'selling' ? '出售中' : '已售'}</Text>
          </View>
        </View>
      </View>
    </View>
  )

  return (
    <View className='student-secondhand-page'>
      <View className='student-secondhand-header'>
        <View className='student-secondhand-header__title-row'>
          <Text className='student-secondhand-header__title'>二手集市</Text>
          <View
            className='student-secondhand-search'
            onClick={() => showToast('搜索功能下一页接入')}
          >
            <View className='student-secondhand-search__icon' />
            <Text>搜二手好物</Text>
          </View>
        </View>

        <ScrollView className='student-secondhand-categories' scrollX enhanced showScrollbar={false}>
          <View className='student-secondhand-categories__inner'>
            {categories.map((item) => (
              <View
                key={item}
                className={`student-secondhand-category ${activeCategory === item ? 'student-secondhand-category--active' : ''}`}
                onClick={() => setActiveCategory(item)}
              >
                <Text>{item}</Text>
              </View>
            ))}
          </View>
        </ScrollView>
      </View>

      <ScrollView className='student-secondhand-scroll' scrollY enhanced showScrollbar={false}>
        <View className='student-secondhand-notice'>
          <Text className='student-secondhand-notice__icon'>i</Text>
          <Text>平台仅展示信息，交易请与卖家私下当面交付，验货后再付款。</Text>
        </View>

        <View className='student-secondhand-waterfall'>
          <View className='student-secondhand-waterfall__column'>
            {columns.left.map(renderCard)}
          </View>
          <View className='student-secondhand-waterfall__column'>
            {columns.right.map(renderCard)}
          </View>
        </View>
      </ScrollView>

      <View className='student-bottom-nav student-bottom-nav--secondhand'>
        {navItems.slice(0, 2).map((item) => (
          <View
            key={item.key}
            className='student-bottom-nav__item'
            onClick={() => void Taro.redirectTo({ url: item.url })}
          >
            <View className={`student-bottom-nav__icon student-bottom-nav__icon--${item.icon}`} />
            <Text>{item.label}</Text>
          </View>
        ))}
        <View className='student-bottom-nav__fab' onClick={() => void Taro.navigateTo({ url: '/pages/publish-post/index' })}>
          <Text>+</Text>
        </View>
        {navItems.slice(2).map((item) => (
          <View
            key={item.key}
            className={`student-bottom-nav__item ${item.key === 'market' ? 'student-bottom-nav__item--active' : ''}`}
            onClick={() => item.key === 'market' ? undefined : void Taro.redirectTo({ url: item.url })}
          >
            <View className={`student-bottom-nav__icon student-bottom-nav__icon--${item.icon}`} />
            <Text>{item.label}</Text>
          </View>
        ))}
      </View>
    </View>
  )
}
